# Slack Integration Research: Morning Digest Tool

## Key Facts

- **Auth**: User Token (xoxp-*) required, NOT Bot Token
- **Rate Limit**: Tier 3 (~50 req/min) for search.messages and conversations.history
- **Primary Query**: `search.messages` with `<@USER_ID> after:YYYY-MM-DD`
- **Critical Scope**: `search:read` (only way to efficiently find mentions)

## Dependencies

- Slack App with OAuth flow (api.slack.com)
- User token with proper scopes (channels:history, im:history, search:read)
- Enterprise Grid: may require admin approval for OAuth apps

## Implementation Notes

- Cannot search DM content via `search.messages` - must list DMs then fetch history for each
- Thread replies NOT in regular channel history - use `conversations.replies`
- User tokens don't expire automatically but can be revoked
- Enterprise Grid may require org-level token for cross-workspace search
- Parse mrkdwn format: `<@U12345|displayname>` for mentions, `<#C12345|name>` for channels

## Decision Points

1. Token storage: keychain, encrypted file, or environment variable?
2. Multiple workspaces? (Enterprise Grid consideration)
3. Include @channel/@here mentions or just direct @mentions?
4. Thread expansion: fetch full thread context for mentions in threads?

---

## Overview

Research findings for integrating Slack mentions and messages into a morning digest tool, assuming enterprise/corporate Slack workspace with direct user authentication.

## Authentication

### User Token Required (Not Bot Token)

For a personal morning digest accessing your own messages and mentions:

**User Token (xoxp-*)** - REQUIRED
- Acts on behalf of the authenticated user
- Can see all channels/DMs the user has access to
- Can search for mentions across workspace
- Can access private channels user is in
- Works with Enterprise Grid

**Why NOT Bot Token:**
- Cannot see DMs to users (only DMs to the bot)
- Cannot use `search.messages` (requires user token)
- Limited to channels bot is invited to
- Not suitable for personal digest use case

### Enterprise Workspace Considerations

- Enterprise Grid uses org-level tokens vs workspace tokens
- User tokens work across the entire Enterprise Grid org
- Admin-approved apps may be required
- Check with IT for app installation policies
- Some workspaces require admin consent for OAuth scopes

### Required OAuth Scopes (User Token)

```
channels:history       # Read messages in public channels
channels:read          # View basic info about public channels
groups:history         # Read messages in private channels
groups:read            # View basic info about private channels
im:history             # Read DM message history
im:read                # View DMs
mpim:history           # Read group DM history
mpim:read              # View group DMs
search:read            # Search messages and files (CRITICAL for mentions)
users:read             # View people in workspace
```

**Note:** `search:read` is essential - it's the only efficient way to find mentions.

### Token Acquisition Options

1. **Slack App with OAuth Flow**
   - Create app at api.slack.com
   - Request user token scopes
   - User authorizes via OAuth flow
   - Receive and store user token

2. **Legacy Token (Deprecated)**
   - No longer available for new apps
   - Existing legacy tokens still work

3. **Enterprise Grid: Org-Level Tokens**
   - Requires admin approval
   - Works across all workspaces in org

## Key API Methods

### 1. search.messages (Primary for Mentions)

**Purpose:** Find all messages where user is mentioned
**Tier:** 3 (20+ requests/minute)
**Pagination:** Page-based (not cursor)

**Query Syntax:**
- `<@USERID>` - Direct user mention
- `@channel` - Channel-wide mention (if user in channel)
- `@here` - Active users mention
- `after:YYYY-MM-DD` - Date filter

**Capabilities:**
- Search across all accessible channels
- Filter by date range
- Returns permalink to each message
- Includes channel context

**Limitations:**
- Max 100 results per page
- Max 1000 total results
- Cannot search DMs directly (use conversations.history)

### 2. conversations.history

**Purpose:** Get messages from specific channels/DMs
**Tier:** 3 (50+ requests/minute)
**Pagination:** Cursor-based

**Capabilities:**
- Filter by time range (oldest/latest Unix timestamps)
- Returns up to 200 messages per request
- Works for channels, DMs, group DMs
- Includes thread metadata

**Use Cases:**
- Get DM messages since specific time
- Get messages in specific channels
- Find unread messages (compare with last_read)

### 3. users.conversations

**Purpose:** List all channels/DMs user is a member of
**Tier:** 3

**Capabilities:**
- Filter by conversation type (channel, im, mpim, group)
- Exclude archived channels
- Returns channel metadata

**Use Cases:**
- Get list of DM conversations to check
- Find all channels user participates in

### 4. conversations.info

**Purpose:** Get channel details including last_read timestamp
**Tier:** 3

**Capabilities:**
- Get `last_read` timestamp for unread calculation
- Get channel metadata (name, topic, purpose)
- Check if user is member

### 5. conversations.replies

**Purpose:** Get thread replies
**Tier:** 3

**Note:** Thread replies don't appear in regular history - must fetch separately.

## Rate Limits

### Tier System

| Tier | Rate | Common Methods |
|------|------|----------------|
| 1 | ~1/min | chat.postMessage |
| 2 | ~20/min | Light reads |
| 3 | ~50/min | search.messages, conversations.history |
| 4 | ~100/min | Posting messages |

### Handling Rate Limits

- Check `X-Rate-Limit-Remaining` header
- On 429: Wait for `Retry-After` seconds
- Use exponential backoff
- Stay at 80% of stated limit for safety

### Typical Digest Load

- 1 search.messages call (mentions)
- 10-20 conversations.history calls (DMs)
- 5-10 conversations.info calls
- **Total:** ~30-40 requests, well within limits

## Data Available

### Message Object

- `ts` - Timestamp (unique ID)
- `user` - User ID who posted
- `text` - Message content (with mrkdwn)
- `channel` - Channel ID
- `thread_ts` - Parent thread timestamp (if reply)
- `reply_count` - Number of thread replies
- `permalink` - Direct link to message

### Search Result

- `channel` - Channel info (name, ID)
- `username` - Poster's display name
- `text` - Message text
- `ts` - Timestamp
- `permalink` - Direct link

### Channel Object

- `id` - Channel ID
- `name` - Channel name
- `is_im` - Is direct message
- `is_mpim` - Is group DM
- `is_private` - Is private channel
- `last_read` - User's last read timestamp

## Important Gotchas

### 1. Enterprise Grid Specifics
- Check `auth.test` response for `enterprise_id`
- Tokens may be scoped to specific workspaces
- Cross-workspace search may require org-level token

### 2. Threading Behavior
- Thread replies NOT in regular channel history
- Must use `conversations.replies` to get thread messages
- Check `reply_count > 0` to detect threads needing expansion

### 3. Message Formatting (mrkdwn)
- User mentions: `<@U12345|displayname>`
- Channel mentions: `<#C12345|channelname>`
- Links: `<https://url|display text>`
- Need to parse/strip for plain text display

### 4. Timestamp Format
- Slack uses: `"1735689600.123456"`
- Format: Unix seconds + microseconds as decimal
- Unique identifier for each message

### 5. Search Limitations
- Cannot search DM content directly via search.messages
- Must list DM channels then fetch history for each
- Search results capped at 1000 total

### 6. Private Channels
- User token inherits user's channel memberships
- No way to see channels user isn't in
- Private channel names not visible to non-members

### 7. Archived Content
- Use `exclude_archived: true` in API calls
- Archived channels still searchable but may be stale

### 8. Token Refresh
- User tokens don't expire automatically
- Token revoked if user deauthorizes app
- Token revoked if user leaves workspace
- No automatic refresh - must re-authenticate

## Recommended Approach

### For Morning Digest

1. **Primary: Search for mentions**
   - Query: `<@YOUR_USER_ID> after:YESTERDAY`
   - Gets all mentions in accessible channels

2. **Secondary: Check DMs**
   - List IM conversations
   - Fetch history since cutoff time
   - Filter out own messages

3. **Optional: Thread expansion**
   - For mentions in threads, fetch full thread context
   - Only if thread context is valuable

### Data to Collect

- Channel name and type (public/private/DM)
- Sender name/ID
- Message preview (stripped formatting)
- Timestamp
- Permalink
- Thread context (if applicable)

## Go Libraries

### slack-go/slack
- GitHub: github.com/slack-go/slack
- Production-ready, actively maintained
- Complete API coverage
- Built-in rate limit handling

### Alternative: Direct HTTP
- Use standard `net/http` with Slack REST API
- More control, fewer dependencies
- Must implement pagination/rate limiting

## Security Considerations

### Token Storage
- Never commit tokens to version control
- Store in encrypted file or system keychain
- Use environment variables for runtime

### Minimal Scopes
- Only request scopes actually needed
- `search:read` + `channels:history` + `im:history` covers most cases

### Enterprise Compliance
- Check workspace DLP policies
- Some workspaces log all API access
- Message content may be monitored

## Integration with Digest Tool

### Outputs Needed
- Count of unreplied mentions
- Count of unread DMs
- List of mentions with context
- List of DM messages

### Priority Ordering
1. Direct DMs (highest priority)
2. Direct @mentions
3. @channel/@here mentions (if relevant)

---

**Document Status**: Research complete
**Last Updated**: 2025-12-05
**Auth Type**: User token (enterprise/corporate workspace)
