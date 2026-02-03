package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Client wraps AWS SDK clients
type Client struct {
	Config    aws.Config
	EKSClient *eks.Client
	STSClient *sts.Client
}

// NewClient initializes AWS clients for a specific profile and region
func NewClient(ctx context.Context, profile, region string) (*Client, error) {
	// Load AWS config with specific profile
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
