# nclaw

Telegram bot that wraps the Claude Code CLI. Users message a Telegram bot, which invokes Claude Code in a Docker container and returns the response. Each chat/thread gets its own persistent Claude session.

## Architecture

```
Telegram -> Handler -> Claude Code CLI (via go-binwrapper) -> Telegram
                    -> Scheduler (gocron + SQLite) for recurring/one-time tasks
```

## Project Structure

- `cmd/nclaw/main.go` - Entrypoint: config init, DB setup, bot creation, scheduler start
- `internal/config/` - Viper-based config (env prefix `NCLAW_`, `.env` support, optional `config.yaml`)
- `internal/handler/` - Telegram message handling, file attachments, reply context, sendfile processing
- `internal/claude/` - Claude Code CLI wrapper using `go-binwrapper` (fluent builder API)
- `internal/model/` - GORM models: `ScheduledTask`, `TaskRunLog`
- `internal/db/` - Database operations (SQLite with WAL mode)
- `internal/scheduler/` - Task scheduling via `gocron`, command parsing from Claude replies
- `data/` - Runtime data directory (gitignored)

## Key Patterns

### Claude CLI Integration
The `claude` package wraps the CLI binary with a fluent builder. The handler calls `claude.New().Dir(dir).SkipPermissions().AppendSystemPrompt(prompt).Continue(query)` to continue the session in a per-chat directory.

### Scheduled Tasks
Claude's replies are scanned for `nclaw:schedule` code blocks containing JSON commands (`create`, `pause`, `resume`, `cancel`). Tasks support cron, interval, and one-time schedules. Tasks persist in SQLite and reload on startup.

### File Handling
- **Inbound**: Attachments (documents, photos, audio, video, stickers) are downloaded to the chat directory and referenced in prompts. Files are cached by unique ID and size.
- **Outbound**: Claude's replies are scanned for `nclaw:sendfile` code blocks. Matched files are sent as Telegram documents.

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
- `github.com/stretchr/testify` - Testing

## CI/CD

GitHub Actions pipeline (`.github/workflows/ci.yml`):
1. **Lint** - golangci-lint
2. **Test** - `go test -v ./...`
3. **Docker** - Build and push to GHCR on push to main or tagged releases

## Docker

The runtime image (`node:24-alpine` based) includes Claude Code, Go, git, kubectl, flux, kustomize, gh CLI, Chromium, and Python/uv. Claude Code skills (`schedule`, `send-file`) are copied into the global skills directory.
