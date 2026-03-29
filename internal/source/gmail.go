package source

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

// Gmail fetches email activity via the gws CLI.
type Gmail struct {
	runner exec.Runner
}

// NewGmail returns a Gmail source. If runner is nil, a real subprocess runner is used.
func NewGmail(runner exec.Runner) *Gmail {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &Gmail{runner: runner}
}

func (g *Gmail) Name() string { return "gmail" }

func (g *Gmail) CheckAuth(ctx context.Context) error {
	_, err := g.runner.Run(ctx, "gws", "auth", "status")
	if err != nil {
		return fmt.Errorf("gmail auth check: %w", err)
	}
	return nil
}

func (g *Gmail) RefreshAuth(ctx context.Context) error {
	_, err := g.runner.Run(ctx, "gws", "auth", "login")
	if err != nil {
		return fmt.Errorf("gmail auth refresh: %w", err)
	}
	return nil
}

func (g *Gmail) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	after := time.Now().Add(-since).Format("2006/01/02")
	query := fmt.Sprintf("is:unread after:%s", after)
	listParams := fmt.Sprintf(`{"userId":"me","q":"%s","maxResults":100}`, query)

	// Phase 1: List message IDs
	listResult, err := g.runner.Run(ctx, "gws", "gmail", "users", "messages", "list", "--params", listParams)
	if err != nil {
		return nil, fmt.Errorf("gmail list: %w", err)
	}

	var listResp struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(listResult.Stdout), &listResp); err != nil {
		return nil, fmt.Errorf("gmail parse list: %w", err)
	}

	if len(listResp.Messages) == 0 {
		empty, _ := json.Marshal(map[string]any{"messages": []any{}})
		return empty, nil
	}

	// Phase 2: Fetch each message's metadata in parallel
	var mu sync.Mutex
	messages := make([]json.RawMessage, 0, len(listResp.Messages))
	var fetchErr error

	var wg sync.WaitGroup
	for _, msg := range listResp.Messages {
		wg.Add(1)
		go func() {
			defer wg.Done()
			getParams := fmt.Sprintf(`{"userId":"me","id":"%s","format":"metadata","metadataHeaders":["From","Subject","Date"]}`, msg.ID)
			getResult, err := g.runner.Run(ctx, "gws", "gmail", "users", "messages", "get", "--params", getParams)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if fetchErr == nil {
					fetchErr = fmt.Errorf("gmail get %s: %w", msg.ID, err)
				}
				return
			}
			messages = append(messages, json.RawMessage(getResult.Stdout))
		}()
	}
	wg.Wait()

	if fetchErr != nil {
		return nil, fetchErr
	}

	data, err := json.Marshal(map[string]any{"messages": messages})
	if err != nil {
		return nil, fmt.Errorf("gmail marshal: %w", err)
	}
	return data, nil
}
