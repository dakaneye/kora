# Gmail Integration Research: Morning Digest Tool

## Key Facts

- **Auth**: OAuth 2.0 with user consent (Desktop App flow for CLI)
- **Rate Limit**: 250 units/sec, effectively unlimited for digest use
- **Primary Query**: `is:unread OR is:important newer_than:16h`
- **Recommended Scope**: `gmail.readonly` (read-only, cannot modify)

## Dependencies

- Google Cloud Project with OAuth 2.0 credentials
- OAuth consent via browser flow
- Refresh token storage (access tokens expire after 1 hour)
- Google Workspace: check admin console for third-party app restrictions

## Implementation Notes

- Use `metadata` format for speed (headers only, no body)
- Query syntax identical to Gmail web search
- `messages.list` returns IDs only, then batch fetch details
- Handle multipart MIME and base64 encoding for body content
- Standard label IDs: UNREAD, IMPORTANT, INBOX, STARRED

## Decision Points

1. Metadata only or full message body?
2. Time window: overnight only or include older unread?
3. Which labels to track? (Inbox only? Include starred?)
4. Token storage location: file, keychain, or env var?

---

## Overview

Research findings for integrating Gmail into a morning digest tool, assuming Google Workspace (enterprise) account with direct user authentication.

## Authentication

### OAuth 2.0 for Google Workspace Users

**Requirements:**
- Google Cloud Project (can be personal or org-provided)
- OAuth 2.0 credentials (Desktop app type for CLI)
- User consent via browser-based OAuth flow

### Enterprise/Workspace Considerations

- **Domain-wide delegation**: Not needed for personal access
- **Admin console**: Check if third-party apps are allowed
- **App approval**: Some orgs require admin approval for OAuth apps
- **Audit logging**: Google Workspace logs all API access

### Required OAuth Scopes

**Minimal (Recommended):**
```
https://www.googleapis.com/auth/gmail.readonly
```
- Read all email content
- Read labels and metadata
- Cannot modify or send

**Metadata Only (More Restricted):**
```
https://www.googleapis.com/auth/gmail.metadata
```
- Read headers only (From, Subject, Date)
- Read labels
- Cannot read message body

### OAuth Flow for CLI Tools

1. **Desktop App Flow**
   - Register as "Desktop application" in Google Cloud Console
   - Opens browser for user consent
   - Localhost callback receives auth code
   - Exchange code for access + refresh tokens

2. **Token Storage**
   - Access token expires (1 hour default)
   - Refresh token used to get new access tokens
   - Store refresh token securely (file or keychain)

### Google Workspace Restrictions

- Some orgs block third-party app access
- May require app to be on "trusted" list
- Check admin console: Security > API controls
- Contact IT if OAuth consent is blocked

## Key API Methods

### 1. messages.list

**Purpose:** List messages matching query
**Quota:** 5 units per request

**Query Parameters:**
- `q` - Gmail search query (same syntax as Gmail web)
- `maxResults` - Max messages per page (default 100)
- `pageToken` - Pagination token
- `labelIds` - Filter by label

**Returns:**
- Message IDs only (not full content)
- Pagination token for next page
- Result size estimate

### 2. messages.get

**Purpose:** Get full message details
**Quota:** 5 units per request

**Format Options:**
- `full` - Complete message with body
- `metadata` - Headers only (faster)
- `minimal` - Just ID and labels

**Metadata Headers Available:**
- From, To, Cc, Bcc
- Subject
- Date
- Message-ID
- Reply-To

### 3. labels.list

**Purpose:** Get all labels for the user
**Use:** Map label IDs to names (IMPORTANT, INBOX, etc.)

### 4. threads.list / threads.get

**Purpose:** Get conversation threads
**Use:** Group related messages together

## Gmail Query Syntax

### Date Filters
```
after:2025/12/04           # After specific date
after:1733356800           # After Unix timestamp
newer_than:1d              # Within last day
newer_than:2h              # Within last 2 hours
before:2025/12/05          # Before date
```

### Read Status
```
is:unread                  # Unread messages
is:read                    # Read messages
```

### Importance/Priority
```
is:important               # Gmail's importance markers
is:starred                 # Starred messages
has:yellow-star            # Specific star color
label:important            # Important label
```

### Sender Filters
```
from:sender@example.com    # Specific sender
from:*@company.com         # Domain wildcard
from:me                    # Sent by you
to:me                      # Sent to you
```

### Subject/Content
```
subject:"meeting notes"    # Subject contains
"exact phrase"             # Body or subject contains
```

### Labels/Location
```
in:inbox                   # Inbox messages
in:sent                    # Sent messages
label:work                 # Custom label
-label:promotions          # Exclude label
```

### Attachments
```
has:attachment             # Has any attachment
filename:pdf               # Has PDF attachment
filename:*.xlsx            # Has Excel files
larger:10M                 # Larger than 10MB
```

### Combined Examples
```
is:unread is:important after:2025/12/04
from:boss@company.com is:unread newer_than:1d
in:inbox -label:promotions -label:social newer_than:12h
has:attachment from:*@vendor.com newer_than:1d
```

## Rate Limits

### Quota Units
- **Daily limit:** 1 billion units per day (effectively unlimited)
- **Per-user rate:** 250 units per second

### Cost per Operation
| Operation | Units |
|-----------|-------|
| messages.list | 5 |
| messages.get | 5 |
| labels.list | 1 |
| threads.list | 5 |

### Typical Digest Load
- 1-2 messages.list calls (with query filters): 10 units
- 20-50 messages.get calls (metadata format): 100-250 units
- **Total:** ~300 units, negligible vs limits

### Rate Limit Handling
- 429 errors include `Retry-After` header
- Use exponential backoff
- Rate limiting rarely triggered for digest use case

## Data Available

### Message Object
- `id` - Unique message ID
- `threadId` - Conversation thread ID
- `labelIds` - Applied labels (UNREAD, IMPORTANT, etc.)
- `snippet` - Preview text (~100 chars)
- `payload` - Headers and body parts
- `internalDate` - Timestamp (milliseconds)

### Standard Label IDs
- `INBOX` - Inbox
- `UNREAD` - Unread status
- `IMPORTANT` - Importance marker
- `STARRED` - Starred
- `SENT` - Sent mail
- `DRAFT` - Drafts
- `SPAM` - Spam folder
- `TRASH` - Trash
- `CATEGORY_PERSONAL` - Personal tab
- `CATEGORY_SOCIAL` - Social tab
- `CATEGORY_PROMOTIONS` - Promotions tab
- `CATEGORY_UPDATES` - Updates tab

### Headers Available
- From (may include display name)
- To, Cc, Bcc
- Subject
- Date (RFC 2822 format)
- Message-ID
- In-Reply-To
- References

## Important Gotchas

### 1. Google Workspace Policies
- Admin may block third-party API access
- App may need to be "trusted" in admin console
- Check Security > API Controls in admin console
- DLP policies may restrict data export

### 2. Two-Factor Authentication
- OAuth still works with 2FA enabled
- User authenticates via normal Google login
- No special handling needed

### 3. Message Body Encoding
- Body can be multipart MIME
- May need to decode base64
- Handle both text/plain and text/html parts

### 4. Thread vs Message
- Gmail groups messages into threads
- Same subject = same thread (usually)
- `threadId` groups related messages
- Consider showing thread-level summary

### 5. Label vs Query
- `labelIds` filter is exact match
- Query `label:name` supports custom labels
- Some labels only via labelIds (UNREAD, IMPORTANT)

### 6. Pagination
- Use `pageToken` for subsequent requests
- Don't assume order without explicit sort
- `resultSizeEstimate` is approximate

### 7. Metadata vs Full Format
- `metadata` format much faster
- Only returns headers, not body
- Sufficient for digest (subject, from, date)
- Use `full` only if body preview needed

### 8. Time Zone Handling
- `internalDate` is UTC milliseconds
- `Date` header may have sender's timezone
- Parse carefully for filtering

## Recommended Approach

### For Morning Digest

1. **Primary: Unread + Important**
   - Query: `is:unread OR is:important newer_than:16h`
   - Covers overnight emails

2. **Format: Metadata**
   - Use `metadata` format for speed
   - Request headers: From, Subject, Date

3. **Batch Fetching**
   - List message IDs first
   - Fetch details in parallel (respect rate limits)

4. **Priority Display**
   - Important emails first
   - Then unread by time
   - Limit to top 20-30

### Data to Collect
- Sender (name and email)
- Subject line
- Timestamp
- Read/unread status
- Important flag
- Snippet (preview text)

## Go Libraries

### google.golang.org/api/gmail/v1
- Official Google client library
- Complete API coverage
- Handles auth and pagination

### golang.org/x/oauth2
- Standard OAuth 2.0 implementation
- Token refresh handling
- Works with Google OAuth endpoints

### Alternative: Direct HTTP
- Use Google's REST API directly
- More control, fewer dependencies
- Must implement OAuth flow manually

## Security Considerations

### Token Storage
- Store refresh token securely
- Use file with 0600 permissions
- Consider system keychain (macOS Keychain, GNOME Keyring)
- Never commit tokens to version control

### Credential Management
- OAuth client credentials are semi-public (embedded in CLI)
- Secret is client-side (no true secret for desktop apps)
- Security through OAuth flow, not client secret

### Minimal Scope
- Use `gmail.readonly` not `gmail.modify`
- Prevents accidental email changes
- Reduces security risk if token compromised

### Workspace Audit Logs
- All API access is logged
- Visible to workspace admins
- No privacy expectation for work email

## Integration with Digest Tool

### Outputs Needed
- Total unread count
- Important email count
- List of recent unread emails
- Sender, subject, timestamp for each

### Priority Ordering
1. Important + Unread
2. Important + Read (recent)
3. Unread (non-important)

---

**Document Status**: Research complete
**Last Updated**: 2025-12-05
**Auth Type**: OAuth 2.0 user consent (Google Workspace)
