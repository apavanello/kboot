package main

import (
	"context"
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
	"kboot/internal/config"
	"kboot/internal/kube"
	"kboot/internal/ui"
)

func main() {
	// 0. Check CLI commands (config)
	if len(os.Args) > 1 && os.Args[1] == "config" {
		ui.RunManager()
		return
	}

	headlessFunc := flag.Bool("headless", false, "Generate kubeconfigs and exit without launching k9s")
	flag.Parse()

	// 1. Load Config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Clusters) == 0 {
		fmt.Println("No clusters configured. Please add clusters to ~/.kboot.yaml or run 'kboot config'")
		os.Exit(0)
	}

	// 1.5. Check for interactively enabled clusters (Optional flow)
	// Only run interactive prompt if not headless (headless defaults to all? or none?
	// Usually headless implies automation, so maybe we skip options or boot all.
	// Let's assume headless boots ALL unless we add a flag. For now, prompt only in UI mode)
	if !*headlessFunc {
		finalClusters, err := ui.PromptOptionalClusters(cfg.Clusters)
		if err != nil {
			fmt.Println("Selection cancelled.")
			os.Exit(0)
		}
		cfg.Clusters = finalClusters
	}
	// Note: In headless mode, we currently process ALL clusters (including optional ones) as "mandatory".
	// Features like --include-optional could be added later.

	if len(cfg.Clusters) == 0 {
		fmt.Println("No clusters selected for boot.")
		os.Exit(0)
	}

	// 2. Prepare Communication
	// Channel for worker -> UI
	eventChan := make(chan app.Event, len(cfg.Clusters)*2)

	// Orchestrator
	orchestrator := app.NewOrchestrator(cfg)
	ctx := context.Background()

	var results map[string]app.Result

	// 3. UI or Headless?
	if *headlessFunc {
		// Headless: Just wait for results, no UI
		close(eventChan)                     // Not using UI channel for processing
		results = orchestrator.Run(ctx, nil) // Handle nil chan in orchestrator
	} else {
		// UI Mode
		p := tea.NewProgram(ui.NewLoadingModel(cfg, eventChan))

		// Run orchestrator in BG
		go func() {
			results = orchestrator.Run(ctx, eventChan)
			p.Send(true) // Signal done to UI
		}()

		if _, err := p.Run(); err != nil {
			fmt.Printf("UI Error: %v\n", err)
			os.Exit(1)
		}
	}

	// 4. Process Results (Same as before)
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

	// 5. Handle Headless or Launch
	kubeconfigEnv := strings.Join(validPaths, string(filepath.ListSeparator))

	if *headlessFunc {
		fmt.Println(kubeconfigEnv)
		os.Exit(0)
	}

	// Launch K9s
	runTool("k9s", kubeconfigEnv)
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
		cmd.Run()
	} else {
		// Replace process
		syscall.Exec(toolPath, []string{toolName}, env)
	}
}
