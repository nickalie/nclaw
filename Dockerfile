FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o nclaw ./cmd/nclaw

FROM node:24-alpine

RUN apk add --no-cache \
    git \
    bash \
    curl \
    openssh-client \
    github-cli \
    kubectl \
    flux \
    kustomize

# Install Claude Code (native install, auto-updates)
RUN curl -fsSL https://claude.ai/install.sh | bash

# Install Claude Code skills
RUN npx -y skills add https://github.com/vercel-labs/skills --skill find-skills && \
    npx -y skills add https://github.com/anthropics/skills --skill skill-creator

# Install Go (copy from builder)
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

# Copy application binary
COPY --from=builder /build/nclaw /usr/local/bin/nclaw

WORKDIR /app

ENTRYPOINT ["nclaw"]
