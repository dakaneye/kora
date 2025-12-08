# Glean AI Integration Specification

**Version:** 1.0
**Status:** Draft
**Date:** 2025-12-08
**Author:** Samuel Dacanay

## Overview

This specification defines a data integration between Glean AI and Kora CLI for accessing Slack, Google Calendar, and Gmail data through a Google Sheets intermediary. This architecture addresses company restrictions that prevent direct API access to these services.

### Problem Statement

The user cannot access Slack, Google Calendar, or Gmail APIs directly due to company security restrictions. However, Glean AI (an enterprise search tool) has authorized access to these services and can export data to Google Sheets.

### Solution Architecture

```
                                   ┌─────────────────────────────────┐
                                   │         GLEAN AI AGENT          │
                                   │   "Kora Morning Digest Agent"   │
                                   │                                 │
                                   │   Schedule: Daily @ 6:00 AM     │
                                   │                                 │
                                   │   Searches:                     │
                                   │   1. Slack @mentions            │
                                   │   2. Calendar events            │
                                   │   3. Gmail unread               │
                                   │                                 │
                                   │   Aggregates & Exports          │
                                   └────────────────┬────────────────┘
                                                    │
                                                    │ Writes to
                                                    ▼
                                   ┌─────────────────────────────────┐
                                   │       GOOGLE SHEETS             │
                                   │    "kora-digest" Spreadsheet    │
                                   │                                 │
                                   │   Sheet: "inbox"                │
                                   │                                 │
                                   │   Columns:                      │
                                   │   A: source                     │
                                   │   B: type                       │
                                   │   C: title                      │
                                   │   D: url                        │
                                   │   E: author                     │
                                   │   F: timestamp                  │
                                   │   G: priority                   │
                                   │   H: requires_action            │
                                   │   I: summary                    │
                                   └────────────────┬────────────────┘
                                                    │
                                                    │ Reads from
                                                    ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                    KORA                                          │
│                             (Rich Data Layer)                                    │
│                                                                                  │
│  DataSource: GSheetDataSource                                                    │
│  Location: internal/datasources/gsheet/                                          │
│                                                                                  │
│  • Reads Google Sheet via Sheets API                                            │
│  • Parses rows into Event structs (EFA 0001)                                    │
│  • Validates each event before returning                                        │
│  • Handles metadata row at end of sheet                                         │
└──────────────────────────────────┬──────────────────────────────────────────────┘
                                   │
                                   │ Returns events
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                  CLAUDE                                          │
│                           (Intelligence Layer)                                   │
│                                                                                  │
│  • Filters by relevance                                                         │
│  • Adjusts priority based on user context                                       │
│  • Synthesizes patterns                                                         │
│  • Presents actionable digest                                                   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Glean Agent Configuration

### Agent Identity

| Property | Value |
|----------|-------|
| Name | Kora Morning Digest Agent |
| Description | Aggregates Slack mentions, Calendar events, and Gmail into a Google Sheet for Kora |
| Schedule | Daily at 6:00 AM (user's local timezone) |
| Output | Google Sheet: `kora-digest` (sheet: `inbox`) |

### Agent Workflow

The Glean agent executes a 5-step workflow:

1. **Search Slack** - Find @mentions in last 16 hours
2. **Search Calendar** - Find today's meetings
3. **Search Gmail** - Find unread emails requiring attention
4. **Aggregate** - Combine results, assign priorities, sort
5. **Export** - Write to Google Sheet

---

## Search Prompts

### Step 1: Slack Search Prompt

```
Search Slack for messages where I was @mentioned in the last 16 hours.

Include:
- Direct messages (DMs) where someone sent me a message
- Channel messages where I was @mentioned
- Thread replies where I was @mentioned

Exclude:
- Messages sent by me
- Messages from bots or automated systems (webhooks, integrations)
- Messages in channels I've muted

For each result, extract:
- Source: "slack"
- Type: "mention" for channel mentions, "dm" for direct messages
- Title: First 100 characters of the message
- URL: Link to the message (if available)
- Author: Name of the person who sent it
- Timestamp: When the message was sent (ISO 8601 format)
- Summary: First 200 characters of the message content

Priority assignment:
- Priority 1: Messages containing "urgent", "ASAP", "critical", "blocking", or "emergency"
- Priority 2: Direct messages OR messages that contain a question (? character)
- Priority 3: Thread replies where I'm mentioned
- Priority 5: All other channel mentions

Set requires_action to TRUE if the message contains a question (?) or request words like "can you", "please", "need you to", "could you".
```

### Step 2: Calendar Search Prompt

```
Search Google Calendar for events happening today (from midnight to midnight in my timezone).

Include:
- Meetings where I am an attendee and my response is "accepted" or "tentative" or "needsAction"
- Meetings where I am the organizer
- Events that have a video conference link (Google Meet, Zoom, etc.)

Exclude:
- Events where my response status is "declined"
- Cancelled events
- All-day events that are informational only (holidays, OOO markers)

For each result, extract:
- Source: "calendar"
- Type: "meeting"
- Title: Event title (max 100 characters)
- URL: Google Calendar event link or video conference link
- Author: Organizer name
- Timestamp: Event start time (ISO 8601 format)
- Summary: Event description first 200 characters, or attendee list if no description

Priority assignment:
- Priority 1: 1:1 meetings (only 2 attendees including me)
- Priority 2: Meetings where I am the organizer, OR daily standups/syncs
- Priority 3: Small meetings (3-5 attendees)
- Priority 4: Medium meetings (6-10 attendees)
- Priority 5: Large meetings (>10 attendees) or optional meetings

Set requires_action to:
- TRUE if my response status is "needsAction" (haven't responded yet)
- TRUE if I am the organizer and there are pending RSVPs
- FALSE otherwise
```

### Step 3: Gmail Search Prompt

```
Search Gmail using this exact query:

is:unread in:inbox -category:promotions -category:social -category:updates -from:noreply -from:no-reply -from:notifications -from:donotreply -from:automated

Additional filters to apply:
- Only include emails received in the last 16 hours
- Exclude emails from mailing lists (has List-Unsubscribe header)
- Exclude emails where I am only in CC (unless from VIP senders)

For each result, extract:
- Source: "gmail"
- Type: "email"
- Title: Email subject line (max 100 characters)
- URL: Gmail web link to the email
- Author: Sender name and email
- Timestamp: Email received time (ISO 8601 format)
- Summary: Email snippet/preview (first 200 characters)

Priority assignment:
- Priority 1: Emails from starred/VIP contacts OR with Gmail "Important" marker AND I'm in To: field
- Priority 2: Emails where I am directly in the To: field from internal company domain AND unread
- Priority 3: Emails where I am in CC: from internal company domain
- Priority 4: Emails from external senders where I'm in To: field
- Priority 5: All other qualifying emails

Set requires_action to:
- TRUE if I am in the To: field (directly addressed)
- TRUE if the subject contains "action required", "response needed", "please review"
- FALSE if I am only in CC
```

### Step 4: Aggregation Prompt

```
Combine all results from Slack, Calendar, and Gmail searches into a single list.

Sorting rules:
1. Primary sort: Priority (1 first, then 2, then 3, etc.)
2. Secondary sort: Timestamp (most recent first within same priority)

Deduplication rules:
- If the same item appears from multiple searches, keep only one entry
- For duplicates, keep the entry with the higher priority (lower number)

Output format:
Create a list where each item has these fields in order:
1. source (string): "slack", "calendar", or "gmail"
2. type (string): "mention", "dm", "meeting", or "email"
3. title (string): Max 100 characters, truncate with "..." if longer
4. url (string or empty): Direct link to the item, or empty string if unavailable
5. author (string or empty): Person who sent/created it, or empty for calendar
6. timestamp (string): ISO 8601 format (e.g., "2025-12-08T09:00:00Z")
7. priority (integer): 1-5 where 1 is highest priority
8. requires_action (boolean): TRUE or FALSE
9. summary (string): Max 200 characters, truncate with "..." if longer

Limit: Maximum 50 items total. If more than 50, keep only the 50 highest priority items.
```

### Step 5: Google Sheet Export Prompt

```
Write the aggregated results to a Google Sheet.

Sheet configuration:
- Spreadsheet name: "kora-digest"
- Sheet/tab name: "inbox"

Before writing:
1. Clear all existing data in the "inbox" sheet (preserve headers in row 1)
2. Write the header row if it doesn't exist:
   A1: source
   B1: type
   C1: title
   D1: url
   E1: author
   F1: timestamp
   G1: priority
   H1: requires_action
   I1: summary

Data writing:
- Start writing data from row 2
- One item per row
- Write values in columns A through I matching the header order
- For boolean requires_action: write "TRUE" or "FALSE" as text

Metadata row:
After writing all data rows, add a metadata row at the end with:
- Column A: ISO 8601 timestamp of when this export was generated
- Column B: Total count of items written
- Leave other columns empty

Example:
If 15 items were exported at 6:00 AM on Dec 8, 2025:
- Row 17 (after 15 data rows + header) would be:
  - A17: "2025-12-08T06:00:00Z"
  - B17: "15"
```

---

## Data Schema

### Google Sheet Schema

The `kora-digest` spreadsheet `inbox` sheet has this structure:

| Column | Field | Type | Description | EFA 0001 Mapping |
|--------|-------|------|-------------|------------------|
| A | source | string | "slack", "calendar", or "gmail" | Event.Source |
| B | type | string | "mention", "dm", "meeting", or "email" | Maps to EventType |
| C | title | string | Max 100 chars | Event.Title |
| D | url | string/empty | Link to original item | Event.URL |
| E | author | string/empty | Who created/sent it | Event.Author.Username |
| F | timestamp | string | ISO 8601 format | Event.Timestamp |
| G | priority | int | 1-5 (1=critical) | Event.Priority |
| H | requires_action | boolean | TRUE/FALSE as text | Event.RequiresAction |
| I | summary | string | Max 200 chars | Event.Metadata["summary"] |

### Metadata Row

The last row contains export metadata:

| Column | Content | Example |
|--------|---------|---------|
| A | generated_at (ISO 8601) | "2025-12-08T06:00:00Z" |
| B | total_count (string) | "15" |
| C-I | Empty | - |

### Type Mapping to EFA 0001 EventTypes

| Sheet Type | Sheet Source | EFA 0001 EventType |
|------------|--------------|-------------------|
| "mention" | "slack" | `slack_mention` |
| "dm" | "slack" | `slack_dm` |
| "meeting" | "calendar" | `calendar_meeting` |
| "email" | "gmail" | `email_direct` or `email_cc` |

**Note**: The GSheet datasource will map sheet types to the closest EFA 0001 EventType. For emails, if `requires_action=TRUE`, map to `email_direct`; otherwise map to `email_cc`.

---

## Priority Rules

### Slack Priority Assignment

| Condition | Priority | RequiresAction |
|-----------|----------|----------------|
| Contains urgent keywords ("urgent", "ASAP", "critical", "blocking", "emergency") | 1 | TRUE |
| Direct message (DM) | 2 | TRUE |
| Contains question mark (?) | 2 | TRUE |
| Thread reply where mentioned | 3 | FALSE |
| Other channel mentions | 5 | Depends on content |

**Urgent Keywords** (case-insensitive):
- urgent
- ASAP
- critical
- blocking
- emergency
- blocker

**Request Patterns** (set requires_action=TRUE):
- Contains "?"
- Contains "can you"
- Contains "please"
- Contains "need you to"
- Contains "could you"
- Contains "would you"

### Calendar Priority Assignment

| Condition | Priority | RequiresAction |
|-----------|----------|----------------|
| 1:1 meeting (2 attendees) | 1 | Based on RSVP |
| User is organizer | 2 | TRUE if pending RSVPs |
| Daily standup/sync | 2 | FALSE |
| Small meeting (3-5 attendees) | 3 | Based on RSVP |
| Medium meeting (6-10 attendees) | 4 | Based on RSVP |
| Large meeting (>10 attendees) | 5 | Based on RSVP |
| Optional meeting | 5 | Based on RSVP |

**RSVP-based requires_action**:
- `needsAction` response status -> TRUE
- `accepted` or `tentative` -> FALSE
- Organizer with pending RSVPs -> TRUE

### Gmail Priority Assignment

| Condition | Priority | RequiresAction |
|-----------|----------|----------------|
| Starred/VIP contact + Important marker + To: field | 1 | TRUE |
| Internal domain + To: field + unread | 2 | TRUE |
| Internal domain + CC: field | 3 | FALSE |
| External sender + To: field | 4 | TRUE |
| Other qualifying emails | 5 | FALSE |

**Action Keywords in Subject** (set requires_action=TRUE):
- "action required"
- "response needed"
- "please review"
- "approval needed"
- "decision needed"

---

## Kora GSheet DataSource Implementation

### Directory Structure

```
internal/datasources/gsheet/
├── gsheet.go           # DataSource implementation
├── gsheet_test.go      # Unit tests
├── parser.go           # Row parsing logic
├── parser_test.go      # Parser tests
└── testdata/
    ├── valid_sheet.json        # Test fixture
    ├── empty_sheet.json        # Empty sheet test
    └── malformed_sheet.json    # Error handling test
```

### DataSource Interface Implementation

```go
// Package gsheet provides a DataSource that reads events from a Google Sheet.
// Ground truth for data pipeline defined in specs/glean-datasource.md
//
// This datasource reads from a Google Sheet populated by a Glean AI agent.
// It is NOT a direct integration with Slack/Calendar/Gmail APIs.
package gsheet

import (
    "context"
    "github.com/dakaneye/kora/internal/datasources"
    "github.com/dakaneye/kora/internal/models"
)

// GSheetDataSource reads events from a Google Sheet populated by Glean AI.
// IT IS FORBIDDEN TO CHANGE THIS to directly call Slack/Calendar/Gmail APIs.
type GSheetDataSource struct {
    spreadsheetID string
    sheetName     string
    authProvider  auth.AuthProvider // Google OAuth
}

func (d *GSheetDataSource) Name() string {
    return "gsheet-glean"
}

func (d *GSheetDataSource) Service() models.Source {
    // Return source based on row data, not a single source
    // This datasource aggregates multiple sources
    return models.SourceGSheet // NEW: Add to EFA 0001
}

func (d *GSheetDataSource) Fetch(ctx context.Context, opts datasources.FetchOptions) (*datasources.FetchResult, error) {
    // Implementation:
    // 1. Authenticate with Google Sheets API
    // 2. Read all rows from sheet
    // 3. Parse each row into Event struct
    // 4. Skip metadata row (last row)
    // 5. Filter by opts.Since
    // 6. Validate each event
    // 7. Return FetchResult
}
```

### Configuration

Add to `~/.kora/config.yaml`:

```yaml
datasources:
  gsheet:
    enabled: true
    spreadsheet_id: "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"  # Example
    sheet_name: "inbox"

google:
  # Shared OAuth credentials (also used by Calendar/Gmail if direct access later)
  credentials_file: ~/.kora/google-credentials.json
```

### Row Parsing Logic

```go
// parseRow converts a Google Sheet row to a models.Event
func parseRow(row []interface{}) (*models.Event, error) {
    if len(row) < 9 {
        return nil, fmt.Errorf("row has %d columns, expected 9", len(row))
    }

    // Parse source and map to models.Source
    source, err := parseSource(row[0])
    if err != nil {
        return nil, err
    }

    // Parse type and map to models.EventType
    eventType, err := parseEventType(row[1], source)
    if err != nil {
        return nil, err
    }

    // Parse timestamp
    timestamp, err := time.Parse(time.RFC3339, toString(row[5]))
    if err != nil {
        return nil, fmt.Errorf("invalid timestamp: %w", err)
    }

    // Parse priority
    priority, err := strconv.Atoi(toString(row[6]))
    if err != nil || priority < 1 || priority > 5 {
        return nil, fmt.Errorf("invalid priority: %s", toString(row[6]))
    }

    // Parse requires_action
    requiresAction := strings.ToUpper(toString(row[7])) == "TRUE"

    event := &models.Event{
        Type:           eventType,
        Title:          truncate(toString(row[2]), 100),
        Source:         source,
        URL:            toString(row[3]),
        Author: models.Person{
            Username: toString(row[4]),
            Name:     toString(row[4]), // Use author as both name and username
        },
        Timestamp:      timestamp,
        Priority:       models.Priority(priority),
        RequiresAction: requiresAction,
        Metadata: map[string]interface{}{
            "summary":       toString(row[8]),
            "glean_source":  toString(row[0]), // Original source before mapping
        },
    }

    return event, nil
}
```

### Type Mapping Functions

```go
func parseSource(val interface{}) (models.Source, error) {
    source := strings.ToLower(toString(val))
    switch source {
    case "slack":
        return models.SourceSlack, nil
    case "calendar":
        return models.SourceGoogleCalendar, nil
    case "gmail":
        return models.SourceGmail, nil
    default:
        return "", fmt.Errorf("unknown source: %s", source)
    }
}

func parseEventType(val interface{}, source models.Source) (models.EventType, error) {
    typeStr := strings.ToLower(toString(val))

    switch source {
    case models.SourceSlack:
        switch typeStr {
        case "dm":
            return models.EventTypeSlackDM, nil
        case "mention":
            return models.EventTypeSlackMention, nil
        default:
            return models.EventTypeSlackMention, nil
        }

    case models.SourceGoogleCalendar:
        return models.EventTypeCalendarMeeting, nil

    case models.SourceGmail:
        // Will be adjusted based on requires_action in parseRow
        return models.EventTypeEmailDirect, nil

    default:
        return "", fmt.Errorf("cannot map type %s for source %s", typeStr, source)
    }
}
```

### Metadata Row Detection

```go
// isMetadataRow checks if this is the final metadata row
func isMetadataRow(row []interface{}) bool {
    if len(row) < 2 {
        return false
    }

    // Metadata row has timestamp in A and count in B
    // Check if column A looks like an ISO timestamp and B is numeric
    aStr := toString(row[0])
    bStr := toString(row[1])

    _, errTime := time.Parse(time.RFC3339, aStr)
    _, errNum := strconv.Atoi(bStr)

    // It's a metadata row if A is a valid timestamp, B is numeric,
    // and there's nothing substantial in other columns
    return errTime == nil && errNum == nil && len(row) <= 2
}
```

---

## EFA 0001 Updates Required

This integration requires adding a new Source constant:

```go
// Add to models/source.go
const (
    // ... existing sources ...
    SourceGSheet Source = "gsheet"  // Google Sheet aggregator
)

// Update validSources map
var validSources = map[Source]struct{}{
    // ... existing sources ...
    SourceGSheet: {},
}
```

**Note**: Events from the GSheet datasource will have their `Source` field set to the original source (Slack, Calendar, Gmail) from the sheet, not `gsheet`. The `gsheet` source is only used for datasource identification, not event sourcing.

### New Metadata Keys for GSheet

Add to `allowedMetadataKeys[SourceSlack]`, `allowedMetadataKeys[SourceGoogleCalendar]`, and `allowedMetadataKeys[SourceGmail]`:

```go
// Common metadata key for GSheet-sourced events
"summary":      {},  // Summary text from Glean
"glean_source": {},  // Indicates data came through Glean pipeline
```

---

## Authentication

### Google OAuth Credential Sharing

The GSheet datasource uses the same Google OAuth credentials as the planned Google Calendar and Gmail direct integrations:

```yaml
# ~/.kora/config.yaml
google:
  credentials_file: ~/.kora/google-credentials.json
  token_file: ~/.kora/google-token.json
  scopes:
    - https://www.googleapis.com/auth/spreadsheets.readonly
```

**Credential Flow**:
1. User runs `kora auth google` to authenticate
2. OAuth flow stores token in `~/.kora/google-token.json`
3. GSheet datasource uses stored token for Sheets API access
4. Token refresh handled by GoogleAuthProvider (EFA 0002)

### Sheet Sharing

The Google Sheet must be shared with the user's Google account:
1. Create the `kora-digest` sheet in Google Sheets
2. Share with the personal Google account used for Kora auth
3. Grant "Viewer" permission (read-only is sufficient)

---

## Setup Instructions

### Step 1: Create Google Sheet

1. Open Google Sheets (sheets.google.com)
2. Create a new spreadsheet named `kora-digest`
3. Create a sheet/tab named `inbox`
4. Add header row in row 1:
   ```
   A1: source
   B1: type
   C1: title
   D1: url
   E1: author
   F1: timestamp
   G1: priority
   H1: requires_action
   I1: summary
   ```
5. Note the spreadsheet ID from the URL:
   ```
   https://docs.google.com/spreadsheets/d/[SPREADSHEET_ID]/edit
   ```

### Step 2: Share Sheet with Personal Account

1. Click "Share" in Google Sheets
2. Add your personal email address (the one you'll use with Kora)
3. Set permission to "Viewer"
4. Click "Send"

### Step 3: Create Glean Agent

1. Open Glean AI (glean.com or your company's Glean instance)
2. Navigate to Agents/Assistants section
3. Create new agent with:
   - **Name**: "Kora Morning Digest Agent"
   - **Schedule**: Daily at 6:00 AM
4. Add the 5 workflow steps using the prompts from this specification
5. Configure output to write to your `kora-digest` spreadsheet

### Step 4: Test Glean Agent

1. Run the agent manually to verify it works
2. Check the Google Sheet for populated data
3. Verify data format matches specification
4. Check timestamps are in ISO 8601 format
5. Verify priorities are integers 1-5

### Step 5: Configure Kora

1. Authenticate with Google:
   ```bash
   kora auth google
   ```

2. Add GSheet datasource to config:
   ```bash
   # Edit ~/.kora/config.yaml
   datasources:
     gsheet:
       enabled: true
       spreadsheet_id: "YOUR_SPREADSHEET_ID_HERE"
       sheet_name: "inbox"
   ```

3. Test the datasource:
   ```bash
   kora digest --since 24h --format json
   ```

### Step 6: Verify Integration

Run a digest and verify events from the sheet appear:

```bash
$ kora digest --since 16h --format terminal

Morning Digest - December 8, 2025 9:00 AM PST
==================================================

Priority 1 - Requires Action (2 items)
--------------------------------------------------
[Slack DM] Question about deployment timeline
  Author: @bobmanager • 4 hours ago
  Summary: Can you help me understand the deployment...

[Email] Q4 Planning Meeting Required
  Author: vp@company.com • 5 hours ago
  Summary: We need to finalize Q4 priorities...

Priority 2 - High Priority (3 items)
--------------------------------------------------
[Calendar] 1:1 with Alice
  Starts: 10:00 AM • 30 minutes
  Summary: Weekly sync

...
```

---

## Setup Checklist

```
[ ] Google Sheet Setup
    [ ] Create "kora-digest" spreadsheet
    [ ] Create "inbox" sheet/tab
    [ ] Add header row (source, type, title, url, author, timestamp, priority, requires_action, summary)
    [ ] Note spreadsheet ID from URL
    [ ] Share sheet with personal Google account

[ ] Glean Agent Setup
    [ ] Create new Glean agent
    [ ] Name: "Kora Morning Digest Agent"
    [ ] Configure daily schedule at 6:00 AM
    [ ] Add Step 1: Slack search prompt
    [ ] Add Step 2: Calendar search prompt
    [ ] Add Step 3: Gmail search prompt
    [ ] Add Step 4: Aggregation prompt
    [ ] Add Step 5: Google Sheet export prompt
    [ ] Test agent manually
    [ ] Verify data appears in sheet
    [ ] Verify format matches specification

[ ] Kora Configuration
    [ ] Run: kora auth google
    [ ] Complete OAuth flow
    [ ] Edit ~/.kora/config.yaml
    [ ] Add gsheet datasource configuration
    [ ] Set correct spreadsheet_id
    [ ] Set sheet_name to "inbox"

[ ] Integration Testing
    [ ] Run: kora digest --since 24h --format json
    [ ] Verify events from sheet appear
    [ ] Check source mapping (slack, calendar, gmail)
    [ ] Check type mapping to EFA 0001 EventTypes
    [ ] Check priority values (1-5)
    [ ] Check requires_action boolean
    [ ] Verify timestamps parsed correctly
    [ ] Verify metadata includes summary
```

---

## Implementation Notes

### Rate Limits

Google Sheets API has generous limits (500 requests per 100 seconds per project). With daily fetches, rate limiting is not a concern for this use case.

### Error Handling

| Error Condition | Handling |
|-----------------|----------|
| Sheet not found | Return `ErrServiceUnavailable` |
| Sheet empty | Return empty `FetchResult` (not an error) |
| Malformed row | Log warning, skip row, continue |
| Invalid timestamp | Log warning, skip row, continue |
| Auth failure | Return `ErrNotAuthenticated` |

### Partial Success

If some rows fail to parse but others succeed, return the successfully parsed events with `Partial: true` in the `FetchResult`.

### Freshness

The GSheet datasource is inherently delayed (Glean runs at 6 AM). Events will be up to 24 hours old when first fetched. This is acceptable for the morning digest use case.

For fresher data, consider:
1. Running Glean agent more frequently
2. Implementing direct Slack/Calendar/Gmail APIs when company restrictions allow

---

## Security Considerations

### Data Flow Security

- Glean AI operates within company security boundary
- Google Sheet is access-controlled via sharing
- Kora authenticates via Google OAuth
- No credentials stored in the sheet itself

### Sensitive Data

The sheet may contain:
- Email subjects and snippets
- Slack message previews
- Calendar event titles
- People's names

Ensure:
- Sheet is not publicly shared
- Only user's personal account has access
- Kora follows credential security practices (EFA 0002)

### Credential Management

Google OAuth tokens follow EFA 0002 guidelines:
- Never log tokens
- Store in secure token file
- Token refresh handled automatically
- Delegation to OS credential store where possible

---

## Limitations

### Known Limitations

1. **Staleness**: Data is only as fresh as last Glean run (daily by default)
2. **No real-time**: Cannot detect new mentions between Glean runs
3. **Glean dependency**: Requires working Glean AI access
4. **Sheet structure**: Fixed schema; changes require spec update
5. **Single user**: Sheet is per-user; no multi-user support

### Future Improvements

If company restrictions are lifted, consider:
1. Direct Slack API integration (real-time)
2. Direct Google Calendar API (fresher data)
3. Direct Gmail API (unread state accuracy)

These would provide:
- Real-time updates
- More accurate metadata
- Richer event types
- Better deduplication

---

## References

- **EFA 0001**: Event Model Ground Truth
- **EFA 0002**: Auth Provider Interface
- **EFA 0003**: DataSource Interface
- **EFA 0004**: Tool Responsibility and Separation of Concerns
- **specs/architecture.md**: Kora Architecture
- **specs/google-oauth-flow.md**: Google OAuth implementation
- **specs/google-datasources.md**: Google datasource patterns
