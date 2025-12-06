# Morning Digest Tool - Implementation Guide

## Key Facts

- **Purpose**: Aggregate overnight activity into a prioritized morning briefing
- **Language**: Go (CLI tool)
- **Authentication**: Enterprise/corporate accounts with SSO support
- **Architecture**: Modular datasources with unified output format
- **Repository**: github.com/dakaneye/dakassistant (private)

## Datasource Priority (Implementation Order)

| Priority | Datasource | Document | Why This Order |
|----------|------------|----------|----------------|
| 1 | GitHub PRs | `./research/github-prs.md` | Review requests block others; highest urgency |
| 2 | GitHub Issues | `./research/github-issues.md` | Direct mentions need response |
| 3 | Slack | `./research/slack.md` | DMs and mentions may be urgent |
| 4 | Gmail | `./research/email.md` | Work email requiring attention |
| 5 | Git Changelog | `./research/git-changelog.md` | Context on overnight changes |
| 6 | Hooks | `./research/hooks.md` | Optional: track Claude work sessions |

## Datasource Documents

| Document | Description |
|----------|-------------|
| `./research/github-prs.md` | GitHub Pull Request review requests, mentions, and reviews |
| `./research/github-issues.md` | GitHub Issue mentions and assignments |
| `./research/slack.md` | Slack DMs and @mentions via User Token |
| `./research/email.md` | Gmail unread/important emails via OAuth |
| `./research/git-changelog.md` | Multi-repo commit aggregation |
| `./research/hooks.md` | Claude Code accomplishment tracking for quarterly reviews |

## Authentication Summary

| Datasource | Auth Method | Storage |
|------------|-------------|---------|
| GitHub PRs | gh CLI | Already configured |
| GitHub Issues | gh CLI | Already configured |
| Slack | User token (xoxp-*) | Keychain/env var |
| Gmail | OAuth 2.0 | Token file |
| Git | SSH keys | Already configured |

## Rate Limit Summary

| Datasource | Limit | Notes |
|------------|-------|-------|
| GitHub Search | 30/minute | Stricter than REST API |
| GitHub REST | 5000/hour | Generous for digest use |
| Slack | ~50/minute | Tier 3 for most methods |
| Gmail | 250 units/sec | Effectively unlimited |
| Git (local) | None | Local operations |

## Decision Points for User

Before implementation, ask:

1. Which datasources to enable?
2. What time window? (default: 5PM previous day PST)
3. Output format preference? (terminal, markdown, JSON)
4. Token storage location? (keychain, env vars, encrypted file)
5. Which GitHub orgs/repos to track?
6. Which Slack workspaces? (if multiple)
7. Gmail account? (personal vs workspace)

## Integration Points

All datasources should produce standardized output:

- **Type**: Category (pr_review, issue_mention, dm, email, commit)
- **Title**: Brief description
- **Source**: Origin (github, slack, gmail, git)
- **URL**: Direct link
- **Author**: Who created/sent it
- **Timestamp**: When it happened
- **Priority**: 1-5 (lower = higher priority)
- **RequiresAction**: Does this need response?

## Implementation Notes

- Use context.Context for cancellation and timeouts
- Process datasources concurrently
- Fail gracefully if one datasource errors
- Cache tokens appropriately
- Log verbose output only with --verbose flag

## Error Handling Strategy

- **Auth failures**: Prompt to re-authenticate
- **Rate limits**: Exponential backoff, then warn user
- **Network errors**: Retry with timeout, then skip source
- **Partial failures**: Show what succeeded with warning

## Performance Targets

- Total execution: < 10 seconds
- Per datasource: < 3 seconds each
- Concurrent execution for independent sources

---

**Document Status**: Ready for implementation
**Last Updated**: 2025-12-05
