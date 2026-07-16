BINARY  := claude-profile
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test lint fmt vet clean install cross hooks

all: lint test build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test -race ./...

lint: vet
	@test -z "$$(gofmt -s -l .)" || { echo "gofmt -s needed on:"; gofmt -s -l .; exit 1; }
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; \
	else echo "golangci-lint not installed — skipping (CI runs it; install: https://golangci-lint.run/welcome/install/)"; fi

fmt:
	gofmt -s -w .

vet:
	go vet ./...

install: build
	install -m 0755 $(BINARY) $(HOME)/.local/bin/$(BINARY)

cross:
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)_linux_amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)_linux_arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)_darwin_amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)_darwin_arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)_windows_amd64.exe .

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks enabled: lint on commit, tests on push (skip with --no-verify)."

clean:
	rm -rf $(BINARY) $(BINARY).exe dist/
