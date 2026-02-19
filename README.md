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
```

Claude runs inside a Docker container that serves as the security sandbox. The image ships with git, kubectl, flux, kustomize, GitHub CLI, and Chromium — making it a capable DevOps assistant out of the box.

## Features

- **Session persistence** — Each chat thread maintains its own Claude session. Pick up where you left off.
- **File attachments** — Send photos, documents, audio, video to Claude.
- **File delivery** — Claude can send files back to you (generated reports, exports, code).
- **Scheduled tasks** — Create recurring or one-time jobs using natural language.
- **Rich runtime** — Docker image includes git, kubectl, flux, kustomize, gh CLI, Chromium browser.
- **Markdown replies** — Responses render in Telegram's Markdown format with plain-text fallback.

## Configuration

Supports `.env` files, `config.yaml`, or `$HOME/.nclaw/config.yaml`. Environment variables use the `NCLAW_` prefix.

| Variable | Required | Description |
|---|---|---|
| `NCLAW_TELEGRAM_BOT_TOKEN` | Yes | Telegram bot token from [@BotFather](https://t.me/BotFather) |
| `NCLAW_TELEGRAM_WHITELIST_CHAT_IDS` | Yes | Comma-separated list of allowed Telegram chat IDs |
| `NCLAW_DATA_DIR` | Yes | Base directory for data storage |
| `NCLAW_DB_PATH` | No | SQLite path (default: `{data_dir}/nclaw.db`) |
| `NCLAW_TIMEZONE` | No | Timezone for scheduler (default: system local) |

## Scheduling

Talk naturally to create scheduled tasks:

```
Remind me to check the deployment every weekday at 9am
Every 30 minutes, check if the staging server is healthy
At 3pm today, generate a summary of today's git commits
```

Tasks persist across restarts. Each task can either continue the existing chat session or run in a fresh isolated context.

## Skills

Three skills ship with nclaw:

| Skill | Purpose |
|---|---|
| `schedule` | Create and manage scheduled tasks via natural language |
| `send-file` | Send generated files back to the user via Telegram |
| `skill-creator` | Guide for creating new Claude Code skills |

## Development

```bash
make run     # Run locally
make lint    # Lint
```

## Requirements

- Go 1.25+
- Docker (for deployment)
- [Claude Code](https://claude.ai/download) (installed in container via official script)
- Telegram bot token

## License

MIT
