package main

import (
	"io"
	"os"
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
	t.Skip("Skipping TUI interactive test")
}

func TestAuthAdd_SSO(t *testing.T) {
	t.Skip("Skipping TUI interactive test")
}

func TestClusterAdd(t *testing.T) {
	t.Skip("Skipping TUI interactive test")
}
