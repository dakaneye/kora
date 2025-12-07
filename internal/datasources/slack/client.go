package slack

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/dakaneye/kora/internal/datasources"
)

const (
	// defaultBaseURL is the Slack API base URL.
	defaultBaseURL = "https://slack.com/api"

	// defaultTimeout is the HTTP client timeout.
	defaultTimeout = 30 * time.Second

	// maxResponseSize is the maximum response body size (10MB).
	// This prevents memory exhaustion from malicious or malformed responses.
	maxResponseSize = 10 * 1024 * 1024
)

// apiRequest makes an authenticated request to the Slack API.
//
// SECURITY per EFA 0002:
//   - Token is ONLY used in the Authorization header
//   - Token is NEVER logged or included in error messages
//   - Response size is limited to prevent memory exhaustion
//
// Handles rate limiting (HTTP 429) per EFA 0003.
func (d *DataSource) apiRequest(ctx context.Context, token, method string, params url.Values) ([]byte, error) {
	reqURL := fmt.Sprintf("%s/%s", d.baseURL, method)
	if params != nil {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// SECURITY: Token is ONLY used here, in the Authorization header.
	// IT IS FORBIDDEN TO LOG THIS VALUE. See EFA 0002.
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	//nolint:errcheck // Ignore close error on read-only body
	defer resp.Body.Close()

	// Handle rate limiting per EFA 0003
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		return nil, fmt.Errorf("%w: retry after %s", datasources.ErrRateLimited, retryAfter)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	// Read response body with size limit to prevent memory exhaustion
	// LimitReader ensures we don't read more than maxResponseSize bytes
	limitedReader := io.LimitReader(resp.Body, maxResponseSize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check if response exceeded limit
	if len(body) > maxResponseSize {
		return nil, fmt.Errorf("response too large: exceeds %d bytes", maxResponseSize)
	}

	return body, nil
}
