.PHONY: build test install-completions

build:
	go build -o bin/hidrive ./cmd/hidrive

test:
	go test ./...

install-completions:
	sh scripts/install-completions.sh
