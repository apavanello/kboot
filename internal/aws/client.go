package aws

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
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

// NewClient initializes AWS clients for a specific profile and region
func NewClient(ctx context.Context, profile, region string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
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
	input := &sts.GetCallerIdentityInput{}
	out, err := c.STSClient.GetCallerIdentity(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %w", err)
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
	input := &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}

	out, err := c.EKSClient.DescribeCluster(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe cluster %s: %w", clusterName, err)
	}

	if out.Cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterName)
	}

	return &ClusterInfo{
		Name:     *out.Cluster.Name,
		Endpoint: *out.Cluster.Endpoint,
		CAData:   *out.Cluster.CertificateAuthority.Data,
		ARN:      *out.Cluster.Arn,
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
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
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

func payloadHash(body interface{}) string {
	if body == nil {
		hash := sha256.Sum256([]byte{})
		return hex.EncodeToString(hash[:])
	}
	return emptyBodyHash
}
