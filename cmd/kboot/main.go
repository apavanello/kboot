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
	Kind       string          `json:"kind"`
	APIVersion string          `json:"apiVersion"`
	Status     execCredStatus  `json:"status"`
}

type execCredStatus struct {
	Token string `json:"token"`
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			ui.RunManager()
			return
		case "token":
			handleTokenCommand(os.Args[2:])
			return
		}
	}

	headlessFunc := flag.Bool("headless", false, "Generate kubeconfigs and exit without launching k9s")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Clusters) == 0 {
		fmt.Println("No clusters configured. Please add clusters to ~/.kboot.yaml or run 'kboot config'")
		os.Exit(0)
	}

	if !*headlessFunc {
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

	if *headlessFunc {
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
	failedCount := 0

	for _, c := range cfg.Clusters {
		res, ok := results[c.Alias]
		if !ok || res.Error != nil {
			failedCount++
			continue
		}

		path, err := kube.Generate(tempDir, c.Alias, res.ClusterInfo, c.Region, c.Profile)
		if err != nil {
			fmt.Printf("Failed to write kubeconfig for %s: %v\n", c.Alias, err)
			failedCount++
		} else {
			validPaths = append(validPaths, path)
		}
	}

	if len(validPaths) == 0 {
		fmt.Println("\nFatal: All clusters failed to sync.")
		os.Exit(1)
	}

	kubeconfigEnv := strings.Join(validPaths, string(filepath.ListSeparator))

	if *headlessFunc {
		fmt.Println(kubeconfigEnv)
		os.Exit(0)
	}

	runTool("k9s", kubeconfigEnv)
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

	ctx := context.Background()
	token, err := aws.GenerateEKSToken(ctx, *profile, *region, *clusterName)
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
