# nclaw

A personal Claude assistant that lives in Telegram. Built in Go, runs in Docker, gives you a persistent AI coding agent you can message from your phone.

Inspired by [NanoClaw](https://github.com/qwibitai/nanoclaw) — same philosophy of simplicity, different stack and channel.

## Why

I wanted a Claude Code assistant I could reach from anywhere. Not a chatbot wrapper — the real Claude Code CLI with full tool access, session persistence, and the ability to manage my infrastructure. Small enough to understand, powerful enough to be useful.

## Quick Start

```bash
git clone https://github.com/nickalie/nclaw.git
cd nclaw
cp .env.example .env  # Add your Telegram bot token
docker build -t nclaw .
docker run -d --name nclaw --env-file .env nclaw
```

## How It Works

You message the Telegram bot. It invokes the Claude Code CLI, preserving conversation history per chat thread, and sends back the response.

```
Telegram --> nclaw --> Claude Code CLI (in container) --> Telegram
                   --> Scheduler (recurring/one-time tasks)
                   --> Webhook Server (incoming HTTP callbacks)
```

Claude runs inside a Docker container that serves as the security sandbox. The image ships with git, kubectl, flux, kustomize, GitHub CLI, and Chromium — making it a capable DevOps assistant out of the box.

## Features

- **Session persistence** — Each chat thread maintains its own Claude session. Pick up where you left off.
- **File attachments** — Send photos, documents, audio, video to Claude.
- **File delivery** — Claude can send files back to you (generated reports, exports, code).
- **Scheduled tasks** — Create recurring or one-time jobs using natural language.
- **Webhooks** — Register HTTP endpoints that forward incoming requests to Claude in your chat.
- **Rich runtime** — Docker image includes git, kubectl, flux, kustomize, gh CLI, Chromium browser.
- **HTML-formatted replies** — Responses render using Telegram's HTML formatting with plain-text fallback.

## Configuration

Supports `.env` files, `config.yaml`, or `$HOME/.nclaw/config.yaml`. Environment variables use the `NCLAW_` prefix.

| Variable | Required | Description |
|---|---|---|
| `NCLAW_TELEGRAM_BOT_TOKEN` | Yes | Telegram bot token from [@BotFather](https://t.me/BotFather) |
| `NCLAW_TELEGRAM_WHITELIST_CHAT_IDS` | Yes | Comma-separated list of allowed Telegram chat IDs |
| `NCLAW_DATA_DIR` | Yes | Base directory for data storage |
| `NCLAW_DB_PATH` | No | SQLite path (default: `{data_dir}/nclaw.db`) |
| `NCLAW_TIMEZONE` | No | Timezone for scheduler (default: system local) |
| `NCLAW_WEBHOOK_BASE_DOMAIN` | No | Base domain for webhook URLs (required when using webhooks) |
| `NCLAW_WEBHOOK_PORT` | No | Webhook HTTP server listen address (default: `:3000`) |

## Scheduling

Talk naturally to create scheduled tasks:

```
Remind me to check the deployment every weekday at 9am
Every 30 minutes, check if the staging server is healthy
At 3pm today, generate a summary of today's git commits
```

Tasks persist across restarts. Each task can either continue the existing chat session or run in a fresh isolated context.

## Webhooks

Register HTTP endpoints that forward incoming requests to Claude:

```
Create a webhook that receives GitHub push events and summarizes the changes
Set up a webhook for my monitoring alerts
```

When an external service calls the webhook URL, the request (method, headers, query params, body) is forwarded to Claude in the originating chat. The HTTP endpoint returns 200 immediately; Claude processes the request asynchronously. Webhooks persist across restarts.

Requires `NCLAW_WEBHOOK_BASE_DOMAIN` to be set. Webhook URLs follow the pattern `https://{BASE_DOMAIN}/webhooks/{UUID}`.

## Skills

Six skills ship with nclaw:

| Skill | Source | Purpose |
|---|---|---|
| `schedule` | Custom | Create and manage scheduled tasks via natural language |
| `send-file` | Custom | Send generated files back to the user via Telegram |
| `webhook` | Custom | Register HTTP endpoints that forward requests to Claude |
| `find-skills` | [vercel-labs/skills](https://github.com/vercel-labs/skills) | Discover and install additional agent skills |
| `skill-creator` | [anthropics/skills](https://github.com/anthropics/skills) | Guide for creating new custom skills |
| `agent-browser` | [vercel-labs/agent-browser](https://github.com/vercel-labs/agent-browser) | Browse the web using system Chromium |

## Development

```bash
make run     # Run locally
make lint    # Lint
make test    # Run tests (requires CGO)
```

## Requirements

- Go 1.25+
- Docker (for deployment)
- [Claude Code](https://claude.ai/download) (installed in container via official script)
- Telegram bot token

## License

MIT
