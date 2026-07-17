.PHONY: test build release

test:
	go test ./...

build:
	go build ./cmd/netprobe-server ./cmd/netcheck

release:
	./scripts/release-local.sh
