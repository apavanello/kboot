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
	"time"

	"kboot/internal/app"
	"kboot/internal/config"
	"kboot/internal/kube"
	// Temporarily importing UI logic only if needed,
	// but strictly headless for now or based on flag.
)

func main() {
	headlessFunc := flag.Bool("headless", false, "Generate kubeconfigs and exit without launching k9s")
	flag.Parse()

	// 1. Load Config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Clusters) == 0 {
		fmt.Println("No clusters configured. Please add clusters to ~/.kboot.yaml")
		os.Exit(0)
	}

	fmt.Printf("Booting %d clusters (Parallel Workers=5)...\n", len(cfg.Clusters))

	// 2. Run Orchestrator
	orchestrator := app.NewOrchestrator(cfg)
	ctx := context.Background()
	results := orchestrator.Run(ctx)

	// 3. Process Results
	tempDir := filepath.Join(os.TempDir(), "kboot")
	// Clean old files
	os.RemoveAll(tempDir)

	validPaths := []string{}
	failedCount := 0

	fmt.Println("\n--- Summary ---")
	for alias, res := range results {
		if res.Error != nil {
			fmt.Printf("[x] %s: Failed (%v)\n", alias, res.Error)
			failedCount++
			continue
		}

		// Generate Kubeconfig for successful ones
		// Find the cluster config to get profile/region (orchestrator result just has info)
		// We could have passed config through result, but lookup is cheap enough for small N
		var clusterConfig config.Cluster
		for _, c := range cfg.Clusters {
			if c.Alias == alias {
				clusterConfig = c
				break
			}
		}

		path, err := kube.Generate(tempDir, alias, res.ClusterInfo, clusterConfig.Region, clusterConfig.Profile)
		if err != nil {
			fmt.Printf("[x] %s: Failed to write kubeconfig (%v)\n", alias, err)
			failedCount++
		} else {
			fmt.Printf("[v] %s: Ready\n", alias)
			validPaths = append(validPaths, path)
		}
	}

	if len(validPaths) == 0 {
		fmt.Println("\nFatal: All clusters failed to sync.")
		os.Exit(1)
	}

	if failedCount > 0 {
		fmt.Printf("\nWarning: %d clusters failed to load. Proceeding with %d available.\n", failedCount, len(validPaths))
		// We pause briefly so user sees the error in headless/CLI mode
		time.Sleep(2 * time.Second)
	}

	// 4. Handle Headless or Launch
	kubeconfigEnv := strings.Join(validPaths, string(filepath.ListSeparator))

	if *headlessFunc {
		fmt.Println("\nKUBECONFIG generated at:")
		fmt.Println(kubeconfigEnv)
		fmt.Println("\nYou can use it with:")
		fmt.Printf("$env:KUBECONFIG=\"%s\"\n", kubeconfigEnv) // PowerShell syntax hint
		os.Exit(0)
	}

	// 5. Launch K9s
	runTool("k9s", kubeconfigEnv)
}

func runTool(toolName string, kubeconfigEnv string) {
	// Simple run wrapper
	toolPath, err := exec.LookPath(toolName)
	if err != nil {
		fmt.Printf("Tool '%s' not found. KUBECONFIG generated but cannot launch app.\n", toolName)
		fmt.Println(kubeconfigEnv)
		os.Exit(0)
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
		syscall.Exec(toolPath, []string{toolName}, env)
	}
}
