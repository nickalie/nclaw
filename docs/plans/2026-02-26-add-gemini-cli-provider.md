# Add Gemini CLI as a CLI Provider

## Overview

Add Google's Gemini CLI (`gemini`) as a new CLI backend, following the same provider pattern as existing backends (claude, codex, copilot). Gemini CLI uses its own stream-json NDJSON format (different from Claude's), writes system prompts to `GEMINI.md`, and uses `--approval-mode yolo` for auto-approve.

## Context

- Files involved: `internal/cli/gemini/` (new package), `internal/config/config.go`, `cmd/nclaw/main.go`, `docker/Dockerfile`, `.github/workflows/ci.yml`, `Makefile`
- Related patterns: Copilot adapter (file-based system prompt), Codex adapter (custom JSONL parser)
- Dependencies: `gemini` binary (Node.js tool installed via `npm install -g @google/gemini-cli`)

## Gemini CLI Reference

- Binary: `gemini`
- Non-interactive: `-p "prompt"`
- Resume session: `--resume latest`
- Output format: `--output-format stream-json` (NDJSON with event types: init, message, tool_use, tool_result, error, result)
- Auto-approve: `--approval-mode yolo`
- Model selection: `--model <model>`
- Version: `--version`
- System prompt: writes `GEMINI.md` file in working directory
- Working directory: uses CWD (set via go-binwrapper's `Dir()`)
- Auth: `GEMINI_API_KEY` env var or Google account login

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Follow the Copilot adapter as the closest reference pattern (file-based system prompt, own output parser)
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Create Gemini stream-json parser

**Files:**
- Create: `internal/cli/gemini/result.go`

- [x] Define Go structs for Gemini's stream-json event types (message events with role/content, result event with status)
- [x] Implement `parseStreamJSONOutput(output []byte) *cli.Result` that scans NDJSON lines, collects assistant message content into FullText, and extracts the final assistant message as Text
- [x] Handle edge cases: empty output, no assistant messages, malformed JSON lines (skip gracefully)
- [x] Write tests for the parser
- [x] Run project test suite - must pass before task 2

### Task 2: Create Gemini client and provider

**Files:**
- Create: `internal/cli/gemini/gemini.go`
- Create: `internal/cli/gemini/provider.go`

- [x] Implement `Gemini` struct with fields: `bin`, `dir`, `systemPrompt`, `skipPermissions`, `model`
- [x] Implement `cli.Client` interface: `Dir()`, `SkipPermissions()`, `AppendSystemPrompt()`, `Ask()`, `Continue()`
- [x] `Ask()`: writes GEMINI.md, runs `gemini -p <query> --output-format stream-json`, parses output
- [x] `Continue()`: same as Ask but adds `--resume latest`
- [x] `SkipPermissions()`: sets `--approval-mode yolo`
- [x] `writeSystemPrompt()`: writes `GEMINI.md` in working directory (same pattern as Copilot)
- [x] Implement `Provider` struct (stateless, like Copilot). `PreInvoke()` is no-op. `Name()` returns "gemini"
- [x] `Version()`: runs `gemini --version`
- [x] Add compile-time interface assertions (`var _ cli.Provider = (*Provider)(nil)`, etc.)
- [x] Write tests for client and provider
- [x] Run project test suite - must pass before task 3

### Task 3: Register Gemini backend in config and main

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/nclaw/main.go`

- [x] Add "gemini" to `ValidCLIBackends()` return value
- [x] Add `case "gemini"` in `newProvider()` switch in `main.go`, returning `gemini.NewProvider()`
- [x] Update existing tests for `ValidCLIBackends` to include "gemini"
- [x] Run project test suite - must pass before task 4

### Task 4: Add Docker and CI support

**Files:**
- Modify: `docker/Dockerfile`
- Modify: `.github/workflows/ci.yml`
- Modify: `Makefile`

- [ ] Add `gemini` stage in Dockerfile: `FROM base AS gemini`, install `@google/gemini-cli` via npm, copy skills
- [ ] Add gemini variant to `all` target (install alongside other CLIs)
- [ ] Add `{ name: "Gemini", suffix: "-gemini", target: "gemini" }` to CI matrix
- [ ] Add `docker-gemini` target to Makefile
- [ ] Add `.PHONY` entry for `docker-gemini`
- [ ] Run project test suite - must pass before task 5

### Task 5: Update documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md` (if user-facing changes documented there)

- [ ] Add `internal/cli/gemini/` to Project Structure section in CLAUDE.md
- [ ] Add Gemini adapter description to CLI Adapters section
- [ ] Add `NCLAW_CLI=gemini` to Configuration section
- [ ] Add `docker-gemini` to Commands section
- [ ] Add gemini Docker target to Docker section
- [ ] Update README.md with Gemini as a supported backend
- [ ] Move this plan to `docs/plans/completed/`

### Task 6: Verify acceptance criteria

- [ ] manual test: set `NCLAW_CLI=gemini` and verify the bot starts and reports gemini version
- [ ] run full test suite (`make test`)
- [ ] run linter (`make lint`)
- [ ] verify test coverage meets 80%+
