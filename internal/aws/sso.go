package aws

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
)

// SSOLogin performs SSO login using the AWS SDK device authorization flow.
func SSOLogin(ctx context.Context, ssoSession string) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load default config: %w", err)
	}
	return doSSOLogin(ctx, cfg, ssoSession)
}

// SSOLoginProfile performs SSO login using the profile name.
func SSOLoginProfile(ctx context.Context, profile string) error {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		return fmt.Errorf("failed to load config for profile %s: %w", profile, err)
	}
	return doSSOLogin(ctx, cfg, profile)
}

func doSSOLogin(ctx context.Context, cfg aws.Config, identifier string) error {
	ssoSession, ssoStartURL, ssoRegion, err := resolveSSOConfig(identifier)
	if err != nil {
		return err
	}

	oidcClient := ssooidc.NewFromConfig(cfg, func(o *ssooidc.Options) {
		o.Region = ssoRegion
	})

	clientID, clientSecret, err := getOrCreateSSOClient(ctx, oidcClient, ssoSession)
	if err != nil {
		return fmt.Errorf("SSO client registration: %w", err)
	}

	deviceAuth, err := oidcClient.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     aws.String(clientID),
		ClientSecret: aws.String(clientSecret),
		StartUrl:     aws.String(ssoStartURL),
	})
	if err != nil {
		return fmt.Errorf("start device authorization: %w", err)
	}

	verificationURI := aws.ToString(deviceAuth.VerificationUriComplete)
	fmt.Printf(">> Opening browser for SSO login...\n")
	fmt.Printf(">> If browser does not open, visit: %s\n", verificationURI)

	if err := openBrowser(verificationURI); err != nil {
		fmt.Printf(">> Could not open browser: %v\n", err)
	}

	fmt.Printf(">> Waiting for SSO login approval...\n")
	token, err := pollForToken(ctx, oidcClient, clientID, clientSecret, aws.ToString(deviceAuth.DeviceCode), int(deviceAuth.ExpiresIn))
	if err != nil {
		return fmt.Errorf("SSO login failed: %w", err)
	}

	if err := writeSSOTokenCache(ssoSession, ssoStartURL, ssoRegion, aws.ToString(token.AccessToken), clientID, clientSecret, token.ExpiresIn); err != nil {
		fmt.Printf("Warning: could not cache SSO token: %v\n", err)
	}

	fmt.Printf(">> SSO login successful!\n")
	return nil
}

func resolveSSOConfig(identifier string) (session, startURL, region string, err error) {
	home := os.Getenv("HOME")
	configPath := filepath.Join(home, ".aws", "config")
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		return "", "", "", fmt.Errorf("cannot read %s: %w", configPath, readErr)
	}

	ssoSessions := parseSSOSessions(string(data))

	profileSection := fmt.Sprintf("[profile %s]", identifier)
	ssoSessionName, profileStartURL, profileRegion := parseProfileSection(string(data), profileSection)

	if profileStartURL != "" && profileRegion != "" {
		return identifier, profileStartURL, profileRegion, nil
	}

	if ssoSessionName != "" {
		if sess, ok := ssoSessions[ssoSessionName]; ok {
			return ssoSessionName, sess.StartURL, sess.Region, nil
		}
	}

	if identifier != "" {
		if sess, ok := ssoSessions[identifier]; ok {
			return identifier, sess.StartURL, sess.Region, nil
		}
	}

	return "", "", "", fmt.Errorf("could not resolve SSO configuration for %q", identifier)
}

type ssoSessionInfo struct {
	StartURL string
	Region   string
}

func parseSSOSessions(content string) map[string]ssoSessionInfo {
	sessions := make(map[string]ssoSessionInfo)
	lines := strings.Split(content, "\n")
	var currentName string
	var info ssoSessionInfo

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[sso-session ") && strings.HasSuffix(trimmed, "]") {
			if currentName != "" {
				sessions[currentName] = info
			}
			currentName = trimmed[13 : len(trimmed)-1]
			info = ssoSessionInfo{}
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			if currentName != "" {
				sessions[currentName] = info
				currentName = ""
			}
			continue
		}
		if currentName == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "sso_start_url":
			info.StartURL = value
		case "sso_region":
			info.Region = value
		}
	}
	if currentName != "" {
		sessions[currentName] = info
	}

	return sessions
}

func parseProfileSection(content, sectionName string) (ssoSession, startURL, region string) {
	lines := strings.Split(content, "\n")
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == sectionName {
			inSection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			if inSection {
				break
			}
			continue
		}
		if !inSection {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "sso_session":
			ssoSession = value
		case "sso_start_url":
			startURL = value
		case "sso_region", "region":
			region = value
		}
	}

	return
}

func getOrCreateSSOClient(ctx context.Context, client *ssooidc.Client, ssoSession string) (clientID, clientSecret string, err error) {
	cachePath := filepath.Join(os.Getenv("HOME"), ".aws", "sso", "cache", ssoSession+"-client.json")

	if data, err := os.ReadFile(cachePath); err == nil {
		var cached struct {
			ClientID     string    `json:"clientId"`
			ClientSecret string    `json:"clientSecret"`
			ExpiresAt    time.Time `json:"expiresAt"`
		}
		if err := json.Unmarshal(data, &cached); err == nil && time.Now().Before(cached.ExpiresAt) {
			return cached.ClientID, cached.ClientSecret, nil
		}
	}

	registered, err := client.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String("kboot"),
		ClientType: aws.String("public"),
	})
	if err != nil {
		return "", "", err
	}

	clientID = aws.ToString(registered.ClientId)
	clientSecret = aws.ToString(registered.ClientSecret)
	expiresAt := time.Now().Add(24 * time.Hour)

	cacheData, _ := json.Marshal(map[string]any{
		"clientId":              clientID,
		"clientSecret":          clientSecret,
		"expiresAt":             expiresAt,
		"clientSecretExpiresAt": expiresAt,
	})

	os.MkdirAll(filepath.Dir(cachePath), 0700)
	os.WriteFile(cachePath, cacheData, 0600)

	return clientID, clientSecret, nil
}

func pollForToken(ctx context.Context, client *ssooidc.Client, clientID, clientSecret, deviceCode string, expiresInSeconds int) (*ssooidc.CreateTokenOutput, error) {
	deadline := time.Now().Add(time.Duration(expiresInSeconds) * time.Second)
	interval := 5 * time.Second

	for time.Now().Before(deadline) {
		token, err := client.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     aws.String(clientID),
			ClientSecret: aws.String(clientSecret),
			DeviceCode:   aws.String(deviceCode),
			GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
		})
		if err == nil {
			return token, nil
		}

		var authPending *ssooidctypes.AuthorizationPendingException
		var slowDown *ssooidctypes.SlowDownException
		var accessDenied *ssooidctypes.AccessDeniedException
		var expiredToken *ssooidctypes.ExpiredTokenException
		var invalidClient *ssooidctypes.InvalidClientException
		var invalidGrant *ssooidctypes.InvalidGrantException

		switch {
		case errors.As(err, &slowDown):
			interval = 10 * time.Second
			time.Sleep(interval)
			continue
		case errors.As(err, &authPending):
			time.Sleep(interval)
			continue
		case errors.As(err, &accessDenied):
			return nil, fmt.Errorf("SSO login denied by user or administrator")
		case errors.As(err, &expiredToken):
			return nil, fmt.Errorf("device code expired, please try again")
		case errors.As(err, &invalidClient):
			return nil, fmt.Errorf("SSO client registration invalid, please try again")
		case errors.As(err, &invalidGrant):
			return nil, fmt.Errorf("invalid device code, please try again")
		default:
			return nil, err
		}
	}

	return nil, fmt.Errorf("SSO login timed out after %d seconds", expiresInSeconds)
}

func writeSSOTokenCache(ssoSession, startURL, region, accessToken, clientID, clientSecret string, expiresIn int32) error {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".aws", "sso", "cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return err
	}

	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	registrationExpiresAt := time.Now().Add(24 * time.Hour)

	hash := sha1.Sum([]byte(strings.ToLower(ssoSession)))
	cacheFile := fmt.Sprintf("%x.json", hash)

	tokenData := map[string]any{
		"startUrl":              startURL,
		"accessToken":           accessToken,
		"expiresAt":             expiresAt.Format(time.RFC3339),
		"region":                region,
		"clientId":              clientID,
		"clientSecret":          clientSecret,
		"registrationExpiresAt": registrationExpiresAt.Format(time.RFC3339),
	}

	path := filepath.Join(cacheDir, cacheFile)
	data, _ := json.MarshalIndent(tokenData, "", "  ")
	return os.WriteFile(path, data, 0600)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
