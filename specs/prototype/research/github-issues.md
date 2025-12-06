# GitHub Issues Integration Research: Morning Digest Tool

## Key Facts

- **Auth**: gh CLI (handles enterprise SSO automatically)
- **Rate Limit**: Search API 30/min, REST API 5000/hour
- **Primary Query**: `mentions:@me is:open updated:>=DATE`
- **Use `involves:` for broader match**: author/assignee/mention/commenter

## Dependencies

- `gh` CLI installed and authenticated
- Enterprise SSO must be active

## Implementation Notes

- Use `updated:` not `created:` for overnight activity
- Filter out bot-generated mentions if noisy
- Deduplicate issues appearing in multiple searches
- `@me` works in gh CLI as username placeholder
- Prefer single searches over many API calls (search limit is stricter)

## Decision Points

1. Include closed issues with recent updates?
2. Which repos/orgs to search?
3. Filter by specific labels?
4. Include issues you authored with new comments?

---

## Overview

Research findings for integrating GitHub Issues into a morning digest tool, using enterprise/corporate GitHub account with direct user authentication.

## Authentication

### gh CLI (Recommended)

**Why gh CLI:**
- Already authenticated via `gh auth login`
- Handles token refresh automatically
- Works with SSO/Enterprise authentication
- No token management in application

**Enterprise Considerations:**
- SSO authentication handled by gh CLI
- SAML re-authentication prompted automatically
- Works with GitHub Enterprise Server and Cloud

**Verify Auth:**
```bash
gh auth status
```

### Alternative: Personal Access Token

**Classic Token Scopes:**
- `repo` - Full control of private repositories
- `read:org` - Read org membership

**Fine-grained Token:**
- Repository access: Select specific repos
- Permissions: Issues (read), Metadata (read)

**Enterprise Note:**
- Some orgs require SSO authorization for tokens
- Run `gh auth refresh` after SSO timeout

## Key API Operations

### 1. Search Issues - Mentions

**Query:** `mentions:USERNAME updated:>=DATE`

**gh CLI:**
```bash
gh search issues "mentions:USERNAME updated:>=2025-12-04" \
  --owner ORG \
  --state open \
  --json number,title,url,repository,updatedAt \
  --limit 100
```

**Capabilities:**
- Cross-repo search within org
- Filter by state (open/closed)
- Filter by date
- Returns structured JSON

### 2. Search Issues - Assigned

**Query:** `assignee:USERNAME updated:>=DATE`

**gh CLI:**
```bash
gh search issues "assignee:USERNAME updated:>=DATE" \
  --owner ORG \
  --json number,title,url,repository,updatedAt,state
```

### 3. Search Issues - New Issues

**Query:** `created:>=DATE`

**gh CLI:**
```bash
gh search issues "created:>=DATE" \
  --owner ORG \
  --repo REPO \
  --json number,title,url,repository,createdAt
```

**Use:** Track new issues in specific repos you watch.

### 4. Get Issue Comments

**gh API:**
```bash
gh api repos/OWNER/REPO/issues/NUMBER/comments --paginate
```

**Returns:**
- Comment ID
- Author login
- Body text
- Created timestamp
- HTML URL (permalink)

### 5. Get Issue Details

**gh CLI:**
```bash
gh issue view NUMBER --repo OWNER/REPO --json number,title,body,state,author,assignees,labels,comments
```

## GitHub Search Qualifiers

### User-Related
```
mentions:USERNAME          # Mentioned in issue/comments
assignee:USERNAME          # Assigned to user
author:USERNAME            # Created by user
commenter:USERNAME         # User commented on
involves:USERNAME          # Any involvement (author/assignee/mention/commenter)
```

### Date Filters
```
created:>=2025-12-04       # Created on or after
updated:>=2025-12-04       # Updated on or after
closed:>=2025-12-04        # Closed on or after
created:2025-12-04..2025-12-05  # Date range
```

### State and Type
```
is:open                    # Open issues
is:closed                  # Closed issues
is:issue                   # Issues only (not PRs)
type:issue                 # Alternative syntax
```

### Organization/Repository
```
org:ORG-NAME              # Within organization
repo:OWNER/REPO           # Specific repository
user:USERNAME             # User's repos
```

### Labels and Milestones
```
label:bug                  # Has label
-label:wontfix            # Doesn't have label
milestone:"v1.0"          # In milestone
no:milestone              # No milestone assigned
```

### Combined Examples
```
mentions:@me is:open updated:>=2025-12-04
assignee:@me is:open org:chainguard-dev
involves:@me is:open -author:@me updated:>=2025-12-04
label:urgent assignee:@me is:open
```

## Rate Limits

### REST API
- **Authenticated:** 5,000 requests/hour
- **With GITHUB_TOKEN:** 5,000 requests/hour

### Search API (More Restrictive)
- **Authenticated:** 30 requests/minute
- **Applies to:** `gh search issues` commands

### gh CLI Rate Limit Handling
- Automatic retry on rate limit
- Shows warning when approaching limit

### Typical Digest Load
- 1-3 search queries: ~3 requests
- 10-20 issue detail fetches: ~20 requests
- **Total:** ~25 requests, well within limits

## Data Available

### Issue Object (Search)
```json
{
  "number": 123,
  "title": "Issue title",
  "url": "https://github.com/...",
  "repository": {
    "name": "repo-name",
    "nameWithOwner": "org/repo-name"
  },
  "state": "open",
  "createdAt": "2025-12-04T17:00:00Z",
  "updatedAt": "2025-12-05T08:00:00Z",
  "author": {"login": "username"},
  "assignees": [{"login": "username"}],
  "labels": [{"name": "bug"}]
}
```

### Comment Object
```json
{
  "id": 12345,
  "user": {"login": "username"},
  "body": "Comment text...",
  "created_at": "2025-12-05T08:00:00Z",
  "html_url": "https://github.com/.../issues/123#issuecomment-12345"
}
```

### Available JSON Fields (gh search)
- `number`, `title`, `url`, `state`
- `repository` (name, nameWithOwner)
- `author`, `assignees`
- `labels`, `milestone`
- `createdAt`, `updatedAt`, `closedAt`
- `body` (full text)
- `comments` (count)

## Important Gotchas

### 1. Enterprise SSO
- gh CLI handles SSO seamlessly
- May prompt for re-authentication
- Token expiration varies by org policy

### 2. Search vs API Limits
- Search API: 30/minute (stricter)
- REST API: 5000/hour (generous)
- Prefer single searches over many API calls

### 3. Private Repositories
- User token inherits user's repo access
- Can't see repos user doesn't have access to
- Enterprise may restrict API access to specific repos

### 4. Organization-Wide Search
- Use `--owner ORG` to search all org repos
- Use `--repo OWNER/REPO` for specific repo
- Cannot search across multiple orgs in single query

### 5. Updated vs Created
- `updated:` includes any activity (comments, labels, etc.)
- `created:` is only the initial creation date
- For digest, usually want `updated:`

### 6. Pagination
- Default limit varies by command
- Use `--limit N` to control
- For API calls, use `--paginate` flag

### 7. Bot Comments
- Search returns all mentions including bots
- May want to filter out bot-generated mentions
- Check author login for bot patterns

## Recommended Approach

### For Morning Digest

1. **Mentions (Highest Priority)**
   - Query: `mentions:@me is:open updated:>=YESTERDAY_5PM`
   - Things requiring your attention

2. **Assigned Issues (High Priority)**
   - Query: `assignee:@me is:open updated:>=YESTERDAY_5PM`
   - Your work items with activity

3. **New Issues in Watched Repos (Medium Priority)**
   - Query per repo: `created:>=YESTERDAY_5PM`
   - New issues you might care about

### Data to Collect
- Issue number and title
- Repository name
- Last update timestamp
- Direct link (URL)
- Type of involvement (mention/assigned/new)

### Deduplication
- Same issue may appear in multiple searches
- Use issue URL or `repo:number` as unique key

## Go Libraries

### os/exec with gh CLI
- Execute gh commands as subprocess
- Parse JSON output
- Leverage existing auth

### google/go-github
- Direct API access
- Type-safe Go structs
- Requires token management

### shurcooL/githubv4 (GraphQL)
- Efficient for complex queries
- Single request for nested data
- Steeper learning curve

## Integration with Digest Tool

### Outputs Needed
- Count of mentions needing response
- Count of assigned issues with updates
- Count of new issues in watched repos
- List with: title, repo, URL, type

### Priority Ordering
1. Mentions (action needed)
2. Assigned issue updates
3. New issues in watched repos

---

**Document Status**: Research complete
**Last Updated**: 2025-12-05
**Auth Type**: gh CLI (enterprise SSO supported)
