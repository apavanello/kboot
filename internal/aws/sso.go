package aws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Login performs the SSO login flow.
// Currently uses the AWS CLI as it handles the complex interactive device flow robustly.
func SSOLogin(ctx context.Context, ssoSession string) error {
	return runLoginCmd(ctx, "--sso-session", ssoSession)
}

// SSOLoginProfile performs login using the profile name
func SSOLoginProfile(ctx context.Context, profile string) error {
	return runLoginCmd(ctx, "--profile", profile)
}

func runLoginCmd(ctx context.Context, argType, argValue string) error {
	// Check if "aws" is in path
	_, err := exec.LookPath("aws")
	if err != nil {
		return fmt.Errorf("aws cli not found: required for sso login")
	}

	// Lock stdin - multiple threads might try to login?
	// Actually, the worker pool logic might launch multiple logins.
	// AWS CLI interactive login requires user interaction in browser, but CLI waits.
	// If multiple pop up, it might be chaotic.
	// For MVP, we let them happen, but usually only one is needed per session.
	// Since we share session, maybe we should sync logging in?
	// Leaving as is for now (Parallel login is a feature request), but interactive login usually isn't parallel friendly on STDIN.
	// However, `aws sso login` just opens browser and polls. It doesn't use STDIN much unless configured otherwise.

	cmd := exec.CommandContext(ctx, "aws", "sso", "login", argType, argValue)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// cmd.Stdin = os.Stdin // Interactive input usually not needed for standard SSO browser flow

	fmt.Printf(">> Triggering SSO Login for %s %s\n", argType, argValue)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sso login failed: %w", err)
	}
	return nil
}
