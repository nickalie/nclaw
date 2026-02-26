# Multi-CLI Backend Support

## Overview

Extract the Claude CLI integration into a generic interface and implement adapters for OpenAI Codex and GitHub Copilot CLIs. Claude remains the default backend. A new `NCLAW_CLI` config option selects the active backend.

## Context

- Files involved: `internal/claude/`, `internal/handler/handler.go`, `internal/scheduler/scheduler.go`, `internal/webhook/webhook.go`, `internal/pipeline/pipeline.go`, `cmd/nclaw/main.go`, `internal/config/config.go`
- New packages: `internal/cli/`, `internal/codex/`, `internal/copilot/`
- Related patterns: fluent builder API in `internal/claude/claude.go`, `pipeline.BlockExecutor` interface pattern
- Dependencies: `github.com/nickalie/go-binwrapper` (already used for Claude, reused for Codex/Copilot)

## Interface Design

```go
package cli

type Result struct {
    Text     string // final assistant message (display)
    FullText string // all assistant messages (command block scanning)
}

type Client interface {
    Dir(dir string) Client
    SkipPermissions() Client
    AppendSystemPrompt(prompt string) Client
    Ask(query string) (*Result, error)
    Continue(query string) (*Result, error)
}

type Provider interface {
    NewClient() Client
    PreInvoke() error          // e.g., token refresh; no-op for codex/copilot
    Version() (string, error)
    Name() string
}
```

Key design decisions:
- `Client` is a per-request builder; `Provider` is a singleton per backend
- Claude-specific methods (Model, Resume, FallbackModel, etc.) remain on the concrete `Claude` struct
- System prompt for Codex: written to `AGENTS.md` in chat directory before invocation
- System prompt for Copilot: written to `.github/copilot-instructions.md` in chat directory before invocation
- Copilot limitation: `-s` (silent) mode outputs only final text, so `Text == FullText`; command blocks in intermediate messages won't be captured

## Development Approach

- Testing approach: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Create CLI interface package

**Files:**
- Create: `internal/cli/cli.go`

- [x] Define `Result` struct, `Client` interface, and `Provider` interface
- [x] Add compile-time interface satisfaction checks (to be used by implementations)
- [x] Run project test suite - must pass before task 2

### Task 2: Refactor Claude package to implement CLI interfaces

**Files:**
- Modify: `internal/claude/claude.go`
- Modify: `internal/claude/result.go`
- Create: `internal/claude/provider.go`
- Modify: `internal/claude/claude_test.go`
- Modify: `internal/claude/result_test.go`

- [x] Remove `Result` from `claude/result.go`, use `cli.Result` throughout the package
- [x] Update `parseStreamOutput` and related functions to return `*cli.Result`
- [x] Change builder methods (`Dir`, `SkipPermissions`, `AppendSystemPrompt`) to return `cli.Client`
- [x] Create `provider.go` with `Provider` struct: wraps `EnsureValidToken()` as `PreInvoke()`, `New()` as `NewClient()`, existing `Version()` logic
- [x] Keep all Claude-specific methods (Model, FallbackModel, Resume, etc.) on the concrete `*Claude` type
- [x] Update all existing tests
- [x] Run project test suite - must pass before task 3

### Task 3: Update consumers to use CLI interfaces

**Files:**
- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/handler/handler.go`
- Modify: `internal/scheduler/scheduler.go`
- Modify: `internal/webhook/webhook.go`
- Modify: `cmd/nclaw/main.go`

- [x] Update `pipeline.Process()` signature to accept `*cli.Result` instead of `*claude.Result`
- [x] Add `cli.Provider` field to `handler.Handler`; rename `callClaude` to `callCLI`, use provider
- [x] Add `cli.Provider` parameter to `scheduler.New()`; rename `invokeClaudeForTask` to `invokeCLI`, use provider
- [x] Add `cli.Provider` parameter to `webhook.NewManager()`; rename `callClaude` to `callCLI`, use provider
- [x] Update `cmd/nclaw/main.go`: create `claude.NewProvider()`, pass to handler/scheduler/webhook, use for startup version check
- [x] Update all existing tests for modified packages
- [x] Run project test suite - must pass before task 4

### Task 4: Implement Codex CLI adapter

**Files:**
- Create: `internal/codex/codex.go`
- Create: `internal/codex/provider.go`
- Create: `internal/codex/result.go`
- Create: `internal/codex/codex_test.go`
- Create: `internal/codex/result_test.go`

CLI mapping:
- Ask: `codex exec --json --full-auto --cd <dir> "<prompt>"`
- Continue: `codex exec --json --full-auto --cd <dir> resume --last "<prompt>"`
- SkipPermissions: `--full-auto`
- SystemPrompt: write `AGENTS.md` in working directory
- Version: `codex --version`
- Output: parse JSONL events from `--json` flag

- [x] Create `Codex` struct implementing `cli.Client` with go-binwrapper
- [x] Create `Provider` struct implementing `cli.Provider` (PreInvoke is no-op)
- [x] Create JSONL output parser extracting assistant messages for FullText and final message for Text
- [x] Write tests for JSONL parsing with sample Codex output
- [x] Write tests for command argument construction
- [x] Run project test suite - must pass before task 5

### Task 5: Implement Copilot CLI adapter

**Files:**
- Create: `internal/copilot/copilot.go`
- Create: `internal/copilot/provider.go`
- Create: `internal/copilot/copilot_test.go`

CLI mapping:
- Ask: `copilot -p "<prompt>" -s --allow-all-tools --no-ask-user`
- Continue: `copilot --continue -p "<prompt>" -s --allow-all-tools --no-ask-user`
- SkipPermissions: `--allow-all-tools --no-ask-user`
- SystemPrompt: write `.github/copilot-instructions.md` in working directory
- Version: `copilot version`
- Output: plain text via `-s` flag; `Text == FullText`
- Known limitation: no structured output, so command blocks in intermediate messages are not captured

- [x] Create `Copilot` struct implementing `cli.Client` with go-binwrapper
- [x] Create `Provider` struct implementing `cli.Provider` (PreInvoke is no-op)
- [x] Write tests for command argument construction
- [x] Write tests for plain text output parsing
- [x] Run project test suite - must pass before task 6

### Task 6: Add CLI backend configuration and provider selection

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/nclaw/main.go`

- [x] Add `CLI()` function to config returning backend name (default: `"claude"`; valid: `"claude"`, `"codex"`, `"copilot"`)
- [x] Support via `NCLAW_CLI` env var or `cli` key in config.yaml
- [x] Create provider factory function in `main.go` selecting the appropriate `cli.Provider`
- [x] Fail startup with clear error if CLI binary is not found for the selected backend
- [x] Update startup log to show active CLI backend and version
- [x] Write tests for config `CLI()` function
- [x] Run project test suite - must pass before task 7

### Task 7: Verify acceptance criteria

- [ ] Manual test: send a Telegram message with default (Claude) backend, verify response
- [ ] Manual test: verify scheduled tasks work with Claude backend
- [ ] Manual test: verify webhooks work with Claude backend
- [ ] Run full test suite (`make test`)
- [ ] Run linter (`make lint`)
- [ ] Verify test coverage meets 80%+

### Task 8: Update documentation

- [ ] Update CLAUDE.md: add `internal/cli/` to project structure, document CLI interface pattern, add `NCLAW_CLI` to config section, note codex/copilot packages
- [ ] Move this plan to `docs/plans/completed/`
