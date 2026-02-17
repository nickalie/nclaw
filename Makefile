.PHONY: run lint

run:
	go run ./cmd/nclaw

lint:
	golangci-lint run ./...
