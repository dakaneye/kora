# Session 5: Implement GitHub DataSource

## Agent Requirements (MANDATORY)

**YOU MUST invoke the following agents using the Task tool. Do NOT implement directly.**

| Agent | Invoke For | Task Tool Call |
|-------|------------|----------------|
| `golang-pro` | GitHub datasource implementation | `Task(subagent_type="golang-pro", prompt="...")` |
| `security-auditor` | Review credential delegation | `Task(subagent_type="security-auditor", prompt="...")` |
| `test-automator` | Unit tests with mock responses | `Task(subagent_type="test-automator", prompt="...")` |

### Pre-Flight Checklist
- [ ] I will use Task tool to invoke `golang-pro` for implementation
- [ ] I will use Task tool to invoke `security-auditor` for credential review
- [ ] I will use Task tool to invoke `test-automator` for tests
- [ ] I will NOT write Go code directly
- [ ] I have read EFAs 0001, 0002, 0003 before starting

---

## Objective
Implement GitHubDataSource that fetches PR review requests, mentions, and assigned issues.

## Dependencies
- Session 4 complete (DataSource interface exists)
- Session 3 complete (GitHubAuthProvider exists)
- Session 2 complete (Event model exists)

## Files to Create
```
internal/datasources/github/datasource.go   # Implements DataSource
internal/datasources/github/client.go       # gh CLI wrapper for API calls
internal/datasources/github/prs.go          # PR review requests, mentions
internal/datasources/github/issues.go       # Issue mentions, assignments
internal/datasources/github/transform.go    # API response -> Event conversion
internal/datasources/github/types.go        # GitHub API response types
internal/datasources/github/helpers.go      # Utility functions
internal/datasources/github/datasource_test.go
tests/testdata/github_prs.json              # Mock API responses
tests/testdata/github_issues.json
tests/testdata/github_search.json
```

---

## Task 1: Invoke golang-pro Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="golang-pro",
  prompt="""
Implement the GitHub DataSource for Kora.

Reference EFAs:
- EFA 0001 (specs/efas/0001-event-model.md) for Event transformation
- EFA 0002 (specs/efas/0002-auth-provider.md) for credential handling
- EFA 0003 (specs/efas/0003-datasource-interface.md) for interface

1. internal/datasources/github/types.go:
   - ghSearchResult struct: TotalCount, Items
   - ghItem struct: Number, Title, State, HTMLURL, RepositoryURL, User, Labels, UpdatedAt, PullRequest
   - ghUser struct: Login
   - ghLabel struct: Name
   - ghPR struct: URL (to detect if item is a PR)

2. internal/datasources/github/datasource.go:
   - DataSource struct with authProvider and orgs fields
   - WithOrgs([]string) functional option
   - NewDataSource(authProvider, opts...) constructor
   - Name() returns "github"
   - Service() returns models.SourceGitHub
   - Fetch() implementation that:
     - Validates FetchOptions
     - Gets GitHubDelegatedCredential from authProvider
     - Calls fetch methods for each event type
     - Deduplicates by URL
     - Filters by FetchOptions
     - Validates all events before returning

3. internal/datasources/github/prs.go:
   - fetchPRReviewRequests(): query "review-requested:@me is:open -draft:true"
     - EventType: pr_review, Priority: High (2), RequiresAction: true
   - fetchPRMentions(): query "mentions:@me type:pr"
     - EventType: pr_mention, Priority: Medium (3), RequiresAction: false

4. internal/datasources/github/issues.go:
   - fetchIssueMentions(): query "mentions:@me type:issue"
     - EventType: issue_mention, Priority: Medium (3), RequiresAction: false
   - fetchAssignedIssues(): query "assignee:@me is:open type:issue"
     - EventType: issue_assigned, Priority: Medium (3), RequiresAction: true

5. internal/datasources/github/transform.go:
   - ghItemToEvent() conversion function
   - truncateTitle() to enforce 100 char limit
   - extractLabels() helper
   - extractRepoFromURL() helper

6. internal/datasources/github/helpers.go:
   - buildOrgFilter([]string) for org:X queries
   - deduplicateEvents() by URL
   - filterEvents() by FetchOptions

CRITICAL per EFA 0002:
- Use GitHubDelegatedCredential.ExecuteAPI() for ALL API calls
- NEVER extract or store tokens
- NEVER log any credential information

All events MUST pass Event.Validate() per EFA 0001.
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
Security review of GitHub datasource (internal/datasources/github/).

Per EFA 0002, verify:

1. Credential Delegation:
   - ALL API calls use GitHubDelegatedCredential.ExecuteAPI()
   - No direct http.Client usage with tokens
   - No token extraction or storage

2. Logging Audit:
   - No credential values in log statements
   - No API responses logged that might contain tokens

3. Error Messages:
   - No credentials in error strings
   - Safe error wrapping

4. Input Validation:
   - Search queries properly escaped
   - No injection vulnerabilities in query building

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
Create comprehensive tests for GitHub datasource (internal/datasources/github/).

1. Create mock GitHubDelegatedCredential:
   - Mock ExecuteAPI() to return test data from files
   - Support error injection for testing

2. Create test data files:
   - tests/testdata/github_pr_reviews.json: PR review search results
   - tests/testdata/github_pr_mentions.json: PR mention search results
   - tests/testdata/github_issue_mentions.json: Issue mention results
   - tests/testdata/github_assigned_issues.json: Assigned issues results
   - Include edge cases: empty results, 100+ char titles

3. internal/datasources/github/datasource_test.go:

   Fetch Tests:
   - Test all searches succeed -> all events returned
   - Test partial success: one search fails -> others still return
   - Test FetchOptions.Since filtering works
   - Test deduplication removes duplicate URLs

   Transform Tests:
   - Test ghItemToEvent() produces valid events
   - Test title truncation at 100 chars
   - Test metadata keys match EFA 0001 allowed keys
   - Test priority assignment per EFA 0001 rules

   Error Tests:
   - Test auth failure returns ErrNotAuthenticated
   - Test invalid JSON returns ErrInvalidResponse

4. Verify all test events pass Event.Validate()

Target >80% coverage.
"""
)
```

---

## Search Queries Reference

| Query | EventType | Priority |
|-------|-----------|----------|
| `review-requested:@me is:open -draft:true` | pr_review | High (2) |
| `mentions:@me type:pr` | pr_mention | Medium (3) |
| `mentions:@me type:issue` | issue_mention | Medium (3) |
| `assignee:@me is:open type:issue` | issue_assigned | Medium (3) |

## EFA Constraints Summary

- **EFA 0001**: EventTypes, metadata keys, title length, validation
- **EFA 0002**: Use ExecuteAPI(), never store tokens, never log credentials
- **EFA 0003**: DataSource interface, partial success, event validation

---

## Definition of Done
- [ ] GitHubDataSource implements DataSource interface
- [ ] Fetches PR review requests (Priority High)
- [ ] Fetches PR/issue mentions (Priority Medium)
- [ ] Fetches assigned issues (Priority Medium)
- [ ] All events pass `Event.Validate()`
- [ ] Uses ExecuteAPI() for all GitHub calls
- [ ] Security audit passes
- [ ] Test coverage >80%
- [ ] `make test` passes

## Next Session
Session 6: Implement Slack DataSource
