# Checks for Hermit (IMPLEMENTATION_PLAN.md, milestone 0).
#
#   make check   everything CI runs; the Stop hook requires it to pass
#   make fuzz    every fuzz target, FUZZTIME each (default 60s)
#
# Development tools come from the separate module in tools/, so they are never
# linked into hermit. They are built into bin/, which is gitignored.

# Hooks may run without the developer's shell PATH; fall back to the standard
# install location. depcheck's package loader runs `go` from PATH too.
ifeq ($(shell command -v go 2>/dev/null),)
ifneq ($(wildcard /usr/local/go/bin/go),)
export PATH := /usr/local/go/bin:$(PATH)
GO ?= /usr/local/go/bin/go
endif
endif

GO       ?= go
BIN      := $(CURDIR)/bin
GOOS_ALL := darwin,linux,windows
FUZZTIME ?= 60s

export CGO_ENABLED := 0
export GOWORK      := off

.PHONY: check fmt-check mod-check vet staticcheck test tools-test depcheck pincheck vulncheck scripts-test fuzz tools

check: fmt-check mod-check vet staticcheck test tools-test depcheck pincheck vulncheck scripts-test
	@echo "make check: all checks passed"

tools:
	@$(GO) -C tools build -o $(BIN)/ honnef.co/go/tools/cmd/staticcheck golang.org/x/vuln/cmd/govulncheck ./cmd/depcheck ./cmd/pincheck

fmt-check:
	@out="$$(gofmt -l cmd internal tools)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# go.mod and go.sum are tidy, and vendor/ matches them once there are
# dependencies (approved third-party code is vendored).
mod-check:
	$(GO) mod tidy -diff
	$(GO) -C tools mod tidy -diff
	@if [ -d vendor ]; then rm -rf "$(BIN)/vendor-check" && $(GO) mod vendor -o "$(BIN)/vendor-check" && diff -r vendor "$(BIN)/vendor-check"; fi

vet:
	$(GO) vet ./...
	$(GO) -C tools vet ./...

staticcheck: tools
	$(BIN)/staticcheck ./...
	cd tools && $(BIN)/staticcheck ./...

test:
	$(GO) test ./...

tools-test:
	$(GO) -C tools test ./...

depcheck: tools
	$(BIN)/depcheck -approved deps/approved-modules.txt -goos $(GOOS_ALL) .

pincheck: tools
	$(BIN)/pincheck .github

vulncheck: tools
	$(BIN)/govulncheck ./...

scripts-test:
	bash scripts/test-scripts.sh

fuzz:
	bash scripts/fuzz.sh $(FUZZTIME)
