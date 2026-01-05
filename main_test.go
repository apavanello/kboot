package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelperProcess is used to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	// Check arguments to determine which command is being mocked
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd := args[0]
	subCmd := args[1]

	// specialized mocking
	if cmd == "aws" && subCmd == "eks" {
		// describe-cluster
		// We expect output appropriate for the cluster in context
		// We can switch on specific args if needed, or just return generic success for now
		// The tool parses: { "cluster": { "endpoint": "...", "certificateAuthority": { "data": "..." }, "arn": "..." } }
		fmt.Printf(`{
			"cluster": {
				"endpoint": "https://example.com/k8s",
				"certificateAuthority": {
					"data": "dGVzdC1jZXJ0LWRhdGE="
				},
				"arn": "arn:aws:eks:us-east-1:123456789012:cluster/eks-prod"
			}
		}`)
		return
	}

	fmt.Fprintf(os.Stderr, "Unknown mock command: %s %s\n", cmd, subCmd)
	os.Exit(2)
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".kboot.yaml")

	content := `
sso_session: my-sso
clusters:
  - alias: test-cluster
    profile: aws-profile
    region: us-east-1
    name: real-cluster-name
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write mock config: %v", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.SSOSession != "my-sso" {
		t.Errorf("Expected sso_session 'my-sso', got '%s'", cfg.SSOSession)
	}
	if len(cfg.Clusters) != 1 {
		t.Fatalf("Expected 1 cluster, got %d", len(cfg.Clusters))
	}
	c := cfg.Clusters[0]
	if c.Alias != "test-cluster" {
		t.Errorf("Expected alias 'test-cluster', got '%s'", c.Alias)
	}
}

func TestGenerateKubeconfig(t *testing.T) {
	// Mock execCommand
	execCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	// Restore after test
	defer func() { execCommand = exec.Command }()

	tmpDir := t.TempDir()

	cluster := Cluster{
		Alias:   "test-alias",
		Profile: "test-profile",
		Region:  "us-west-2",
		Name:    "test-cluster",
	}

	// Run function
	path, err := generateKubeconfig(tmpDir, cluster)
	if err != nil {
		t.Fatalf("generateKubeconfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Kubeconfig file not created at %s", path)
	}

	// Verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read generated kubeconfig: %v", err)
	}

	// Simple string checks
	s := string(content)
	if !strings.Contains(s, "current-context: test-alias") {
		t.Errorf("Kubeconfig missing current-context: test-alias")
	}
	if !strings.Contains(s, "server: https://example.com/k8s") {
		t.Errorf("Kubeconfig missing mocked server endpoint")
	}
	if !strings.Contains(s, "certificate-authority-data: dGVzdC1jZXJ0LWRhdGE=") {
		t.Errorf("Kubeconfig missing mocked CA data")
	}
}
