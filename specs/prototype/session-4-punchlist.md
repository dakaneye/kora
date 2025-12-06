# Session 4: Implement DataSource Interface (EFA 0003)

## Agent Requirements (MANDATORY)

**YOU MUST invoke the following agents using the Task tool. Do NOT implement directly.**

| Agent | Invoke For | Task Tool Call |
|-------|------------|----------------|
| `golang-pro` | DataSource interface, runner | `Task(subagent_type="golang-pro", prompt="...")` |
| `test-automator` | Concurrency and error tests | `Task(subagent_type="test-automator", prompt="...")` |

### Pre-Flight Checklist
- [ ] I will use Task tool to invoke `golang-pro` for implementation
- [ ] I will use Task tool to invoke `test-automator` for tests
- [ ] I will NOT write Go code directly
- [ ] I have read EFA 0003 before starting

---

## Objective
Create DataSource interface, FetchOptions, FetchResult, and DataSourceRunner for concurrent execution.

## Dependencies
- Session 3 complete (AuthProvider exists)
- Session 2 complete (Event model exists)

## Files to Create
```
internal/datasources/datasource.go     # DataSource interface
internal/datasources/options.go        # FetchOptions, FetchFilter
internal/datasources/result.go         # FetchResult, FetchStats
internal/datasources/errors.go         # Sentinel errors
internal/datasources/runner.go         # DataSourceRunner with errgroup
internal/datasources/runner_test.go    # Concurrency and error tests
```

---

## Task 1: Invoke golang-pro Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="golang-pro",
  prompt="""
Implement the DataSource interface for Kora per EFA 0003 (specs/efas/0003-datasource-interface.md).

Read the EFA first, then implement:

1. internal/datasources/datasource.go:
   - DataSource interface: Name(), Service(), Fetch(ctx, FetchOptions)
   - Add file header: // Ground truth defined in specs/efas/0003-datasource-interface.md
   - Add EFA protection comment

2. internal/datasources/options.go:
   - FetchOptions struct: Since (required), Limit, Filter
   - FetchFilter struct: EventTypes, MinPriority, RequiresAction
   - Validate() method on FetchOptions (Since must not be zero, Limit >= 0)

3. internal/datasources/result.go:
   - FetchResult struct: Events, Partial, Errors, RateLimited, RateLimitReset, Stats
   - FetchStats struct: Duration, APICallCount, EventsFetched, EventsReturned
   - Helper methods: HasEvents(), HasErrors(), CombinedError()

4. internal/datasources/errors.go:
   - Sentinel errors: ErrNotAuthenticated, ErrRateLimited, ErrServiceUnavailable,
     ErrTimeout, ErrInvalidResponse

5. internal/datasources/runner.go:
   - DataSourceRunner struct with sources and timeout
   - Functional options: WithTimeout(duration)
   - NewRunner(sources, opts...) constructor
   - Run(ctx, FetchOptions) method using errgroup for concurrency
   - RunResult struct: Events (sorted), SourceResults, SourceErrors
   - Helper methods: Success(), Partial(), TotalEvents()

CRITICAL REQUIREMENTS per EFA 0003:
- Context MUST be used for all operations
- One datasource failure MUST NOT block others (partial success)
- All returned events MUST pass Event.Validate()
- Events MUST be sorted by Timestamp ascending

Implementation pattern for Run():
- Create channel for results
- Launch goroutine for each datasource with per-source timeout
- Collect results, aggregate events, track errors
- Sort combined events by timestamp
- Return even if some sources fail
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
Create comprehensive tests for Kora DataSource system (internal/datasources/).

1. Create MockDataSource for testing:
   - Configurable: name, service, events to return, error to return, delay
   - Implements DataSource interface

2. internal/datasources/runner_test.go:

   Concurrent Execution Tests:
   - Test multiple datasources run concurrently (verify timing)
   - Test context cancellation propagates to all sources
   - Test per-datasource timeout works

   Partial Failure Tests:
   - Test: Source A succeeds, Source B fails -> get Source A events
   - Test: All sources fail -> empty events, errors populated
   - Test: All sources succeed -> all events, no errors
   - Test: Partial=true when some sources fail

   Event Handling Tests:
   - Test events are sorted by Timestamp ascending
   - Test events from multiple sources are merged correctly
   - Test invalid events are reported in Errors

   Options Tests:
   - Test FetchOptions.Validate() with zero Since (fails)
   - Test FetchOptions.Validate() with negative Limit (fails)
   - Test FetchOptions.Validate() with valid options (passes)

3. Test FetchResult helpers:
   - HasEvents() returns true/false correctly
   - HasErrors() returns true/false correctly
   - CombinedError() joins multiple errors

4. Test RunResult helpers:
   - Success() returns true when no errors
   - Partial() returns true when some succeed, some fail
   - TotalEvents() returns correct count

Use table-driven tests where appropriate.
Target >80% coverage on internal/datasources package.
"""
)
```

---

## EFA 0003 Key Constraints

### DataSource Interface (PROTECTED)
```go
type DataSource interface {
    Name() string
    Service() models.Source
    Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error)
}
```

### Critical Requirements
1. **Context**: ALL operations MUST use provided context
2. **Partial Success**: One failure MUST NOT block others
3. **Event Validation**: ALL returned events MUST pass Validate()
4. **Sorting**: Events MUST be sorted by Timestamp ascending

### Sentinel Errors
```go
var (
    ErrNotAuthenticated   = errors.New("datasource: not authenticated")
    ErrRateLimited        = errors.New("datasource: rate limited")
    ErrServiceUnavailable = errors.New("datasource: service unavailable")
    ErrTimeout            = errors.New("datasource: timeout")
    ErrInvalidResponse    = errors.New("datasource: invalid response")
)
```

---

## Definition of Done
- [ ] DataSource interface matches EFA 0003 exactly
- [ ] DataSourceRunner executes datasources concurrently
- [ ] Partial success works (one fails, others return)
- [ ] All errors collected in FetchResult.Errors
- [ ] Context cancellation propagates correctly
- [ ] Events sorted by timestamp
- [ ] Test coverage >80%
- [ ] `make test` passes

## Next Session
Session 5: Implement GitHub DataSource
