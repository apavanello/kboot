package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"kboot/internal/aws"
	"kboot/internal/config"
)

// EventType defines what happened
type EventType int

const (
	EventStart EventType = iota
	EventSuccess
	EventError
)

// Event is sent back to UI to update state
type Event struct {
	ClusterAlias string
	Type         EventType
	Message      string
	Result       *aws.ClusterInfo // Populated on Success
	Err          error            // Populated on Error
}

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
		MaxWorkers: 5,
	}
}

// Run executes the parallel sync process sending events to the channel
func (o *Orchestrator) Run(ctx context.Context, eventChan chan<- Event) map[string]Result {
	results := make(map[string]Result)
	var mu sync.Mutex

	// Semaphore
	sem := make(chan struct{}, o.MaxWorkers)
	var wg sync.WaitGroup

	for _, cluster := range o.Config.Clusters {
		wg.Add(1)

		go func(c config.Cluster) {
			defer wg.Done()

			// Wait for slot
			sem <- struct{}{}
			defer func() { <-sem }()

			// Notify Start
			if eventChan != nil {
				eventChan <- Event{ClusterAlias: c.Alias, Type: EventStart, Message: "Authenticating..."}
			}

			workerCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()

			res := o.processCluster(workerCtx, c)

			// Notify Finish
			if eventChan != nil {
				if res.Error != nil {
					eventChan <- Event{ClusterAlias: c.Alias, Type: EventError, Message: res.Error.Error(), Err: res.Error}
				} else {
					eventChan <- Event{ClusterAlias: c.Alias, Type: EventSuccess, Message: "Ready", Result: res.ClusterInfo}
				}
			}

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

	client, err := aws.NewClient(ctx, c.Profile, c.Region)
	if err != nil {
		res.Error = fmt.Errorf("client init: %w", err)
		return res
	}

	// 1. Check Identity
	_, err = client.CheckIdentity(ctx)
	if err != nil {
		// 2. Login
		if loginErr := aws.SSOLoginProfile(ctx, c.Profile); loginErr != nil {
			res.Error = fmt.Errorf("login failed: %v", loginErr)
			return res
		}

		if _, err := client.CheckIdentity(ctx); err != nil {
			res.Error = fmt.Errorf("verify failed: %w", err)
			return res
		}
	}

	// 3. Describe
	info, err := client.DescribeCluster(ctx, c.Name)
	if err != nil {
		res.Error = fmt.Errorf("describe error: %w", err)
		return res
	}

	res.ClusterInfo = info
	return res
}
