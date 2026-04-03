package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"kboot/internal/app"
	"kboot/internal/aws"
	"kboot/internal/config"
	"kboot/internal/kube"
	"kboot/internal/ui"
)

// execCredential matches the Kubernetes client.authentication.k8s.io/v1beta1 API
type execCredential struct {
	Kind       string         `json:"kind"`
	APIVersion string         `json:"apiVersion"`
	Status     execCredStatus `json:"status"`
}

type execCredStatus struct {
	Token string `json:"token"`
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			if len(os.Args) > 2 && os.Args[2] == "add" {
				handleConfigAdd(os.Args[3:])
				return
			}
			if len(os.Args) > 2 && os.Args[2] == "list" {
				handleConfigList()
				return
			}
			ui.RunManager()
			return
		case "token":
			handleTokenCommand(os.Args[2:])
			return
		}
	}

	headlessFlag := flag.Bool("headless", false, "Generate kubeconfigs and exit without launching k9s")
	nonInteractiveFlag := flag.Bool("non-interactive", false, "Skip TUI prompts, process clusters directly")
	clusterFlag := flag.String("cluster", "", "Target a single cluster by alias (implies --non-interactive)")
	flag.Parse()

	nonInteractive := *nonInteractiveFlag || *clusterFlag != ""

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Clusters) == 0 {
		fmt.Println("No clusters configured. Add one with: kboot config add --alias prod --name my-cluster --region us-east-1 --profile aws-prod")
		os.Exit(0)
	}

	if *clusterFlag != "" {
		var found bool
		for _, c := range cfg.Clusters {
			if c.Alias == *clusterFlag {
				cfg.Clusters = []config.Cluster{c}
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Error: cluster %q not found in config. Available clusters:\n", *clusterFlag)
			for _, c := range cfg.Clusters {
				fmt.Fprintf(os.Stderr, "  - %s (%s)\n", c.Alias, c.Name)
			}
			os.Exit(1)
		}
	} else if !nonInteractive && !*headlessFlag {
		finalClusters, err := ui.PromptOptionalClusters(cfg.Clusters)
		if err != nil {
			fmt.Println("Selection cancelled.")
			os.Exit(0)
		}
		cfg.Clusters = finalClusters
	}

	if len(cfg.Clusters) == 0 {
		fmt.Println("No clusters selected for boot.")
		os.Exit(0)
	}

	eventChan := make(chan app.Event, len(cfg.Clusters)*2)

	orchestrator := app.NewOrchestrator(cfg)
	ctx := context.Background()

	var results map[string]app.Result

	if *headlessFlag || nonInteractive {
		close(eventChan)
		results = orchestrator.Run(ctx, nil)
	} else {
		p := tea.NewProgram(ui.NewLoadingModel(cfg, eventChan))

		go func() {
			results = orchestrator.Run(ctx, eventChan)
			p.Send(true)
		}()

		if _, err := p.Run(); err != nil {
			fmt.Printf("UI Error: %v\n", err)
			os.Exit(1)
		}
	}

	tempDir := filepath.Join(os.TempDir(), "kboot")
	os.RemoveAll(tempDir)

	validPaths := []string{}
	failedClusters := []string{}

	for _, c := range cfg.Clusters {
		res, ok := results[c.Alias]
		if !ok || res.Error != nil {
			if res.Error != nil {
				failedClusters = append(failedClusters, fmt.Sprintf("  ✗ %s: %v", c.Alias, res.Error))
			} else {
				failedClusters = append(failedClusters, fmt.Sprintf("  ✗ %s: no result", c.Alias))
			}
			continue
		}

		path, err := kube.Generate(tempDir, c.Alias, res.ClusterInfo, c.Region, c.Profile)
		if err != nil {
			failedClusters = append(failedClusters, fmt.Sprintf("  ✗ %s: %v", c.Alias, err))
		} else {
			validPaths = append(validPaths, path)
		}
	}

	if len(failedClusters) > 0 {
		fmt.Fprintf(os.Stderr, "\nFailed clusters:\n%s\n", strings.Join(failedClusters, "\n"))
	}

	if len(validPaths) == 0 {
		fmt.Fprintf(os.Stderr, "\nFatal: All clusters failed to sync.\n")
		os.Exit(1)
	}

	kubeconfigEnv := strings.Join(validPaths, string(filepath.ListSeparator))

	if *headlessFlag {
		fmt.Println(kubeconfigEnv)
		os.Exit(0)
	}

	if nonInteractive {
		fmt.Printf("\nKubeconfig generated: %s\n", validPaths[0])
		fmt.Printf("To use with kubectl:\n")
		fmt.Printf("  export KUBECONFIG=%s\n", validPaths[0])
		fmt.Printf("  kubectl get nodes\n")
		fmt.Printf("  kubectl get pods -A\n")
		os.Exit(0)
	}

	runTool("k9s", kubeconfigEnv)
}

func handleConfigAdd(args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	alias := fs.String("alias", "", "Friendly cluster alias (required)")
	name := fs.String("name", "", "EKS cluster name (required)")
	region := fs.String("region", "", "AWS region (required)")
	profile := fs.String("profile", "", "AWS profile name (required)")
	optional := fs.Bool("optional", false, "Mark cluster as optional")
	fs.Parse(args)

	if *alias == "" || *name == "" || *region == "" || *profile == "" {
		fmt.Fprintf(os.Stderr, "Error: --alias, --name, --region, and --profile are required\n")
		fmt.Fprintf(os.Stderr, "Usage: kboot config add --alias prod --name my-cluster --region us-east-1 --profile aws-prod\n")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	for _, c := range cfg.Clusters {
		if c.Alias == *alias {
			fmt.Fprintf(os.Stderr, "Error: cluster with alias %q already exists. Use 'kboot config' to edit.\n", *alias)
			os.Exit(1)
		}
	}

	cfg.Clusters = append(cfg.Clusters, config.Cluster{
		Alias:    *alias,
		Name:     *name,
		Region:   *region,
		Profile:  *profile,
		Optional: *optional,
	})

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Cluster %q added successfully.\n", *alias)
}

func handleConfigList() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Clusters) == 0 {
		fmt.Println("No clusters configured.")
		return
	}

	fmt.Printf("%-15s %-30s %-12s %-15s %s\n", "ALIAS", "NAME", "REGION", "PROFILE", "OPTIONAL")
	fmt.Println(strings.Repeat("-", 85))
	for _, c := range cfg.Clusters {
		fmt.Printf("%-15s %-30s %-12s %-15s %v\n", c.Alias, c.Name, c.Region, c.Profile, c.Optional)
	}
}

func handleTokenCommand(args []string) {
	fs := flag.NewFlagSet("token", flag.ExitOnError)
	clusterName := fs.String("cluster-name", "", "EKS cluster name")
	region := fs.String("region", "", "AWS region")
	profile := fs.String("profile", "", "AWS profile")
	fs.Parse(args)

	if *clusterName == "" || *region == "" || *profile == "" {
		fmt.Fprintf(os.Stderr, "Error: --cluster-name, --region, and --profile are required\n")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	opts := aws.ClientOptions{
		CredentialsFile: cfg.AWSCredentialsPath(),
		ConfigFile:      cfg.AWSConfigPath(),
		SSOCacheDir:     cfg.AWSSSOCachePath(),
	}

	ctx := context.Background()
	token, err := aws.GenerateEKSTokenWithOptions(ctx, *profile, *region, *clusterName, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating EKS token: %v\n", err)
		os.Exit(1)
	}

	output := execCredential{
		Kind:       "ExecCredential",
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Status: execCredStatus{
			Token: token,
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.Encode(output)
}

func runTool(toolName string, kubeconfigEnv string) {
	toolPath, err := exec.LookPath(toolName)
	if err != nil {
		fmt.Printf("Tool '%s' not found.\n", toolName)
		return
	}

	env := os.Environ()
	env = append(env, fmt.Sprintf("KUBECONFIG=%s", kubeconfigEnv))

	if runtime.GOOS == "windows" {
		cmd := exec.Command(toolPath)
		cmd.Env = env
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running %s: %v\n", toolName, err)
		}
	} else {
		if err := syscall.Exec(toolPath, []string{toolName}, env); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to exec %s: %v\n", toolName, err)
			os.Exit(1)
		}
	}
}
