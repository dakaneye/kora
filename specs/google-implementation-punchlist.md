# Google Calendar & Gmail Implementation Punchlist

> Ordered implementation plan for adding Google Calendar and Gmail datasources to Kora.

## Prerequisites

Read before starting:
- `specs/google-datasources.md` - Main spec
- `specs/google-oauth-flow.md` - OAuth details
- `specs/efas/0001-event-model.md` - Event model
- `specs/efas/0002-auth-provider.md` - Auth patterns
- `specs/efas/0003-datasource-interface.md` - DataSource interface

---

## Agent Requirements

**THIS SECTION IS BINDING.** Claude MUST invoke the specified agents for each task. Direct implementation without agent delegation is FORBIDDEN.

### Core Rules

1. **NEVER implement Go code directly** - ALL Go implementation MUST use `golang-pro` agent
2. **NEVER write tests directly** - ALL test creation MUST use `test-automator` agent
3. **NEVER modify EFA documents directly** - ALL EFA updates MUST use `documentation-engineer` agent
4. **NEVER skip security review** - ALL auth/credential code MUST be reviewed by `security-auditor` agent
5. **NEVER mark a task complete** without agent confirmation

### Agent Assignment Matrix

| Task Category | Primary Agent | Secondary Agent | Security Review |
|---------------|---------------|-----------------|-----------------|
| EFA Updates (1.1, 2.1, 3.1, 4.1) | `documentation-engineer` | `golang-pro` | No |
| Go Implementation | `golang-pro` | - | If auth-related |
| Auth/Security Code (2.2, 2.4, 2.5) | `golang-pro` | `security-auditor` | **YES** |
| Unit Tests | `test-automator` | `golang-pro` | No |
| Integration Tests (2.7, 6.x) | `test-automator` | `golang-pro` | Yes |
| Security Review (7.2) | `security-auditor` | - | **IS THE REVIEW** |
| Documentation (5.4) | `documentation-engineer` | - | No |

### Security-Critical Tasks

Tasks 2.2, 2.4, 2.5, 2.7, 6.1-6.3 involve credentials. Workflow:

```
1. golang-pro implements code
2. test-automator creates tests
3. security-auditor reviews BOTH
4. Only after approval can task be marked complete
```

### Forbidden Actions

- Writing >10 lines of Go without `golang-pro`
- Creating test files without `test-automator`
- Modifying `specs/efas/` without `documentation-engineer`
- Completing auth tasks without `security-auditor` sign-off

---

## Phase 1: Event Model Extensions

### 1.1 Update EFA 0001 with Google Event Types

**Required Agent(s):** `documentation-engineer`
**Security Review:** No
**Depends On:** None

Add to `specs/efas/0001-event-model.md`:
- 6 calendar types: `calendar_upcoming`, `calendar_needs_rsvp`, `calendar_organizer_pending`, `calendar_tentative`, `calendar_meeting`, `calendar_all_day`
- 3 email types: `email_important`, `email_direct`, `email_cc`
- 2 sources: `SourceGoogleCalendar`, `SourceGmail`
- Metadata keys for both

**Acceptance:**
- [ ] EFA updated with 9 event types
- [ ] Metadata keys documented
- [ ] AI Agent Rules updated

---

### 1.2 Implement Event Types in Code

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 1.1

Update `internal/models/event.go`:
- Add EventType constants
- Add Source constants
- Update validation maps

**Acceptance:**
- [ ] `EventType.IsValid()` passes for all new types
- [ ] Code compiles

---

### 1.3 Unit Tests for Event Model

**Required Agent(s):** `test-automator`
**Security Review:** No
**Depends On:** 1.2

Create tests in `internal/models/event_test.go`:
- Table-driven tests for 9 event types
- Metadata validation per source

**Acceptance:**
- [ ] All tests pass
- [ ] >90% coverage

---

## Phase 2: Google OAuth Authentication

### 2.1 Update EFA 0002 with OAuth Design

**Required Agent(s):** `documentation-engineer`, `golang-pro`
**Security Review:** No
**Depends On:** None

Add to `specs/efas/0002-auth-provider.md`:
- `ServiceGoogle` constant
- `GoogleOAuthCredential` type
- OAuth 2.0 flow documentation
- Keychain keys: `google-oauth-{email}`

**Acceptance:**
- [ ] EFA updated with OAuth section
- [ ] `golang-pro` confirms feasibility

---

### 2.2 Implement GoogleOAuthCredential

**Required Agent(s):** `golang-pro`, `security-auditor`
**Security Review:** **YES**
**Depends On:** 2.1

Create `internal/auth/google_credential.go`:
- Struct: `AccessToken`, `RefreshToken`, `Expiry`, `Email`
- Implement `Credential` interface
- `Redacted()` never exposes tokens
- `IsExpired()` method

**Acceptance:**
- [ ] Implements interface
- [ ] `security-auditor` confirms no token leakage

---

### 2.3 Unit Tests for Credential

**Required Agent(s):** `test-automator`
**Security Review:** No
**Depends On:** 2.2

Create `internal/auth/google_credential_test.go`:
- Validation scenarios
- Redacted output verification
- Expiry edge cases

**Acceptance:**
- [ ] All tests pass
- [ ] >90% coverage

---

### 2.4 Implement OAuth Flow

**Required Agent(s):** `golang-pro`, `security-auditor`
**Security Review:** **YES**
**Depends On:** 2.2

Create `internal/auth/google/oauth.go`:
- `InitiateOAuthFlow(email) (*GoogleOAuthCredential, error)`
- CSRF state token
- Localhost callback server
- Browser launch
- Token exchange
- 60s timeout

**Acceptance:**
- [ ] OAuth completes successfully
- [ ] Tokens never logged
- [ ] `security-auditor` approves

---

### 2.5 Implement GoogleAuthProvider

**Required Agent(s):** `golang-pro`, `security-auditor`
**Security Review:** **YES**
**Depends On:** 2.4

Create `internal/auth/google/provider.go`:
- Implement `AuthProvider` interface
- Keychain get/store
- Auto-refresh expired tokens

**Acceptance:**
- [ ] Implements interface
- [ ] Refreshes automatically
- [ ] `security-auditor` approves

---

### 2.6 Unit Tests for Auth Provider

**Required Agent(s):** `test-automator`
**Security Review:** No
**Depends On:** 2.5

Create `internal/auth/google/provider_test.go`:
- Mock keychain
- Token refresh scenarios
- Error propagation

**Acceptance:**
- [ ] All tests pass
- [ ] No real OAuth calls

---

### 2.7 Integration Test for OAuth

**Required Agent(s):** `test-automator`, `security-auditor`
**Security Review:** Yes
**Depends On:** 2.6

Create `tests/integration/google_auth_test.go`:
- Tagged `//go:build integration`
- Full OAuth flow
- Skip if creds missing

**Acceptance:**
- [ ] Passes with real creds
- [ ] `security-auditor` approves

---

## Phase 3: Google Calendar DataSource

### 3.1 Update EFA 0003 with Calendar Design

**Required Agent(s):** `documentation-engineer`
**Security Review:** No
**Depends On:** 1.1

Add to `specs/efas/0003-datasource-interface.md`:
- Calendar API calls
- Filtering rules
- Priority calculation

**Acceptance:**
- [ ] EFA updated

---

### 3.2 Implement Calendar API Client

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 2.5, 3.1

Create `internal/datasources/google_calendar/client.go`:
- `ListCalendars(ctx)`
- `ListEvents(ctx, calendarID, since, until)`
- Rate limiting with backoff

**Acceptance:**
- [ ] Makes authenticated requests
- [ ] Handles rate limits

---

### 3.3 Implement Event Conversion

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 3.2

Create `internal/datasources/google_calendar/mapper.go`:
- `ToEvent(calEvent, email) (models.Event, error)`
- All 6 event types
- All metadata keys

**Acceptance:**
- [ ] Events pass `Event.Validate()`

---

### 3.4 Unit Tests for Mapper

**Required Agent(s):** `test-automator`
**Security Review:** No
**Depends On:** 3.3

Create `internal/datasources/google_calendar/mapper_test.go`:
- JSON fixtures in `testdata/`
- All event type conversions

**Acceptance:**
- [ ] All tests pass

---

### 3.5 Implement GoogleCalendarDataSource

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 3.3

Create `internal/datasources/google_calendar/datasource.go`:
- Implement `DataSource` interface
- Concurrent calendar fetching
- Partial results on errors

**Acceptance:**
- [ ] Implements interface
- [ ] Partial success works

---

### 3.6 Unit Tests for DataSource

**Required Agent(s):** `test-automator`
**Security Review:** No
**Depends On:** 3.5

Create `internal/datasources/google_calendar/datasource_test.go`:
- Mock API client
- Multi-calendar scenarios

**Acceptance:**
- [ ] All tests pass
- [ ] >85% coverage

---

## Phase 4: Gmail DataSource

### 4.1 Update EFA 0003 with Gmail Design

**Required Agent(s):** `documentation-engineer`
**Security Review:** No
**Depends On:** 1.1

Add to `specs/efas/0003-datasource-interface.md`:
- Gmail API calls
- Filtering (mailing lists, automated)
- Priority calculation

**Acceptance:**
- [ ] EFA updated

---

### 4.2 Implement Gmail API Client

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 2.5, 4.1

Create `internal/datasources/gmail/client.go`:
- `ListMessages(ctx, query, maxResults)`
- `GetMessage(ctx, msgID)`
- `BatchGetMessages(ctx, msgIDs)`

**Acceptance:**
- [ ] Batch operations work

---

### 4.3 Implement Message Filtering

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 4.2

Create `internal/datasources/gmail/filter.go`:
- `IsMailingList(msg)` - List-Unsubscribe header
- `IsAutomated(msg)` - noreply patterns
- `IsImportant(msg, importantSenders)`

**Acceptance:**
- [ ] Correctly identifies all categories

---

### 4.4 Implement Message Conversion

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 4.3

Create `internal/datasources/gmail/mapper.go`:
- `ToEvent(msg, userEmail, importantSenders)`
- All 3 event types
- All metadata keys

**Acceptance:**
- [ ] Events pass `Event.Validate()`

---

### 4.5 Unit Tests for Mapper and Filter

**Required Agent(s):** `test-automator`
**Security Review:** No
**Depends On:** 4.4

Create:
- `internal/datasources/gmail/mapper_test.go`
- `internal/datasources/gmail/filter_test.go`

**Acceptance:**
- [ ] All tests pass

---

### 4.6 Implement GmailDataSource

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 4.4

Create `internal/datasources/gmail/datasource.go`:
- Implement `DataSource` interface
- Query unread + unreplied
- Deduplicate, filter, convert

**Acceptance:**
- [ ] Implements interface
- [ ] Filtering works

---

### 4.7 Unit Tests for DataSource

**Required Agent(s):** `test-automator`
**Security Review:** No
**Depends On:** 4.6

Create `internal/datasources/gmail/datasource_test.go`:
- Mock API client
- Filtering and deduplication

**Acceptance:**
- [ ] All tests pass
- [ ] >85% coverage

---

## Phase 5: Configuration & CLI

### 5.1 Update Configuration Schema

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** None

Update `internal/config/config.go` with:
```go
type GoogleConfig struct {
    Calendars []CalendarConfig `yaml:"calendars"`
    Gmail     []GmailConfig    `yaml:"gmail"`
}
```

**Acceptance:**
- [ ] Config loads from YAML

---

### 5.2 Update DataSource Registry

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 3.5, 4.6, 5.1

Update `internal/datasources/datasource.go`:
- Factory functions
- Wire auth provider

**Acceptance:**
- [ ] Datasources load from config

---

### 5.3 Add CLI Auth Commands

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 2.5

Add commands:
- `kora auth google login --email`
- `kora auth google status --email`
- `kora auth google logout --email`

**Acceptance:**
- [ ] Commands work

---

### 5.4 Update Documentation

**Required Agent(s):** `documentation-engineer`
**Security Review:** No
**Depends On:** All implementation

Update `README.md`, create `docs/google-setup.md`

**Acceptance:**
- [ ] Documentation complete

---

## Phase 6: Integration Testing

### 6.1 Calendar Integration Test

**Required Agent(s):** `test-automator`, `security-auditor`
**Security Review:** Yes
**Depends On:** 3.6

Create `tests/integration/google_calendar_test.go`

**Acceptance:**
- [ ] Passes with real creds

---

### 6.2 Gmail Integration Test

**Required Agent(s):** `test-automator`, `security-auditor`
**Security Review:** Yes
**Depends On:** 4.7

Create `tests/integration/gmail_test.go`

**Acceptance:**
- [ ] Passes with real creds

---

### 6.3 End-to-End Digest Test

**Required Agent(s):** `test-automator`
**Security Review:** Yes
**Depends On:** 6.1, 6.2

Create `tests/integration/digest_google_test.go`

**Acceptance:**
- [ ] All sources in digest

---

## Phase 7: CI/CD & Security

### 7.1 Update CI

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** All tests

Update `.github/workflows/ci.yml`

**Acceptance:**
- [ ] CI passes

---

### 7.2 Security Scan

**Required Agent(s):** `security-auditor`
**Security Review:** **IS THE REVIEW**
**Depends On:** All implementation

Run gosec, verify no credential leaks, CSRF validation

**Acceptance:**
- [ ] No security issues

---

### 7.3 Performance Testing

**Required Agent(s):** `golang-pro`
**Security Review:** No
**Depends On:** 6.3

Verify <30s per datasource, timeouts respected

**Acceptance:**
- [ ] Performance requirements met

---

## Dependencies Graph

```
Phase 1 ─┬─► Phase 3 (Calendar)
         │
Phase 2 ─┼─► Phase 4 (Gmail)     ──► Phase 5 ──► Phase 6 ──► Phase 7
         │
         └─► (parallel)
```
