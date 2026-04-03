package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	eksTokenPrefix     = "k8s-aws-v1."
	eksTokenExpiration = 60
	clusterIDHeader    = "x-k8s-aws-id"
	emptyBodyHash      = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// Client wraps AWS SDK clients
type Client struct {
	Config    aws.Config
	EKSClient *eks.Client
	STSClient *sts.Client
}

// ClientOptions holds custom AWS file paths for isolated mode
type ClientOptions struct {
	CredentialsFile string
	ConfigFile      string
	SSOCacheDir     string
}

// NewClient initializes AWS clients for a specific profile and region
func NewClient(ctx context.Context, profile, region string) (*Client, error) {
	return NewClientWithOptions(ctx, profile, region, ClientOptions{})
}

// NewClientWithOptions initializes AWS clients with custom file paths
func NewClientWithOptions(ctx context.Context, profile, region string, opts ClientOptions) (*Client, error) {
	loadOpts := []func(*config.LoadOptions) error{
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	}

	if opts.CredentialsFile != "" {
		if _, err := os.Stat(opts.CredentialsFile); err == nil {
			loadOpts = append(loadOpts, config.WithSharedCredentialsFiles([]string{opts.CredentialsFile}))
		}
	}
	if opts.ConfigFile != "" {
		if _, err := os.Stat(opts.ConfigFile); err == nil {
			loadOpts = append(loadOpts, config.WithSharedConfigFiles([]string{opts.ConfigFile}))
		}
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config for profile %s: %w", profile, err)
	}

	return &Client{
		Config:    cfg,
		EKSClient: eks.NewFromConfig(cfg),
		STSClient: sts.NewFromConfig(cfg),
	}, nil
}

// CheckIdentity verifies if the current credentials are valid
func (c *Client) CheckIdentity(ctx context.Context) (string, error) {
	out, err := c.STSClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %w", err)
	}
	if out.Arn == nil {
		return "", fmt.Errorf("caller identity returned nil ARN")
	}
	return *out.Arn, nil
}

// ClusterInfo holds essential data to generate kubeconfig
type ClusterInfo struct {
	Name     string
	Endpoint string
	CAData   string
	ARN      string
}

// DescribeCluster fetches cluster details
func (c *Client) DescribeCluster(ctx context.Context, clusterName string) (*ClusterInfo, error) {
	out, err := c.EKSClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe cluster %s: %w", clusterName, err)
	}

	if out.Cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterName)
	}

	if out.Cluster.Endpoint == nil {
		return nil, fmt.Errorf("cluster %s has no endpoint", clusterName)
	}
	if out.Cluster.CertificateAuthority == nil || out.Cluster.CertificateAuthority.Data == nil {
		return nil, fmt.Errorf("cluster %s has no certificate authority data", clusterName)
	}
	if out.Cluster.Arn == nil {
		return nil, fmt.Errorf("cluster %s has no ARN", clusterName)
	}

	return &ClusterInfo{
		Name:     aws.ToString(out.Cluster.Name),
		Endpoint: aws.ToString(out.Cluster.Endpoint),
		CAData:   aws.ToString(out.Cluster.CertificateAuthority.Data),
		ARN:      aws.ToString(out.Cluster.Arn),
	}, nil
}

// GetEKSToken generates a presigned STS GetCallerIdentity token for EKS authentication.
func (c *Client) GetEKSToken(ctx context.Context, clusterName string) (string, error) {
	return generateEKSToken(ctx, c.Config, clusterName)
}

// GetCredentials retrieves temporary credentials for the current profile.
func (c *Client) GetCredentials(ctx context.Context) (*Credentials, error) {
	creds, err := c.Config.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	return &Credentials{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Expires:         creds.Expires,
	}, nil
}

// Credentials holds AWS credential data
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expires         time.Time
}

// GenerateEKSToken is a standalone function that creates an EKS auth token
// without requiring a full Client instance. Used by kubeconfig generator.
func GenerateEKSToken(ctx context.Context, profile, region, clusterName string) (string, error) {
	return GenerateEKSTokenWithOptions(ctx, profile, region, clusterName, ClientOptions{})
}

// GenerateEKSTokenWithOptions creates an EKS auth token with custom AWS file paths.
func GenerateEKSTokenWithOptions(ctx context.Context, profile, region, clusterName string, opts ClientOptions) (string, error) {
	loadOpts := []func(*config.LoadOptions) error{
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	}

	if opts.CredentialsFile != "" {
		if _, err := os.Stat(opts.CredentialsFile); err == nil {
			loadOpts = append(loadOpts, config.WithSharedCredentialsFiles([]string{opts.CredentialsFile}))
		}
	}
	if opts.ConfigFile != "" {
		if _, err := os.Stat(opts.ConfigFile); err == nil {
			loadOpts = append(loadOpts, config.WithSharedConfigFiles([]string{opts.ConfigFile}))
		}
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	return generateEKSToken(ctx, cfg, clusterName)
}

func generateEKSToken(ctx context.Context, cfg aws.Config, clusterName string) (string, error) {
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	stsURL := fmt.Sprintf("https://sts.%s.amazonaws.com/?Action=GetCallerIdentity&Version=2011-06-15&X-Amz-Expires=%d", cfg.Region, eksTokenExpiration)

	req, err := http.NewRequest(http.MethodGet, stsURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set(clusterIDHeader, clusterName)

	signer := v4.NewSigner()

	signedURL, _, err := signer.PresignHTTP(
		ctx,
		creds,
		req,
		emptyBodyHash,
		"sts",
		cfg.Region,
		time.Now(),
	)
	if err != nil {
		return "", fmt.Errorf("failed to presign request: %w", err)
	}

	token := eksTokenPrefix + base64.RawURLEncoding.EncodeToString([]byte(signedURL))

	return token, nil
}
