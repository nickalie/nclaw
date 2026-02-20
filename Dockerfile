FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG BUILD_NUMBER=
ARG DOCKER_TAG=
RUN go build -ldflags "\
    -X github.com/nickalie/nclaw/internal/version.Version=${VERSION} \
    -X github.com/nickalie/nclaw/internal/version.Commit=${COMMIT} \
    -X github.com/nickalie/nclaw/internal/version.BuildDate=${BUILD_DATE} \
    -X github.com/nickalie/nclaw/internal/version.BuildNumber=${BUILD_NUMBER} \
    -X github.com/nickalie/nclaw/internal/version.DockerTag=${DOCKER_TAG}" \
    -o nclaw ./cmd/nclaw

FROM node:24-alpine

RUN apk add --no-cache \
    git \
    bash \
    curl \
    openssh-client \
    github-cli \
    kubectl \
    flux \
    kustomize \
    chromium \
    harfbuzz \
    nss \
    freetype \
    ttf-freefont \
    font-noto-emoji \
    gcompat \
    python3 \
    py3-pip \
    pipx

ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1
ENV AGENT_BROWSER_EXECUTABLE_PATH=/usr/bin/chromium-browser
ENV IS_SANDBOX=1

# Install uv (provides uv and uvx)
RUN curl -LsSf https://astral.sh/uv/install.sh | bash

# Install Claude Code (native install, auto-updates)
RUN curl -fsSL https://claude.ai/install.sh | bash

# Install agent-browser (uses system Chromium via env vars above)
RUN npm install -g agent-browser

# Install Claude Code skills
RUN npx -y skills add https://github.com/vercel-labs/skills --skill find-skills -g -y && \
    npx -y skills add https://github.com/anthropics/skills --skill skill-creator -g -y && \
    npx -y skills add https://github.com/vercel-labs/agent-browser --skill agent-browser -g -y

# Install Go (copy from builder)
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/root/.local/bin:/usr/local/go/bin:${PATH}"

# Copy application binary
COPY --from=builder /build/nclaw /usr/local/bin/nclaw

# Copy skills globally for Claude Code
COPY .claude/skills/schedule /root/.claude/skills/schedule
COPY .claude/skills/send-file /root/.claude/skills/send-file
COPY .claude/skills/webhook /root/.claude/skills/webhook

WORKDIR /app

ENTRYPOINT ["nclaw"]
