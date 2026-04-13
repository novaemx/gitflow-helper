BINARY   := gitflow
MODULE   := github.com/luis-lozano/gitflow-helper
VERSION  ?= $(shell cat VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.version=$(VERSION)
BUILD    := CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)'
DIST     := dist

.PHONY: build build-all universal clean test vet lint release install uninstall
.PHONY: package-homebrew package-choco package-winget package-skill package-all

# ── Install directory detection ──────────────────────────────
# Prefer /usr/local/bin (non-OS, universally in PATH).
# Override: make install INSTALL_DIR=/your/path
INSTALL_DIR ?= /usr/local/bin

# Detect OS/arch for the running system
UNAME_S := $(shell uname -s | tr '[:upper:]' '[:lower:]')
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),darwin)
  HOST_OS := darwin
else ifeq ($(UNAME_S),linux)
  HOST_OS := linux
else
  HOST_OS := windows
endif

ifeq ($(UNAME_M),x86_64)
  HOST_ARCH := amd64
else ifeq ($(UNAME_M),amd64)
  HOST_ARCH := amd64
else ifeq ($(UNAME_M),aarch64)
  HOST_ARCH := arm64
else ifeq ($(UNAME_M),arm64)
  HOST_ARCH := arm64
else
  HOST_ARCH := $(UNAME_M)
endif

## build: compile for current platform
build:
	$(BUILD) -o $(BINARY) ./cmd/gitflow

## build-all: cross-compile all targets
build-all: clean
	@mkdir -p $(DIST)
	@echo "→ Linux amd64"
	GOOS=linux   GOARCH=amd64 $(BUILD) -o $(DIST)/$(BINARY)-linux-amd64       ./cmd/gitflow
	@echo "→ Windows amd64"
	GOOS=windows GOARCH=amd64 $(BUILD) -o $(DIST)/$(BINARY)-windows-amd64.exe ./cmd/gitflow
	@echo "→ macOS amd64"
	GOOS=darwin  GOARCH=amd64 $(BUILD) -o $(DIST)/$(BINARY)-darwin-amd64      ./cmd/gitflow
	@echo "→ macOS arm64"
	GOOS=darwin  GOARCH=arm64 $(BUILD) -o $(DIST)/$(BINARY)-darwin-arm64      ./cmd/gitflow
	@echo "Done. Binaries in $(DIST)/"

## universal: create macOS universal binary (requires lipo)
universal: build-all
	@echo "→ macOS universal binary"
	lipo -create -output $(DIST)/$(BINARY)-darwin-universal \
		$(DIST)/$(BINARY)-darwin-amd64 \
		$(DIST)/$(BINARY)-darwin-arm64
	@echo "Created $(DIST)/$(BINARY)-darwin-universal"

## clean: remove build artifacts
clean:
	rm -rf $(DIST) $(BINARY)

## test: run all tests
test:
	go test ./... -v

## vet: run go vet
vet:
	go vet ./...

## lint: run staticcheck (install with go install honnef.co/go/tools/cmd/staticcheck@latest)
lint:
	@command -v staticcheck >/dev/null 2>&1 || { echo "install staticcheck first"; exit 1; }
	staticcheck ./...

## release: run goreleaser (requires goreleaser installed)
release:
	goreleaser release --clean

## release-snapshot: test goreleaser locally without publishing
release-snapshot:
	goreleaser release --snapshot --clean

## install: build for current OS/arch and install to INSTALL_DIR (/usr/local/bin)
## Usage: make install              (may need sudo)
##        sudo make install         (if /usr/local/bin is root-owned)
##        make install INSTALL_DIR=~/.local/bin   (no sudo needed)
install: build
	@echo "→ Installing $(BINARY) to $(INSTALL_DIR) ($(HOST_OS)/$(HOST_ARCH))"
	@mkdir -p $(INSTALL_DIR) 2>/dev/null || { echo "Cannot create $(INSTALL_DIR). Try: sudo make install"; exit 1; }
	@install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY) 2>/dev/null \
		|| { echo "Permission denied on $(INSTALL_DIR). Try: sudo make install"; exit 1; }
	@echo "Installed: $(INSTALL_DIR)/$(BINARY) v$(VERSION)"
	@echo "Verify:    $(BINARY) --version"

## uninstall: remove gitflow binary from INSTALL_DIR
uninstall:
	@if [ -f "$(INSTALL_DIR)/$(BINARY)" ]; then \
		rm -f "$(INSTALL_DIR)/$(BINARY)"; \
		echo "Removed $(INSTALL_DIR)/$(BINARY)"; \
	else \
		echo "$(BINARY) not found in $(INSTALL_DIR), nothing to remove"; \
	fi

## version: print the current version
version:
	@echo $(VERSION)

# ── Packaging Targets ─────────────────────────────────────

## package-homebrew: build snapshot and prepare Homebrew formula via goreleaser
package-homebrew:
	@echo "→ Building Homebrew snapshot..."
	goreleaser release --snapshot --clean
	@echo "Homebrew formula generated in dist/."
	@echo "Test locally: brew install --formula dist/homebrew/Formula/gitflow-helper.rb"

## package-choco: package Chocolatey nupkg (requires choco CLI on Windows/Mono)
package-choco: build-all
	@echo "→ Stamping version $(VERSION) into Chocolatey package..."
	@sed 's|<version>.*</version>|<version>$(VERSION)</version>|' \
		packaging/chocolatey/gitflow-helper.nuspec > packaging/chocolatey/gitflow-helper.nuspec.tmp \
		&& mv packaging/chocolatey/gitflow-helper.nuspec.tmp packaging/chocolatey/gitflow-helper.nuspec
	@sed "s|\$$version.*=.*|\$$version     = '$(VERSION)'|" \
		packaging/chocolatey/tools/chocolateyinstall.ps1 > packaging/chocolatey/tools/chocolateyinstall.ps1.tmp \
		&& mv packaging/chocolatey/tools/chocolateyinstall.ps1.tmp packaging/chocolatey/tools/chocolateyinstall.ps1
	@echo "→ Packaging Chocolatey nupkg..."
	cd packaging/chocolatey && choco pack 2>/dev/null || echo "  (choco not found — nupkg not built, but manifests are updated)"
	@echo "Done. To publish: choco push packaging/chocolatey/gitflow-helper.$(VERSION).nupkg --source https://push.chocolatey.org/"

## package-winget: stamp version into winget manifest
package-winget:
	@echo "→ Updating winget manifest to v$(VERSION)..."
	@sed 's|PackageVersion:.*|PackageVersion: $(VERSION)|' \
		packaging/winget/luis-lozano.gitflow-helper.yaml > packaging/winget/luis-lozano.gitflow-helper.yaml.tmp \
		&& mv packaging/winget/luis-lozano.gitflow-helper.yaml.tmp packaging/winget/luis-lozano.gitflow-helper.yaml
	@sed 's|/v[0-9][0-9.]*/|/v$(VERSION)/|g; s|gitflow-[0-9][0-9.]*-|gitflow-$(VERSION)-|g' \
		packaging/winget/luis-lozano.gitflow-helper.yaml > packaging/winget/luis-lozano.gitflow-helper.yaml.tmp \
		&& mv packaging/winget/luis-lozano.gitflow-helper.yaml.tmp packaging/winget/luis-lozano.gitflow-helper.yaml
	@echo "Done. Validate: winget validate packaging/winget/luis-lozano.gitflow-helper.yaml"
	@echo "Submit PR to microsoft/winget-pkgs with the updated manifest."

## package-skill: publish gitflow skill to skills.sh
package-skill:
	@echo "→ Publishing gitflow skill to skills.sh..."
	npx @anthropic/skills-cli publish .cursor/skills/gitflow/SKILL.md 2>/dev/null \
		|| npx skillsadd publish .cursor/skills/gitflow/SKILL.md 2>/dev/null \
		|| echo "  (skills CLI not available — install with: npm i -g @anthropic/skills-cli)"
	@echo "Done."

## package-all: build all package formats
package-all: package-homebrew package-choco package-winget package-skill
	@echo "All packages built/updated for v$(VERSION)."
