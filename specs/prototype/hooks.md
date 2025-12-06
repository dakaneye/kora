# Claude Code Hooks System: Accomplishment Tracking Tool

## Key Facts

- **Primary Hook**: PostToolUse (most reliable signal of completed work)
- **Input Method**: JSON via stdin, 60-second timeout per hook
- **Exit Codes**: 0=success, 2=blocking error, other=non-blocking error
- **Storage Recommended**: File-based JSONL (simple, no dependencies)

## Dependencies

- Claude Code CLI
- `jq` for JSON parsing in hook scripts
- Storage location with write access (~/.claude/accomplishments/)
- Bash shell for hook scripts

## Implementation Notes

- PostToolUse captures: Write, Edit, Bash tool completions with results
- SessionStart/SessionEnd for session boundaries and aggregation
- All matching hooks run in parallel (be aware of race conditions)
- Use matchers to target specific tools: `"matcher": "Write|Edit|Bash"`
- Hook scripts must complete quickly to not slow down Claude workflow

## Decision Points

1. Storage format: JSONL files or SQLite database?
2. Which tools to track? (Write, Edit, Bash, or more?)
3. Aggregation frequency: per-session, daily, both?
4. Include UserPromptSubmit for task intent extraction?
5. Quarterly review auto-generation or manual trigger?

---

## Overview

This document describes a comprehensive system for building an accomplishment tracking tool using Claude Code's hooks system. The tool will automatically record work accomplishments throughout the day and integrate with a morning digest system for quarterly review generation.

## Key Findings: Hook Types & Capabilities

### Available Hook Events

Claude Code provides 10 primary hook events that fire at different points in the workflow:

#### 1. **PreToolUse** (Before Tool Execution)
- **When it fires**: After Claude creates tool parameters, before processing the tool call
- **Data available**:
  - `tool_name` (Write, Edit, Bash, Read, Glob, Grep, etc.)
  - `tool_input` (varies by tool - file paths, commands, search patterns)
  - `session_id`, `transcript_path`, `cwd`, `permission_mode`
  - `tool_use_id` (unique identifier for this tool use)
- **Capabilities**: Can allow, deny, or ask user permission; can modify inputs before execution
- **Use for tracking**: Early signal that work is starting (file edits, commands)

#### 2. **PostToolUse** (After Tool Execution)
- **When it fires**: Immediately after a tool completes successfully
- **Data available**:
  - `tool_name`, `tool_input` (what was sent)
  - `tool_response` (the result of tool execution)
  - `session_id`, `transcript_path`, `cwd`, `tool_use_id`
- **Capabilities**: Provide feedback to Claude, log outcomes
- **Use for tracking**: Record completed actions with their results (best for recording accomplishments)

#### 3. **SessionStart** (Session Initialization)
- **When it fires**: When Claude Code starts a new session or resumes an existing one
- **Matchers**: `startup`, `resume`, `clear`, `compact`
- **Data available**:
  - `session_id`, `transcript_path`, `source` (startup/resume/clear/compact)
- **Special capability**: Access to `CLAUDE_ENV_FILE` for persisting environment variables
- **Use for tracking**: Load previous session context, initialize tracking database

#### 4. **SessionEnd** (Session Termination)
- **When it fires**: When a Claude Code session ends
- **Data available**:
  - `session_id`, `transcript_path`, `reason` (clear/logout/prompt_input_exit/other)
- **Capabilities**: Run cleanup tasks, aggregate session statistics
- **Use for tracking**: Finalize session accomplishments, save summary for review

#### 5. **Stop** (Main Agent Completion)
- **When it fires**: When the main Claude Code agent has finished responding
- **Data available**: `session_id`, `transcript_path`, `stop_hook_active` (bool)
- **Capabilities**: Can block Claude from stopping and request continuation
- **Use for tracking**: Mid-session milestone, can capture state at work boundaries

#### 6. **UserPromptSubmit** (User Input)
- **When it fires**: When user submits a prompt, before Claude processes it
- **Data available**: `prompt` (the user's message), `session_id`, `transcript_path`
- **Capabilities**: Add context, validate, or block prompts
- **Use for tracking**: Extract task intent from user prompts

#### 7. **PermissionRequest** (Permission Dialog)
- **When it fires**: When user is shown a permission dialog
- **Data available**: Tool name and inputs requesting permission
- **Capabilities**: Allow or deny on behalf of user
- **Use for tracking**: Record permission-sensitive operations (deployments, deletions)

#### 8. **Notification** (System Notifications)
- **When it fires**: When Claude Code sends notifications (permission requests, idle alerts)
- **Data available**: `message`, `notification_type`
- **Capabilities**: Custom notification handling
- **Use for tracking**: Log notification events (less useful for accomplishments)

#### 9. **PreCompact** (Before Context Compaction)
- **When it fires**: Before Claude Code runs a compact operation
- **Matchers**: `manual`, `auto`
- **Use for tracking**: Could signal need to save session state before compaction

#### 10. **SubagentStop** (Subagent Completion)
- **When it fires**: When a subagent task completes
- **Data available**: Subagent result, task ID
- **Use for tracking**: Record subagent accomplishments separately

### Hook Execution Model

**Critical Implementation Details:**

- **Parallelization**: All matching hooks run in parallel (multiple hooks for one event execute simultaneously)
- **Deduplication**: Identical hook commands are deduplicated automatically
- **Timeout**: 60-second default execution limit per hook (configurable)
- **Input method**: JSON via stdin
- **Exit codes**:
  - `0`: Success (stdout shown in verbose mode, except UserPromptSubmit/SessionStart where stdout is added to context)
  - `2`: Blocking error (only stderr shown, blocks the action)
  - Other: Non-blocking error (stderr shown in verbose mode, execution continues)
- **Environment**:
  - `CLAUDE_PROJECT_DIR`: Absolute path to project root
  - `CLAUDE_CODE_REMOTE`: Set to "true" if running in web environment
  - All standard shell environment variables available

**Output Control:**

Hooks can return structured JSON to control behavior:
```json
{
  "continue": true,           // Whether Claude continues (default: true)
  "stopReason": "message",    // Shown to user if continue=false
  "suppressOutput": true,     // Hide from transcript
  "systemMessage": "warning"  // Warning shown to user
}
```

### Hook Configuration Structure

**Location**: `~/.claude/settings.json` (user-level) or `.claude/settings.json` (project-level)

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit|Bash",
        "hooks": [
          {
            "type": "command",
            "command": "path/to/script.sh",
            "timeout": 30
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "path/to/initialize.sh"
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "path/to/finalize.sh"
          }
        ]
      }
    ]
  }
}
```

**Matchers**:
- Apply to: `PreToolUse`, `PermissionRequest`, `PostToolUse`, `Notification`
- Don't apply to: `UserPromptSubmit`, `Stop`, `SubagentStop`, `SessionStart`, `SessionEnd`, `PreCompact`
- Format: Case-sensitive, supports regex (`Edit|Write`), glob patterns, or `*` for all
- Tool names available: Write, Edit, Bash, Read, Glob, Grep, Task, WebFetch, WebSearch, etc.

## Recommended Hook Strategy for Accomplishment Tracking

### Best Hooks to Use

**1. PostToolUse (Primary)**
- **Why**: Most reliable signal of completed work
- **Tools to track**: Write, Edit, Bash (file changes, command execution)
- **Data captured**: File paths, code changes, command results
- **Advantage**: Captures both inputs and outcomes

**2. SessionStart (Initialization)**
- **Why**: Initialize tracking for new sessions
- **Data captured**: Load session context, start timestamp
- **Advantage**: Clean starting point for each session

**3. SessionEnd (Finalization)**
- **Why**: Aggregate session results before shutdown
- **Data captured**: Session duration, total accomplishments, final summary
- **Advantage**: Natural boundary for session-level rollup

**4. UserPromptSubmit (Optional Enhancement)**
- **Why**: Extract high-level intent from user prompts
- **Data captured**: Task descriptions, goals
- **Advantage**: Provides human context for automation

### Hooks to Avoid

- **PreToolUse**: Too noisy, fires before execution
- **Stop**: Fires multiple times per session, not reliable for state capture
- **Notification**: Low-value for accomplishments
- **PermissionRequest**: Only fires on permission-required operations

## Data Storage Patterns

### Option 1: File-Based (Recommended for simplicity)

**Structure**:
```
~/.claude/
├── accomplishments/
│   ├── sessions/
│   │   └── {session_id}.jsonl          # One per session (newline-delimited JSON)
│   ├── daily/
│   │   └── 2025-12-05.jsonl            # Rolled up daily summaries
│   └── quarterly/
│       └── 2025-Q4.md                  # Quarterly reviews (generated from daily)
```

**Format per session** (`sessions/{session_id}.jsonl`):
```json
{"timestamp":"2025-12-05T09:15:30Z","type":"session_start","session_id":"abc123"}
{"timestamp":"2025-12-05T09:16:45Z","type":"file_write","file":"/path/to/file.ts","lines_changed":42,"operation":"edit"}
{"timestamp":"2025-12-05T09:17:10Z","type":"command_run","command":"npm test","status":"success"}
{"timestamp":"2025-12-05T09:45:00Z","type":"session_end","session_id":"abc123","duration_minutes":30,"accomplishment_count":5}
```

**Advantages**:
- Simple file I/O with JSON
- Easy to read and audit
- No external dependencies
- Supports compression/archival over time

### Option 2: SQLite (For querying & analysis)

**Schema**:
```sql
CREATE TABLE accomplishments (
  id INTEGER PRIMARY KEY,
  session_id TEXT NOT NULL,
  timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
  tool_name TEXT,
  operation TEXT,
  file_path TEXT,
  status TEXT,
  details TEXT,  -- JSON
  FOREIGN KEY(session_id) REFERENCES sessions(id)
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  start_time DATETIME,
  end_time DATETIME,
  duration_minutes INTEGER,
  accomplishment_count INTEGER
);
```

**Advantages**:
- Queryable (find accomplishments by date, tool type, status)
- Efficient storage with indexing
- Supports aggregation for quarterly reviews

**Disadvantage**:
- Adds SQLite dependency
- More complex schema to maintain

## Complete Hook Configuration

### User-Level Configuration (~/.claude/settings.json)

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "$HOME/.claude/hooks/track-file-change.sh",
            "timeout": 5
          }
        ]
      },
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "$HOME/.claude/hooks/track-command.sh",
            "timeout": 5
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "$HOME/.claude/hooks/session-init.sh",
            "timeout": 10
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "$HOME/.claude/hooks/session-finalize.sh",
            "timeout": 15
          }
        ]
      }
    ]
  }
}
```

### Hook Scripts

**1. Track File Changes** (`~/.claude/hooks/track-file-change.sh`):
```bash
#!/bin/bash
set -e

# Read input from stdin
INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path')

# Determine if create, edit, or delete
OPERATION=$(echo "$INPUT" | jq -r '.tool_input | if .content then "edit" else "read" end')

# Prepare accomplishment record
RECORD=$(jq -n \
  --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg session "$SESSION_ID" \
  --arg file "$FILE_PATH" \
  --arg op "$OPERATION" \
  '{timestamp: $ts, type: "file_change", session_id: $session, file_path: $file, operation: $op}')

# Store in session log
SESSION_LOG="$HOME/.claude/accomplishments/sessions/$SESSION_ID.jsonl"
mkdir -p "$(dirname "$SESSION_LOG")"
echo "$RECORD" >> "$SESSION_LOG"

exit 0
```

**2. Track Commands** (`~/.claude/hooks/track-command.sh`):
```bash
#!/bin/bash
set -e

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id')
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command')
STATUS=$(echo "$INPUT" | jq -r '.tool_response.success // "unknown"')

RECORD=$(jq -n \
  --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg session "$SESSION_ID" \
  --arg cmd "$COMMAND" \
  --arg status "$STATUS" \
  '{timestamp: $ts, type: "command_executed", session_id: $session, command: $cmd, status: $status}')

SESSION_LOG="$HOME/.claude/accomplishments/sessions/$SESSION_ID.jsonl"
mkdir -p "$(dirname "$SESSION_LOG")"
echo "$RECORD" >> "$SESSION_LOG"

exit 0
```

**3. Session Initialization** (`~/.claude/hooks/session-init.sh`):
```bash
#!/bin/bash

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id')
SOURCE=$(echo "$INPUT" | jq -r '.source')

# Initialize session record
SESSION_LOG="$HOME/.claude/accomplishments/sessions/$SESSION_ID.jsonl"
mkdir -p "$(dirname "$SESSION_LOG")"

INIT_RECORD=$(jq -n \
  --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg session "$SESSION_ID" \
  --arg source "$SOURCE" \
  '{timestamp: $ts, type: "session_start", session_id: $session, source: $source}')

echo "$INIT_RECORD" >> "$SESSION_LOG"

# Load yesterday's summary if available
YESTERDAY=$(date -d 'yesterday' +%Y-%m-%d 2>/dev/null || date -v-1d +%Y-%m-%d)
YESTERDAY_SUMMARY="$HOME/.claude/accomplishments/daily/$YESTERDAY.json"

if [ -f "$YESTERDAY_SUMMARY" ]; then
  echo "Yesterday's context loaded"
fi

exit 0
```

**4. Session Finalization** (`~/.claude/hooks/session-finalize.sh`):
```bash
#!/bin/bash

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id')
SESSION_LOG="$HOME/.claude/accomplishments/sessions/$SESSION_ID.jsonl"

if [ ! -f "$SESSION_LOG" ]; then
  exit 0
fi

# Count accomplishments
COUNT=$(wc -l < "$SESSION_LOG")

# Get session duration from timestamps
START_TS=$(jq -r 'select(.type == "session_start") | .timestamp' "$SESSION_LOG" | head -1)
END_TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Create summary record
SUMMARY=$(jq -n \
  --arg ts "$END_TS" \
  --arg session "$SESSION_ID" \
  --arg start "$START_TS" \
  --arg count "$COUNT" \
  '{timestamp: $ts, type: "session_end", session_id: $session, start_time: $start, accomplishment_count: $count}')

echo "$SUMMARY" >> "$SESSION_LOG"

# Roll up to daily log
TODAY=$(date +%Y-%m-%d)
DAILY_LOG="$HOME/.claude/accomplishments/daily/$TODAY.jsonl"
mkdir -p "$(dirname "$DAILY_LOG")"

# Append session log to daily log
cat "$SESSION_LOG" >> "$DAILY_LOG"

exit 0
```

## Integration with Morning Digest

### Hook for SessionStart Loading Previous Context

```bash
#!/bin/bash
# ~/.claude/hooks/morning-digest-init.sh

if [ "$1" = "startup" ]; then
  # Load yesterday's accomplishments into context
  YESTERDAY=$(date -d 'yesterday' +%Y-%m-%d)
  YESTERDAY_FILE="$HOME/.claude/accomplishments/daily/$YESTERDAY.jsonl"

  if [ -f "$YESTERDAY_FILE" ]; then
    SUMMARY=$(jq -s 'group_by(.type) | map({type: .[0].type, count: length})' "$YESTERDAY_FILE")
    echo "Yesterday's accomplishments: $SUMMARY"
  fi
fi
exit 0
```

### Integration Points

- SessionStart hooks load previous day's summary
- Morning digest tool can generate summaries
- Summaries feed into quarterly review system

## Quarterly Review Generation

**Aggregation script** (`~/.claude/scripts/generate-quarterly-review.sh`):
```bash
#!/bin/bash
# Generate quarterly summary from daily accomplishment logs

QUARTER="$1"  # e.g., "2025-Q4"
ACCOMPLISHMENTS_DIR="$HOME/.claude/accomplishments"

# Aggregate daily logs for the quarter
find "$ACCOMPLISHMENTS_DIR/daily" -name "2025-10*.jsonl" \
  -o -name "2025-11*.jsonl" \
  -o -name "2025-12*.jsonl" | \
  xargs cat | \
  jq -s 'group_by(.type) | map({
    type: .[0].type,
    count: length,
    examples: (.[0:3] | map(.file_path))
  })' > "$ACCOMPLISHMENTS_DIR/quarterly/$QUARTER.json"
```

## Limitations & Considerations

### What Hooks CAN'T Do

1. **Read transcript history** - Hooks don't have access to conversation history, only current tool data
2. **Modify Claude's responses** - Hooks can't alter what Claude outputs to the user
3. **Access file contents directly** - Must parse from tool inputs/outputs only
4. **Store state across sessions** - Each hook runs independently; state must be externalized (files, DB)
5. **Run indefinitely** - 60-second timeout per hook
6. **Block indefinitely** - Hooks must complete quickly to not slow down workflow

### What Hooks CAN Do

1. **Track tool execution** - See every file edit, command run, search performed
2. **Modify tool inputs** - Can sanitize, validate, or transform data before execution
3. **Block specific operations** - Prevent edits to sensitive files
4. **Aggregate session data** - Collect and summarize within session
5. **Trigger external systems** - Call webhooks, APIs, or scripts
6. **Persist data** - Write to files or databases (with caveats)

### Data Completeness Issues

**Hooks may miss accomplishments:**
- Work done via Claude's internal reasoning (text manipulation, planning)
- Tools without hook support (TodoWrite, some MCP tools)
- Multi-step operations that Claude optimizes internally
- Operations blocked by permissions (not recorded as failures)

**Recommendation**: Combine hooks with occasional manual summaries or UserPromptSubmit hooks to capture high-level intent.

## Security Considerations

### Risks

1. **Hook execution with user credentials** - Hooks run with your environment variables, can access secrets
2. **Data collection** - Accomplishment logs may contain sensitive file paths or code
3. **Hook modification** - Malicious hooks can exfiltrate data or modify files
4. **Permission bypass** - Hooks could theoretically bypass Claude's permission system

### Mitigations

1. **Keep hooks in .claude/** - Use version control, don't commit to sensitive repos
2. **Sanitize file paths** - Remove sensitive information before logging
3. **Use absolute paths** - Never use relative paths that could be manipulated
4. **Review regularly** - Audit hook configuration and outputs
5. **Limit hook scope** - Use matchers to only track relevant tools
6. **Environment variables** - Don't log `ANTHROPIC_API_KEY`, credentials, tokens

## Approvers

- **Primary reviewer**: User implementation team
- **Security review**: Before storing credentials or sensitive paths
- **Integration**: With morning digest and review systems

## Success Criteria

1. **Accuracy**: >90% of actual accomplishments captured
2. **Performance**: Hooks complete in <100ms average
3. **Reliability**: 99%+ success rate (no data loss)
4. **Usability**: Quarterly reviews generated automatically with <5s runtime
5. **Completeness**: All major work categories represented (code, tests, docs, deployments)

---

**Document Status**: Research complete, ready for implementation planning
**Last Updated**: 2025-12-05
