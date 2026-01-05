package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to mock home dir
func mockHomeDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "kboot_test_home")
	if err != nil {
		t.Fatalf("Failed to create temp home: %v", err)
	}
	// Mock the package level variables
	getHomeDir = func() (string, error) {
		return tempDir, nil
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
		// Restore default
		getHomeDir = os.UserHomeDir
	})

	return tempDir
}

// Helper to mock input
func mockInput(t *testing.T, input string) {
	getInput = func() io.Reader {
		return strings.NewReader(input)
	}
	t.Cleanup(func() {
		getInput = func() io.Reader { return os.Stdin } // Restore
	})
}

func TestAuthNew_Static(t *testing.T) {
	tempHome := mockHomeDir(t)
	// Input: "1" (Static)
	mockInput(t, "1\n")

	// Redirect stdout to avoid littering test logs (optional)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create dummy credentials file to verify backup
	awsDir := filepath.Join(tempHome, ".aws")
	os.MkdirAll(awsDir, 0700)
	dummyCreds := filepath.Join(awsDir, "credentials")
	os.WriteFile(dummyCreds, []byte("old=data"), 0600)

	// Run
	authNew()

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	// Verification
	// 1. Check if backup exists
	files, _ := os.ReadDir(awsDir)
	foundBackup := false
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "credentials.bak") {
			foundBackup = true
			break
		}
	}
	if !foundBackup {
		t.Error("Expected backup file to be created")
	}

	// 2. Check if credentials file is empty (recreated)
	content, _ := os.ReadFile(dummyCreds)
	if len(content) > 0 {
		t.Error("Expected credentials file to be empty after auth new")
	}
}

func TestAuthAdd_Static(t *testing.T) {
	tempHome := mockHomeDir(t)
	// Input: "1" for choice, then "myprofile", "starturl", "region", "accid", "role"
	// Wait, choice 1 is Static: Profile, Key, Secret, Token (optional)
	// Input: "1\n" (Choice), "test-profile\n" (Name), "AKIATEST\n" (Key), "SECRET123\n" (Secret), "\n" (Token skip)
	mockInput(t, "1\ntest-profile\nAKIATEST\nSECRET123\n\n")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	authAdd()

	w.Close()
	os.Stdout = oldStdout
	io.Copy(io.Discard, r) // drain

	// Verify file content
	credsPath := filepath.Join(tempHome, ".aws", "credentials")
	content, err := os.ReadFile(credsPath)
	if err != nil {
		t.Fatalf("Failed to read credentials: %v", err)
	}

	strContent := string(content)
	if !strings.Contains(strContent, "[test-profile]") {
		t.Error("Expected [test-profile] in credentials")
	}
	if !strings.Contains(strContent, "aws_access_key_id = AKIATEST") {
		t.Error("Expected Access Key in credentials")
	}
}

func TestClusterAdd(t *testing.T) {
	tempHome := mockHomeDir(t)

	// 1. Setup mock profiles to be discovered
	awsDir := filepath.Join(tempHome, ".aws")
	os.MkdirAll(awsDir, 0700)
	os.WriteFile(filepath.Join(awsDir, "credentials"), []byte("[default]\nkey=val"), 0600)
	os.WriteFile(filepath.Join(awsDir, "config"), []byte("[profile sso-dev]\nregion=us"), 0600)

	// 2. Setup initial kboot config
	kbootConfig := filepath.Join(tempHome, ".kboot.yaml")
	initialYaml := "clusters:\n"
	os.WriteFile(kbootConfig, []byte(initialYaml), 0644)

	// 3. Mock Inputs
	// Prompts: Alias, Real Name, Region, Profile
	// Input: "my-alias\n", "eks-cluster-1\n", "us-west-2\n", "default\n"
	mockInput(t, "my-alias\neks-cluster-1\nus-west-2\ndefault\n")

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	clusterAdd()

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	output := buf.String()
	if !strings.Contains(output, "Available Profiles") {
		t.Error("Expected clusterAdd to list available profiles")
	}
	if !strings.Contains(output, "sso-dev") { // Should be discovered
		t.Error("Expected discovered profile 'sso-dev' to be listed")
	}

	// Verify .kboot.yaml content
	content, _ := os.ReadFile(kbootConfig)
	strContent := string(content)

	if !strings.Contains(strContent, "alias: my-alias") {
		t.Error("Expected alias to be saved in .kboot.yaml")
	}
	if !strings.Contains(strContent, "name: eks-cluster-1") {
		t.Error("Expected cluster name to be saved")
	}
	if !strings.Contains(strContent, "region: us-west-2") {
		t.Error("Expected region to be saved")
	}
}
