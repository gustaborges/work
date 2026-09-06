# Work — build orchestration.
#
# Everyday:  `make build`   — host-only seed, small binary
#            `make install` — copy it onto PATH (default ~/.local/bin)
# Release:   `make release` — cross-compile per platform, each binary carrying
#                             only its own platform's seed components.

GO          ?= go
BIN_DIR     := bin
SEED_DIR    := seed/dist
RELEASE_DIR := bin/release

# Install prefix. `make install` writes $(DESTDIR)$(PREFIX)/bin/work; packagers
# override both, a system install is `sudo make PREFIX=/usr/local install`.
PREFIX  ?= $(HOME)/.local
DESTDIR ?=

# GOOS/GOARCH pairs a release ships prebuilt binaries for.
RELEASE_PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

HOST_GOOS   := $(shell $(GO) env GOOS)
HOST_GOARCH := $(shell $(GO) env GOARCH)

.PHONY: all build build-all seed seed-all seed-one install uninstall release test lint clean

all: build

## seed: build the seed starter+locator for the host platform only.
seed:
	@rm -rf $(SEED_DIR)
	@$(MAKE) --no-print-directory seed-one GOOS=$(HOST_GOOS) GOARCH=$(HOST_GOARCH)

## seed-all: build the seed components for every release platform.
seed-all:
	@rm -rf $(SEED_DIR)
	@for platform in $(RELEASE_PLATFORMS); do \
		$(MAKE) --no-print-directory seed-one GOOS=$${platform%/*} GOARCH=$${platform#*/} || exit 1; \
	done

## seed-one: internal — build one platform's seed pair (GOOS/GOARCH required).
seed-one:
	@outdir=$(SEED_DIR)/$(GOOS)_$(GOARCH); ext=; [ "$(GOOS)" = windows ] && ext=.exe; \
	mkdir -p $$outdir; \
	echo "seed: $(GOOS)/$(GOARCH)"; \
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -trimpath -o $$outdir/starter$$ext ./seed/starter && \
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -trimpath -o $$outdir/locator$$ext ./seed/locator

## build: build the work CLI for the host (embeds host-only seed).
build: seed
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -o $(BIN_DIR)/work ./cmd/work

## build-all: a host build that embeds every release platform's seed (large binary).
build-all: seed-all
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -o $(BIN_DIR)/work ./cmd/work

## install: install the built binary to $(DESTDIR)$(PREFIX)/bin (default ~/.local/bin).
install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 0755 $(BIN_DIR)/work $(DESTDIR)$(PREFIX)/bin/work
	@echo "installed: $(DESTDIR)$(PREFIX)/bin/work"

## uninstall: remove a previously installed binary.
uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/work

## release: cross-compile work for every release platform into
## bin/release/<goos>_<goarch>/, each embedding only its own platform's seed.
release:
	@rm -rf $(RELEASE_DIR)
	@for platform in $(RELEASE_PLATFORMS); do \
		goos=$${platform%/*}; goarch=$${platform#*/}; ext=; [ "$$goos" = windows ] && ext=.exe; \
		echo "release: $$goos/$$goarch"; \
		rm -rf $(SEED_DIR); \
		$(MAKE) --no-print-directory seed-one GOOS=$$goos GOARCH=$$goarch || exit 1; \
		outdir=$(RELEASE_DIR)/$${goos}_$${goarch}; mkdir -p $$outdir; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch \
			$(GO) build -trimpath -o $$outdir/work$$ext ./cmd/work || exit 1; \
	done
	@$(MAKE) --no-print-directory seed
	@echo "release binaries in $(RELEASE_DIR)/"

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
	rm -rf $(BIN_DIR) $(SEED_DIR) $(RELEASE_DIR)
