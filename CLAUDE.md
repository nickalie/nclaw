# nclaw

Telegram bot that wraps AI coding CLIs (Claude Code, OpenAI Codex, GitHub Copilot). Users message a Telegram bot, which invokes the configured CLI backend in a Docker container and returns the response. Each chat/thread gets its own persistent session.

## Architecture

```
Telegram ----------->\
Scheduler ------------>  CLI Backend (cli.Provider) -> Pipeline.Process() -> Telegram
Webhook  ----------->/
```

Three input channels (handler, scheduler, webhook) each invoke the configured CLI backend via the `cli.Provider` interface. All post-processing (command block execution, stripping, sendfile, reply delivery) is handled by a shared `Pipeline.Process()` call, ensuring consistent behavior across all channels and backends.

## Project Structure

- `cmd/nclaw/main.go` - Entrypoint: config init, DB setup, bot creation, scheduler start, CLI backend selection
- `internal/config/` - Viper-based config (env prefix `NCLAW_`, `.env` support, optional `config.yaml`)
- `internal/cli/` - Generic CLI interfaces (`Client`, `Provider`, `Result`) that all backends implement
- `internal/cli/streamjson/` - Shared stream-json output parser (used by Claude and Claudish adapters)
- `internal/claude/` - Claude Code CLI adapter (fluent builder, stream-json parsing, OAuth token refresh)
- `internal/cli/claudish/` - Multi-model CLI adapter (via OpenRouter, Gemini, OpenAI, Ollama, etc.)
- `internal/codex/` - OpenAI Codex CLI adapter (JSONL event parsing, AGENTS.md system prompt)
- `internal/copilot/` - GitHub Copilot CLI adapter (plain text output, `.github/copilot-instructions.md` system prompt)
- `internal/handler/` - Telegram message handling, file attachments, reply context
- `internal/pipeline/` - Unified post-processing: block execution, stripping, sendfile, reply delivery
- `internal/sendfile/` - Shared sendfile processing: parses `nclaw:sendfile` blocks, validates paths, sends documents
- `internal/model/` - GORM models: `ScheduledTask`, `TaskRunLog`, `WebhookRegistration`
- `internal/db/` - Database operations (SQLite with WAL mode)
- `internal/scheduler/` - Task scheduling via `gocron`, command parsing from CLI replies
- `internal/webhook/` - GoFiber HTTP server, webhook manager, and command parsing from CLI replies
- `data/` - Runtime data directory (gitignored)

## Key Patterns

### CLI Backend Interface (`internal/cli/`)
All CLI backends implement two interfaces: `cli.Client` (per-request builder with `Dir()`, `SkipPermissions()`, `AppendSystemPrompt()`, `Ask()`, `Continue()`) and `cli.Provider` (singleton with `NewClient()`, `PreInvoke()`, `Version()`, `Name()`). The `*cli.Result` struct has `Text` (final message for display) and `FullText` (all messages for command block scanning). Consumers use only these interfaces, making them backend-agnostic.

### CLI Adapters
- **Claude** (`internal/claude/`): Stream-json output parsing via shared `streamjson` package. Claude-specific methods (`Model`, `FallbackModel`, `Resume`) remain on the concrete `*Claude` type. `PreInvoke()` handles OAuth token refresh.
- **Claudish** (`internal/cli/claudish/`): Wraps Claude Code via [claudish](https://github.com/MadAppGang/claudish), proxying API calls to alternative providers (OpenRouter, Gemini, OpenAI, Ollama, LM Studio, etc.). Uses the same stream-json output format as Claude, parsed via the shared `streamjson` package. Passes model config (`--model` flag) and model tier overrides (`CLAUDISH_MODEL_OPUS/SONNET/HAIKU/SUBAGENT`) as environment variables. Provider API keys (e.g. `OPENROUTER_API_KEY`, `GEMINI_API_KEY`) pass through from the OS environment. `PreInvoke()` is a no-op.
- **Codex** (`internal/codex/`): JSONL event parsing (`item.completed` with `type: "agent_message"`). System prompt written to `AGENTS.md` in the working directory.
- **Copilot** (`internal/copilot/`): Plain text output via `-s` flag (`Text == FullText`). System prompt written to `.github/copilot-instructions.md`. Known limitation: no structured output, so command blocks in intermediate messages are not captured.

### OAuth Token Refresh
Before each Claude CLI invocation, `claude.EnsureValidToken()` (called via `Provider.PreInvoke()`) proactively refreshes the OAuth token if it expires within 5 minutes. Credentials are read from `~/.claude/.credentials.json` using field-preserving JSON round-tripping. Refresh failures are logged as warnings and do not block the CLI call. Codex and Copilot providers have no-op `PreInvoke()`.

### Unified Pipeline (`internal/pipeline/`)
All post-CLI processing goes through `Pipeline.Process()`, which:
1. **Execute** — Runs each `BlockExecutor.ExecuteBlocks()` on `Result.FullText` (all assistant messages), plus `sendfile.ExecuteBlocks()`. Skipped when the CLI returned an error.
2. **Strip** — Removes all command block syntax (`nclaw:sendfile`, `nclaw:schedule`, `nclaw:webhook`) from `Result.Text`.
3. **Append status** — Adds status/error messages from block execution.
4. **Send reply** — Delivers the final text via Telegram with HTML-then-plain-text fallback, splitting long messages.

The scheduler and webhook manager implement the `BlockExecutor` interface and are passed to the pipeline as executors. The handler, scheduler, and webhook packages each invoke the CLI backend independently, then call `Pipeline.Process()` for all post-processing.

### Scheduled Tasks
`nclaw:schedule` code blocks contain JSON commands (`create`, `pause`, `resume`, `cancel`). Tasks support cron, interval, and one-time schedules. Tasks persist in SQLite and reload on startup. The scheduler implements `BlockExecutor` for the pipeline.

### Webhooks
`nclaw:webhook` code blocks contain JSON commands (`create`, `delete`, `list`). Webhooks register HTTP endpoints at `https://{BASE_DOMAIN}/webhooks/{UUID}`. When an external service calls a webhook URL, the request (method, headers, query params, body) is forwarded to the CLI backend in the originating chat via `Continue()`. The HTTP server returns 200 immediately; CLI processing happens asynchronously in a goroutine. Webhooks persist in SQLite alongside scheduled tasks. The webhook manager implements `BlockExecutor` for the pipeline.

### File Handling
- **Inbound**: Attachments (documents, photos, audio, video, stickers) are downloaded to the chat directory and referenced in prompts. Files are cached by unique ID and size.
- **Outbound**: `sendfile.ExecuteBlocks()` scans for `nclaw:sendfile` code blocks and sends matched files as Telegram documents. File paths must resolve to within the chat directory or the OS temp directory; paths outside these locations are rejected.

### Message Formatting
Replies use Telegram HTML formatting with plain-text fallback. Long messages are split at newline boundaries (max 4096 chars per message).

## Configuration

Required env vars (prefix `NCLAW_`):
- `NCLAW_TELEGRAM_BOT_TOKEN` - Telegram bot token
- `NCLAW_DATA_DIR` - Base directory for session data and files

Optional:
- `NCLAW_CLI` - CLI backend to use: `claude` (default), `claudish` (multi-model), `codex`, or `copilot`. Auto-selects `claudish` when `NCLAW_MODEL` is set
- `NCLAW_MODEL` - Model for multi-model backend (e.g. `g@gemini-2.5-pro`, `oai@gpt-4o`)
- `NCLAW_MODEL_OPUS` - Claudish Opus-tier model override
- `NCLAW_MODEL_SONNET` - Claudish Sonnet-tier model override
- `NCLAW_MODEL_HAIKU` - Claudish Haiku-tier model override
- `NCLAW_MODEL_SUBAGENT` - Claudish subagent model override
- `NCLAW_TELEGRAM_WHITELIST_CHAT_IDS` - Comma-separated list of allowed Telegram chat IDs (if unset, bot accepts all chats with a security warning)
- `NCLAW_DB_PATH` - SQLite path (default: `{data_dir}/nclaw.db`)
- `NCLAW_TIMEZONE` - Timezone for scheduler (default: system local)
- `NCLAW_WEBHOOK_BASE_DOMAIN` - Base domain for webhook URLs (required when webhooks enabled)
- `NCLAW_WEBHOOK_PORT` - Webhook HTTP server listen address (default: `:3000`)

## Commands

```
make run             # go run ./cmd/nclaw
make lint            # golangci-lint run ./...
make test            # CGO_ENABLED=1 go test ./...
make docker          # Build and run all-in-one image
make docker-claude   # Build Claude-only image
make docker-multi-model # Build multi-model image
make docker-codex    # Build Codex-only image
make docker-copilot  # Build Copilot-only image
```

## Code Style

- golangci-lint v2 with gofmt formatter
- Max cyclomatic complexity: 8
- Max line length: 140
- Keep methods small to stay under complexity limit
- Standard Go conventions
- Never mention "Claude Code" in commit messages or PR titles/descriptions

## Tech Stack

- Go 1.25
- `github.com/go-telegram/bot` - Telegram bot framework
- `github.com/nickalie/go-binwrapper` - Binary wrapper for CLI backends (Claude, Codex, Copilot)
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
3. **Release** - GoReleaser cross-compilation (on tag push)
4. **Chocolatey** - Build and push `.nupkg` to Chocolatey (on tag push, Windows runner)
5. **Docker** - Build and push 5 image variants to GHCR on push to main or tagged releases (matrix strategy)
6. **Helm** - Push Helm chart to GHCR OCI registry (on tag push)
7. **Publish** - Promote draft release to published (after all jobs pass)

### Chocolatey Package
The nuspec is generated inline in the CI workflow. It must include: `title` (distinct from id), `summary`, `tags` (space-separated), `packageSourceUrl`, `iconUrl`, and a valid `releaseNotes` URL. Icon lives at `assets/icon.svg` in the repo (served via jsDelivr CDN).

## Docker

A single `docker/Dockerfile` uses multi-stage targets to produce 5 image variants. A shared `base` stage contains all common tools (git, gh CLI, Chromium, Go, Python/uv, skills); each variant adds only its CLI backend:

- `--target all` — All-in-one: Claude Code + Claudish + Codex + Copilot (tagged `latest`)
- `--target claude` — Claude Code only (tagged `claude`)
- `--target multi-model` — Claude Code + Multi-Model (tagged `multi-model`)
- `--target codex` — OpenAI Codex only (tagged `codex`)
- `--target copilot` — GitHub Copilot only (tagged `copilot`)

CI builds and pushes all five variants to GHCR using a matrix strategy. Custom nclaw skills (`schedule`, `send-file`, `webhook`) and third-party skills are included in all variants.
