# Work — build orchestration.
#
# `make build` is the primary target: it cross-compiles the embedded seed
# binaries first, then builds the `work` CLI that embeds them.

GO       ?= go
BIN_DIR  := bin
SEED_DIR := seed/dist

# GOOS/GOARCH pairs the release binary carries embedded seed components for.
SEED_PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

.PHONY: all build seed test lint clean

all: build

## seed: cross-compile the seed starter and locator for every target platform
## into seed/dist/<goos>_<goarch>/.
seed:
	@rm -rf $(SEED_DIR)
	@for platform in $(SEED_PLATFORMS); do \
		goos=$${platform%/*}; goarch=$${platform#*/}; \
		outdir=$(SEED_DIR)/$${goos}_$${goarch}; \
		ext=; [ "$${goos}" = "windows" ] && ext=.exe; \
		mkdir -p $${outdir}; \
		echo "seed: $${goos}/$${goarch}"; \
		CGO_ENABLED=0 GOOS=$${goos} GOARCH=$${goarch} \
			$(GO) build -trimpath -o $${outdir}/starter$${ext} ./seed/starter || exit 1; \
		CGO_ENABLED=0 GOOS=$${goos} GOARCH=$${goarch} \
			$(GO) build -trimpath -o $${outdir}/locator$${ext} ./seed/locator || exit 1; \
	done

## build: build the work CLI (depends on seed).
build: seed
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -o $(BIN_DIR)/work ./cmd/work

## test: run the full test suite.
test:
	$(GO) test ./...

## lint: formatting, vet, and staticcheck must all be clean.
lint:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$${unformatted}" ]; then \
		echo "gofmt needs to run on:"; echo "$${unformatted}"; exit 1; \
	fi
	$(GO) vet ./...
	$(GO) tool staticcheck ./...

## clean: remove build output.
clean:
	rm -rf $(BIN_DIR) $(SEED_DIR)
