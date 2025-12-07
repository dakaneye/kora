# Kora Digest User Review Analysis

Analysis of 18 items from a 48-hour Kora digest to validate relevance and categorization accuracy.

## Executive Summary

| Category | Total | Actually Relevant | Accuracy |
|----------|-------|-------------------|----------|
| HIGH - PR Review Requests | 5 | 0 | 0% |
| MEDIUM - Assigned Issues | 5 | 5 | 100% |
| MEDIUM - Issue Mentions | 7 | 3 | 43% |
| MEDIUM - PR Mentions | 1 | 0 | 0% |
| **Total** | **18** | **8** | **44%** |

**Key Finding**: Over half (56%) of items were not directly relevant to the user.

---

## Detailed Analysis

### Pattern 1: Team-Based PR Review Requests (5 items - 0% relevant)

All 5 PR review requests were triggered by **team membership** in "Ecosystems Team", not direct requests.

| PR | Title | Why Not Relevant |
|----|-------|------------------|
| #29499 | fulcio bump in lifecycle/pkg | Dependabot auto-update, team-wide CODEOWNERS |
| #29479 | fulcio bump in api-internal | Dependabot auto-update, team-wide CODEOWNERS |
| #29492 | fulcio bump in ecosystems | Dependabot auto-update, team-wide CODEOWNERS |
| #29198 | advisories/list filter | Team review request, different feature area |
| #29466 | all-others group bump | Dependabot auto-update across 51 directories |

**Root Cause**: GitHub's `review-requested:@me` search includes team-based review requests. User is on `chainguard-dev/ecosystems-team` which is auto-added via CODEOWNERS for many directories.

**Suggested Filter**: Exclude dependabot PRs OR exclude team-based review requests (show only direct user requests).

---

### Pattern 2: Direct Issue Assignments (5 items - 100% relevant)

These issues have `assignee: dakaneye` explicitly set. All are JavaScript Libraries onboarding tickets.

| Issue | Customer | Status |
|-------|----------|--------|
| #24643 | Linklaters | Needs triage |
| #24617 | Sagesure | Needs triage |
| #24626 | gabb | Needs triage |
| #24579 | DWP | Active |
| #24673 | Collibra | Needs triage |

**Result**: Correctly identified as requiring action.

---

### Pattern 3: Issue Mentions - Mixed Relevance (7 items - 43% relevant)

| Issue | Mention Type | Relevant? | Reason |
|-------|--------------|-----------|--------|
| #24372 | **Assigned** | **YES** | Kora miscategorized - user is actually assignee (Retool NPM) |
| #21428 | **Assigned** | **YES** | Kora miscategorized - user is actually assignee (Foresite JS) |
| #24598 | **Assigned** | **YES** | Kora miscategorized - user is actually assignee (NScale JS) |
| #20681 | Direct request | **YES** | BriaGioCG asked to re-run coverage by Dec 9th |
| #21212 | Direct request | **YES** | angela-zhang asked to re-run coverage (Wingspan) |
| #24338 | Bot ping | Maybe | octo-sts bot asking for update |
| #11130 | Status update | No | Historical mention in status update, no action needed |

**Root Cause #1**: Kora's `mentions:@me` search finds issues where user is @mentioned in comments, but GitHub's `assignee:@me` search is separate. Some issues appear in mentions because the user's assignment triggered a notification.

**Root Cause #2**: Bot mentions (octo-sts) and historical status updates create noise.

**Suggested Filter**:
- Deduplicate: If user is assignee, don't also show as mention
- Filter bot mentions separately (lower priority)
- Distinguish "direct request" vs "informational mention"

---

### Pattern 4: PR Mentions (1 item - 0% relevant)

| PR | Title | Why Not Relevant |
|----|-------|------------------|
| #29304 | ALF3 analyzer infrastructure | **User's own PR** - author is dakaneye |

**Root Cause**: `mentions:@me type:pr` finds PRs where user is mentioned, but this includes the user's own PRs where they're mentioned in the body/comments.

**Suggested Filter**: Exclude PRs authored by the user.

---

## Recommendations for Kora v2

### High Priority Filters

1. **Exclude user's own PRs from mentions**
   - Add check: `author != currentUser` for PR mentions

2. **Deduplicate assigned vs mentioned**
   - If issue appears in `assignee:@me`, exclude from `mentions:@me` results
   - Or merge into single item with "assigned + mentioned" indicator

3. **Separate dependabot PRs**
   - Create separate category: "Dependency Updates (team)"
   - Or filter: `author != dependabot[bot]`

### Medium Priority Filters

4. **Distinguish team vs direct review requests**
   - GitHub API returns `reviewRequests` with `__typename: "Team"` vs `"User"`
   - Filter to only show direct user requests OR add "(team)" indicator

5. **Filter/categorize bot mentions**
   - Detect common bots: `octo-sts[bot]`, `dependabot[bot]`, etc.
   - Lower priority or separate category

### Low Priority Enhancements

6. **Detect "actionable" mentions**
   - NLP: Look for phrases like "could you", "@user -", "please", request patterns
   - vs informational: "update:", status reports

7. **Time-based relevance decay**
   - Old status update mentions (>30 days) should be filtered

---

## Corrected Relevance Assessment

### Actually Requires Action (8 items)

**Direct Assignments:**
- Issue #24643 - Linklaters JS (assigned)
- Issue #24617 - Sagesure JS (assigned)
- Issue #24626 - gabb JS (assigned)
- Issue #24579 - DWP JS (assigned)
- Issue #24673 - Collibra JS (assigned)
- Issue #24372 - Retool NPM (assigned, miscategorized as mention)
- Issue #21428 - Foresite JS (assigned, miscategorized as mention)
- Issue #24598 - NScale JS (assigned, miscategorized as mention)

**Actionable Mentions:**
- Issue #20681 - Forerunner JS (direct request to re-run)
- Issue #21212 - Wingspan JS (direct request to re-run)

### Noise (10 items)

**Team-based PR Reviews (not individually relevant):**
- PR #29499, #29479, #29492, #29198, #29466 - Ecosystems Team CODEOWNERS

**User's Own Work:**
- PR #29304 - ALF3 analyzer (user is author)

**Informational/Bot:**
- Issue #11130 - JS Closed Beta (historical status mention)
- Issue #24338 - T-Mobile (bot ping, maybe relevant)

---

## Data Collection Date
2025-12-06, 48-hour window
