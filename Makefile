.PHONY: test build release

PNPM ?= pnpm

test:
	go test ./...
	$(PNPM) --dir web typecheck

build:
	$(PNPM) --dir web build
	go build ./cmd/netprobe-server ./cmd/netcheck ./cmd/netprobe-deploy

release:
	go run ./cmd/netprobe-release
