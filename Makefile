.PHONY: run lint test

run:
	go run ./cmd/nclaw

lint:
	golangci-lint run ./...

test:
	CGO_ENABLED=1 go test ./...
