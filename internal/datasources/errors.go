// Package datasources provides abstractions for fetching events from external services.
// Ground truth defined in specs/efas/0003-datasource-interface.md
package datasources

import "errors"

// Sentinel errors for datasource operations.
// IT IS FORBIDDEN TO ADD ERRORS without updating EFA 0003.
var (
	// ErrNotAuthenticated indicates the datasource's auth provider has no valid credentials.
	ErrNotAuthenticated = errors.New("datasource: not authenticated")

	// ErrRateLimited indicates the service rate limit was exceeded.
	// Check FetchResult.RateLimitReset for when to retry.
	ErrRateLimited = errors.New("datasource: rate limited")

	// ErrServiceUnavailable indicates the external service is down or unreachable.
	ErrServiceUnavailable = errors.New("datasource: service unavailable")

	// ErrTimeout indicates the operation exceeded the context deadline.
	ErrTimeout = errors.New("datasource: timeout")

	// ErrInvalidResponse indicates the service returned malformed data.
	ErrInvalidResponse = errors.New("datasource: invalid response")
)
