# Git Changelog Aggregation Research: Morning Digest Tool

## Key Facts

- **Primary Method**: go-git library for local repos (fast, no subprocess)
- **Fallback**: GitHub API for remote-only repos
- **Auth**: SSH keys for local clones, gh CLI for API access
- **Time Handling**: Git stores UTC, display in local timezone

## Dependencies

- Local clones for go-git approach
- SSH keys configured for private repos
- gh CLI authenticated for GitHub API fallback
- Config file listing repos to track

## Implementation Notes

- Use `--no-merges` to filter out merge commits (reduces noise)
- Author vs Committer: filter by author for "who did the work"
- Shallow clones (`--depth`) speed up initial fetch but may miss history
- Process repos concurrently with worker pool
- Time zone explicit in config (e.g., "America/Los_Angeles")

## Decision Points

1. Which repos to track? (config file)
2. Local-only or include remote repos via API?
3. Exclude merge commits?
4. Filter by specific author or show all?
5. Include commit stats (files changed, +/- lines)?

---

## Overview

Research findings for aggregating commit changelogs from multiple repositories since a specific time (e.g., 5pm PST previous day).

## Approaches Comparison

### 1. GitHub API (Remote)

**Method:** Use `gh api` or `google/go-github` to fetch commits

**Pros:**
- No local clones needed
- Works for repos you don't have locally
- Can get PR associations

**Cons:**
- Network dependent
- Rate limits apply (5000/hour)
- Slower for many repos

**Best For:** Remote-only workflows, repos not cloned locally

### 2. Git CLI (Local)

**Method:** Execute `git log` commands via subprocess

**Pros:**
- Familiar commands
- Fast for local repos
- No API limits

**Cons:**
- Requires local clones
- Text parsing fragile
- Platform differences in output

**Best For:** Quick implementation with existing clones

### 3. go-git Library (Local)

**Method:** Use `github.com/go-git/go-git/v5` pure Go library

**Pros:**
- Pure Go (no subprocess)
- Type-safe, no text parsing
- Fast and concurrent-safe
- Cross-platform consistent

**Cons:**
- Requires local clones
- Learning curve
- No PR metadata

**Best For:** Production tool with local repos

### Recommended: Hybrid Approach

- **Primary:** go-git for local repos (fast, reliable)
- **Fallback:** GitHub API for remote-only repos
- **Optional:** GitHub API to enrich with PR associations

## Git Operations Needed

### 1. List Commits Since Date

**Git CLI:**
```bash
git log --since="2025-12-04T17:00:00" --pretty=format:"%H|%an|%ae|%at|%s"
```

**Output Fields:**
- `%H` - Full commit hash
- `%h` - Short hash
- `%an` - Author name
- `%ae` - Author email
- `%at` - Author timestamp (Unix)
- `%s` - Subject (first line)
- `%b` - Body

### 2. Filter by Author

**Git CLI:**
```bash
git log --since="DATE" --author="email@example.com"
```

**Use:** Optional filter to see only your commits

### 3. Get Commit Stats

**Git CLI:**
```bash
git log --since="DATE" --stat --pretty=format:"..."
```

**Stats Available:**
- Files changed count
- Lines added
- Lines deleted

### 4. Clone/Fetch Repository

**For new repos:** Clone with shallow depth for speed
**For existing:** Fetch latest changes

## GitHub API Alternative

### List Commits

**gh CLI:**
```bash
gh api repos/OWNER/REPO/commits \
  --jq '.[] | {sha, author: .commit.author, message: .commit.message}' \
  -f since=2025-12-04T17:00:00Z
```

**API Endpoint:** `GET /repos/{owner}/{repo}/commits`

**Parameters:**
- `since` - ISO 8601 timestamp
- `until` - ISO 8601 timestamp
- `author` - Filter by author (login or email)
- `sha` - Branch/commit to start from

**Rate Limit:** 5000/hour, ~30 requests/minute for bursts

## Data Available

### Commit Object
```json
{
  "hash": "a1b2c3d4...",
  "shortHash": "a1b2c3d",
  "author": "John Doe",
  "email": "john@example.com",
  "timestamp": "2025-12-05T08:30:00Z",
  "subject": "feat: add new feature",
  "body": "Extended description...",
  "filesChanged": 5,
  "additions": 150,
  "deletions": 30
}
```

### Repository Config
```yaml
name: "project-name"
local_path: "~/code/project"       # Optional: local clone path
remote: "git@github.com:org/repo"  # Git remote URL
branch: "main"                     # Branch to track
```

## Important Gotchas

### 1. Time Zone Handling
- Git stores timestamps in UTC
- Display in user's local timezone
- Be explicit about timezone in config (e.g., "America/Los_Angeles")

### 2. Merge Commits
- May want to exclude merge commits
- Git: `--no-merges` flag
- Reduces noise in changelog

### 3. Author vs Committer
- Author: who wrote the code
- Committer: who applied the commit
- Usually same, differ for rebases/cherry-picks
- Filter by author for "who did the work"

### 4. Branch Tracking
- Ensure fetching the right branch
- Default branch may not be "main"
- Consider tracking multiple branches

### 5. Shallow Clones
- Can use `--depth` for faster clones
- May miss commits if depth too shallow
- Full clone needed for complete history

### 6. Submodules
- Submodule commits are separate
- May need to recurse into submodules
- Or treat submodule update as single change

### 7. Large Repos
- Very active repos may have many commits
- Consider pagination/limits
- Stats calculation can be slow

### 8. Private Repos
- SSH key or token needed for clone/fetch
- gh CLI handles auth automatically
- go-git can use SSH agent

## Configuration Structure

```yaml
# Repositories to track
repositories:
  - name: "internal-dev"
    local_path: "~/code/internal-dev"
    remote: "git@github.com:chainguard-dev/internal-dev.git"
    branch: "main"

  - name: "mono"
    local_path: "~/code/mono"
    remote: "git@github.com:chainguard-dev/mono.git"
    branch: "main"

  - name: "images"
    local_path: "~/code/images"
    remote: "git@github.com:chainguard-images/images.git"
    branch: "main"

# Time settings
time:
  since: "5:00PM"
  timezone: "America/Los_Angeles"

# Filters
filters:
  exclude_merges: true
  author: ""  # Empty = all authors

# Processing
concurrency: 5  # Parallel repo processing
```

## Recommended Approach

### For Morning Digest

1. **Configure Repos**
   - List repos to track in config file
   - Specify local paths for cloned repos
   - Optionally specify remote for clone-on-demand

2. **Update Repos**
   - Fetch latest from remotes
   - Handle auth via SSH or gh CLI

3. **Aggregate Commits**
   - Get commits since cutoff time
   - Process repos concurrently
   - Merge and sort by timestamp

4. **Format Output**
   - Group by repository
   - Show: hash, author, subject, timestamp
   - Optional: files changed, additions/deletions

### Data to Collect

Per commit:
- Short hash (for reference)
- Author name
- Subject line (first line of message)
- Timestamp
- Stats (optional: +/- lines)

Per repo:
- Repo name
- Commit count
- Active authors

## Output Format

### Text (Terminal)
```
Morning Changelog - Thu Dec 5, 2025
Since: Wed Dec 4, 5:00 PM PST
================================

[internal-dev] 5 commits
  a1b2c3d | john | feat: add authentication
  b2c3d4e | jane | fix: resolve race condition
  c3d4e5f | john | docs: update README
  ...

[mono] 12 commits
  ...

Total: 17 commits across 2 repositories
```

### Markdown
```markdown
# Morning Changelog
**Period:** Wed Dec 4, 5:00 PM to Thu Dec 5, 8:00 AM

## internal-dev (5 commits)

- `a1b2c3d` feat: add authentication - *john*
- `b2c3d4e` fix: resolve race condition - *jane*
...
```

## Go Libraries

### go-git (github.com/go-git/go-git/v5)
- Pure Go Git implementation
- Clone, fetch, log operations
- No external dependencies

### google/go-github
- GitHub REST API client
- Commits endpoint
- Need for PR associations

### spf13/viper
- Configuration management
- YAML/JSON/TOML support
- Environment variable binding

## Integration with Digest Tool

### Outputs Needed
- Total commit count
- Per-repo commit counts
- List of commits with details
- Optional: most active authors

### Priority in Digest
- Usually lower priority than mentions/reviews
- Good for "what happened overnight" context
- Can be collapsed/summarized

---

**Document Status**: Research complete
**Last Updated**: 2025-12-05
**Auth Type**: SSH keys / gh CLI for git operations
