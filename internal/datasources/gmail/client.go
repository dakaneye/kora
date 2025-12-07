// Package gmail provides Gmail API client and datasource.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// SECURITY: This client uses GoogleOAuthCredential from the auth provider.
// Access tokens are used only in Authorization headers and MUST NEVER be logged.
// See EFA 0002 for credential security requirements.
//
//nolint:revive // Package name matches directory structure convention
package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dakaneye/kora/internal/auth/google"
	"github.com/dakaneye/kora/internal/datasources"
	"golang.org/x/sync/errgroup"
)

const (
	// gmailAPIBase is the base URL for Gmail API v1.
	gmailAPIBase = "https://www.googleapis.com/gmail/v1"

	// defaultMaxResults is the default limit for messages.list.
	defaultMaxResults = 100

	// maxBatchSize is the maximum number of concurrent message fetches.
	maxBatchSize = 10

	// maxRetries is the maximum number of retries for rate-limited requests.
	maxRetries = 3

	// initialBackoff is the initial backoff duration for exponential backoff.
	initialBackoff = 1 * time.Second
)

// GmailClient wraps the Gmail API v1.
//
// SECURITY: Uses GoogleOAuthCredential for authentication.
// Access tokens are used only in Authorization headers and MUST NEVER be logged.
type GmailClient struct {
	httpClient *http.Client
	credential *google.GoogleOAuthCredential
}

// NewGmailClient creates a new Gmail API client.
// The credential must be valid and non-expired.
//
// The httpClient parameter is optional; if nil, a client with 30s timeout is used.
// For production use, provide a client with appropriate timeouts.
func NewGmailClient(credential *google.GoogleOAuthCredential, httpClient *http.Client) *GmailClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return &GmailClient{
		httpClient: httpClient,
		credential: credential,
	}
}

// MessageID represents a minimal message reference from list response.
type MessageID struct {
	ID       string `json:"id"`
	ThreadID string `json:"threadId"`
}

// Message represents a full Gmail message with headers.
//
//nolint:govet // Field order matches API response for clarity
type Message struct {
	ID           string         `json:"id"`
	ThreadID     string         `json:"threadId"`
	LabelIDs     []string       `json:"labelIds"`
	Snippet      string         `json:"snippet"`
	Payload      MessagePayload `json:"payload"`
	InternalDate string         `json:"internalDate"` // Unix timestamp in milliseconds
}

// MessagePayload contains the message content structure.
//
//nolint:govet // Field order matches API response for clarity
type MessagePayload struct {
	Headers  []Header      `json:"headers"`
	Body     *MessageBody  `json:"body,omitempty"`
	Parts    []MessagePart `json:"parts,omitempty"`
	MimeType string        `json:"mimeType"`
}

// Header represents an email header (name-value pair).
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MessageBody contains the body data.
//
//nolint:govet // Field order matches API response for clarity
type MessageBody struct {
	Data string `json:"data"` // Base64 URL-encoded
	Size int    `json:"size"`
}

// MessagePart represents a MIME part of a multipart message.
type MessagePart struct {
	PartID   string        `json:"partId"`
	MimeType string        `json:"mimeType"`
	Filename string        `json:"filename"`
	Headers  []Header      `json:"headers"`
	Body     *MessageBody  `json:"body,omitempty"`
	Parts    []MessagePart `json:"parts,omitempty"`
}

// messagesListResponse is the API response for messages.list.
//
//nolint:govet // Field order matches API response for clarity
type messagesListResponse struct {
	Messages           []MessageID `json:"messages"`
	NextPageToken      string      `json:"nextPageToken"`
	ResultSizeEstimate int         `json:"resultSizeEstimate"`
}

// GetHeader returns the value of a header by name (case-insensitive).
// Returns empty string if the header is not found.
func (m *Message) GetHeader(name string) string {
	for _, h := range m.Payload.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// From returns the From header value.
func (m *Message) From() string {
	return m.GetHeader("From")
}

// FromEmail extracts just the email address from the From header.
// Returns empty string if parsing fails.
func (m *Message) FromEmail() string {
	from := m.From()
	if from == "" {
		return ""
	}
	addr, err := mail.ParseAddress(from)
	if err != nil {
		// Fallback: try to extract email with regex
		return extractEmail(from)
	}
	return addr.Address
}

// FromName extracts the display name from the From header.
// Returns empty string if no name is present.
func (m *Message) FromName() string {
	from := m.From()
	if from == "" {
		return ""
	}
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return ""
	}
	return addr.Name
}

// To returns the To header addresses as a slice of email addresses.
func (m *Message) To() []string {
	return parseAddressList(m.GetHeader("To"))
}

// CC returns the CC header addresses as a slice of email addresses.
func (m *Message) CC() []string {
	return parseAddressList(m.GetHeader("Cc"))
}

// Subject returns the Subject header value.
func (m *Message) Subject() string {
	return m.GetHeader("Subject")
}

// Date returns the email date.
// Tries to parse the Date header first, falls back to InternalDate.
func (m *Message) Date() time.Time {
	// Try Date header first
	dateStr := m.GetHeader("Date")
	if dateStr != "" {
		t, err := mail.ParseDate(dateStr)
		if err == nil {
			return t
		}
	}

	// Fall back to InternalDate (Unix milliseconds)
	if m.InternalDate != "" {
		ms, err := strconv.ParseInt(m.InternalDate, 10, 64)
		if err == nil {
			return time.UnixMilli(ms)
		}
	}

	return time.Time{}
}

// IsUnread returns true if the message has the UNREAD label.
func (m *Message) IsUnread() bool {
	return m.hasLabel("UNREAD")
}

// IsImportant returns true if the message has the IMPORTANT label.
func (m *Message) IsImportant() bool {
	return m.hasLabel("IMPORTANT")
}

// IsStarred returns true if the message has the STARRED label.
func (m *Message) IsStarred() bool {
	return m.hasLabel("STARRED")
}

// IsInInbox returns true if the message has the INBOX label.
func (m *Message) IsInInbox() bool {
	return m.hasLabel("INBOX")
}

// hasLabel checks if the message has a specific label.
func (m *Message) hasLabel(label string) bool {
	for _, l := range m.LabelIDs {
		if l == label {
			return true
		}
	}
	return false
}

// HasAttachments returns true if the message has attachments.
func (m *Message) HasAttachments() bool {
	return hasAttachmentParts(m.Payload.Parts)
}

// hasAttachmentParts recursively checks for attachment parts.
func hasAttachmentParts(parts []MessagePart) bool {
	for _, part := range parts {
		if part.Filename != "" {
			return true
		}
		if hasAttachmentParts(part.Parts) {
			return true
		}
	}
	return false
}

// IsMailingList returns true if the message appears to be from a mailing list.
// Checks for List-Unsubscribe and List-Id headers.
func (m *Message) IsMailingList() bool {
	if m.GetHeader("List-Unsubscribe") != "" {
		return true
	}
	if m.GetHeader("List-Id") != "" {
		return true
	}
	return false
}

// GetBodyText returns the plain text body of the message.
// For multipart messages, extracts the text/plain part.
func (m *Message) GetBodyText() string {
	// Try to get text from payload body directly
	if m.Payload.Body != nil && m.Payload.Body.Data != "" {
		if decoded, err := decodeBase64URL(m.Payload.Body.Data); err == nil {
			return decoded
		}
	}

	// Search in parts for text/plain
	return findTextPart(m.Payload.Parts, "text/plain")
}

// findTextPart recursively searches for a part with the given MIME type.
func findTextPart(parts []MessagePart, mimeType string) string {
	for _, part := range parts {
		if part.MimeType == mimeType && part.Body != nil && part.Body.Data != "" {
			if decoded, err := decodeBase64URL(part.Body.Data); err == nil {
				return decoded
			}
		}
		// Recurse into nested parts
		if result := findTextPart(part.Parts, mimeType); result != "" {
			return result
		}
	}
	return ""
}

// decodeBase64URL decodes a base64 URL-encoded string (Gmail's format).
func decodeBase64URL(data string) (string, error) {
	decoded, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		// Try standard base64 as fallback
		decoded, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			return "", err
		}
	}
	return string(decoded), nil
}

// parseAddressList parses a comma-separated list of email addresses.
func parseAddressList(header string) []string {
	if header == "" {
		return nil
	}
	addrs, err := mail.ParseAddressList(header)
	if err != nil {
		// Fallback: split by comma and extract emails
		parts := strings.Split(header, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if email := extractEmail(strings.TrimSpace(p)); email != "" {
				result = append(result, email)
			}
		}
		return result
	}
	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		result = append(result, addr.Address)
	}
	return result
}

// emailRegex matches email addresses in various formats.
var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

// extractEmail extracts an email address from a string.
func extractEmail(s string) string {
	match := emailRegex.FindString(s)
	return strings.ToLower(match)
}

// ListMessages searches for messages matching a query.
// Returns message IDs that can be used with GetMessage.
//
// Parameters:
//   - query: Gmail search query (e.g., "is:unread after:1704067200")
//   - maxResults: Maximum results to return (0 for default, max 500)
//
// Respects context cancellation and handles rate limiting with exponential backoff.
func (c *GmailClient) ListMessages(ctx context.Context, query string, maxResults int) ([]MessageID, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}
	if maxResults > 500 {
		maxResults = 500
	}

	var allMessages []MessageID
	pageToken := ""

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			if len(allMessages) > 0 {
				return allMessages, ctx.Err()
			}
			return nil, ctx.Err()
		default:
		}

		// Build request URL with parameters
		params := url.Values{}
		params.Set("q", query)
		params.Set("maxResults", strconv.Itoa(maxResults))
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}

		reqURL := fmt.Sprintf("%s/users/me/messages?%s", gmailAPIBase, params.Encode())

		// Execute request with retry
		body, err := c.doRequestWithRetry(ctx, reqURL)
		if err != nil {
			if len(allMessages) > 0 {
				return allMessages, fmt.Errorf("listing messages: %w", err)
			}
			return nil, fmt.Errorf("listing messages: %w", err)
		}

		// Parse response
		var resp messagesListResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return allMessages, fmt.Errorf("parsing messages list response: %w", err)
		}

		allMessages = append(allMessages, resp.Messages...)

		// Check if we have enough results or no more pages
		if len(allMessages) >= maxResults || resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	// Trim to maxResults
	if len(allMessages) > maxResults {
		allMessages = allMessages[:maxResults]
	}

	return allMessages, nil
}

// GetMessage retrieves a full message with headers.
//
// Parameters:
//   - messageID: The message ID from ListMessages
//
// Returns the full message with headers and body.
// Respects context cancellation and handles rate limiting with exponential backoff.
func (c *GmailClient) GetMessage(ctx context.Context, messageID string) (*Message, error) {
	if messageID == "" {
		return nil, fmt.Errorf("message ID required")
	}

	// Build request URL - use full format to get headers and body
	params := url.Values{}
	params.Set("format", "full")

	reqURL := fmt.Sprintf("%s/users/me/messages/%s?%s",
		gmailAPIBase,
		url.PathEscape(messageID),
		params.Encode(),
	)

	// Execute request with retry
	body, err := c.doRequestWithRetry(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("getting message %s: %w", messageID, err)
	}

	// Parse response
	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("parsing message response: %w", err)
	}

	return &msg, nil
}

// BatchGetMessages retrieves multiple messages efficiently.
// Uses concurrent GetMessage calls with bounded concurrency.
//
// Parameters:
//   - messageIDs: List of message IDs to fetch
//
// Returns messages in the same order as messageIDs.
// Errors are returned per-message; a nil message indicates an error at that index.
// This allows partial success - some messages may be retrieved even if others fail.
func (c *GmailClient) BatchGetMessages(ctx context.Context, messageIDs []string) ([]*Message, []error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}

	// Results and errors indexed by position
	results := make([]*Message, len(messageIDs))
	errors := make([]error, len(messageIDs))
	var mu sync.Mutex

	// Use errgroup with bounded concurrency
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxBatchSize)

	for i, msgID := range messageIDs {
		i, msgID := i, msgID // Capture loop variables
		g.Go(func() error {
			msg, err := c.GetMessage(gctx, msgID)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errors[i] = err
			} else {
				results[i] = msg
			}

			// Don't return error - we want all fetches to complete
			// and report individual errors
			return nil
		})
	}

	// Wait for all goroutines to complete.
	// Error is intentionally ignored because individual errors are tracked
	// in the errors slice for partial success support per EFA 0003.
	_ = g.Wait() //nolint:errcheck // Individual errors tracked in errors slice

	return results, errors
}

// doRequestWithRetry executes an HTTP GET request with exponential backoff on 429 responses.
//
// Returns the response body on success, or an error with appropriate sentinel type.
// SECURITY: Access token is used only in Authorization header.
func (c *GmailClient) doRequestWithRetry(ctx context.Context, reqURL string) ([]byte, error) {
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context before making request
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		// Set authorization header
		// SECURITY: Token used only here, never logged
		req.Header.Set("Authorization", "Bearer "+c.credential.Value())
		req.Header.Set("Accept", "application/json")

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("executing request: %w", err)
		}

		// Read body
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck // Ignore close error after read

		if readErr != nil {
			return nil, fmt.Errorf("reading response body: %w", readErr)
		}

		// Handle response status
		switch resp.StatusCode {
		case http.StatusOK:
			return body, nil

		case http.StatusUnauthorized:
			// 401: Re-auth needed
			return nil, fmt.Errorf("%w: access token invalid or expired", datasources.ErrNotAuthenticated)

		case http.StatusForbidden:
			// 403: Permission denied (may need gmail.readonly scope)
			return nil, fmt.Errorf("permission denied (gmail.readonly scope required): %s", string(body))

		case http.StatusNotFound:
			// 404: Message not found
			return nil, fmt.Errorf("message not found: %s", string(body))

		case http.StatusTooManyRequests:
			// 429: Rate limited - retry with backoff
			if attempt < maxRetries {
				// Check for Retry-After header
				if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
					if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
						backoff = time.Duration(seconds) * time.Second
					}
				}

				// Wait before retry
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
					// Double backoff for next attempt
					backoff *= 2
					continue
				}
			}
			return nil, fmt.Errorf("%w: exceeded retry attempts", datasources.ErrRateLimited)

		case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
			// 5xx: Service unavailable - retry with backoff
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
					backoff *= 2
					continue
				}
			}
			return nil, fmt.Errorf("%w: status %d", datasources.ErrServiceUnavailable, resp.StatusCode)

		default:
			// Other error
			return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
		}
	}

	// Should not reach here
	return nil, fmt.Errorf("%w: request failed", datasources.ErrServiceUnavailable)
}
