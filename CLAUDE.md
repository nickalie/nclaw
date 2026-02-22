# nclaw

Telegram bot that wraps the Claude Code CLI. Users message a Telegram bot, which invokes Claude Code in a Docker container and returns the response. Each chat/thread gets its own persistent Claude session.

## Architecture

```
Telegram -> Handler  -\
Scheduler ------------>  Claude Code CLI -> Pipeline.Process() -> Telegram
Webhook  ----------->/
```

Three input channels (handler, scheduler, webhook) each invoke the Claude CLI independently. All post-Claude processing (command block execution, stripping, sendfile, reply delivery) is handled by a shared `Pipeline.Process()` call, ensuring consistent behavior across all channels.

## Project Structure

- `cmd/nclaw/main.go` - Entrypoint: config init, DB setup, bot creation, scheduler start
- `internal/config/` - Viper-based config (env prefix `NCLAW_`, `.env` support, optional `config.yaml`)
- `internal/handler/` - Telegram message handling, file attachments, reply context
- `internal/claude/` - Claude Code CLI wrapper using `go-binwrapper` (fluent builder API), plus OAuth token refresh
- `internal/pipeline/` - Unified post-Claude processing: block execution, stripping, sendfile, reply delivery
- `internal/sendfile/` - Shared sendfile processing: parses `nclaw:sendfile` blocks, validates paths, sends documents
- `internal/model/` - GORM models: `ScheduledTask`, `TaskRunLog`, `WebhookRegistration`
- `internal/db/` - Database operations (SQLite with WAL mode)
- `internal/scheduler/` - Task scheduling via `gocron`, command parsing from Claude replies
- `internal/webhook/` - GoFiber HTTP server, webhook manager, and command parsing from Claude replies
- `data/` - Runtime data directory (gitignored)

## Key Patterns

### Claude CLI Integration
The `claude` package wraps the CLI binary with a fluent builder. The handler calls `claude.New().Dir(dir).SkipPermissions().AppendSystemPrompt(prompt).Continue(query)` to continue the session in a per-chat directory. `Ask()`, `Continue()`, and `Resume()` use `--output-format stream-json` and return a `*Result` with two fields:
- `Text` — the final assistant message (from the stream-json `result` event), used for display.
- `FullText` — all assistant messages concatenated, used for scanning command blocks that may appear in intermediate messages during multi-turn execution.

### OAuth Token Refresh
Before each CLI invocation (in handler and scheduler), `claude.EnsureValidToken()` proactively refreshes the OAuth token if it expires within 5 minutes. Credentials are read from `~/.claude/.credentials.json` using field-preserving JSON round-tripping. Refresh failures are logged as warnings and do not block the CLI call.

### Unified Pipeline (`internal/pipeline/`)
All post-Claude processing goes through `Pipeline.Process()`, which:
1. **Execute** — Runs each `BlockExecutor.ExecuteBlocks()` on `Result.FullText` (all assistant messages), plus `sendfile.ExecuteBlocks()`. Skipped when Claude returned an error.
2. **Strip** — Removes all command block syntax (`nclaw:sendfile`, `nclaw:schedule`, `nclaw:webhook`) from `Result.Text`.
3. **Append status** — Adds status/error messages from block execution.
4. **Send reply** — Delivers the final text via Telegram with HTML-then-plain-text fallback, splitting long messages.

The scheduler and webhook manager implement the `BlockExecutor` interface and are passed to the pipeline as executors. The handler, scheduler, and webhook packages each invoke Claude independently, then call `Pipeline.Process()` for all post-processing.

### Scheduled Tasks
`nclaw:schedule` code blocks contain JSON commands (`create`, `pause`, `resume`, `cancel`). Tasks support cron, interval, and one-time schedules. Tasks persist in SQLite and reload on startup. The scheduler implements `BlockExecutor` for the pipeline.

### Webhooks
`nclaw:webhook` code blocks contain JSON commands (`create`, `delete`, `list`). Webhooks register HTTP endpoints at `https://{BASE_DOMAIN}/webhooks/{UUID}`. When an external service calls a webhook URL, the request (method, headers, query params, body) is forwarded to Claude in the originating chat via `Continue()`. The HTTP server returns 200 immediately; Claude processing happens asynchronously in a goroutine. Webhooks persist in SQLite alongside scheduled tasks. The webhook manager implements `BlockExecutor` for the pipeline.

### File Handling
- **Inbound**: Attachments (documents, photos, audio, video, stickers) are downloaded to the chat directory and referenced in prompts. Files are cached by unique ID and size.
- **Outbound**: `sendfile.ExecuteBlocks()` scans for `nclaw:sendfile` code blocks and sends matched files as Telegram documents. File paths must resolve to within the chat directory or the OS temp directory; paths outside these locations are rejected.

### Message Formatting
Replies use Telegram HTML formatting with plain-text fallback. Long messages are split at newline boundaries (max 4096 chars per message).

## Configuration

Required env vars (prefix `NCLAW_`):
- `NCLAW_TELEGRAM_BOT_TOKEN` - Telegram bot token
- `NCLAW_TELEGRAM_WHITELIST_CHAT_IDS` - Comma-separated list of allowed Telegram chat IDs
- `NCLAW_DATA_DIR` - Base directory for session data and files

Optional:
- `NCLAW_DB_PATH` - SQLite path (default: `{data_dir}/nclaw.db`)
- `NCLAW_TIMEZONE` - Timezone for scheduler (default: system local)
- `NCLAW_WEBHOOK_BASE_DOMAIN` - Base domain for webhook URLs (required when webhooks enabled)
- `NCLAW_WEBHOOK_PORT` - Webhook HTTP server listen address (default: `:3000`)

## Commands

```
make run     # go run ./cmd/nclaw
make lint    # golangci-lint run ./...
make test    # CGO_ENABLED=1 go test ./...
make docker  # Build and run in Docker
```

## Code Style

- golangci-lint v2 with gofmt formatter
- Max cyclomatic complexity: 8
- Max line length: 140
- Keep methods small to stay under complexity limit
- Standard Go conventions

## Tech Stack

- Go 1.25
- `github.com/go-telegram/bot` - Telegram bot framework
- `github.com/nickalie/go-binwrapper` - Binary wrapper for Claude CLI
- `github.com/spf13/viper` + `github.com/joho/godotenv` - Configuration
- `gorm.io/gorm` + `gorm.io/driver/sqlite` - Database
- `github.com/go-co-op/gocron/v2` - Task scheduling
- `github.com/gofiber/fiber/v2` - Webhook HTTP server
- `github.com/google/uuid` - Webhook ID generation
- `github.com/stretchr/testify` - Testing

## CI/CD

GitHub Actions pipeline (`.github/workflows/ci.yml`):
1. **Lint** - golangci-lint
2. **Test** - `go test -v ./...`
3. **Docker** - Build and push to GHCR on push to main or tagged releases

## Docker

The runtime image (`node:24-alpine` based) includes Claude Code, Go, git, gh CLI, Chromium, and Python/uv. Claude Code skills (`schedule`, `send-file`, `webhook`) are copied into the global skills directory.
