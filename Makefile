.PHONY: run lint test docker

run:
	go run ./cmd/nclaw

lint:
	golangci-lint run ./...

test:
	CGO_ENABLED=1 go test ./...

docker:
	docker rm -f nclaw 2>/dev/null; \
	docker build -t nclaw . && \
	docker run --name nclaw \
		--env-file .env \
		-v $(CURDIR)/data:/app/data:Z \
		-v ~/.claude/.credentials.json:/root/.claude/.credentials.json:rw,Z \
		--network=host \
		nclaw
