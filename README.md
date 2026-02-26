# nclaw

**N**Claw — a**N**other Claw. A lightweight, container-first AI coding assistant accessible through Telegram. Supports Claude Code (default), OpenAI Codex, and GitHub Copilot as CLI backends. Written in Go.

## Table of Contents

- [Why NClaw](#why-nclaw)
- [How It Works](#how-it-works)
- [Features](#features)
- [Docker](#docker)
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

**Telegram topics as projects.** NClaw treats each Telegram topic (thread) as a separate Claude Code session with its own working directory. One group chat with topics becomes a multi-project workspace — each topic gets isolated context, history, and files.

## How It Works

You message the assistant through Telegram. It invokes the configured CLI backend (Claude Code by default), preserving conversation history per chat/topic, and sends back the response.

```
Telegram  -\
Scheduler -->  CLI Backend  -->  Telegram
Webhook   -/
```

The recommended way to run NClaw is inside Docker — the container serves as a security sandbox, and the image ships with all the tools the assistant might need. However, NClaw is a regular executable and can run directly on any machine with the chosen CLI backend installed.

## Features

- **Session persistence** — Each chat/topic maintains its own Claude session. Pick up where you left off.
- **Telegram topics** — Each topic in a group chat is a separate project with isolated context and files.
- **File attachments** — Send photos, documents, audio, video to Claude.
- **File delivery** — Claude can send files back to you (generated reports, exports, code).
- **Scheduled tasks** — Create recurring or one-time jobs using natural language.
- **Webhooks** — Register HTTP endpoints that forward incoming requests to Claude in your chat.
- **Rich runtime** — Docker image includes git, gh CLI, Chromium, Go, Node.js, Python/uv. The assistant can install additional packages on the fly as needed — for example, `apk add ffmpeg` to process video, `npm install -g prettier` to format code, or `pip install pandas` to analyze data.
- **Multiple CLI backends** — Supports Claude Code (default), OpenAI Codex, and GitHub Copilot. Switch backends via the `NCLAW_CLI` environment variable.
- **HTML-formatted replies** — Responses render using Telegram's HTML formatting with plain-text fallback.

## Docker

NClaw provides four Docker images, all based on `node:24-alpine` with shared tools (git, gh CLI, Chromium, Go, Node.js, Python/uv, skills). They differ only in which CLI backend is pre-installed:

| Image | Tag | CLI Backends | Size |
|---|---|---|---|
| **All-in-one** | `latest` | Claude Code + Codex + Copilot | Largest |
| **Claude** | `claude` | Claude Code | Medium |
| **Codex** | `codex` | OpenAI Codex | Medium |
| **Copilot** | `copilot` | GitHub Copilot | Medium |

All images are published to `ghcr.io/nickalie/nclaw`. The assistant can install additional packages at runtime (e.g. `apk add ffmpeg`, `pip install pandas`, `npm install -g typescript`).

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

The all-in-one image includes all three CLI backends. Set `NCLAW_CLI` to `claude` (default), `codex`, or `copilot` to choose the backend. Mount the appropriate credentials for your chosen backend.

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

## Kubernetes (Helm)

The Helm chart is published as an OCI artifact to GHCR.

```bash
helm install nclaw oci://ghcr.io/nickalie/charts/nclaw \
  --set env.telegramBotToken=your-token \
  --set env.whitelistChatIds=your-chat-id \
  --set claudeCredentialsSecret=my-claude-secret
```

Create the credentials secret for your chosen backend:

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
| `env.cli` | `""` | CLI backend: `claude`, `codex`, or `copilot` (empty = image default) |
| `existingSecret` | `""` | Use existing secret for bot token (key: `telegram-bot-token`) |
| `claudeCredentialsSecret` | `""` | Secret with Claude credentials (key: `credentials.json`) |
| `codexCredentialsSecret` | `""` | Secret with Codex credentials (key: `auth.json`) |
| `copilotCredentialsSecret` | `""` | Secret with Copilot credentials (key: `config.json`) |
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

NClaw is a regular executable and can run directly on any machine. The only runtime dependency is the CLI for your chosen backend — [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (default), [OpenAI Codex](https://github.com/openai/codex), or [GitHub Copilot](https://docs.github.com/en/copilot/github-copilot-in-the-cli) — it must be installed and available in `PATH`.

> **Security notice:** Without Docker, Claude Code runs directly on the host with the same permissions as the nclaw process. It has full access to the file system, network, and any credentials available to the user. Run under a dedicated unprivileged user and avoid running as root. For production use, Docker or Kubernetes deployment is strongly recommended.

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

All variables use the `NCLAW_` prefix.

| Variable | Required | Default | Description |
|---|---|---|---|
| `NCLAW_TELEGRAM_BOT_TOKEN` | Yes | — | Telegram bot token from [@BotFather](https://t.me/BotFather) |
| `NCLAW_DATA_DIR` | Yes | — | Base directory for session data and files |
| `NCLAW_CLI` | No | `claude` | CLI backend to use: `claude`, `codex`, or `copilot` |
| `NCLAW_TELEGRAM_WHITELIST_CHAT_IDS` | No | — | Comma-separated list of allowed Telegram chat IDs. If unset, accepts all chats (with a security warning) |
| `NCLAW_DB_PATH` | No | `{data_dir}/nclaw.db` | Path to the SQLite database |
| `NCLAW_TIMEZONE` | No | system local | Timezone for the scheduler (e.g. `Europe/Berlin`) |
| `NCLAW_WEBHOOK_BASE_DOMAIN` | No | — | Base domain for webhook URLs (required when using webhooks) |
| `NCLAW_WEBHOOK_PORT` | No | `:3000` | Webhook HTTP server listen address |

> **Security notice:** If `NCLAW_TELEGRAM_WHITELIST_CHAT_IDS` is not set, the assistant will accept messages from **any** Telegram chat. Since NClaw runs Claude Code with full tool access (file system, shell, network), this effectively gives anyone who discovers your bot unrestricted access to the host environment. Always set this variable in production.

### Config file

NClaw looks for `config.yaml` in the current directory or `$HOME/.nclaw/`. Nested keys map to env vars with underscores (e.g. `telegram.bot_token` = `NCLAW_TELEGRAM_BOT_TOKEN`).

```yaml
telegram:
  bot_token: "your-telegram-bot-token"
  whitelist_chat_ids: "123456789,987654321"

cli: "claude"  # Options: claude, codex, copilot
data_dir: "/app/data"
db_path: "/app/data/nclaw.db"
timezone: "Europe/Berlin"

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

Register HTTP endpoints that forward incoming requests to Claude:

```
Create a webhook that receives GitHub push events and summarizes the changes
Set up a webhook for my package tracking updates
Listen for smart home alerts and notify me about unusual activity
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
