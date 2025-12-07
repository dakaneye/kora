# Kora Rich Data Layer Enhancement

## Overview

Transform Kora from a basic notification aggregator into a rich data layer that provides Claude with complete context for determining relevance without additional GitHub CLI queries.

**Current gap:** To assess 18 digest items, Claude needs 15+ additional `gh` queries for context Kora doesn't provide.

**Goal:** Kora provides UI-level detail for all PRs and Issues so Claude can reason over structured data without external queries.

## Agent Delegation Strategy

**This is a planning document.** Implementation tasks should be delegated to appropriate agents with their domain expertise:

| Task Type | Delegate To | Reason |
|-----------|-------------|--------|
| EFA updates | `documentation-engineer` | EFA authoring expertise |
| Go implementation | `golang-pro` | Idiomatic Go, interfaces, concurrency |
| Testing | `test-automator` | Test design, mocking, coverage |
| GraphQL queries | `golang-pro` | API client design |
| Security review | `security-auditor` | Credential handling, validation |

**Implementation workflow:**
1. Project planner (this document) defines WHAT and WHY
2. User reviews and approves plan
3. For each task, invoke appropriate agent with task-specific context
4. Agent implements with full context of related EFAs
5. User reviews agent output before proceeding to next task

## Ordered Implementation Punchlist

### Milestone 1: Data Model Foundation

#### Task 1.1: Update EFA 0001 - Extend Event Metadata for PRs
**What:** Define canonical metadata keys for rich PR context in EFA 0001

**Why:** GraphQL will return extensive PR data; EFA must define allowed keys before implementation

**Delegate to:** `documentation-engineer`

**Context to provide:**
- Current EFA 0001 (specs/efas/0001-event-model.md)
- Required metadata list from user requirements
- Example GitHub GraphQL PR response structure

**Deliverable:** Updated EFA 0001 with new PR metadata keys in allowedMetadataKeys table

**New metadata keys to add:**
- `author_login`: string - PR author username
- `assignees`: []string - assigned users
- `review_requests`: []map[string]string - with type: "user"/"team"
- `reviews`: []map[string]string - state, author
- `inline_comments_count`: int
- `suggestion_count`: int
- `unresolved_threads`: int
- `pr_comments`: []map[string]string
- `ci_checks`: []map[string]string
- `files_changed`: []map[string]any
- `milestone`: string
- `linked_issues`: []string
- `body`: string (truncated to 500 chars)
- `user_relationships`: []string - ["author", "reviewer", "mentioned", "codeowner", "assignee"]

---

#### Task 1.2: Update EFA 0001 - Extend Event Metadata for Issues
**What:** Define canonical metadata keys for rich Issue context

**Why:** GitHub Issues need similar rich context as PRs

**Delegate to:** `documentation-engineer`

**Context to provide:**
- Updated EFA 0001 from Task 1.1
- Required Issue metadata list
- Example GitHub GraphQL Issue response

**Deliverable:** Updated EFA 0001 with Issue metadata keys

**New metadata keys to add:**
- `author_login`: string
- `assignees`: []string
- `body`: string (truncated)
- `comments`: []map[string]string - recent 10
- `linked_prs`: []string
- `reactions`: map[string]int
- `timeline_events`: []map[string]string
- `user_relationships`: []string

---

#### Task 1.3: Update EFA 0001 - Create New EventTypes
**What:** Add EventTypePRAuthor and EventTypePRCodeowner to EFA 0001

**Why:** Distinguish user's own PRs and codeowner scenarios from review requests

**Delegate to:** `documentation-engineer`

**Context to provide:**
- Updated EFA 0001 from Task 1.2
- Priority assignment rules for new types

**Deliverable:** Updated EFA 0001 with new EventTypes and priority rules

**New EventTypes:**
- `EventTypePRAuthor` = "pr_author" - User's own PRs
- `EventTypePRCodeowner` = "pr_codeowner" - User owns changed files

**Priority rules:**
- PR author (failing CI): Priority 1 (Critical)
- PR author (changes requested): Priority 2 (High)
- PR author (pending reviews): Priority 3 (Medium)
- PR codeowner: Priority 3 (Medium)

---

#### Task 1.4: Implement Typed Metadata Accessors
**What:** Create internal/models/metadata.go with typed structs and accessors

**Why:** Provide type-safe access to rich metadata while keeping map[string]any storage

**Delegate to:** `golang-pro`

**Context to provide:**
- Updated EFA 0001 from Task 1.3
- Current internal/models/event.go
- Requirement: backward compatible with existing Event struct

**Deliverable:**
- internal/models/metadata.go with PRMetadata, IssueMetadata structs
- Event.AsPRMetadata() and Event.AsIssueMetadata() methods
- Unit tests in metadata_test.go

**Key implementation points:**
```go
type PRMetadata struct {
    Repo string
    Number int
    AuthorLogin string
    // ... all fields from EFA 0001
}

func (e Event) AsPRMetadata() (*PRMetadata, error)
```

---

### Milestone 2: GitHub GraphQL Integration

#### Task 2.1: Create GraphQL Client Infrastructure
**What:** Implement internal/github/graphql.go with gh CLI delegation

**Why:** Need GraphQL client before writing queries

**Delegate to:** `golang-pro`

**Context to provide:**
- EFA 0002 (auth-provider.md) - credential handling rules
- Existing internal/auth/github.go for credential pattern
- Requirement: use gh CLI delegation, never handle tokens directly

**Deliverable:**
- internal/github/graphql.go with client wrapper
- Execute queries via `gh api graphql -f query='...'`
- Error handling for GraphQL errors, rate limiting
- Unit tests with mock gh CLI

**Key implementation points:**
```go
type GraphQLClient struct {
    cred *auth.GitHubDelegatedCredential
}

func (c *GraphQLClient) Execute(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error)
```

---

#### Task 2.2: Design and Implement GraphQL Query for Rich PR Context
**What:** Create GraphQL query to fetch all PR metadata in single call

**Why:** Replace multiple REST calls with one GraphQL query

**Delegate to:** `golang-pro`

**Context to provide:**
- Task 2.1 GraphQL client
- Updated EFA 0001 metadata requirements
- GitHub GraphQL schema docs
- Current internal/datasources/github/ REST implementation

**Deliverable:**
- internal/github/queries.go with PR query builder
- Parse response to populate all EFA 0001 metadata fields
- Handle pagination, errors, missing data
- Unit tests with mock GraphQL responses in testdata/

**Query must fetch:**
- PR metadata: title, state, author, assignees
- Review requests with __typename (User vs Team)
- Reviews: state, author, comments count
- Review threads: isResolved, locations
- Comments: general discussion
- CI checks: statusCheckRollup
- Files changed: path, additions, deletions
- Labels, milestone, linked issues, body

---

#### Task 2.3: Design and Implement GraphQL Query for Rich Issue Context
**What:** Create GraphQL query to fetch all Issue metadata

**Why:** Issues need similar rich context as PRs

**Delegate to:** `golang-pro`

**Context to provide:**
- Task 2.1 GraphQL client
- Updated EFA 0001 Issue metadata requirements
- Task 2.2 as pattern reference

**Deliverable:**
- Issue query builder in internal/github/queries.go
- Parser for Issue responses
- Unit tests

**Query must fetch:**
- Issue metadata: title, state, author, assignees, body
- Comments: recent 10 with author/timestamp
- Timeline events: assignments, mentions, labels
- Linked PRs via crossReferencedEvents
- Reactions summary

---

### Milestone 3: CODEOWNERS Support

#### Task 3.1: Implement CODEOWNERS Parser
**What:** Create internal/codeowners/parser.go to parse CODEOWNERS file format

**Why:** Need to match PR files against ownership rules

**Delegate to:** `golang-pro`

**Context to provide:**
- GitHub CODEOWNERS format documentation
- Example CODEOWNERS files from popular repos
- Requirement: support glob patterns, user/team references

**Deliverable:**
- internal/codeowners/parser.go with Parse() function
- Pattern matching using glob library
- Unit tests with example CODEOWNERS files

**Key implementation points:**
```go
type Rule struct {
    Pattern string
    Owners  []string
}

type Ruleset struct {
    Rules []Rule
}

func Parse(content []byte) (*Ruleset, error)
func (r *Ruleset) Match(filepath string) []string
```

---

#### Task 3.2: Implement CODEOWNERS Fetcher with Caching
**What:** Create internal/codeowners/fetcher.go to fetch and cache CODEOWNERS

**Why:** Avoid re-fetching CODEOWNERS for every PR in same repo

**Delegate to:** `golang-pro`

**Context to provide:**
- Task 3.1 parser
- EFA 0002 for gh CLI delegation
- Requirement: thread-safe in-memory cache

**Deliverable:**
- internal/codeowners/fetcher.go with Fetcher struct
- Fetch from standard locations (.github/, docs/, root)
- Thread-safe cache using sync.RWMutex
- Unit tests

**Key implementation points:**
```go
type Fetcher struct {
    cache map[string]*Ruleset
    mu    sync.RWMutex
}

func (f *Fetcher) GetRuleset(ctx context.Context, repo string) (*Ruleset, error)
```

---

#### Task 3.3: Implement Team Membership Resolution
**What:** Create internal/codeowners/teams.go to resolve team memberships

**Why:** CODEOWNERS references teams; need to know if user is member

**Delegate to:** `golang-pro`

**Context to provide:**
- Task 3.2 fetcher
- EFA 0002 for gh CLI delegation
- GitHub API docs for team membership

**Deliverable:**
- internal/codeowners/teams.go with TeamResolver
- Fetch team members via gh API
- Cache team memberships
- Unit tests with mock API responses

**Key implementation points:**
```go
type TeamResolver struct {
    cache map[string][]string  // team slug -> members
    mu    sync.RWMutex
}

func (t *TeamResolver) IsMember(ctx context.Context, org, team, login string) (bool, error)
```

---

### Milestone 4: Enhanced GitHub DataSource

#### Task 4.1: Refactor GitHub DataSource to Use GraphQL
**What:** Update internal/datasources/github/datasource.go to use GraphQL queries

**Why:** Replace REST searches with GraphQL for rich context

**Delegate to:** `golang-pro`

**Context to provide:**
- Current internal/datasources/github/datasource.go
- Task 2.2, 2.3 GraphQL queries
- Task 1.4 typed metadata
- EFA 0003 (datasource-interface.md) - must preserve interface

**Deliverable:**
- Updated datasource.go using GraphQL client
- Populate all new metadata fields from EFA 0001
- Preserve DataSource interface from EFA 0003
- Integration tests comparing REST vs GraphQL output

**Key changes:**
- Replace 4 REST searches with 2 GraphQL queries
- Parse GraphQL responses to Event structs with rich metadata
- Maintain error handling and partial success semantics

---

#### Task 4.2: Add CODEOWNERS Matching to PR Events
**What:** Integrate CODEOWNERS matching into GitHub DataSource

**Why:** Detect when user is codeowner but not explicitly requested

**Delegate to:** `golang-pro`

**Context to provide:**
- Task 4.1 updated datasource
- Task 3.1, 3.2, 3.3 CODEOWNERS implementation
- EFA 0001 EventTypePRCodeowner definition

**Deliverable:**
- CODEOWNERS matching in datasource Fetch()
- Create EventTypePRCodeowner events
- Add "codeowner" to user_relationships
- Unit tests with mock CODEOWNERS

**Key implementation points:**
- For each PR with files_changed
- Get CODEOWNERS ruleset for repo
- Match changed files against rules
- If user in owners, flag as codeowner
- Avoid duplication with explicit review requests

---

#### Task 4.3: Implement Event Deduplication by Relationships
**What:** Create internal/models/dedup.go to merge duplicate events

**Why:** Same PR/Issue can appear for multiple reasons; show once with all reasons

**Delegate to:** `golang-pro`

**Context to provide:**
- Task 4.2 datasource output
- EFA 0001 user_relationships field
- Requirement: merge by URL, pick primary type by priority

**Deliverable:**
- internal/models/dedup.go with DeduplicateEvents()
- Merge logic: group by URL, combine user_relationships
- Primary type selection by priority
- Unit tests covering all scenarios

**Key implementation points:**
```go
func DeduplicateEvents(events []Event) []Event
```
- Group by URL
- Merge user_relationships arrays
- Pick primary EventType: PR review (direct) > Issue assigned > PR author (failing CI) > PR codeowner > mentions
- Set RequiresAction based on primary type

---

#### Task 4.4: Distinguish Team vs User Review Requests
**What:** Add type field to review_requests metadata

**Why:** Direct user reviews are higher priority than team reviews

**Delegate to:** `golang-pro`

**Context to provide:**
- Task 4.1 GraphQL response parsing
- EFA 0001 review_requests structure
- GitHub GraphQL __typename for User vs Team

**Deliverable:**
- review_requests metadata includes type field
- Priority adjusted: user review = 2, team review = 3
- Unit tests verify type detection

---

#### Task 4.5: Add Author Detection for Own PRs
**What:** Create EventTypePRAuthor events for user's own PRs

**Why:** Track status of user's PRs (CI failures, review status)

**Delegate to:** `golang-pro`

**Context to provide:**
- Task 4.1 GraphQL datasource
- EFA 0001 EventTypePRAuthor definition
- Priority rules based on CI/review status

**Deliverable:**
- Add author:@me search query
- Create EventTypePRAuthor events
- Priority based on CI status and review state
- Deduplication: don't duplicate if also reviewer/mentioned

---

### Milestone 5: Update Models and Validation

#### Task 5.1: Update Event Model with New Types
**What:** Add new EventTypes to internal/models/event.go per EFA 0001

**Why:** Implement EFA changes in code

**Delegate to:** `golang-pro`

**Context to provide:**
- Updated EFA 0001 from Task 1.3
- Current internal/models/event.go
- Current internal/models/validation.go

**Deliverable:**
- EventTypePRAuthor, EventTypePRCodeowner added to constants
- validEventTypes map updated
- Validation updated for new metadata keys
- Unit tests

---

#### Task 5.2: Update Priority Calculation Rules
**What:** Adjust priority assignment based on rich context

**Why:** More granular priorities using CI status, review state

**Delegate to:** `golang-pro`

**Context to provide:**
- Task 5.1 updated models
- EFA 0001 priority rules from Task 1.3
- Task 4.4, 4.5 implementations

**Deliverable:**
- Updated priority calculation in datasource
- EFA 0001 priority table reflects implementation
- Unit tests verify correct priorities

---

### Milestone 6: Testing and Documentation

#### Task 6.1: Create Unit Tests for Metadata Accessors
**What:** Test internal/models/metadata.go typed accessors

**Why:** Ensure parsing from map[string]any works correctly

**Delegate to:** `test-automator`

**Context to provide:**
- Task 1.4 metadata.go implementation
- EFA 0001 metadata structure
- Requirement: >90% coverage

**Deliverable:**
- metadata_test.go with comprehensive tests
- Valid metadata parsing
- Error cases
- JSON round-trip

---

#### Task 6.2: Create Integration Tests for GraphQL Fetching
**What:** Test internal/datasources/github/ with mock GraphQL responses

**Why:** Verify rich context fetching end-to-end

**Delegate to:** `test-automator`

**Context to provide:**
- Task 4.1, 4.2, 4.3, 4.4, 4.5 implementations
- Example GraphQL responses
- Requirement: compare old REST vs new GraphQL output

**Deliverable:**
- Integration tests in datasources/github/
- Mock GraphQL responses in testdata/
- Verify all metadata populated
- Verify CODEOWNERS matching
- Verify deduplication

---

#### Task 6.3: Create Integration Tests for CODEOWNERS
**What:** Test internal/codeowners/ parsing and matching

**Why:** Verify CODEOWNERS logic works correctly

**Delegate to:** `test-automator`

**Context to provide:**
- Task 3.1, 3.2, 3.3 implementations
- Real CODEOWNERS examples
- Edge cases (missing file, team resolution)

**Deliverable:**
- Integration tests for parser, fetcher, teams
- Cover edge cases
- Mock gh API for team lookups

---

#### Task 6.4: Security Review of Credential Handling
**What:** Review all new code for credential exposure

**Why:** Ensure GraphQL client and CODEOWNERS fetcher follow EFA 0002

**Delegate to:** `security-auditor`

**Context to provide:**
- Task 2.1 GraphQL client
- Task 3.2 CODEOWNERS fetcher
- EFA 0002 security rules
- All code changes from Milestone 4

**Deliverable:**
- Security audit report
- Verify no credentials logged
- Verify no credentials in error messages
- Verify TLS validation enabled
- Any issues found + fixes

---

#### Task 6.5: Update EFA Documents with Implementation Details
**What:** Document completed implementation in EFAs

**Why:** Keep EFAs as source of truth

**Delegate to:** `documentation-engineer`

**Context to provide:**
- All completed implementations
- Current EFA 0001, EFA 0003
- Example outputs from integration tests

**Deliverable:**
- Updated EFA 0001 with examples showing rich metadata
- Updated EFA 0003 with GraphQL approach
- AI Agent Rules reflect new constraints

---

#### Task 6.6: Add Performance Benchmarks
**What:** Measure API call reduction and latency

**Why:** Quantify improvement over current approach

**Delegate to:** `golang-pro`

**Context to provide:**
- Task 6.2 integration tests
- Current REST implementation stats
- Requirement: compare API calls, latency, completeness

**Deliverable:**
- Benchmark comparing REST vs GraphQL
- Metrics: API calls (~20 → ~3), latency, data completeness
- Documentation in specs/plans/ with results

---

## Dependency Graph

```
Foundation:
  1.1 → 1.2 → 1.3 → 1.4

GraphQL (parallel):
  2.1 → 2.2
       → 2.3

CODEOWNERS (parallel):
  3.1 → 3.2 → 3.3

Enhanced DataSource (sequential):
  (1.4, 2.2, 2.3) → 4.1
  (3.1, 3.2, 3.3, 4.1) → 4.2
  (4.1, 4.2) → 4.3
  4.1 → 4.4
  4.1 → 4.5

Models:
  (1.1, 1.2, 1.3) → 5.1
  (1.3, 4.4, 4.5) → 5.2

Testing (parallel after implementation):
  1.4 → 6.1
  (4.1-4.5) → 6.2
  (3.1-3.3) → 6.3
  (2.1, 3.2, 4.x) → 6.4
  ALL → 6.5
  6.2 → 6.6
```

## Agent Invocation Workflow

For each task:

1. **Prepare agent context:**
   - Task description and requirements
   - Relevant EFA documents
   - Related code files
   - Previous task outputs (dependencies)

2. **Invoke agent:**
   ```
   Use appropriate agent (golang-pro, test-automator, etc.)
   Provide full context including "Context to provide" section
   Reference relevant EFAs explicitly
   ```

3. **Review agent output:**
   - Verify deliverables complete
   - Check EFA compliance
   - Ensure no protected code modified without EFA update

4. **Proceed to next task** only after user approval

## Success Criteria

**Data completeness:**
- [ ] Claude requires 0 additional gh queries for relevance assessment
- [ ] All metadata from reference script included
- [ ] UI-level detail for PRs and Issues

**Performance:**
- [ ] API calls reduced from ~20 to ~3 per digest
- [ ] Overall latency within 2x of current implementation

**Code quality:**
- [ ] All EFA changes documented and approved
- [ ] Unit test coverage >80%
- [ ] Integration tests cover all new features
- [ ] Security audit passes (no credential exposure)

**Agent delegation:**
- [ ] All implementation tasks delegated to appropriate agents
- [ ] Each agent receives full context and requirements
- [ ] No implementation done outside agent delegation
