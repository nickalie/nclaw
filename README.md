# nclaw

**N**Claw — a**N**other Claw. A lightweight, container-first AI coding assistant accessible through Telegram. Supports Claude Code (default), 580+ models via multi-model backend (OpenRouter, Gemini, OpenAI, Ollama, and more), OpenAI Codex, GitHub Copilot, and Google Gemini CLI as CLI agents. Written in Go.

## Table of Contents

- [Why NClaw](#why-nclaw)
- [How It Works](#how-it-works)
- [Features](#features)
- [Quickstart](#quickstart)
  - [Step 1: Create a Telegram Bot](#step-1-create-a-telegram-bot)
  - [Step 2: Find Your Chat ID](#step-2-find-your-chat-id)
  - [Step 3: Run NClaw](#step-3-run-nclaw)
- [Docker](#docker)
- [Multi-Model](#multi-model-1)
- [Kubernetes (Helm)](#kubernetes-helm)
- [Running without Docker](#running-without-docker)
  - [Installation](#installation)
  - [Usage](#usage)
- [Configuration](#configuration)
  - [Environment variables](#environment-variables)
  - [Config file](#config-file)
- [Scheduling](#scheduling)
- [Webhooks](#webhooks)
- [Skills](#skills)
- [GitOps Deployment](#gitops-deployment)
  - [FluxCD](#fluxcd)
  - [ArgoCD](#argocd)
- [Development](#development)
- [License](#license)

## Why NClaw

There are many AI assistants already — [OpenClaw](https://openclaw.ai/), [NanoClaw](https://github.com/qwibitai/nanoclaw), [ClaudeClaw](https://github.com/moazbuilds/claudeclaw), and others. NClaw exists because none of them satisfied three requirements at once:

**Container-first.** NClaw is built to run in Docker and Kubernetes from day one. The repo ships a multi-stage Dockerfile and a Helm chart. No manual setup, no runtime dependency resolution — `docker run` or `helm install` and you're done.

**Lightweight.** A single Go binary. Idles at ~10 MB of RAM. No runtime interpreter, no package manager overhead, no garbage collection pauses that matter.

**Telegram topics as projects.** NClaw treats each Telegram topic (thread) as a separate session with its own working directory. One group chat with topics becomes a multi-project workspace — each topic gets isolated context, history, and files.

## How It Works

You message the assistant through Telegram. It invokes the configured CLI agent (Claude Code by default), preserving conversation history per chat/topic, and sends back the response.

```
Telegram  -\
Scheduler -->  CLI Backend  -->  Telegram
Webhook   -/
```

The recommended way to run NClaw is inside Docker — the container serves as a security sandbox, and the image ships with all the tools the assistant might need. However, NClaw is a regular executable and can run directly on any machine with the chosen CLI agent installed.

## Features

- **Session persistence** — Each chat/topic maintains its own session. Pick up where you left off.
- **Telegram topics** — Each topic in a group chat is a separate project with isolated context and files.
- **File attachments** — Send photos, documents, audio, video to the assistant.
- **File delivery** — The assistant can send files back to you (generated reports, exports, code).
- **Scheduled tasks** — Create recurring or one-time jobs using natural language.
- **Webhooks** — Register HTTP endpoints that forward incoming requests to the assistant in your chat.
- **Rich runtime** — Docker image includes git, gh CLI, Chromium, Go, Node.js, Python/uv. The assistant can install additional packages on the fly as needed — for example, `apk add ffmpeg` to process video, `npm install -g prettier` to format code, or `pip install pandas` to analyze data.
- **Multiple CLI agents** — Supports Claude Code (default), multi-model (580+ models via OpenRouter, Gemini, OpenAI, Ollama, etc.), OpenAI Codex, GitHub Copilot, and Google Gemini CLI. Switch agents via the `NCLAW_CLI` environment variable.
- **HTML-formatted replies** — Responses render using Telegram's HTML formatting with plain-text fallback.

## Quickstart

Get NClaw running in under 5 minutes using Docker.

### Step 1: Create a Telegram Bot

1. Open Telegram and search for **@BotFather** (or open [t.me/BotFather](https://t.me/BotFather)).
2. Send `/newbot`.
3. Choose a **display name** for your bot (e.g. "My Coding Assistant").
4. Choose a **username** — must end in `bot` (e.g. `my_coding_assistant_bot`).
5. BotFather replies with your **bot token** — a string like `123456789:ABCdefGhIjKlMnOpQrStUvWxYz`. Save it.

> **Tip:** You can customize the bot later — send `/mybots` to BotFather to change the name, description, profile picture, and more.

If you want the bot in a **group with topics** (one topic per project), also configure these via BotFather:

6. Send `/mybots` → select your bot → **Bot Settings** → **Group Privacy** → **Turn off**. This lets the bot read all messages in group chats, not just commands.
7. Send `/setjoingroups` → select your bot → **Enable**. This allows adding the bot to groups.

### Step 2: Find Your Chat ID

NClaw uses `NCLAW_TELEGRAM_WHITELIST_CHAT_IDS` to restrict which chats the bot responds in. This setting is optional, but **strongly recommended** — without it, anyone who discovers your bot can send it commands with full access to the container's file system, shell, and network. You need the numeric chat ID.

**For a private chat (1-on-1 with the bot):**

1. Message your bot (send anything — it won't reply yet).
2. Open this URL in a browser, replacing `<TOKEN>` with your bot token:
   ```
   https://api.telegram.org/bot<TOKEN>/getUpdates
   ```
3. Find `"chat":{"id":123456789}` in the JSON response. That number is your chat ID.

**For a group chat:**

1. Add the bot to the group.
2. Send a message in the group.
3. Use the same `getUpdates` URL above. The group chat ID is a **negative number** (e.g. `-1001234567890`).

> **Tip:** You can whitelist multiple chat IDs by separating them with commas: `123456789,-1001234567890`.

### Step 3: Run NClaw

The fastest way to get started is with the **multi-model** image using a free Gemini API key:

1. Get a free API key from [Google AI Studio](https://aistudio.google.com/apikey).

2. Run:
   ```bash
   docker run -d --name nclaw \
     -e NCLAW_TELEGRAM_BOT_TOKEN=your-bot-token \
     -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
     -e NCLAW_DATA_DIR=/app/data \
     -e NCLAW_MODEL=g@gemini-2.5-pro \
     -e GEMINI_API_KEY=your-gemini-key \
     -v ./data:/app/data \
     ghcr.io/nickalie/nclaw:multi-model
   ```

3. Message your bot in Telegram — it should reply.

To use **Claude Code** instead (requires an Anthropic account with Claude Code access):

1. Install Claude Code and authenticate:
   ```bash
   curl -fsSL https://claude.ai/install.sh | bash
   claude login
   ```

2. Run:
   ```bash
   docker run -d --name nclaw \
     -e NCLAW_TELEGRAM_BOT_TOKEN=your-bot-token \
     -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
     -e NCLAW_DATA_DIR=/app/data \
     -v ./data:/app/data \
     -v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro \
     ghcr.io/nickalie/nclaw:claude
   ```

See [Docker](#docker) for all image variants and [Configuration](#configuration) for the full list of options.

## Docker

NClaw provides six Docker images, all based on `node:24-alpine` with shared tools (git, gh CLI, Chromium, Go, Node.js, Python/uv, skills). They differ only in which CLI agent is pre-installed:

| Image | Tag | CLI Backends | Size |
|---|---|---|---|
| **All-in-one** | `latest` | Claude Code + Multi-Model + Codex + Copilot + Gemini | Largest |
| **Claude** | `claude` | Claude Code | Medium |
| **Multi-Model** | `multi-model` | Claude Code + Multi-Model | Medium |
| **Codex** | `codex` | OpenAI Codex | Medium |
| **Copilot** | `copilot` | GitHub Copilot | Medium |
| **Gemini** | `gemini` | Google Gemini CLI | Medium |

All images are published to `ghcr.io/nickalie/nclaw` and built for **linux/amd64** and **linux/arm64**. Docker automatically pulls the correct architecture — no extra flags needed. This means you can run NClaw on:

- **Raspberry Pi** (4/5 or any arm64 board) — a dedicated AI coding assistant on a $35 device
- **AWS Graviton** instances — lower cost and better price-performance than x86
- **Apple Silicon** Macs — native arm64 without Rosetta emulation
- **Oracle Cloud Ampere** or any other arm64 cloud VM

The assistant can install additional packages at runtime (e.g. `apk add ffmpeg`, `pip install pandas`, `npm install -g typescript`).

### Claude (default)

```bash
docker run -d --name nclaw \
  -e NCLAW_TELEGRAM_BOT_TOKEN=your-token \
  -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
  -e NCLAW_DATA_DIR=/app/data \
  -v ./data:/app/data \
  -v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro \
  ghcr.io/nickalie/nclaw:claude
```

Claude Code uses OAuth authentication. Mount your credentials file from `~/.claude/.credentials.json`. To obtain credentials, install Claude Code locally and run `claude login`.

### Multi-Model

```bash
docker run -d --name nclaw \
  -e NCLAW_TELEGRAM_BOT_TOKEN=your-token \
  -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
  -e NCLAW_DATA_DIR=/app/data \
  -e NCLAW_MODEL=g@gemini-2.5-pro \
  -e GEMINI_API_KEY=your-gemini-key \
  -v ./data:/app/data \
  ghcr.io/nickalie/nclaw:multi-model
```

```bash
docker run -d --name nclaw \
  -e NCLAW_TELEGRAM_BOT_TOKEN=your-token \
  -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
  -e NCLAW_DATA_DIR=/app/data \
  -e NCLAW_MODEL=zai@glm-4 \
  -e ZAI_API_KEY=your-zai-key \
  -v ./data:/app/data \
  ghcr.io/nickalie/nclaw:multi-model
```

Setting `NCLAW_MODEL` automatically selects the multi-model backend. No Anthropic credentials are needed — only an API key from your chosen provider. See [Multi-Model](#multi-model-1) for full configuration details.

### Codex

```bash
docker run -d --name nclaw \
  -e NCLAW_TELEGRAM_BOT_TOKEN=your-token \
  -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
  -e NCLAW_DATA_DIR=/app/data \
  -e NCLAW_CLI=codex \
  -v ./data:/app/data \
  -v ~/.codex/auth.json:/root/.codex/auth.json:ro \
  ghcr.io/nickalie/nclaw:codex
```

Codex uses ChatGPT OAuth authentication. Mount your auth file from `~/.codex/auth.json`. To obtain credentials, install Codex locally (`npm install -g @openai/codex`) and sign in on first run.

### Copilot

```bash
docker run -d --name nclaw \
  -e NCLAW_TELEGRAM_BOT_TOKEN=your-token \
  -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
  -e NCLAW_DATA_DIR=/app/data \
  -e NCLAW_CLI=copilot \
  -v ./data:/app/data \
  -v ~/.copilot/config.json:/root/.copilot/config.json:ro \
  ghcr.io/nickalie/nclaw:copilot
```

Copilot uses GitHub OAuth authentication. Mount your config file from `~/.copilot/config.json`. To obtain credentials, install Copilot CLI locally (`npm install -g @githubnext/github-copilot-cli`) and run `/login`.

### Gemini

```bash
docker run -d --name nclaw \
  -e NCLAW_TELEGRAM_BOT_TOKEN=your-token \
  -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
  -e NCLAW_DATA_DIR=/app/data \
  -e NCLAW_CLI=gemini \
  -v ./data:/app/data \
  -v ~/.gemini/oauth_creds.json:/root/.gemini/oauth_creds.json:ro \
  ghcr.io/nickalie/nclaw:gemini
```

Gemini CLI uses Google account OAuth authentication. Mount your credentials file from `~/.gemini/oauth_creds.json`. To obtain credentials, install Gemini CLI locally (`npm install -g @google/gemini-cli`) and sign in on first run.

### All-in-one

```bash
docker run -d --name nclaw \
  -e NCLAW_TELEGRAM_BOT_TOKEN=your-token \
  -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
  -e NCLAW_DATA_DIR=/app/data \
  -v ./data:/app/data \
  -v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro \
  ghcr.io/nickalie/nclaw:latest
```

The all-in-one image includes all five CLI agents. Set `NCLAW_CLI` to `claude` (default), `claudish` (multi-model), `codex`, `copilot`, or `gemini` to choose the agent. Mount the appropriate credentials for your chosen agent.

### Webhooks

To enable [webhooks](#webhooks), add the webhook base domain and expose the port:

```bash
docker run -d --name nclaw \
  -e NCLAW_TELEGRAM_BOT_TOKEN=your-token \
  -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
  -e NCLAW_DATA_DIR=/app/data \
  -e NCLAW_WEBHOOK_BASE_DOMAIN=example.com \
  -e NCLAW_WEBHOOK_PORT=:3000 \
  -p 3000:3000 \
  -v ./data:/app/data \
  -v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro \
  ghcr.io/nickalie/nclaw:latest
```

## Multi-Model

NClaw's multi-model backend (powered by [claudish](https://github.com/MadAppGang/claudish)) lets you use 580+ models from OpenRouter, Google Gemini, OpenAI, Vertex AI, Ollama, LM Studio, and more — while retaining full agentic capabilities (tool use, file editing, scheduled tasks, webhooks, file delivery).

### Supported Providers

| Provider | Prefix | Example | Auth |
|---|---|---|---|
| **OpenRouter** | `or@` | `or@deepseek/deepseek-r1` | `OPENROUTER_API_KEY` |
| **Google Gemini** | `g@` | `g@gemini-2.0-flash` | `GEMINI_API_KEY` |
| **OpenAI** | `oai@` | `oai@o3-mini` | `OPENAI_API_KEY` |
| **Vertex AI** | `v@` | `v@gemini-2.5-flash` | `VERTEX_API_KEY` |
| **OllamaCloud** | `oc@` | `oc@llama-3.1-70b` | `OLLAMA_API_KEY` |
| **Kimi** | `kimi@` | `kimi@kimi-k2` | `MOONSHOT_API_KEY` |
| **GLM (Zhipu)** | `glm@` | `glm@glm-4` | `ZHIPU_API_KEY` |
| **Z.AI** | `zai@` | `zai@glm-4` | `ZAI_API_KEY` |
| **MiniMax** | `mm@` | `mm@MiniMax-M2.1` | `MINIMAX_API_KEY` |
| **Poe** | `poe@` | `poe@GPT-4o` | `POE_API_KEY` |
| **OpenCode Zen** | `zen@` | `zen@grok-code` | Free (no key) |
| **Gemini CodeAssist** | `go@` | `go@gemini-2.5-flash` | OAuth |
| **Ollama** | `ollama@` | `ollama@llama3.2` | Local (no key) |
| **LM Studio** | `lms@` | `lms@qwen2.5-coder` | Local (no key) |
| **vLLM** | `vllm@` | `vllm@mistral-7b` | Local (no key) |
| **MLX** | `mlx@` | `mlx@llama-3.2-3b` | Local (no key) |

Well-known model names (e.g. `gemini-2.0-flash`, `llama-3.1-70b`) are auto-detected without a provider prefix.

### Model Selection

Set `NCLAW_MODEL` to choose the default model. This automatically selects the multi-model backend — no need to set `NCLAW_CLI` explicitly:

```bash
# Use Gemini via direct API
docker run -d --name nclaw \
  -e NCLAW_TELEGRAM_BOT_TOKEN=your-token \
  -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
  -e NCLAW_DATA_DIR=/app/data \
  -e NCLAW_MODEL=g@gemini-2.5-pro \
  -e GEMINI_API_KEY=your-key \
  -v ./data:/app/data \
  ghcr.io/nickalie/nclaw:multi-model

# Use GLM-4 via Z.AI
docker run -d --name nclaw \
  -e NCLAW_MODEL=zai@glm-4 \
  -e ZAI_API_KEY=your-key \
  ...

# Use any model via OpenRouter
docker run -d --name nclaw \
  -e NCLAW_MODEL=or@mistralai/mistral-large \
  -e OPENROUTER_API_KEY=your-key \
  ...

# Use a local model via Ollama
docker run -d --name nclaw \
  -e NCLAW_MODEL=ollama@llama3.2 \
  ...
```

### Local Models

For fully offline operation, use Ollama or LM Studio. Your code never leaves your machine:

```bash
# Start Ollama and pull a model
ollama pull llama3.2

# Run nclaw with a local model
docker run -d --name nclaw \
  -e NCLAW_TELEGRAM_BOT_TOKEN=your-token \
  -e NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id \
  -e NCLAW_DATA_DIR=/app/data \
  -e NCLAW_MODEL=ollama@llama3.2 \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  -v ./data:/app/data \
  ghcr.io/nickalie/nclaw:multi-model
```

Set `OLLAMA_BASE_URL`, `LMSTUDIO_BASE_URL`, `VLLM_BASE_URL`, or `MLX_BASE_URL` to connect to custom endpoints.

## Kubernetes (Helm)

The Helm chart is published as an OCI artifact to GHCR. Since all Docker images are multi-arch (amd64/arm64), the chart works on mixed-architecture clusters — including AWS Graviton node pools, Raspberry Pi k3s clusters, and Apple Silicon dev machines.

```bash
helm install nclaw oci://ghcr.io/nickalie/charts/nclaw \
  --set env.telegramBotToken=your-token \
  --set env.whitelistChatIds=your-chat-id \
  --set claudeCredentialsSecret=my-claude-secret
```

Create the credentials secret for your chosen agent:

```bash
# Claude
kubectl create secret generic my-claude-secret \
  --from-file=credentials.json=$HOME/.claude/.credentials.json

# Codex
kubectl create secret generic my-codex-secret \
  --from-file=auth.json=$HOME/.codex/auth.json

# Copilot
kubectl create secret generic my-copilot-secret \
  --from-file=config.json=$HOME/.copilot/config.json

# Gemini
kubectl create secret generic my-gemini-secret \
  --from-file=oauth_creds.json=$HOME/.gemini/oauth_creds.json
```

### Helm values

| Parameter | Default | Description |
|---|---|---|
| `image.repository` | `ghcr.io/nickalie/nclaw` | Docker image |
| `image.tag` | Chart appVersion | Image tag |
| `env.dataDir` | `/app/data` | Data directory inside container |
| `env.telegramBotToken` | `""` | Telegram bot token |
| `env.whitelistChatIds` | `""` | Comma-separated allowed chat IDs |
| `env.webhookBaseDomain` | `""` | Base domain for webhook URLs |
| `env.cli` | `""` | CLI agent: `claude`, `claudish` (multi-model), `codex`, `copilot`, or `gemini` (empty = image default) |
| `env.model` | `""` | Model for multi-model backend (e.g. `g@gemini-2.5-pro`). Setting this auto-selects multi-model |
| `existingSecret` | `""` | Use existing secret for bot token (key: `telegram-bot-token`) |
| `claudeCredentialsSecret` | `""` | Secret with Claude credentials (key: `credentials.json`) |
| `codexCredentialsSecret` | `""` | Secret with Codex credentials (key: `auth.json`) |
| `copilotCredentialsSecret` | `""` | Secret with Copilot credentials (key: `config.json`) |
| `geminiCredentialsSecret` | `""` | Secret with Gemini credentials (key: `oauth_creds.json`) |
| `persistence.enabled` | `true` | Enable persistent storage |
| `persistence.size` | `1Gi` | PVC size |
| `persistence.storageClass` | `""` | Storage class |
| `persistence.existingClaim` | `""` | Use existing PVC |
| `rbac.create` | `true` | Create ServiceAccount and ClusterRoleBinding |
| `rbac.clusterRole` | `cluster-admin` | ClusterRole to bind |
| `proxy.enabled` | `false` | Enable HTTP proxy |
| `proxy.httpProxy` | `""` | HTTP_PROXY value |
| `proxy.httpsProxy` | `""` | HTTPS_PROXY value |
| `resources.requests.cpu` | `100m` | CPU request |
| `resources.requests.memory` | `128Mi` | Memory request |
| `resources.limits.cpu` | `1000m` | CPU limit |
| `resources.limits.memory` | `2Gi` | Memory limit |

## Running without Docker

NClaw is a regular executable and can run directly on any machine. The only runtime dependency is the CLI for your chosen agent — [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (default), [claudish](https://github.com/MadAppGang/claudish) (multi-model), [OpenAI Codex](https://github.com/openai/codex), [GitHub Copilot](https://docs.github.com/en/copilot/github-copilot-in-the-cli), or [Gemini CLI](https://github.com/google-gemini/gemini-cli) — it must be installed and available in `PATH`.

> **Security notice:** Without Docker, the CLI agent runs directly on the host with the same permissions as the nclaw process. It has full access to the file system, network, and any credentials available to the user. Run under a dedicated unprivileged user and avoid running as root. For production use, Docker or Kubernetes deployment is strongly recommended.

### Installation

#### Homebrew (macOS/Linux)

```bash
brew install --cask nickalie/apps/nclaw
```

#### Scoop (Windows)

```powershell
scoop bucket add nickalie https://github.com/nickalie/scoop-bucket
scoop install nclaw
```

#### Chocolatey (Windows)

```powershell
choco install nclaw
```

#### Winget (Windows)

```powershell
winget install nickalie.nclaw
```

#### AUR (Arch Linux)

```bash
yay -S nclaw-bin
```

#### DEB / RPM / APK

Download the appropriate package from the [Releases](https://github.com/nickalie/nclaw/releases) page:

```bash
# Debian/Ubuntu
sudo dpkg -i nclaw_*.deb

# Fedora/RHEL
sudo rpm -i nclaw_*.rpm

# Alpine
sudo apk add --allow-untrusted nclaw_*.apk
```

#### Binary download

Pre-built binaries for Linux, macOS, and Windows (amd64/arm64) are available on the [Releases](https://github.com/nickalie/nclaw/releases) page.

#### Go install

```bash
CGO_ENABLED=1 go install github.com/nickalie/nclaw/cmd/nclaw@latest
```

Requires Go 1.25+ and a C compiler (CGO is needed for SQLite).

### Usage

1. Install Claude Code CLI and authenticate:
   ```bash
   curl -fsSL https://claude.ai/install.sh | bash
   claude login
   ```
2. Create a `.env` file or export environment variables:
   ```bash
   export NCLAW_TELEGRAM_BOT_TOKEN=your-token
   export NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=your-chat-id
   export NCLAW_DATA_DIR=./data
   ```
3. Run:
   ```bash
   nclaw
   ```

Any tools you want the assistant to use (git, gh, python, etc.) should be installed on the host. The assistant will use whatever is available in the system `PATH`.

## Configuration

NClaw reads configuration from environment variables, `.env` files, or YAML config files.

### Environment variables

NClaw variables use the `NCLAW_` prefix. Provider API keys use the provider's native env var name (no prefix) — they pass through to the multi-model backend automatically.

| Variable | Required | Default | Description |
|---|---|---|---|
| `NCLAW_TELEGRAM_BOT_TOKEN` | Yes | — | Telegram bot token from [@BotFather](https://t.me/BotFather) |
| `NCLAW_DATA_DIR` | Yes | — | Base directory for session data and files |
| `NCLAW_CLI` | No | `claude` | CLI agent: `claude`, `claudish` (multi-model), `codex`, `copilot`, or `gemini`. Auto-selects `claudish` when `NCLAW_MODEL` is set |
| `NCLAW_MODEL` | No | — | Model for multi-model backend (e.g. `g@gemini-2.5-pro`). Setting this auto-selects multi-model |
| `NCLAW_TELEGRAM_WHITELIST_CHAT_IDS` | No | — | Comma-separated list of allowed Telegram chat IDs. If unset, accepts all chats (with a security warning) |
| `NCLAW_DB_PATH` | No | `{data_dir}/nclaw.db` | Path to the SQLite database |
| `NCLAW_TIMEZONE` | No | system local | Timezone for the scheduler (e.g. `Europe/Berlin`) |
| `NCLAW_WEBHOOK_BASE_DOMAIN` | No | — | Base domain for webhook URLs (required when using webhooks) |
| `NCLAW_WEBHOOK_PORT` | No | `:3000` | Webhook HTTP server listen address |

> **Security notice:** If `NCLAW_TELEGRAM_WHITELIST_CHAT_IDS` is not set, the assistant will accept messages from **any** Telegram chat. Since NClaw runs the CLI agent with full tool access (file system, shell, network), this effectively gives anyone who discovers your bot unrestricted access to the host environment. Always set this variable in production.

### Config file

NClaw looks for `config.yaml` in the current directory or `$HOME/.nclaw/`. Nested keys map to env vars with underscores (e.g. `telegram.bot_token` = `NCLAW_TELEGRAM_BOT_TOKEN`).

```yaml
telegram:
  bot_token: "your-telegram-bot-token"
  whitelist_chat_ids: "123456789,987654321"

cli: "claude"  # Options: claude, claudish, codex, copilot, gemini
data_dir: "/app/data"
db_path: "/app/data/nclaw.db"
timezone: "Europe/Berlin"

# Multi-model settings (setting model auto-selects multi-model backend)
model: ""                  # e.g. "g@gemini-2.5-pro", "or@mistralai/mistral-large"
# Provider API keys are set as regular env vars (not in this file):
# OPENROUTER_API_KEY, GEMINI_API_KEY, OPENAI_API_KEY, etc.

webhook:
  base_domain: "example.com"
  port: ":3000"
```

## Scheduling

Talk naturally to create scheduled tasks:

```
Remind me to take a break every 2 hours
Every morning at 8am, give me a weather summary and top news headlines
At 3pm today, generate a summary of today's git commits
```

Tasks persist across restarts. Each task can either continue the existing chat session or run in a fresh isolated context.

## Webhooks

Register HTTP endpoints that forward incoming requests to the assistant:

```
Create a webhook that receives GitHub push events and summarizes the changes
Set up a webhook for my package tracking updates
Listen for smart home alerts and notify me about unusual activity
```

When an external service calls the webhook URL, the request (method, headers, query params, body) is forwarded to the assistant in the originating chat. The HTTP endpoint returns 200 immediately; the assistant processes the request asynchronously. Webhooks persist across restarts.

Requires `NCLAW_WEBHOOK_BASE_DOMAIN` to be set. Webhook URLs follow the pattern `https://{BASE_DOMAIN}/webhooks/{UUID}`.

## Skills

Six skills ship with nclaw:

| Skill | Source | Purpose |
|---|---|---|
| `schedule` | Custom | Create and manage scheduled tasks via natural language |
| `send-file` | Custom | Send generated files back to the user via Telegram |
| `webhook` | Custom | Register HTTP endpoints that forward requests to the assistant |
| `find-skills` | [vercel-labs/skills](https://github.com/vercel-labs/skills) | Discover and install additional agent skills |
| `skill-creator` | [anthropics/skills](https://github.com/anthropics/skills) | Guide for creating new custom skills |
| `agent-browser` | [vercel-labs/agent-browser](https://github.com/vercel-labs/agent-browser) | Browse the web using system Chromium |

The assistant can also create its own skills on the fly when a task requires specialized or repeatable behavior that isn't covered by the built-in set.

## GitOps Deployment

### FluxCD

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: nclaw
  namespace: flux-system
spec:
  type: oci
  interval: 10m
  url: oci://ghcr.io/nickalie/charts
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: nclaw
  namespace: nclaw
spec:
  interval: 10m
  chart:
    spec:
      chart: nclaw
      sourceRef:
        kind: HelmRepository
        name: nclaw
        namespace: flux-system
  values:
    env:
      whitelistChatIds: "123456789"
      webhookBaseDomain: "example.com"
    existingSecret: nclaw-secrets
    claudeCredentialsSecret: claude-credentials
    persistence:
      size: 5Gi
```

### ArgoCD

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nclaw
  namespace: argocd
spec:
  project: default
  source:
    chart: nclaw
    repoURL: ghcr.io/nickalie/charts
    targetRevision: "*"
    helm:
      valuesObject:
        env:
          whitelistChatIds: "123456789"
          webhookBaseDomain: "example.com"
        existingSecret: nclaw-secrets
        claudeCredentialsSecret: claude-credentials
        persistence:
          size: 5Gi
  destination:
    server: https://kubernetes.default.svc
    namespace: nclaw
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

## Development

```bash
make run     # Run locally
make lint    # Lint with golangci-lint
make test    # Run tests (requires CGO)
make docker  # Build and run in Docker
```

## License

MIT
