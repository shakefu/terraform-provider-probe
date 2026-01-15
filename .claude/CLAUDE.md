# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go project with automated tooling for code quality, conventional commits, and semantic versioning. The project is configured for modern development practices with comprehensive CI/CD automation.

## Quick Setup

This project follows the [Scripts to Rule Them All](https://github.com/github/scripts-to-rule-them-all) pattern:

```bash
./script/setup    # First-time setup (installs deps, configures environment)
./script/test     # Run test suite (uses go test)
./script/ci       # Run CI checks locally (no tests)
./script/cibuild  # Run full CI locally (checks + tests)
```

Other scripts: `./script/bootstrap` (install deps), `./script/update` (after pulling changes)

**When to use each script:**
- `./script/ci` - Quick validation before committing (fmt, lint, pre-commit, security audits)
- `./script/test` - Run unit tests only
- `./script/test --e2e` - Run E2E tests against LocalStack (requires Docker); run after changes to Lambda/API code
- `./script/cibuild` - Full CI validation (update deps + checks + tests)

## Agent Session Protocol

Each session starts with no memory of previous work. Follow this protocol:

1. **Verify clean state**: Run `git status`, `git stash list`, check for unpushed commits. Ask user if any pending changes exist.
2. **Create feature branch**: Checkout main, pull latest changes on main branch, create branch `<type>/<description>-<session-id>`
3. **Start baseline tests**: Run `./script/test` with `run_in_background: true` (skip for docs/context-only changes)
4. **Find current task**: Read `context/current-task.json` for active work
5. **Review recent history**: Run `git log --oneline -5`
6. **Execute**: Find first task where `status=pending` and `blocked_by=null`, complete it, update status

**Using background tests**: After making code changes, use `BashOutput` to check results. Start a new background test run after edits to verify changes.

**Key context files:**
- `context/current-task.json` - Active task and plan file
- `context/traceability-map.md` - Component to documentation mapping

**Archived files** (in `context/completed/`): feature-status.json, implementation plans, testing guides

**Reference docs:** `context/cli-design.md`, `context/purpose.md`, `context/design.md`, `README.md`

**User documentation:** Check `docs/` directory if present for user guides and documentation

### Task Stash Stack

The `context/` directory is not in version control (gitignored). Task files are local working state only.

When interrupted by higher-priority work:

**Push** (preserve current, start new):
1. Rename `current-task.json` → `current-task-stash{N}.json` (skip if current task is null/empty)
2. Create new `current-task.json` with new task
3. **Do not start the new task** without confirmation - user typically wants fresh sessions

**Pop** (resume after completing current):
1. Delete/clear `current-task.json`
2. Rename highest-numbered stash back to `current-task.json`

Stack order: highest number = oldest task.

### Archiving Completed Plans

When all tasks in a plan are complete:
1. `mv context/<plan>.json context/completed/`
2. Update `current-task.json` to next plan or clear

## Agent Effectiveness Guidelines

Based on [Anthropic's "Effective Harnesses for Long-Running Agents"](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents):

- **Follow recommendations precisely** - Read entire sources before proposing solutions; don't paraphrase without justification
- **If corrected, acknowledge and fix** - Don't defend substitutions that contradict the source
- **Work on one task at a time** - Avoid scope creep and doing too much at once
- **Documentation is part of implementation** - When adding functionality, update all related docs (module docs, function docs, user-facing docs) in the same change. Don't defer documentation to later.

### Code Search Tools

Prefer the cgc MCP tools (`mcp__cgc__*`) over built-in Glob/Grep for finding code:
- `mcp__cgc__find_code` - Find functions, classes, or code by keyword
- `mcp__cgc__analyze_code_relationships` - Find callers, callees, class hierarchy, etc.

Use built-in tools (Glob, Grep, Read) for simple file lookups or when cgc isn't indexed.

### Use JSON for Structured Tracking

- **Use JSON, not YAML or markdown** for task lists and progress tracking
- **Do not substitute formats** - If a reference says "use JSON", use JSON
- **Include explicit status fields**: `"status": "pending|in_progress|complete"`
- **Add step-by-step descriptions** that can be verified

Example structure:
```json
{
  "plan_name": "Feature or Project Name",
  "last_updated": "YYYY-MM-DD",
  "session_startup": [
    "Run git status to check branch",
    "Run ./script/test to verify baseline",
    "Read this file to find current task",
    "Find first task where status=pending and blocked_by=null"
  ],
  "tasks": [
    {
      "id": "task-id-kebab-case",
      "name": "Human readable task name",
      "status": "pending",
      "priority": 1,
      "blocked_by": null,
      "output_file": "path/to/output.go",
      "steps": [
        "Concrete step 1 with specific action",
        "Concrete step 2 with verification command"
      ],
      "acceptance_criteria": [
        "File exists at expected path",
        "Tests pass: go test ./...",
        "No lint warnings"
      ]
    },
    {
      "id": "dependent-task",
      "name": "Task that depends on first",
      "status": "pending",
      "priority": 2,
      "blocked_by": "task-id-kebab-case",
      "steps": ["..."],
      "acceptance_criteria": ["..."]
    }
  ],
  "notes": [
    "Tasks with blocked_by=null can run in parallel",
    "Update status to complete and last_updated when done"
  ]
}
```

### Plan-to-Plan Pattern

When a task requires reading excessive files/lines that would consume too much context, break it into sub-plans.

**How it works:**
1. Parent plan includes task: `"Create sub-plan for X"` → complete when sub-plan file created
2. Next parent task: `"Complete sub-plan-X.json"` → stays `in_progress`
3. Update `current-task.json` to point to sub-plan
4. Work through sub-plan tasks
5. When sub-plan is done: archive it, update `current-task.json` back to parent, mark parent task complete

**Example parent plan tasks:**
```json
[
  {
    "id": "create-coverage-subplans",
    "name": "Create sub-plans for each module's test coverage",
    "status": "complete",
    "steps": ["Create context/coverage-config-module.json", "Create context/coverage-git-module.json"]
  },
  {
    "id": "complete-config-coverage",
    "name": "Complete coverage-config-module.json",
    "status": "in_progress",
    "sub_plan": "context/coverage-config-module.json"
  }
]
```

**Rules:**
- Sub-plans are functionally separate (no parent references needed)
- Archive sub-plans separately when complete
- Nesting depth is unlimited, but confirm with the user at 3+ levels deep

### Feature Completion Criteria

Do not mark features complete prematurely:

1. **E2E tests exist and pass** - Unit tests alone are insufficient
2. **All acceptance criteria met** - Check each explicitly
3. **Tests actually run and pass** - Run `./script/test`, don't assume
4. **Documentation updated** - Add new commands/features to relevant docs

### Zero Narration

Do not narrate actions. Tool calls are structured output - the user sees them directly. Text output wastes context.

**Never output:**
- Action announcements ("Let me...", "I'll now...", "I'm going to...")
- Summaries of what was done
- Confirmations of success (visible from tool output)
- Explanations of routine operations

**Only output text when:**
- Asking a question that requires user input
- Reporting an error that blocks progress
- A decision point requires user choice

Otherwise: execute silently.

## Development Commands

### Building
```bash
go build              # Build current package
go build ./...        # Build all packages
go build -o bin/app ./cmd/bootstrap  # Build specific binary
go run ./cmd/bootstrap/main.go       # Run application
```

### Testing

Uses [Ginkgo](https://onsi.github.io/ginkgo/) (BDD-style) with Gomega matchers.

```bash
# Run all tests
./script/test                         # Run all unit tests
./script/test -v                      # Verbose output
./script/test --focus 'pattern'       # Focus on specs matching pattern

# Focus filtering examples
./script/test --focus 'getVersion'    # Run specs containing 'getVersion'
./script/test --focus 'API router'    # Run specs in 'API router' Describe block

# Integration tests (requires network)
go test -tags integration ./...        # Run integration tests
SKIP_NETWORK_TESTS=1 go test -tags integration ./...

# Quick runs (skip dependency update)
QUICK=1 ./script/test  # Also: SKIP_UPDATE=1 or CI=1
```

**Important:**
- Use unit tests during development (fast, no network)
- Run integration tests before major changes
- Integration tests are disabled by default (build tags)
- All tests must pass for CI/CD to succeed
- Use `-race` flag to detect race conditions

### Writing E2E CLI Tests

E2E tests are in `*_test.go` files. Use Go's standard testing package:
```go
// Example E2E test
func TestCLICommand(t *testing.T) {
    cmd := exec.Command("go", "run", "./cmd/bootstrap/main.go", "arg1", "arg2")
    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("command failed: %v", err)
    }
    // Assert on output
}
```

### Test Coverage

```bash
go test -coverprofile=coverage.out ./...  # Generate coverage profile
go tool cover -html=coverage.out           # Open HTML report
go tool cover -func=coverage.out           # Show coverage by function
go test -cover -coverprofile=coverage.out -covermode=atomic ./...
```

## Code Quality & Pre-commit

**Before every commit**, run:
```bash
./script/ci           # Recommended: runs all CI checks (no tests)
prek run --all-files  # Alternative: runs pre-commit hooks only
```

Environment variables for `./script/ci`:
- `SKIP_SECURITY=1` - Skip govulncheck and security audits
- `SKIP_PROSE=1` - Skip prose style check (AI writing patterns)
- `OFFLINE=1` - Skip network-dependent checks

Or individually:
```bash
go fmt ./...                                          # Format code
golangci-lint run                                     # Lint (if using golangci-lint)
staticcheck ./...                                     # Static analysis (if using staticcheck)
go vet ./...                                          # Run go vet
go test ./...                                         # Run tests
govulncheck ./...                                     # Check for vulnerabilities
```

Pre-commit hooks (configured in `.pre-commit-config.yaml`) automatically run: go fmt, go vet, golangci-lint (or staticcheck), conventional commit validation, trailing whitespace/YAML checks.

**Common CI failures:**
- Commit message >100 chars or wrong format
- Code not formatted
- Lint warnings
- AI writing patterns in documentation

## Commit Message Requirements

All commits must follow **conventional commits**:

```
<type>(<scope>): <description>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`

Examples: `feat: add user auth`, `fix: resolve memory leak`, `docs: update install instructions`

Breaking changes: `feat!: description` or `BREAKING CHANGE:` in footer

## Committing Guidelines for Claude Code

1. **Run `./script/ci` before every commit** - Catches formatting, linting, and prose issues
2. **NEVER commit/push without explicit user approval**
3. **Avoid hardcoding values that change** - No version numbers, dates, or timestamps in tests. Use dynamic checks.
4. **When fixing tests** - Understand what's being validated, fix the underlying issue, make expectations flexible
5. **Keep summaries brief** - 1-2 sentences, no code samples unless requested
6. **NEVER add "co-authored-by" to commit messages** - This is a private, paid repository, not open source.

## Documentation Style Guide

- Follow [Go documentation comments](https://go.dev/doc/comment) and [Effective Go](https://go.dev/doc/effective_go)
- **Avoid AI writing patterns** - See `context/ai-writing-patterns.md` for the list of phrases to avoid
- Link to files/documentation appropriately
- No emojis or hype language
- No specific numbers that will change (versions, coverage percentages)
- No line number references
- Review for consistency and accuracy when done
- Use `go doc` to preview generated documentation

### Go Documentation Comments

Go uses special comment conventions for documentation:
- Package comments: `// Package name provides...` at the top of the file
- Exported symbols: Comments starting with the symbol name
- Examples: `func ExampleFunctionName()` for executable examples

```bash
go doc ./...                    # View documentation for all packages
go doc package/name             # View documentation for specific package
go doc package/name.Function    # View documentation for specific function
```

## Important Notes

- Linters are strict: all warnings should be treated as errors
- Binary name is `bootstrap` (built from `cmd/bootstrap/main.go`)
- When reviewing: look for flimsy tests, check for TODOs/stubs
- Before pushing: rebase on main and resolve conflicts
- Use `go mod tidy` to clean up dependencies
- Use `go mod verify` to verify dependencies haven't been modified
