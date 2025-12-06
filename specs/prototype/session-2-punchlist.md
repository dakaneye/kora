# Session 2: Implement Core Models (EFA 0001)

## Agent Requirements (MANDATORY)

**YOU MUST invoke the following agents using the Task tool. Do NOT implement directly.**

| Agent | Invoke For | Task Tool Call |
|-------|------------|----------------|
| `golang-pro` | Event model, types, validation | `Task(subagent_type="golang-pro", prompt="...")` |
| `test-automator` | Comprehensive test suites | `Task(subagent_type="test-automator", prompt="...")` |

### Pre-Flight Checklist
- [ ] I will use Task tool to invoke `golang-pro` for model implementation
- [ ] I will use Task tool to invoke `test-automator` for test creation
- [ ] I will NOT write Go code directly
- [ ] I have read EFA 0001 before starting

---

## Objective
Implement Event model, EventType, Source, Priority, Person, and validation per EFA 0001. This is the foundation for all datasources and formatters.

## Dependencies
- Session 1 complete (project structure exists)

## Files to Create
```
internal/models/event.go           # Event struct, EventType, Source constants
internal/models/priority.go        # Priority type and calculation
internal/models/person.go          # Person struct
internal/models/validation.go      # Validate() implementation
internal/models/event_test.go      # Table-driven validation tests
internal/models/priority_test.go   # Priority assignment tests
internal/models/testutil/helpers.go  # Test helpers: AssertValidEvent()
```

---

## Task 1: Invoke golang-pro Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="golang-pro",
  prompt="""
Implement the Event model for Kora per EFA 0001 (specs/efas/0001-event-model.md).

Read the EFA first, then implement:

1. internal/models/event.go:
   - Event struct with fields: Type, Title, Source, URL, Author, Timestamp, Priority, RequiresAction, Metadata
   - EventType constants: pr_review, pr_mention, issue_mention, issue_assigned, slack_dm, slack_mention
   - Source constants: github, slack
   - validEventTypes and validSources maps
   - IsValid() methods for EventType and Source
   - Add file header: // Ground truth defined in specs/efas/0001-event-model.md

2. internal/models/priority.go:
   - Priority type (int)
   - Constants: PriorityCritical(1), PriorityHigh(2), PriorityMedium(3), PriorityLow(4), PriorityInfo(5)
   - IsValid() method checking range 1-5

3. internal/models/person.go:
   - Person struct with Name (optional) and Username (required)

4. internal/models/validation.go:
   - Validate() method on Event checking:
     - Type is valid EventType
     - Title is 1-100 characters
     - Source is valid Source
     - URL is valid absolute URL or empty
     - Author.Username is non-empty
     - Timestamp is non-zero
     - Priority is 1-5
     - Metadata keys are from allowed set per source
   - allowedMetadataKeys map per EFA 0001:
     - GitHub: repo, number, state, review_state, labels
     - Slack: workspace, channel, thread_ts, is_thread_reply
   - validateURL() and validateMetadataKeys() helper methods

Include EFA protection comments: // IT IS FORBIDDEN TO CHANGE without updating EFA 0001
"""
)
```

---

## Task 2: Invoke test-automator Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="test-automator",
  prompt="""
Create comprehensive tests for the Kora Event model (internal/models/).

The model is defined per EFA 0001. Create:

1. internal/models/event_test.go:
   - Table-driven tests for Validate() covering all validation rules:
     - Valid event passes
     - Empty title fails
     - Title > 100 chars fails
     - Invalid EventType fails
     - Invalid Source fails
     - Invalid URL fails
     - Empty Author.Username fails
     - Zero Timestamp fails
     - Priority outside 1-5 fails
     - Invalid metadata keys fail
   - Test each EventType constant
   - Test each Source constant

2. internal/models/priority_test.go:
   - Test IsValid() for priorities 1-5 (valid)
   - Test IsValid() for 0, 6, -1 (invalid)
   - Test priority constants have correct values

3. internal/models/testutil/helpers.go:
   - AssertValidEvent(t, Event) helper
   - AssertMetadataKeys(t, Event, []string) helper
   - NewTestEvent() factory that creates a valid event for testing

Target >80% coverage on internal/models package.
Use table-driven tests where appropriate.
"""
)
```

---

## EFA 0001 Key Constraints

### Event Structure (PROTECTED)
```go
type Event struct {
    Type           EventType      `json:"type"`
    Title          string         `json:"title"`
    Source         Source         `json:"source"`
    URL            string         `json:"url"`
    Author         Person         `json:"author"`
    Timestamp      time.Time      `json:"timestamp"`
    Priority       Priority       `json:"priority"`
    RequiresAction bool           `json:"requires_action"`
    Metadata       map[string]any `json:"metadata,omitempty"`
}
```

### Allowed Metadata Keys
- **GitHub**: `repo`, `number`, `state`, `review_state`, `labels`
- **Slack**: `workspace`, `channel`, `thread_ts`, `is_thread_reply`

### Priority Assignment Rules
| Condition | Priority |
|-----------|----------|
| PR review requested + blocking | 1 (Critical) |
| PR review requested | 2 (High) |
| Direct message / DM | 2 (High) |
| @mention in issue/PR/channel | 3 (Medium) |
| Issue assigned | 3 (Medium) |
| Thread reply | 4 (Low) |
| FYI / informational | 5 (Info) |

---

## Definition of Done
- [ ] All Event fields implemented per EFA 0001
- [ ] Validate() enforces all EFA 0001 validation rules
- [ ] All EventType and Source constants defined
- [ ] Test coverage >80% on `internal/models`
- [ ] File headers include EFA reference comment
- [ ] `make test` passes

## Next Session
Session 3: Implement Auth Providers (EFA 0002)
