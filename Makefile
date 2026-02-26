.PHONY: run lint test docker docker-claude docker-codex docker-copilot smoke-test

VERSION    ?= dev
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y%m%d%H%M%S)
LDFLAGS     = -X github.com/nickalie/nclaw/internal/version.Version=$(VERSION) \
              -X github.com/nickalie/nclaw/internal/version.Commit=$(COMMIT) \
              -X github.com/nickalie/nclaw/internal/version.BuildDate=$(BUILD_DATE)

run:
	go run -ldflags "$(LDFLAGS)" ./cmd/nclaw

lint:
	golangci-lint run ./...

test:
	CGO_ENABLED=1 go test ./...

docker:
	docker rm -f nclaw 2>/dev/null; \
	docker build -f docker/Dockerfile --target all -t nclaw . && \
	docker run --name nclaw \
		--env-file .env \
		-v $(CURDIR)/data:/app/data:Z \
		-v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro,Z \
		--network=host \
		nclaw

docker-claude:
	docker build -f docker/Dockerfile --target claude -t nclaw:claude .

docker-codex:
	docker build -f docker/Dockerfile --target codex -t nclaw:codex .

docker-copilot:
	docker build -f docker/Dockerfile --target copilot -t nclaw:copilot .

smoke-test:
	./test/install/smoke-test.sh $(TESTS)
