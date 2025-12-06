// Package datasources provides abstractions for fetching events from external services.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE the DataSource interface without updating EFA 0003.
// Claude MUST stop and ask before modifying this file.
package datasources

import (
	"context"

	"github.com/dakaneye/kora/internal/models"
)

// DataSource fetches events from an external service.
// Each service (GitHub, Slack) has one DataSource implementation.
//
// Implementations must:
//   - Respect context cancellation at all stages
//   - Handle rate limiting gracefully (backoff, partial results)
//   - Return partial results when possible (some calls succeed, others fail)
//   - Never log or expose credentials (delegate to AuthProvider)
//   - Validate all events before returning (use Event.Validate())
//
// IT IS FORBIDDEN TO CHANGE THIS INTERFACE without updating EFA 0003.
type DataSource interface {
	// Name returns a human-readable identifier for logging.
	// Format: lowercase with hyphens (e.g., "github-prs", "slack-mentions").
	Name() string

	// Service returns which service this datasource connects to.
	// Used for grouping events and associating with AuthProviders.
	Service() models.Source

	// Fetch retrieves events since the given timestamp.
	// Returns events that occurred after 'since' (exclusive).
	//
	// Error handling:
	//   - Returns (result, nil) on full success
	//   - Returns (result, err) on partial success (some events retrieved)
	//   - Returns (nil, err) on complete failure
	//
	// The returned events MUST:
	//   - Pass Event.Validate()
	//   - Have Timestamp > Since
	//   - Be sorted by Timestamp ascending
	//
	// Context handling:
	//   - Respect ctx.Done() for cancellation
	//   - Return ctx.Err() if canceled
	//   - Use ctx for all network operations
	//
	// EFA 0003: Context must be used for all network operations.
	// EFA 0003: Partial success must be supported.
	// EFA 0001: All returned events must pass Validate().
	Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error)
}
