package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

const maxConcurrentFetches = 10

// Gmail fetches email activity via the gws CLI.
type Gmail struct {
	cliSource
}

// NewGmail returns a Gmail source. If runner is nil, a real subprocess runner is used.
func NewGmail(runner exec.Runner) *Gmail {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &Gmail{cliSource: cliSource{
		name:        "gmail",
		runner:      runner,
		cli:         "gws",
		checkArgs:   []string{"auth", "status"},
		refreshArgs: []string{"auth", "login"},
	}}
}

// Fetch retrieves unread messages within the given time window.
func (g *Gmail) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	after := time.Now().Add(-since).Format("2006/01/02")
	query := fmt.Sprintf("is:unread after:%s", after)
	listParams, err := json.Marshal(map[string]any{"userId": "me", "q": query, "maxResults": 100})
	if err != nil {
		return nil, fmt.Errorf("gmail list params marshal: %w", err)
	}

	// Phase 1: List message IDs
	listResult, err := g.runner.Run(ctx, "gws", "gmail", "users", "messages", "list", "--params", string(listParams))
	if err != nil {
		return nil, fmt.Errorf("gmail list: %w", err)
	}

	var listResp struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if unmarshalErr := json.Unmarshal([]byte(listResult.Stdout), &listResp); unmarshalErr != nil {
		return nil, fmt.Errorf("gmail parse list: %w", unmarshalErr)
	}

	if len(listResp.Messages) == 0 {
		empty, marshalErr := json.Marshal(map[string]any{"messages": []any{}})
		if marshalErr != nil {
			return nil, fmt.Errorf("gmail empty marshal: %w", marshalErr)
		}
		return empty, nil
	}

	// Phase 2: Fetch each message's metadata in parallel (bounded)
	var mu sync.Mutex
	messages := make([]json.RawMessage, 0, len(listResp.Messages))
	var errs []error

	sem := make(chan struct{}, maxConcurrentFetches)
	var wg sync.WaitGroup
	for _, msg := range listResp.Messages {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			getParams, marshalErr := json.Marshal(map[string]any{
				"userId":          "me",
				"id":              msg.ID,
				"format":          "metadata",
				"metadataHeaders": []string{"From", "Subject", "Date"},
			})
			if marshalErr != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("gmail get params marshal %s: %w", msg.ID, marshalErr))
				mu.Unlock()
				return
			}
			getResult, runErr := g.runner.Run(ctx, "gws", "gmail", "users", "messages", "get", "--params", string(getParams))
			mu.Lock()
			defer mu.Unlock()
			if runErr != nil {
				errs = append(errs, fmt.Errorf("gmail get %s: %w", msg.ID, runErr))
				return
			}
			messages = append(messages, json.RawMessage(getResult.Stdout))
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	data, err := json.Marshal(map[string]any{"messages": messages})
	if err != nil {
		return nil, fmt.Errorf("gmail marshal: %w", err)
	}
	return data, nil
}
