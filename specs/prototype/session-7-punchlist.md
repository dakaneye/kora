# Session 7: Implement Output Formatters

## Agent Requirements (MANDATORY)

**YOU MUST invoke the following agents using the Task tool. Do NOT implement directly.**

| Agent | Invoke For | Task Tool Call |
|-------|------------|----------------|
| `golang-pro` | Formatter implementations | `Task(subagent_type="golang-pro", prompt="...")` |
| `test-automator` | Format verification tests | `Task(subagent_type="test-automator", prompt="...")` |

### Pre-Flight Checklist
- [ ] I will use Task tool to invoke `golang-pro` for implementation
- [ ] I will use Task tool to invoke `test-automator` for tests
- [ ] I will NOT write Go code directly

---

## Objective
Create output formatters for terminal (pretty), markdown, and JSON formats.

## Dependencies
- Session 2 complete (Event model exists)

## Files to Create
```
internal/output/formatter.go       # Formatter interface
internal/output/terminal.go        # Terminal with colors and tables
internal/output/markdown.go        # Markdown format
internal/output/json.go            # JSON format
internal/output/helpers.go         # Shared utilities
internal/output/formatter_test.go  # Format verification tests
```

---

## Task 1: Invoke golang-pro Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="golang-pro",
  prompt="""
Implement output formatters for Kora.

1. internal/output/formatter.go:
   - Formatter interface: Format(events []models.Event, stats *FormatStats) string
   - FormatStats struct: TotalEvents, RequiresAction, ExecutionTime, PartialSuccess, SourceErrors
   - NewFormatter(format string) (Formatter, error) factory function
   - Supported formats: "terminal", "markdown", "json"

2. internal/output/helpers.go:
   - relativeTime(t time.Time) string: "2 hours ago", "yesterday", "3 days ago"
   - groupByPriority(events []models.Event) map[models.Priority][]models.Event
   - sortByTimestamp(events []models.Event) []models.Event
   - priorityLabel(p models.Priority) string: "Critical", "High", "Medium", "Low", "Info"

3. internal/output/terminal.go:
   - TerminalFormatter struct with width and noColor options
   - NewTerminalFormatter(opts...) with functional options
   - WithWidth(int), WithNoColor(bool) options
   - Format() produces:
     - Box-drawing header with date/time
     - Events grouped by Priority (1 first)
     - Color coding: Priority 1-2 = Red, 3 = Yellow, 4-5 = Default
     - RequiresAction events in bold
     - Summary line at bottom
   - Use fatih/color or similar for colors

4. internal/output/markdown.go:
   - MarkdownFormatter struct
   - NewMarkdownFormatter() constructor
   - Format() produces valid GitHub-flavored markdown:
     - # Morning Digest header with date
     - ## Requires Action section (checkboxes)
     - ## For Awareness section
     - Events with links, author, time
     - --- footer with generation info

5. internal/output/json.go:
   - JSONFormatter struct with pretty option
   - NewJSONFormatter(pretty bool) constructor
   - Format() produces:
     - JSON object with generated_at, stats, events array
     - Events match Event JSON tags
     - Pretty option adds indentation

Output examples are in specs/repository-layout.md under "Example Output".
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
Create comprehensive tests for Kora output formatters (internal/output/).

1. Create test fixtures:
   - Sample events covering all EventTypes
   - Events with different priorities
   - Events with RequiresAction true/false
   - Event with 100-char title (edge case)
   - Empty event list

2. internal/output/formatter_test.go:

   Factory Tests:
   - Test NewFormatter("terminal") returns TerminalFormatter
   - Test NewFormatter("markdown") returns MarkdownFormatter
   - Test NewFormatter("json") returns JSONFormatter
   - Test NewFormatter("invalid") returns error

   Terminal Tests:
   - Test Format() produces non-empty output
   - Test events grouped by priority
   - Test RequiresAction events appear first within priority
   - Test empty event list handled gracefully
   - Test noColor option disables ANSI codes

   Markdown Tests:
   - Test Format() produces valid markdown
   - Test links are properly formatted
   - Test checkboxes for RequiresAction items
   - Test header contains date
   - Test empty event list produces minimal output

   JSON Tests:
   - Test Format() produces valid JSON (unmarshal test)
   - Test generated_at field present
   - Test stats field present
   - Test events array present
   - Test pretty=true adds indentation
   - Test pretty=false is compact

   Helper Tests:
   - Test relativeTime() for various durations
   - Test groupByPriority() correct grouping
   - Test priorityLabel() returns correct strings

Target >80% coverage.
"""
)
```

---

## Output Format Examples

### Terminal
```
╔═══════════════════════════════════════════════════════════════════════╗
║ Morning Digest - December 6, 2025 9:00 AM PST                        ║
╠═══════════════════════════════════════════════════════════════════════╣
║ Priority 1 - Requires Action (3 items)                               ║
╠═══════════════════════════════════════════════════════════════════════╣
║ [PR Review] Add secure rebuild for core-java                         ║
║ │ github.com/chainguard-dev/internal-dev/pulls/1234                  ║
║ │ @teammate1 • 2 hours ago                                           ║
╚═══════════════════════════════════════════════════════════════════════╝

Summary: 3 items require action, 5 for awareness
```

### Markdown
```markdown
# Morning Digest - December 6, 2025

## Requires Action (3 items)

- [ ] **[Add secure rebuild for core-java](https://github.com/...)**
  - Author: @teammate1 • 2 hours ago
  - Repository: chainguard-dev/internal-dev

## For Awareness (5 items)

...
```

### JSON
```json
{
  "generated_at": "2025-12-06T09:00:00Z",
  "stats": {
    "total_events": 8,
    "requires_action": 3
  },
  "events": [...]
}
```

---

## Definition of Done
- [ ] Formatter interface defined
- [ ] TerminalFormatter with colors and grouping
- [ ] MarkdownFormatter with valid GFM output
- [ ] JSONFormatter with valid JSON output
- [ ] All formatters handle empty event lists
- [ ] Events grouped by RequiresAction and Priority
- [ ] Test coverage >80%
- [ ] `make test` passes

## Next Session
Session 8: Implement CLI Commands
