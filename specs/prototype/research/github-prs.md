# GitHub Pull Requests Integration Research: Morning Digest Tool

## Key Facts

- **Auth**: gh CLI (handles enterprise SSO automatically)
- **Rate Limit**: Search API 30/min, REST API 5000/hour
- **Primary Query**: `review-requested:@me is:open -draft:true`
- **Three comment types**: issue comments, review comments, reviews

## Dependencies

- `gh` CLI installed and authenticated
- Enterprise SSO must be active (gh handles re-auth)

## Implementation Notes

- Always exclude draft PRs from review requests (`-draft:true`)
- Check all three comment types for complete mention coverage
- Use `--json` flag for structured output parsing
- Deduplicate PRs that appear in multiple searches by URL
- Use `@me` as username placeholder in gh CLI

## Decision Points

1. Include draft PRs in results?
2. How far back to search? (default: since 5PM previous day)
3. Which organizations to include?
4. Show PR stats (additions/deletions)?

---

## Overview

Research findings for integrating GitHub Pull Requests into a morning digest tool. PRs have unique features distinct from Issues including reviews, review requests, and multiple comment types.

## Authentication

Same as GitHub Issues - use `gh` CLI with existing enterprise authentication.

## Key Differences from Issues

| Feature | Issues | Pull Requests |
|---------|--------|---------------|
| Comments | Single type | 3 types: issue, review, line-level |
| Reviews | N/A | Approved, Changes Requested, Commented |
| Review Requests | N/A | Pending review assignments |
| Merge Status | N/A | Mergeable, conflicts, checks |
| Draft Status | N/A | Draft vs Ready for Review |
| Commits | N/A | Associated commits |
| CI Status | N/A | Check runs and statuses |

## Key API Operations

### 1. Search PRs - Review Requested

**Query:** `review-requested:USERNAME updated:>=DATE`

**gh CLI:**
```bash
gh search prs "review-requested:USERNAME updated:>=DATE" \
  --owner ORG \
  --state open \
  --json number,title,url,repository,updatedAt,author,isDraft
```

**Importance:** Highest priority - these need your action.

### 2. Search PRs - Mentions

**Query:** `mentions:USERNAME type:pr updated:>=DATE`

**gh CLI:**
```bash
gh search prs "mentions:USERNAME updated:>=DATE" \
  --owner ORG \
  --json number,title,url,repository,updatedAt
```

### 3. Search PRs - Your PRs with Activity

**Query:** `author:USERNAME updated:>=DATE`

**gh CLI:**
```bash
gh search prs "author:USERNAME is:open updated:>=DATE" \
  --owner ORG \
  --json number,title,url,repository,updatedAt,state
```

**Use:** See reviews/comments on your PRs.

### 4. Get PR Reviews

**gh API:**
```bash
gh api repos/OWNER/REPO/pulls/NUMBER/reviews --paginate
```

**Returns:**
- Review ID
- Reviewer login
- State: APPROVED, CHANGES_REQUESTED, COMMENTED, PENDING, DISMISSED
- Body text
- Submitted timestamp

### 5. Get PR Review Comments (Line-Level)

**gh API:**
```bash
gh api repos/OWNER/REPO/pulls/NUMBER/comments --paginate
```

**Returns:**
- Comment ID
- Author
- File path and line number
- Diff context
- Body text
- Created timestamp

### 6. Get PR Issue Comments (Conversation)

Same as Issues API:
```bash
gh api repos/OWNER/REPO/issues/NUMBER/comments --paginate
```

## PR-Specific Search Qualifiers

### Review Status
```
review:none                # No reviews yet
review:required            # Reviews required by branch protection
review:approved            # Has at least one approval
review:changes_requested   # Has changes requested
```

### Review Requests
```
review-requested:USERNAME  # Pending review from user
reviewed-by:USERNAME       # Has been reviewed by user
-reviewed-by:USERNAME      # Not yet reviewed by user
```

### PR State
```
is:open                    # Open PRs
is:closed                  # Closed PRs
is:merged                  # Merged PRs
is:unmerged                # Not yet merged (open or closed without merge)
draft:true                 # Draft PRs
draft:false                # Ready for review
```

### CI/Checks
```
status:success             # All checks passing
status:failure             # Checks failing
status:pending             # Checks in progress
```

### Combined Examples
```
review-requested:@me is:open -draft:true
author:@me is:open review:changes_requested
type:pr mentions:@me updated:>=2025-12-04
review-requested:@me org:chainguard-dev is:open
reviewed-by:@me is:open updated:>=2025-12-04
```

## Three Types of PR Comments

### 1. Issue Comments (Conversation Tab)
- General discussion on PR
- Same API as issue comments
- `repos/OWNER/REPO/issues/NUMBER/comments`

### 2. Review Comments (Line-Level)
- Comments on specific code lines
- Associated with a review
- `repos/OWNER/REPO/pulls/NUMBER/comments`

### 3. Reviews (Approval/Request Changes)
- Formal review submissions
- State: APPROVED, CHANGES_REQUESTED, COMMENTED
- `repos/OWNER/REPO/pulls/NUMBER/reviews`

## Rate Limits

Same as Issues:
- REST API: 5,000/hour
- Search API: 30/minute

Typical PR digest operations stay well within limits.

## Data Available

### PR Object (Search)
```json
{
  "number": 456,
  "title": "feat: add new feature",
  "url": "https://github.com/...",
  "repository": {"nameWithOwner": "org/repo"},
  "state": "open",
  "isDraft": false,
  "author": {"login": "username"},
  "createdAt": "...",
  "updatedAt": "...",
  "additions": 150,
  "deletions": 30,
  "changedFiles": 5
}
```

### Review Object
```json
{
  "id": 12345,
  "user": {"login": "reviewer"},
  "state": "APPROVED",
  "body": "LGTM!",
  "submitted_at": "2025-12-05T08:00:00Z",
  "html_url": "https://github.com/.../pull/456#pullrequestreview-12345"
}
```

### Review Comment Object
```json
{
  "id": 67890,
  "user": {"login": "reviewer"},
  "body": "Consider using...",
  "path": "src/main.go",
  "line": 42,
  "diff_hunk": "@@ -40,6 +40,8 @@...",
  "created_at": "...",
  "html_url": "..."
}
```

## Important Gotchas

### 1. Draft PRs
- May want to exclude from review requests
- Use `-draft:true` in search query
- Draft PRs shouldn't typically need reviews

### 2. Review vs Review Request
- `review-requested:` = pending review
- `reviewed-by:` = has submitted review
- A review clears the request

### 3. Multiple Comment Types
- Must check all 3 comment types for mentions
- Line comments are separate from issue comments
- Reviews themselves can contain body text

### 4. Mergeable State
- `mergeable` field may be null (not yet computed)
- Three states: MERGEABLE, CONFLICTING, UNKNOWN
- Check runs affect merge ability

### 5. Thread Context
- PR review comments can be in threads
- `in_reply_to_id` links to parent comment
- May need to fetch thread context

### 6. Resolved Conversations
- GitHub tracks resolved/unresolved state
- Not directly in REST API
- GraphQL API provides this data

### 7. Check Runs
- Separate from reviews
- Can affect merge ability
- Use `gh pr checks` to see status

## Recommended Approach

### For Morning Digest

1. **Review Requests (Highest Priority)**
   - Query: `review-requested:@me is:open -draft:true`
   - Action needed from you

2. **Reviews on Your PRs (High Priority)**
   - Query: `author:@me is:open updated:>=YESTERDAY`
   - Then fetch reviews for each PR
   - Filter reviews submitted since cutoff

3. **Mentions in PRs (Medium Priority)**
   - Query: `type:pr mentions:@me updated:>=YESTERDAY`
   - May need response

### Priority Ordering
1. Review requests (action blocked on you)
2. Changes requested on your PRs
3. Approvals on your PRs (ready to merge?)
4. Mentions in PRs

### Data to Collect
- PR number and title
- Repository
- Author (for review requests)
- Review state (for your PRs)
- Direct link
- Draft status

## Go Libraries

Same as Issues:
- gh CLI wrapper (recommended)
- google/go-github
- shurcooL/githubv4 (GraphQL)

## Integration with Digest Tool

### Outputs Needed
- Count of pending review requests
- Count of your PRs with new reviews
- Count of PR mentions
- List with details for each

### Display Priority
1. Review Requests (with author, additions/deletions)
2. Your PRs with changes requested
3. Your PRs with approvals
4. PR mentions

---

**Document Status**: Research complete
**Last Updated**: 2025-12-05
**Auth Type**: gh CLI (enterprise SSO supported)
