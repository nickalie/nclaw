# Add Webhook Support with GoFiber

## Overview

Add a GoFiber-based HTTP server that runs alongside the Telegram bot, enabling Claude to register webhooks via a custom skill. External services call webhook URLs (`https://{BASE_DOMAIN}/webhooks/{UUID}`), and the incoming request body, query parameters, and webhook description are forwarded to Claude in the originating chat.

## Context

- Files involved: `cmd/nclaw/main.go`, `internal/config/config.go`, `internal/model/`, `internal/db/`, `internal/handler/handler.go`, new `internal/webhook/` package, `.claude/skills/webhook/SKILL.md`, `Dockerfile`
- Related patterns: Scheduler command parsing (`internal/scheduler/commands.go`), sendfile processing (`internal/handler/sendfile.go`), GORM models (`internal/model/task.go`), DB operations (`internal/db/task.go`)
- Dependencies: `github.com/gofiber/fiber/v2` (new), `github.com/google/uuid` (new)

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Follow existing patterns: GORM models, Viper config, code block parsing with regex
- Webhook calls return 200 OK immediately; Claude processing happens asynchronously with results sent to Telegram
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Webhook model and database operations

**Files:**
- Create: `internal/model/webhook.go`
- Create: `internal/db/webhook.go`

- [x] Define `WebhookRegistration` GORM model with fields: `ID` (UUID string, primary key), `ChatID` (int64), `ThreadID` (int, nullable), `Description` (string), `Status` (string: active/paused), `CreatedAt`, `UpdatedAt`
- [x] Add DB functions: `CreateWebhook`, `GetWebhookByID`, `ListWebhooksByChat`, `DeleteWebhook`, `UpdateWebhookStatus`
- [x] Add `WebhookRegistration` to AutoMigrate in `cmd/nclaw/main.go`
- [x] Write tests for DB operations
- [x] Run project test suite - must pass before task 2

### Task 2: Webhook configuration

**Files:**
- Modify: `internal/config/config.go`

- [x] Add config keys: `webhook.base_domain` (string, required when webhooks enabled), `webhook.port` (string, default `:3000`)
- [x] Add config getter functions following existing pattern
- [x] Write tests for config defaults
- [x] Run project test suite - must pass before task 3

### Task 3: Webhook package - server, manager, and command parsing

**Files:**
- Create: `internal/webhook/server.go` - GoFiber HTTP server
- Create: `internal/webhook/webhook.go` - Webhook manager (create/delete/list, incoming processing)
- Create: `internal/webhook/commands.go` - Parse `nclaw:webhook` code blocks from Claude replies

- [x] Implement `Server` struct wrapping a Fiber app with route `ALL /webhooks/:uuid`
- [x] Implement `Manager` struct holding DB, bot reference, and config; methods: `Create`, `Delete`, `List`, `HandleIncoming`
- [x] In `HandleIncoming`: look up webhook by UUID, return 404 if not found or inactive, return 200 immediately, then in a goroutine: build prompt containing request method, headers, query params, body, and webhook description; call `claude.EnsureValidToken()` then invoke Claude CLI with `Continue()` in the webhook's chat directory; send Claude's response to the originating Telegram chat
- [x] Implement `ProcessWebhookCommands` following `scheduler/commands.go` pattern: regex-extract `nclaw:webhook` JSON code blocks, support actions `create` (returns webhook URL), `delete`, `list`; clean code blocks from reply text
- [x] Write tests for command parsing and webhook creation logic
- [x] Run project test suite - must pass before task 4

### Task 4: Integration in main.go and handler

**Files:**
- Modify: `cmd/nclaw/main.go`
- Modify: `internal/handler/handler.go`

- [x] In `main.go`: create webhook Manager, create webhook Server, start Fiber server in a goroutine, add graceful shutdown (call `app.Shutdown()` on context cancellation)
- [x] Add webhook Manager to `Handler` struct; call `ProcessWebhookCommands` on Claude replies alongside existing schedule/sendfile processing
- [x] Ensure webhook command results (e.g., created URL) are included in the reply sent to the user
- [x] Write/update tests for handler webhook command integration
- [x] Run project test suite - must pass before task 5

### Task 5: Webhook skill for Claude

**Files:**
- Create: `.claude/skills/webhook/SKILL.md`
- Modify: `Dockerfile`

- [x] Write SKILL.md documenting the `nclaw:webhook` code block format with actions (create, delete, list), fields, and examples
- [x] Add skill copy to Dockerfile alongside existing schedule and send-file skills
- [x] Verify skill is correctly referenced

### Task 6: Verify acceptance criteria

- [ ] Manual test: register a webhook via Claude, call it with curl, verify Claude processes the request and responds in Telegram
- [ ] Run full test suite (`make test`)
- [ ] Run linter (`make lint`)
- [ ] Verify test coverage meets 80%+

### Task 7: Update documentation

- [ ] Update CLAUDE.md with webhook architecture, config keys, and patterns
- [ ] Move this plan to `docs/plans/completed/`
