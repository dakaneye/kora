# EFA Authoring Guidelines

## What Are EFAs?

**Explainer For Agents (EFAs)** are formal specifications that define ground truth for AI agents working on this codebase. They prevent **semantic drift**—where Claude reinvents data structures, formulas, or architectures with each iteration—by establishing immutable contracts that must be updated explicitly.

Think of EFAs as:
- **Contracts**: Formal agreements between humans and AI agents about how things work
- **Ground Truth**: The single source of truth that overrides agent assumptions
- **Drift Prevention**: Guards against Claude "improving" stable, correct code

## When to Create an EFA

Create an EFA when:
- **Complex data structures** need precise definitions (models, schemas, APIs)
- **Formulas or algorithms** must remain consistent across iterations
- **Interfaces or contracts** define system boundaries
- **Architecture decisions** affect multiple modules
- **Testing approaches** require specific patterns

Don't create an EFA for:
- Simple utility functions
- Implementation details that can change freely
- Temporary scaffolding or prototypes

## Required Authoring Process

**IMPORTANT**: All EFAs MUST be authored collaboratively using three agents:

1. **`golang-pro`**: Ensures Go idioms, performance, and implementation feasibility
2. **`documentation-engineer`**: Structures the document for clarity and completeness
3. **`prompt-engineer`**: Crafts effective AI Agent Rules sections

This multi-agent approach prevents blind spots and ensures EFAs serve both humans and AI effectively.

## Required Sections

Every EFA must include:

### 1. Frontmatter (YAML)
```yaml
---
authors: Your Name <your.email@example.com>
state: draft | review | accepted
discussion: [optional PR or issue URL]
---
```

### 2. Title
Format: `EFA ####: Brief Descriptive Name`
- Use 4-digit zero-padded numbers (0001, 0002, etc.)
- Keep titles under 60 characters

### 3. Motivation & Prior Art
**Purpose**: Explain why this exists and what problem it solves.

Include:
- What were we doing before?
- What problem does this solve?
- What happens if we don't do this?
- Goals and non-goals

**Length**: 2-4 paragraphs

### 4. Detailed Design
**Purpose**: Explain the design in enough detail to implement it.

Common subsections (choose what fits):
- **Architecture Overview**: System design, components, data flow
- **Data Structures**: Go types, JSON schemas, interfaces
- **Algorithms**: Step-by-step logic, formulas
- **API Specifications**: Interfaces, method signatures
- **Examples**: Concrete usage scenarios (always include these!)

**Critical**: Include runnable Go code examples with comments.

### 5. Why This Design?
**Purpose**: Justify the approach and acknowledge trade-offs.

Be honest about:
- Complexity vs. simplicity
- Performance vs. maintainability
- Development time vs. features

### 6. Alternatives Considered
**Purpose**: Show you've thought through the problem space.

For each alternative:
- What it was
- Why it might be good
- Why you rejected it

### 7. Cross-cutting Concerns
Check boxes and address as needed:
- Security Implications
- Performance Implications
- Testing Implications

### 8. AI Agent Rules
**Purpose**: Explicit instructions for Claude when working with EFA-governed code.

This section is CRITICAL—it's what prevents drift. Write clear, imperative rules:

```markdown
## AI Agent Rules

When modifying code governed by this EFA, Claude MUST:

1. **Never modify data structures** without updating this EFA first
2. **Always use the defined types** - do not create variations
3. **Preserve the formula** for `calculatePriority()` exactly as written
4. **Stop and ask** before changing auth provider interfaces
5. **Reference this EFA** in code comments for protected functions

Protected functions:
- `internal/models/event.go:Event` struct
- `internal/priority/priority.go:Calculate()`
```

**Guidelines for AI Agent Rules**:
- Use imperative voice ("MUST", "NEVER", "ALWAYS")
- List specific files/functions/types
- Include rationale for non-obvious rules
- Reference the EFA number in protected code comments

### 9. Open Questions (Optional)
Use during `state: draft` to seek feedback. Remove before `state: accepted`.

### 10. References
Link to:
- External specifications (RFCs, APIs)
- Related EFAs
- Design docs
- Similar implementations

## Writing Effective AI Agent Rules

The **AI Agent Rules** section is what makes EFAs effective. Bad rules lead to drift; good rules prevent it.

### Bad Rules (Vague)
```markdown
- Don't change the Event struct unnecessarily
- Be careful with priority calculations
- Keep things consistent
```

### Good Rules (Explicit)
```markdown
- NEVER add fields to `Event` struct without updating EFA 0001
- NEVER modify `priority.Calculate()` formula—it implements agreed-upon business logic
- ALWAYS use `EventType` enum constants—do not use raw strings
- STOP and ask before changing `Prioritizer` interface—it's implemented by multiple datasources
```

### Checklist for AI Agent Rules
- [ ] Lists specific files/functions/types protected by the EFA
- [ ] Uses imperative verbs (MUST, NEVER, ALWAYS)
- [ ] Explains *why* rules exist (not just *what*)
- [ ] References the EFA number for code comments
- [ ] Defines what changes require EFA updates vs. what's safe

## Updating Existing EFAs

**When to update vs. create new**:
- **Update** if the change refines/clarifies the existing design
- **Create new** if the change introduces new concepts or deprecates old ones

**Update process**:
1. Propose changes in PR with rationale
2. Update `discussion:` field with PR URL
3. Get review from original author if possible
4. Update AI Agent Rules to reflect changes
5. Search codebase for references to old design
6. Update protected code comments

## Protecting Code with EFA References

Mark EFA-governed code clearly:

```go
// Package models defines the core Event interface and implementations.
// Ground truth defined in specs/efas/0001-event-model.md
//
// IT IS FORBIDDEN TO CHANGE the Event interface without updating EFA 0001.
// Claude MUST stop and ask before modifying this file.
package models

// Event represents a work item from any datasource (GitHub PR, Slack DM, etc.).
// This interface is the contract between datasources and the digest engine.
//
// EFA 0001: Do not add methods without updating the EFA.
type Event interface {
    // ID returns a unique identifier scoped to the datasource
    ID() string

    // Priority returns the calculated priority score (0-100)
    // EFA 0001: Uses the formula defined in priority/calculate.go
    Priority() int

    // ... other methods
}
```

### Protected Code Comment Template
```go
// [Brief description]
// Ground truth defined in specs/efas/XXXX-name.md
//
// IT IS FORBIDDEN TO CHANGE [specific aspect] without updating EFA XXXX.
// Claude MUST stop and ask before modifying [specific elements].
```

## EFA Lifecycle

1. **Draft**: Initial proposal, open questions
2. **Review**: Seeking feedback, iterating
3. **Accepted**: Implemented, enforced

Update `state:` field in frontmatter as it progresses.

## Quick Reference

| Section | Required | Purpose |
|---------|----------|---------|
| Frontmatter | Yes | Metadata, state tracking |
| Motivation | Yes | Why this exists |
| Detailed Design | Yes | How it works |
| Why This Design | Yes | Justify trade-offs |
| Alternatives | Yes | Show you've thought it through |
| Cross-cutting | Conditional | Security, performance, testing |
| AI Agent Rules | **Critical** | Prevent drift |
| Open Questions | Optional | Draft-stage only |
| References | Optional | External links |

## Examples

See existing EFAs:
- `specs/efas/0001-event-model.md` - Data structures and interfaces
- `specs/efas/0002-auth-provider.md` - Architecture and contracts

## Enforcement

EFAs are enforced through:
1. **Code comments** referencing the EFA
2. **Claude's CLAUDE.md** instructions to stop and ask
3. **PR reviews** checking for EFA compliance
4. **CI checks** (future) to detect protected code changes

Remember: EFAs are **living documents**. They prevent drift while remaining flexible enough to evolve with the codebase.
