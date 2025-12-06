# Session 6: Implement Slack DataSource

## Agent Requirements (MANDATORY)

**YOU MUST invoke the following agents using the Task tool. Do NOT implement directly.**

| Agent | Invoke For | Task Tool Call |
|-------|------------|----------------|
| `golang-pro` | Slack datasource implementation | `Task(subagent_type="golang-pro", prompt="...")` |
| `security-auditor` | Review token handling | `Task(subagent_type="security-auditor", prompt="...")` |
| `test-automator` | Unit tests with mock HTTP | `Task(subagent_type="test-automator", prompt="...")` |

### Pre-Flight Checklist
- [ ] I will use Task tool to invoke `golang-pro` for implementation
- [ ] I will use Task tool to invoke `security-auditor` for token review
- [ ] I will use Task tool to invoke `test-automator` for tests
- [ ] I will NOT write Go code directly
- [ ] I have read EFAs 0001, 0002, 0003 before starting

---

## Objective
Implement SlackDataSource that fetches DMs and @mentions using Slack Web API.

## Dependencies
- Session 4 complete (DataSource interface exists)
- Session 3 complete (SlackAuthProvider exists)
- Session 2 complete (Event model exists)

## Files to Create
```
internal/datasources/slack/datasource.go    # Implements DataSource
internal/datasources/slack/client.go        # Slack Web API client
internal/datasources/slack/dms.go           # Direct messages
internal/datasources/slack/mentions.go      # @mentions via search
internal/datasources/slack/transform.go     # API response -> Event conversion
internal/datasources/slack/types.go         # Slack API response types
internal/datasources/slack/helpers.go       # Utility functions
internal/datasources/slack/datasource_test.go
tests/testdata/slack_auth.json              # Mock auth.test response
tests/testdata/slack_search.json            # Mock search.messages response
tests/testdata/slack_conversations.json     # Mock users.conversations response
tests/testdata/slack_history.json           # Mock conversations.history response
```

---

## Task 1: Invoke golang-pro Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="golang-pro",
  prompt="""
Implement the Slack DataSource for Kora.

Reference EFAs:
- EFA 0001 (specs/efas/0001-event-model.md) for Event transformation
- EFA 0002 (specs/efas/0002-auth-provider.md) for credential handling
- EFA 0003 (specs/efas/0003-datasource-interface.md) for interface

1. internal/datasources/slack/types.go:
   - slackSearchResponse: OK, Error, Messages.Matches
   - slackConversationsResponse: OK, Error, Channels
   - slackHistoryResponse: OK, Error, Messages
   - slackMessage: TS, ThreadTS, Text, User, Username, Permalink, Channel
   - slackChannel: ID, Name

2. internal/datasources/slack/client.go:
   - DataSource struct: authProvider, httpClient, baseURL, userID
   - Functional options: WithHTTPClient, WithBaseURL (for testing)
   - NewDataSource(authProvider, opts...) constructor
   - apiRequest(ctx, token, method, params) method:
     - Set Authorization: Bearer header
     - Handle rate limiting (429)
     - Limit response size to 10MB
     - NEVER log the token

3. internal/datasources/slack/datasource.go:
   - Name() returns "slack"
   - Service() returns models.SourceSlack
   - Fetch() implementation:
     - Get credential from authProvider
     - Get user ID via auth.test (cache it)
     - Fetch mentions and DMs
     - Transform, deduplicate, filter, validate

4. internal/datasources/slack/mentions.go:
   - fetchMentions(): use search.messages with query "<@USER_ID>"
   - EventType: slack_mention, Priority: Medium (3)

5. internal/datasources/slack/dms.go:
   - fetchDMs():
     - List IM channels via users.conversations
     - Get history for each via conversations.history
   - EventType: slack_dm, Priority: High (2), RequiresAction: true
   - Skip own messages (compare to userID)

6. internal/datasources/slack/transform.go:
   - slackMessageToEvent() conversion
   - parseSlackTimestamp() for "1234567890.123456" format
   - stripMrkdwn() to remove <@U123|name> format
   - truncateTitle() to enforce 100 char limit

7. internal/datasources/slack/helpers.go:
   - getWorkspaceFromPermalink()
   - buildDMPermalink()
   - deduplicateEvents()
   - filterEvents()

CRITICAL per EFA 0002:
- Token used ONLY in Authorization header
- NEVER log the token value
- NEVER include token in error messages

Metadata keys per EFA 0001:
- workspace, channel, thread_ts, is_thread_reply
"""
)
```

---

## Task 2: Invoke security-auditor Agent

**MANDATORY**: After golang-pro completes:

```
Task(
  subagent_type="security-auditor",
  prompt="""
Security review of Slack datasource (internal/datasources/slack/).

Per EFA 0002, verify:

1. Token Handling:
   - Token obtained via authProvider.GetCredential()
   - Token ONLY used in Authorization header
   - Token never stored in struct fields (except temporarily)
   - Token never logged, even in debug mode

2. HTTP Client Security:
   - TLS verification enabled (no InsecureSkipVerify)
   - Timeout configured on http.Client
   - Response size limited to prevent memory exhaustion

3. Logging Audit:
   - No credential values in any log statement
   - API responses not logged (may contain sensitive data)

4. Error Messages:
   - No token values in error strings
   - Safe error wrapping

Search for patterns:
- log.*(.*token.*)
- fmt.*(.*cred.Value().*)
- Authorization header in logs

Report findings with severity and file:line references.
"""
)
```

---

## Task 3: Invoke test-automator Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="test-automator",
  prompt="""
Create comprehensive tests for Slack datasource (internal/datasources/slack/).

1. Create mock HTTP server for Slack API:
   - Route requests to appropriate test handlers
   - Return test data from testdata files
   - Support error injection

2. Create mock SlackAuthProvider:
   - Return mock SlackToken credential
   - Support auth failure simulation

3. Create test data files:
   - tests/testdata/slack_auth.json: auth.test response with user_id
   - tests/testdata/slack_search.json: search.messages with mentions
   - tests/testdata/slack_conversations.json: users.conversations with IMs
   - tests/testdata/slack_history.json: conversations.history with messages

4. internal/datasources/slack/datasource_test.go:

   Fetch Tests:
   - Test mentions fetch and transform correctly
   - Test DMs fetch from multiple channels
   - Test own messages filtered out
   - Test FetchOptions.Since filtering

   Transform Tests:
   - Test parseSlackTimestamp() with valid/invalid formats
   - Test stripMrkdwn() removes user and channel mentions
   - Test truncateTitle() at 100 chars
   - Test metadata keys match EFA 0001

   Rate Limiting Tests:
   - Test 429 response sets RateLimited=true
   - Test Retry-After header parsed

   Error Tests:
   - Test auth failure returns ErrNotAuthenticated
   - Test invalid JSON returns ErrInvalidResponse

5. Verify all test events pass Event.Validate()

Target >80% coverage.
"""
)
```

---

## Slack API Methods Reference

| Method | Purpose |
|--------|---------|
| `auth.test` | Get authenticated user ID |
| `search.messages` | Find @mentions: `<@USER_ID>` |
| `users.conversations` | List IM (DM) channels |
| `conversations.history` | Get messages from each DM |

## Priority Rules

| Event Type | Priority | RequiresAction |
|------------|----------|----------------|
| slack_dm | High (2) | true |
| slack_mention | Medium (3) | false |

## EFA Constraints Summary

- **EFA 0001**: EventTypes slack_dm/slack_mention, metadata keys, title length
- **EFA 0002**: Token in Authorization header only, never log token
- **EFA 0003**: DataSource interface, partial success, rate limiting

---

## Definition of Done
- [ ] SlackDataSource implements DataSource interface
- [ ] Fetches DMs (Priority High)
- [ ] Fetches @mentions (Priority Medium)
- [ ] All events pass `Event.Validate()`
- [ ] Token used only in Authorization header
- [ ] Security audit passes
- [ ] Rate limiting handled (429 responses)
- [ ] Test coverage >80%
- [ ] `make test` passes

## Next Session
Session 7: Implement Output Formatters
