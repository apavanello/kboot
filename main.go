package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

var execCommand = exec.Command

// Config represents the ~/.kboot.yaml structure
type Config struct {
	SSOSession string    `yaml:"sso_session"`
	Clusters   []Cluster `yaml:"clusters"`
}

type Cluster struct {
	Alias   string `yaml:"alias"`
	Profile string `yaml:"profile"`
	Region  string `yaml:"region"`
	Name    string `yaml:"name"`
}

// Minimal structure for reading AWS SSO cache to check expiration
type SSOCache struct {
	ExpiresAt string `json:"expiresAt"`
}

// KubeConfig structure for generating kubeconfig files native in Go
type KubeConfig struct {
	APIVersion     string          `yaml:"apiVersion"`
	Kind           string          `yaml:"kind"`
	Clusters       []KClusterEntry `yaml:"clusters"`
	Contexts       []KContextEntry `yaml:"contexts"`
	CurrentContext string          `yaml:"current-context"`
	Users          []KUserEntry    `yaml:"users"`
	Preferences    struct{}        `yaml:"preferences"`
}

type KClusterEntry struct {
	Name    string   `yaml:"name"`
	Cluster KCluster `yaml:"cluster"`
}

type KCluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
}

type KContextEntry struct {
	Name    string   `yaml:"name"`
	Context KContext `yaml:"context"`
}

type KContext struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type KUserEntry struct {
	Name string `yaml:"name"`
	User KUser  `yaml:"user"`
}

type KUser struct {
	Exec KExec `yaml:"exec"`
}

type KExec struct {
	APIVersion string   `yaml:"apiVersion"`
	Command    string   `yaml:"command"`
	Args       []string `yaml:"args"`
	Env        []KEnv   `yaml:"env"`
}

type KEnv struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func main() {
	// 1. configuration
	configPath, err := getConfigPath()
	if err != nil {
		fatal("Could not determine config path: %v", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		fatal("Error loading config from %s: %v", configPath, err)
	}

	fmt.Printf("Loaded configuration with %d clusters\n", len(cfg.Clusters))

	// 2. AWS SSO Authentication
	if err := ensureSSOLogin(cfg.SSOSession); err != nil {
		fatal("SSO login failed: %v", err)
	}

	// 3. Generate Kubeconfigs in Parallel
	tempDir := filepath.Join(os.TempDir(), "kboot")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		fatal("Failed to create temp directory: %v", err)
	}

	// Clean old files
	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, 0755)

	var wg sync.WaitGroup
	kubeconfigPaths := make([]string, len(cfg.Clusters))
	errs := make([]error, len(cfg.Clusters))

	for i, cluster := range cfg.Clusters {
		wg.Add(1)
		go func(idx int, c Cluster) {
			defer wg.Done()
			path, err := generateKubeconfig(tempDir, c)
			if err != nil {
				errs[idx] = err
				fmt.Printf("x Failed to sync %s: %v\n", c.Alias, err)
			} else {
				kubeconfigPaths[idx] = path
				fmt.Printf("✓ %s synced\n", c.Alias)
			}
		}(i, cluster)
	}
	wg.Wait()

	// Filter successful paths
	var validPaths []string
	for _, p := range kubeconfigPaths {
		if p != "" {
			validPaths = append(validPaths, p)
		}
	}

	if len(validPaths) == 0 {
		fatal("No clusters configured successfully")
	}

	// 4. Merge Configurations (Env Var)
	// filepath.ListSeparator is ':' on Linux/Mac and ';' on Windows
	kubeconfigEnv := strings.Join(validPaths, string(filepath.ListSeparator))

	// 5. Execution
	runTool("k9s", kubeconfigEnv)
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kboot.yaml"), nil
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func ensureSSOLogin(sessionName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Check SSO cache
	ssoCacheDir := filepath.Join(home, ".aws", "sso", "cache")
	valid := false

	err = filepath.Walk(ssoCacheDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil // ignore errors, just skip
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			// Try to parse as generic cache file with output
			var cache SSOCache
			if err := json.Unmarshal(data, &cache); err == nil && cache.ExpiresAt != "" {
				// Parse time
				// AWS SSO usually uses ISO8601/RFC3339
				t, err := time.Parse(time.RFC3339, cache.ExpiresAt)
				if err == nil {
					if time.Now().Before(t) {
						// Found a valid token?
						// To be more precise we should check if it belongs to the session,
						// but simpler logic: if ANY recent valid sso-token file exists, we assume logged in.
						// Real verification might need to match the session specifically, but this is a heuristic.
						// A stricter check would be to look for the specific session region/start URL match if known.
						valid = true
						return filepath.SkipAll // Stop searching
					}
				}
			}
		}
		return nil
	})

	if valid {
		fmt.Println("✓ AWS SSO session is valid")
		return nil
	}

	fmt.Printf("! AWS SSO session expired or missing. Logging in to session '%s'...\n", sessionName)
	cmd := execCommand("aws", "sso", "login", "--sso-session", sessionName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func generateKubeconfig(dir string, c Cluster) (string, error) {
	// We rely on 'aws eks update-kubeconfig' logic but we want to build it manually to avoid calling the CLI 30 times.
	// However, getting the CA data and Endpoint requires `aws eks describe-cluster`.
	// To keep it strictly purely local without that call would require assuming we know the endpoint/CA.
	// Since we don't have them in .kboot.yaml, we HAVE to query AWS.
	// Optimally, we run `aws eks describe-cluster` for each.

	// Command: aws eks describe-cluster --name <name> --region <region> --profile <profile>
	type EKSDescribe struct {
		Cluster struct {
			Endpoint             string `json:"endpoint"`
			CertificateAuthority struct {
				Data string `json:"data"`
			} `json:"certificateAuthority"`
			Arn string `json:"arn"`
		} `json:"cluster"`
	}

	cmd := execCommand("aws", "eks", "describe-cluster",
		"--name", c.Name,
		"--region", c.Region,
		"--profile", c.Profile, // Important: use the specific profile
		"--output", "json")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("aws describe-cluster failed: %w", err)
	}

	var d EKSDescribe
	if err := json.Unmarshal(out, &d); err != nil {
		return "", fmt.Errorf("parse aws output error: %w", err)
	}

	// Construct Kubeconfig
	kc := KubeConfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: c.Alias,
		Preferences:    struct{}{},
		Clusters: []KClusterEntry{
			{
				Name: d.Cluster.Arn, // Standard AWS practice uses ARN as cluster name key
				Cluster: KCluster{
					Server:                   d.Cluster.Endpoint,
					CertificateAuthorityData: d.Cluster.CertificateAuthority.Data,
				},
			},
		},
		Contexts: []KContextEntry{
			{
				Name: c.Alias,
				Context: KContext{
					Cluster: d.Cluster.Arn,
					User:    c.Alias, // Use alias as username to keep unique
				},
			},
		},
		Users: []KUserEntry{
			{
				Name: c.Alias,
				User: KUser{
					Exec: KExec{
						APIVersion: "client.authentication.k8s.io/v1beta1",
						Command:    "aws",
						Args: []string{
							"eks", "get-token",
							"--cluster-name", c.Name,
							"--region", c.Region,
							"--profile", c.Profile, // Embed profile in the exec arg so auth works context-aware
						},
					},
				},
			},
		},
	}

	// Write to file
	filename := filepath.Join(dir, fmt.Sprintf("%s.yaml", c.Alias))
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := yaml.NewEncoder(f).Encode(kc); err != nil {
		return "", err
	}

	return filename, nil
}

func runTool(toolName string, kubeconfigEnv string) {
	fmt.Printf("Launching %s...\n", toolName)

	// Check if tool exists
	toolPath, err := exec.LookPath(toolName)
	if err != nil {
		// Fallback to shell if tool not found? Or just fatal.
		// User requirement says "launch k9s (or a shell)"
		fmt.Printf("Tool '%s' not found, falling back to shell.\n", toolName)
		if runtime.GOOS == "windows" {
			toolPath, _ = exec.LookPath("powershell")
		} else {
			toolPath = os.Getenv("SHELL")
			if toolPath == "" {
				toolPath = "/bin/sh"
			}
		}
	}

	env := os.Environ()
	// Add/Override KUBECONFIG
	env = append(env, fmt.Sprintf("KUBECONFIG=%s", kubeconfigEnv))

	// Windows does not support syscall.Exec (Exec is a wrapper for CreateProcess but doesn't replace PID in the same way).
	// However, for a CLI wrapper, standard practice is just running it and mapping stdio.
	if runtime.GOOS == "windows" {
		cmd := execCommand(toolPath)
		cmd.Env = env
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fatal("Execution failed: %v", err)
		}
		os.Exit(0)
	} else {
		// Linux/Mac specific syscall.Exec
		// Requires full path, args (including binary name as 0th arg), and env
		args := []string{toolPath}
		if err := syscall.Exec(toolPath, args, env); err != nil {
			fatal("syscall.Exec failed: %v", err)
		}
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
