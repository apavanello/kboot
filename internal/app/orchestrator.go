package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"kboot/internal/aws"
	"kboot/internal/config"
)

// Orchestrator manages the boot process
type Orchestrator struct {
	Config     *config.Config
	MaxWorkers int
}

// Result holds the outcome of a cluster sync
type Result struct {
	ClusterAlias string
	ClusterInfo  *aws.ClusterInfo
	Error        error
}

// NewOrchestrator creates a new manager
func NewOrchestrator(cfg *config.Config) *Orchestrator {
	return &Orchestrator{
		Config:     cfg,
		MaxWorkers: 5, // Requirement: Limit to 5
	}
}

// Run executes the parallel sync process
// It returns a map of results and a global error if something critical failed
func (o *Orchestrator) Run(ctx context.Context) map[string]Result {
	results := make(map[string]Result)
	var mu sync.Mutex

	// Semaphore channel to limit concurrency
	sem := make(chan struct{}, o.MaxWorkers)
	var wg sync.WaitGroup

	for _, cluster := range o.Config.Clusters {
		wg.Add(1)

		// Acquire token
		sem <- struct{}{}

		go func(c config.Cluster) {
			defer wg.Done()
			defer func() { <-sem }() // Release token

			// Process Cluster
			// We give each worker a slightly tighter timeout to avoid one hanging everything
			workerCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()

			res := o.processCluster(workerCtx, c)

			mu.Lock()
			results[c.Alias] = res
			mu.Unlock()
		}(cluster)
	}

	wg.Wait()
	return results
}

func (o *Orchestrator) processCluster(ctx context.Context, c config.Cluster) Result {
	res := Result{ClusterAlias: c.Alias}

	// 1. Init Client
	client, err := aws.NewClient(ctx, c.Profile, c.Region)
	if err != nil {
		res.Error = fmt.Errorf("client init failed: %w", err)
		return res
	}

	// 2. Check Identity (Cache / Valid Session)
	// We try to avoid login if possible
	_, err = client.CheckIdentity(ctx)
	if err != nil {
		// 3. If invalid, try SSO Login
		// Note: We need to find the sso_session name.
		// For now assuming a convention or needing extended config.
		// If the SDK load failed, it usually means no token.
		// But 'aws sso login' needs the session name, which usually is in ~/.aws/config linked to the profile.
		// Since we handle the profile config via external files, we rely on 'aws sso login --profile' if possible,
		// or we need to parse the sso-session from the config.
		// The requirement said "use check, if fail login".

		// TODO: Implement cleaner session discovery.
		// For MVP, we try to run login for the *profile* if simple login fails.
		// However, `aws sso login` often wants a session name or profile.
		// Let's assume we can try logging in via the profile directly.
		loginErr := o.performLogin(ctx, c.Profile)
		if loginErr != nil {
			res.Error = fmt.Errorf("auth failed: %v (initial check: %v)", loginErr, err)
			return res
		}

		// Retry identity check
		_, err = client.CheckIdentity(ctx)
		if err != nil {
			res.Error = fmt.Errorf("still unauthenticated after login: %w", err)
			return res
		}
	}

	// 4. Fetch Cluster Info (Describe)
	info, err := client.DescribeCluster(ctx, c.Name)
	if err != nil {
		res.Error = fmt.Errorf("describe cluster failed: %w", err)
		return res
	}

	res.ClusterInfo = info
	return res
}

func (o *Orchestrator) performLogin(ctx context.Context, profile string) error {
	// We use the AWS CLI wrapper loop we created in internal/aws/sso.go
	// Since we don't have the session name easily available in the Cluster struct (it's in ~/.aws/config),
	// we will try `aws sso login --profile <profile>` which usually works if the profile has sso_session configured.

	// Reusing the sso.go logic but slight adaptation needed since sso.go currently asks for sessionName.
	// Let's update sso.go or use a direct command here.
	// Direct command using profile is safer for users with mixed configs.

	// Using exec directly here for simplicity as we improve aws package abstraction
	// Ideally this goes into internal/aws
	return aws.SSOLoginProfile(ctx, profile)
}
