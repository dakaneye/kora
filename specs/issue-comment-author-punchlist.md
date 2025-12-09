# Issue Comment Author Implementation Punchlist

Implementation checklist for adding `issue_comment_author` support to Kora's GitHub datasource.

## EFA Update
- [ ] **Update EFA 0001** (`specs/efas/0001-event-model.md`)
  - Add `EventTypeIssueCommentAuthor` constant with value `"issue_comment_author"`
  - Add to `validEventTypes` map
  - Add to priority assignment table:
    - Priority: Medium (3)
    - user_relationships: `["commenter"]`
    - RequiresAction: false
  - Add `"commenter"` to user_relationships valid values
  - Verify: EFA document reflects new event type

## Model Changes
- [ ] **Add event type constant** (`internal/models/event.go`)
  - Add `EventTypeIssueCommentAuthor EventType = "issue_comment_author"` after line 66
  - Add to `validEventTypes` map after line 108
  - Verify: `EventType.IsValid()` returns true for new constant

## GraphQL Fetcher
- [ ] **Add fetch method** (`internal/datasources/github/graphql_fetcher.go`)
  - Create `fetchIssueCommentAuthorGraphQL` function after line 820
  - Search query: `commenter:{username} is:issue updated:>=DATE`
  - Apply org filter if configured
  - Use two-phase pattern: `searchIssues()` then `fetchIssueFullContext()`
  - Filter by `since` timestamp
  - Set metadata:
    - `user_relationships`: `[]string{"commenter"}`
  - Set event fields:
    - Type: `models.EventTypeIssueCommentAuthor`
    - Title: `"You commented on: {issue title}"`
    - Priority: `models.PriorityMedium`
    - RequiresAction: `false`
  - Verify: Function follows same pattern as `fetchPRCommentMentionsGraphQL`

## DataSource Integration
- [ ] **Add parallel fetch task** (`internal/datasources/github/datasource.go`)
  - Add task to `tasks` slice in `Fetch()` method (around line 220)
  - Task name: `"issue comment author"`
  - Fetch function: `d.fetchIssueCommentAuthorGraphQL(ctx, gqlClient, ghCred, opts.Since, d.orgs)`
  - Verify: Task runs concurrently with other searches

## Tests
- [ ] **Unit test for fetch method** (`internal/datasources/github/graphql_fetcher_test.go`)
  - Test successful fetch with valid search results
  - Test empty results
  - Test metadata includes `user_relationships: ["commenter"]`
  - Test priority is `PriorityMedium`
  - Test title format: "You commented on: {title}"
  - Verify: All events pass `Validate()`

- [ ] **Integration test** (`tests/integration/github_datasource_test.go`)
  - Mock GraphQL search returning issue comment results
  - Verify events appear in `FetchResult.Events`
  - Verify deduplication works with other issue event types
  - Verify: Event count matches expected results

## Verification
- [ ] **Build passes**: `make build`
- [ ] **Tests pass**: `make test`
- [ ] **Lint passes**: `make lint`
- [ ] **Manual test**: Run `./bin/kora digest --since 24h` with GitHub auth configured
- [ ] **Event validation**: All returned events pass `Event.Validate()`
