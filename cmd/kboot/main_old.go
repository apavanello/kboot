package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v3"
)

var (
	execCommand = exec.Command
	getHomeDir  = os.UserHomeDir
	getInput    = func() io.Reader { return os.Stdin }
)

// Config represents the ~/.kboot.yaml structure
type Config struct {
	SSOSession string    `yaml:"sso_session,omitempty"`
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
	// 1. Check for commands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			runManager()
			return
		case "help", "--help", "-h":
			printHelp()
			return
		}
	}
	// Default behavior: boot logic
	runKboot()
}

func printHelp() {
	fmt.Println("kboot - DevOps CLI for EKS & AWS Auth Management (v2.3.0)")
	fmt.Println("\nUsage: kboot [command]")
	fmt.Println("\nCommands:")
	fmt.Println("  config     Dashboard TUI para gerenciar clusters e credenciais AWS")
	fmt.Println("\n  (vazio)    Sincroniza clusters do ~/.kboot.yaml e inicia o k9s")
	fmt.Println("\nFlags:")
	fmt.Println("  -h, --help Mostra esta mensagem de ajuda")
}

func handleAuthCommand(args []string) {
	if len(args) == 0 {
		printAuthHelp()
		os.Exit(1)
	}
	switch args[0] {
	case "new":
		authNew()
	case "add":
		authAdd()
	case "help", "--help", "-h":
		printAuthHelp()
	default:
		fmt.Printf("Unknown auth command: %s\n", args[0])
		printAuthHelp()
		os.Exit(1)
	}
}

func printAuthHelp() {
	fmt.Println("Usage: kboot auth <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  new   Interactive setup for AWS credentials/config (with backup)")
	fmt.Println("  add   Interactive addition of AWS credentials/profiles")
}

func handleClusterCommand(args []string) {
	if len(args) == 0 {
		printClusterHelp()
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		clusterAdd()
	case "help", "--help", "-h":
		printClusterHelp()
	default:
		fmt.Printf("Unknown cluster command: %s\n", args[0])
		printClusterHelp()
		os.Exit(1)
	}
}

func printClusterHelp() {
	fmt.Println("Usage: kboot cluster <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  add   Interactive addition of a cluster to ~/.kboot.yaml")
}

// ClusterConfig holds the form data
type ClusterConfig struct {
	Alias       string
	ClusterName string
	Region      string
	Profile     string
}

// newClusterForm creates a huh.Form for cluster addition and returns pointers to data
func newClusterForm(data *ClusterConfig) *huh.Form {
	// 1. Load AWS Profiles for Select List
	profiles, _ := listAWSProfiles()

	// Create options
	var profileOptions []huh.Option[string]
	if len(profiles) > 0 {
		for _, p := range profiles {
			profileOptions = append(profileOptions, huh.NewOption(p, p))
		}
	} else {
		profileOptions = append(profileOptions, huh.NewOption("default (No profiles found)", "default"))
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Cluster Alias").
				Description("Short name for display in k9s/kboot").
				Value(&data.Alias).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("alias cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("Real Cluster Name").
				Description("Exact name in AWS EKS").
				Value(&data.ClusterName).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("cluster name cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("AWS Region").
				Value(&data.Region).
				Suggestions([]string{"us-east-1", "us-west-2", "eu-west-1", "sa-east-1"}),

			huh.NewSelect[string]().
				Title("AWS Profile").
				Options(profileOptions...).
				Value(&data.Profile),

			huh.NewNote().
				Title("Navegação").
				Description("Tab → próximo | Shift+Tab → anterior | Enter → confirmar | Esc → cancelar"),
		),
	)
}

func clusterAdd() {
	fmt.Println("Discovering AWS Profiles...") // Feedback before TUI clears screen

	var data ClusterConfig
	form := newClusterForm(&data)

	if err := form.Run(); err != nil {
		fatal("Cancelled")
	}

	saveClusterConfig(data)
}

func saveClusterConfig(data ClusterConfig) {
	alias := strings.TrimSpace(data.Alias)
	clusterName := strings.TrimSpace(data.ClusterName)
	region := strings.TrimSpace(data.Region)
	if region == "" {
		region = "us-east-1"
	}
	profile := strings.TrimSpace(data.Profile)

	// Save
	configPath, err := getConfigPath()
	if err != nil {
		fatal("Error getting config path: %v", err)
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		fatal("Error loading config: %v", err)
	}

	newCluster := Cluster{
		Alias:   alias,
		Name:    clusterName,
		Region:  region,
		Profile: profile,
	}

	cfg.Clusters = append(cfg.Clusters, newCluster)

	if err := saveConfigToFile(configPath, cfg); err != nil {
		fatal("Error saving config: %v", err)
	}
	fmt.Printf("✓ Added cluster '%s' to %s\n", alias, configPath)
}

// Helper to avoid duplication in manage.go
func saveConfigToFile(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(2)
	return encoder.Encode(cfg)
}

func listAWSProfiles() ([]string, error) {
	home, err := getHomeDir()
	if err != nil {
		return nil, err
	}

	profiles := make(map[string]bool)

	// Helper to extract profile names from standard ini-like files
	// Regex or simple parsing. Going simple parsing.
	parseFile := func(path string, isConfig bool) {
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				content := line[1 : len(line)-1]
				// Skip sso-session entries (they are not profiles)
				if strings.HasPrefix(content, "sso-session ") {
					continue
				}
				if isConfig {
					// .aws/config uses [profile name] or [default]
					content = strings.TrimPrefix(content, "profile ")
					content = strings.TrimSpace(content)
				}
				profiles[content] = true
			}
		}
	}

	parseFile(filepath.Join(home, ".aws", "credentials"), false)
	parseFile(filepath.Join(home, ".aws", "config"), true)

	var list []string
	for p := range profiles {
		list = append(list, p)
	}
	return list, nil
}

// --- Auth Features ---

// --- Auth Features ---

func authNew() {
	var choice string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Do you want to setup:").
				Options(
					huh.NewOption("Static Credentials (~/.aws/credentials)", "1"),
					huh.NewOption("SSO Profiles (~/.aws/config)", "2"),
				).
				Value(&choice),
		),
	)

	err := form.Run()
	if err != nil {
		fatal("Auth cancelled: %v", err)
	}

	home, err := getHomeDir()
	if err != nil {
		fatal("Could not find home directory: %v", err)
	}
	awsDir := filepath.Join(home, ".aws")
	if err := os.MkdirAll(awsDir, 0700); err != nil {
		fatal("Could not create .aws directory: %v", err)
	}

	var targetFile string
	switch choice {
	case "1":
		targetFile = filepath.Join(awsDir, "credentials")
	case "2":
		targetFile = filepath.Join(awsDir, "config")
	default:
		fatal("Invalid choice")
	}

	// Backup
	if _, err := os.Stat(targetFile); err == nil {
		backupFile := fmt.Sprintf("%s.bak.%d", targetFile, time.Now().Unix())
		fmt.Printf("Backing up %s to %s...\n", targetFile, backupFile)
		if err := copyFile(targetFile, backupFile); err != nil {
			fatal("Backup failed: %v", err)
		}
	}

	// Create new empty file
	f, err := os.Create(targetFile)
	if err != nil {
		fatal("Failed to create new file: %v", err)
	}
	f.Close()
	fmt.Printf("Created new empty %s\n", targetFile)
}

func authAdd() {
	var choice string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which type of credential to add?").
				Options(
					huh.NewOption("Static Credentials (key/secret)", "1"),
					huh.NewOption("SSO Profile", "2"),
				).
				Value(&choice),
		),
	)

	if err := form.Run(); err != nil {
		fatal("Cancelled: %v", err)
	}

	if choice == "1" {
		addStaticCredential()
	} else if choice == "2" {
		addSSOProfile()
	} else {
		fatal("Invalid choice")
	}
}

func addStaticCredential() {
	var (
		profile string
		key     string
		secret  string
		token   string
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Profile Name").
				Value(&profile).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("profile name cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("AWS Access Key ID").
				Value(&key).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("access key cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("AWS Secret Access Key").
				Password(true).
				Value(&secret).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("secret key cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("AWS Session Token (optional)").
				Value(&token),
		),
	)

	if err := form.Run(); err != nil {
		fatal("Cancelled")
	}

	profile = strings.TrimSpace(profile)
	key = strings.TrimSpace(key)
	secret = strings.TrimSpace(secret)
	token = strings.TrimSpace(token)

	content := fmt.Sprintf("\n[%s]\naws_access_key_id = %s\naws_secret_access_key = %s\n", profile, key, secret)
	if token != "" {
		content += fmt.Sprintf("aws_session_token = %s\n", token)
	}

	home, _ := getHomeDir()
	path := filepath.Join(home, ".aws", "credentials")
	appendToFile(path, content)
	fmt.Printf("Added profile [%s] to %s\n", profile, path)
}

func addSSOProfile() {
	var (
		profile     string
		sessionName string = "my-sso"
		url         string
		region      string
		accId       string
		roleName    string
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Profile Name").
				Value(&profile).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("profile name cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("SSO Session Name").
				Description("Default: my-sso").
				Value(&sessionName),

			huh.NewInput().
				Title("SSO Start URL").
				Value(&url).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("URL cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("SSO Region").
				Value(&region).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("region cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("SSO Account ID").
				Value(&accId).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("Account ID cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("SSO Role Name").
				Value(&roleName).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("Role Name cannot be empty")
					}
					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		fatal("Cancelled")
	}

	profile = strings.TrimSpace(profile)
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		sessionName = "my-sso"
	}
	url = strings.TrimSpace(url)
	region = strings.TrimSpace(region)
	accId = strings.TrimSpace(accId)
	roleName = strings.TrimSpace(roleName)

	home, _ := getHomeDir()
	configPath := filepath.Join(home, ".aws", "config")

	// 1. Ensure [sso-session <sessionName>] exists
	content, err := os.ReadFile(configPath)
	if err == nil {
		strContent := string(content)
		if !strings.Contains(strContent, fmt.Sprintf("[sso-session %s]", sessionName)) {
			fmt.Printf("Creating new [sso-session %s] block...\n", sessionName)
			sessionBlock := fmt.Sprintf("\n[sso-session %s]\nsso_start_url = %s\nsso_region = %s\nsso_registration_scopes = sso:account:access\n", sessionName, url, region)
			appendToFile(configPath, sessionBlock)
		} else {
			fmt.Printf("Using existing [sso-session %s] block.\n", sessionName)
		}
	} else {
		// New file
		fmt.Printf("Creating new config file with [sso-session %s]...\n", sessionName)
		sessionBlock := fmt.Sprintf("[sso-session %s]\nsso_start_url = %s\nsso_region = %s\nsso_registration_scopes = sso:account:access\n", sessionName, url, region)
		appendToFile(configPath, sessionBlock)
	}

	// 2. Add Profile linking to session
	profileBlock := fmt.Sprintf("\n[profile %s]\nsso_session = %s\nsso_account_id = %s\nsso_role_name = %s\n", profile, sessionName, accId, roleName)

	appendToFile(configPath, profileBlock)
	fmt.Printf("Added profile [%s] to %s\n", profile, configPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func appendToFile(path, content string) {
	// Ensure dir exists
	os.MkdirAll(filepath.Dir(path), 0700)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fatal("Failed to open %s: %v", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		fatal("Failed to write to %s: %v", path, err)
	}
}

// --- Main Kboot Logic ---

func runKboot() {
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

	// 2. AWS SSO Authentication (Multi-Session Support v1.7.0)
	sessionsToValidate, err := discoverSessions(cfg.Clusters)
	if err != nil {
		fmt.Printf("! Warning: Failed to discover sessions from config: %v\n", err)
	}

	// If we found explicitly configured sessions, use them
	if len(sessionsToValidate) > 0 {
		fmt.Printf("found %d unique SSO sessions to validate\n", len(sessionsToValidate))
		for sessionName, testProfile := range sessionsToValidate {
			if err := ensureSSOLogin(sessionName, testProfile); err != nil {
				fatal("Failed to login to session %s: %v", sessionName, err)
			}
		}
	} else if cfg.SSOSession != "" {
		// Fallback to global config if no session links found (Legacy)
		fmt.Println("No specific sso_session found in profiles, checking legacy global config...")
		testProfile := ""
		if len(cfg.Clusters) > 0 {
			testProfile = cfg.Clusters[0].Profile
		}
		if err := ensureSSOLogin(cfg.SSOSession, testProfile); err != nil {
			fatal("SSO login failed: %v", err)
		}
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
	home, err := getHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kboot.yaml"), nil
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Auto-create default
			defaultConfig := `clusters: []
# Use 'kboot cluster add' to configure your first cluster
`
			if createErr := os.WriteFile(path, []byte(defaultConfig), 0644); createErr != nil {
				return nil, fmt.Errorf("config not found and failed to create default at %s: %v", path, createErr)
			}
			fmt.Printf("! Config not found. Created default at %s. Please edit it or run 'kboot cluster add'.\n", path)
			// Read again
			data = []byte(defaultConfig)
		} else {
			return nil, err
		}
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func discoverSessions(clusters []Cluster) (map[string]string, error) {
	// Map sessionName -> sampleProfile (to use for verification)
	sessions := make(map[string]string)

	// We need to parse ~/.aws/config to map Profile -> sso_session
	home, err := getHomeDir()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(home, ".aws", "config")

	f, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No config, no sessions
		}
		return nil, err
	}
	defer f.Close()

	// Parse INI manually to find sso_session for each profile
	// We want to avoid heavy external deps if possible, but map iteration is safe.
	// Structure:
	// [profile name]
	// sso_session = name

	type ProfileData struct {
		SSOSession string
	}
	profileMap := make(map[string]*ProfileData)

	var currentProfile string
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			content := line[1 : len(line)-1]
			// Check if it is a profile block
			if strings.HasPrefix(content, "profile ") {
				currentProfile = strings.TrimSpace(strings.TrimPrefix(content, "profile "))
				profileMap[currentProfile] = &ProfileData{}
			} else if content == "default" {
				currentProfile = "default"
				profileMap[currentProfile] = &ProfileData{}
			} else {
				currentProfile = "" // Reset if entering unrelated block (e.g. sso-session block)
			}
			continue
		}

		if currentProfile != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				if key == "sso_session" {
					if pData, ok := profileMap[currentProfile]; ok {
						pData.SSOSession = val
					}
				}
			}
		}
	}

	// Now match clusters to found sessions
	for _, c := range clusters {
		if pData, ok := profileMap[c.Profile]; ok && pData.SSOSession != "" {
			// Save this session to be validated, using this profile as the tester
			sessions[pData.SSOSession] = c.Profile
		}
	}

	return sessions, nil
}

func ensureSSOLogin(sessionName, testProfile string) error {
	// If no session name is configured, assume standard credentials or environment variables
	if sessionName == "" {
		fmt.Println("No sso_session defined in config. Skipping SSO check.")
		return nil
	}

	// If we have a profile to test with, try check if we are authenticated
	if testProfile != "" {
		// Use a quick dry-run call to see if we have valid credentials
		// aws sts get-caller-identity --profile <profile>
		// This validates both the SSO token AND the assumption of the profile role
		cmd := execCommand("aws", "sts", "get-caller-identity", "--profile", testProfile)
		if err := cmd.Run(); err == nil {
			fmt.Println("✓ AWS Session is valid")
			return nil
		}
	} else {
		// If no clusters/profiles configured, we can't easily validated using STS without knowing the proper profile or account.
		// However, kboot without clusters usually just exits or does nothing interesting.
		// We could fallback to simple file check, but let's just warn.
		fmt.Println("! No clusters configured to validate session. Assuming login needed if clusters are added.")
	}

	fmt.Printf("! AWS Session expired or invalid. Logging in to session '%s'...\n", sessionName)
	cmd := execCommand("aws", "sso", "login", "--sso-session", sessionName)
	cmd.Stdin = getInput() // Use mockable input
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func generateKubeconfig(dir string, c Cluster) (string, error) {
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

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("aws describe-cluster failed: %w | stderr: %s", err, stderr.String())
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
