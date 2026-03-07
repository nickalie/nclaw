# Deploying NClaw

## Requirements

- Docker Desktop
- Claude Code installed and authenticated (`claude login`)

## Quick Start

### 1. Create a Telegram Bot

1. Open @BotFather → `/newbot`
2. Save the token
3. Disable Group Privacy: `/mybots` → your bot → **Bot Settings** → **Group Privacy** → **Turn off**

### 2. Configure .env

Copy `.env` and fill in your values:

```env
NCLAW_TELEGRAM_BOT_TOKEN=your-token
NCLAW_TELEGRAM_WHITELIST_CHAT_IDS=123456789

# Path to Claude Code credentials
# Windows: C:/Users/<username>/.claude/.credentials.json
# Linux/Mac: /home/<username>/.claude/.credentials.json
CLAUDE_CREDENTIALS_PATH=C:/Users/<username>/.claude/.credentials.json

# Skills persistence (optional, these are the defaults)
# Skills created by the assistant are stored here and survive image updates.
# CLAUDE_SKILLS_PATH=./claude-skills
# AGENTS_PATH=./agents

# Proxy (optional, leave commented out if not needed)
# HTTP_PROXY=http://user:pass@host:port
```

### 3. Start

```bash
docker compose up -d
```

### 4. Find Your Chat ID

Send a message to the bot, then check the logs:

```bash
docker logs nclaw
# handler: received message from chat=375321681
```

### 5. Add a Group to the Whitelist

1. Add the bot to the group (kick and re-add if it was added before disabling Group Privacy)
2. Send any message in the group
3. Find the group ID in the logs:
   ```bash
   docker logs nclaw
   # handler: ignoring message from non-whitelisted chat=-1009876543210
   ```
4. Add the ID to `NCLAW_TELEGRAM_WHITELIST_CHAT_IDS` in `.env` (comma-separated) and restart:
   ```bash
   docker compose up -d
   ```

## Management

```bash
docker compose up -d      # Start
docker compose down       # Stop
docker compose restart    # Restart
docker logs nclaw -f      # Follow logs
```

## Updating the Token

If the token was revoked or expired:
1. Get a new token from @BotFather
2. Update `NCLAW_TELEGRAM_BOT_TOKEN` in `.env`
3. `docker compose up -d`
