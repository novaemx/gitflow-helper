BINARY   := gitflow
MODULE   := github.com/novaemx/gitflow-helper
VERSION  ?= $(shell cat VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.version=$(VERSION)
BUILD    := CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)'
DIST     := dist
TAG      ?= v$(VERSION)
GITHUB_REPO ?= novaemx/gitflow-helper
WINDOWS_ARCHIVE := $(DIST)/$(BINARY)-$(VERSION)-windows-amd64.zip
LINUX_ARCHIVE   := $(DIST)/$(BINARY)-$(VERSION)-linux-amd64.tar.gz
DARWIN_ARCHIVE  := $(DIST)/$(BINARY)-$(VERSION)-darwin-universal.tar.gz
CHECKSUMS_FILE  := $(DIST)/checksums.txt

.PHONY: build build-all universal clean test vet lint release install uninstall
.PHONY: release-local release-local-github
.PHONY: package-homebrew package-choco package-winget package-all
.PHONY: publish-github publish-homebrew publish-winget publish-choco publish-all

# ── OS/arch detection ────────────────────────────────────────
UNAME_S := $(shell uname -s | tr '[:upper:]' '[:lower:]')
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),darwin)
  HOST_OS := darwin
else ifeq ($(UNAME_S),linux)
  HOST_OS := linux
else
  HOST_OS := windows
endif

# ── Install directory detection ──────────────────────────────
# Automatically selects a user-writable directory already in PATH.
# No sudo/root/admin required. Override: make install INSTALL_DIR=/your/path
ifeq ($(HOST_OS),windows)
  # Git Bash on Windows: use cygpath to get a proper Unix-style path
  HOME_UNIX := $(shell cygpath -u '$(HOME)')
  INSTALL_DIR ?= $(HOME_UNIX)/bin
else ifeq ($(HOST_OS),darwin)
  INSTALL_DIR ?= $(HOME)/.local/bin
else
  # Linux: ~/.local/bin (XDG standard, no root needed)
  INSTALL_DIR ?= $(HOME)/.local/bin
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

$(CHECKSUMS_FILE):
	@echo "→ Building release artifacts locally (no cloud build)..."
	goreleaser release --clean --skip=publish
	@test -f "$(CHECKSUMS_FILE)" || (echo "Expected $(CHECKSUMS_FILE) was not generated" && exit 1)
	@echo "Done. Artifacts and checksums in $(DIST)/"

## release-local: build release artifacts locally only (no GitHub Actions)
release-local: $(CHECKSUMS_FILE)

## release-local-github: build locally and upload artifacts to existing GitHub release tag
## Usage: make release-local-github TAG=v0.5.12
release-local-github: $(CHECKSUMS_FILE)
	@test -n "$(TAG)" || (echo "TAG is required. Example: make release-local-github TAG=v0.5.12" && exit 1)
	@command -v gh >/dev/null 2>&1 || { echo "GitHub CLI (gh) is required"; exit 1; }
	@echo "→ Uploading artifacts from $(DIST)/ to release $(TAG)..."
	@for f in $(DIST)/*; do \
		gh release upload "$(TAG)" "$$f" --clobber; \
	done
	@echo "Done. Local-built artifacts uploaded to GitHub release $(TAG)."

## publish-github: create/update GitHub Release and upload locally-built artifacts
## Usage: make publish-github TAG=v0.5.12
publish-github: $(CHECKSUMS_FILE)
	@test -n "$(TAG)" || (echo "TAG is required. Example: make publish-github TAG=v0.5.12" && exit 1)
	@command -v gh >/dev/null 2>&1 || { echo "GitHub CLI (gh) is required"; exit 1; }
	@echo "→ Ensuring GitHub release $(TAG) exists..."
	@if ! gh release view "$(TAG)" --repo "$(GITHUB_REPO)" >/dev/null 2>&1; then \
		gh release create "$(TAG)" --repo "$(GITHUB_REPO)" --title "$(TAG)" --notes "Local build artifacts"; \
	fi
	@echo "→ Uploading local artifacts to GitHub release $(TAG)..."
	@for f in $(DIST)/*; do \
		gh release upload "$(TAG)" "$$f" --repo "$(GITHUB_REPO)" --clobber; \
	done
	@echo "Done. GitHub release $(TAG) now hosts locally-built artifacts."

## publish-homebrew: update local Homebrew formula to point at current GitHub release artifacts
publish-homebrew: $(CHECKSUMS_FILE)
	@darwin_sha=$$(awk '/gitflow-$(VERSION)-darwin-universal.tar.gz/ {print $$1}' $(CHECKSUMS_FILE)); \
	linux_sha=$$(awk '/gitflow-$(VERSION)-linux-amd64.tar.gz/ {print $$1}' $(CHECKSUMS_FILE)); \
	[ -n "$$darwin_sha" ] || { echo "Missing darwin checksum"; exit 1; }; \
	[ -n "$$linux_sha" ] || { echo "Missing linux checksum"; exit 1; }; \
	awk -v version="$(VERSION)" -v tag="$(TAG)" -v darwin_sha="$$darwin_sha" -v linux_sha="$$linux_sha" ' \
		BEGIN { sha_count = 0 } \
		/^[[:space:]]*version "/ { sub(/version ".*"/, "version \"" version "\"") } \
		/releases\/download\/v[^\/]*\/gitflow-[^"]*-darwin-universal.tar.gz/ { sub(/releases\/download\/v[^\/]*\/gitflow-[^"]*-darwin-universal.tar.gz/, "releases/download/" tag "/gitflow-" version "-darwin-universal.tar.gz") } \
		/releases\/download\/v[^\/]*\/gitflow-[^"]*-linux-amd64.tar.gz/ { sub(/releases\/download\/v[^\/]*\/gitflow-[^"]*-linux-amd64.tar.gz/, "releases/download/" tag "/gitflow-" version "-linux-amd64.tar.gz") } \
		/^[[:space:]]*sha256 "/ { \
			sha_count++; \
			if (sha_count == 1) { sub(/sha256 ".*"/, "sha256 \"" darwin_sha "\"") } \
			else if (sha_count == 2) { sub(/sha256 ".*"/, "sha256 \"" linux_sha "\"") } \
		} \
		{ print } \
	' packaging/homebrew/gitflow-helper.rb > packaging/homebrew/gitflow-helper.rb.tmp; \
	mv packaging/homebrew/gitflow-helper.rb.tmp packaging/homebrew/gitflow-helper.rb
	@echo "Done. Updated packaging/homebrew/gitflow-helper.rb for $(TAG)."

## publish-winget: update local Winget manifest to point at current GitHub release artifact and checksum
publish-winget: $(CHECKSUMS_FILE)
	@windows_sha=$$(awk '/gitflow-$(VERSION)-windows-amd64.zip/ {print $$1}' $(CHECKSUMS_FILE)); \
	[ -n "$$windows_sha" ] || { echo "Missing windows checksum"; exit 1; }; \
	sed -i.bak 's|PackageVersion: .*|PackageVersion: $(VERSION)|' packaging/winget/novaemx.gitflow-helper.yaml; \
	sed -i.bak 's|InstallerUrl: .*|InstallerUrl: https://github.com/$(GITHUB_REPO)/releases/download/$(TAG)/gitflow-$(VERSION)-windows-amd64.zip|' packaging/winget/novaemx.gitflow-helper.yaml; \
	sed -i.bak 's|InstallerSha256: .*|InstallerSha256: '"$$windows_sha"'|' packaging/winget/novaemx.gitflow-helper.yaml; \
	rm -f packaging/winget/novaemx.gitflow-helper.yaml.bak
	@echo "Done. Updated Winget manifest for $(TAG)."

## publish-choco: update Chocolatey metadata to point at current GitHub release artifact and checksum
publish-choco: $(CHECKSUMS_FILE)
	@windows_sha=$$(awk '/gitflow-$(VERSION)-windows-amd64.zip/ {print $$1}' $(CHECKSUMS_FILE)); \
	[ -n "$$windows_sha" ] || { echo "Missing windows checksum"; exit 1; }; \
	sed -i.bak 's|<version>.*</version>|<version>$(VERSION)</version>|' packaging/chocolatey/gitflow-helper.nuspec; \
	sed -i.bak "s|\$$version     = '.*'|\$$version     = '$(VERSION)'|" packaging/chocolatey/tools/chocolateyinstall.ps1; \
	sed -i.bak "s|\$$url      = \".*\"|\$$url      = \"https://github.com/$(GITHUB_REPO)/releases/download/$(TAG)/gitflow-$(VERSION)-windows-amd64.zip\"|" packaging/chocolatey/tools/chocolateyinstall.ps1; \
	sed -i.bak "s|\$$checksum = '.*'|\$$checksum = '$$windows_sha'|" packaging/chocolatey/tools/chocolateyinstall.ps1; \
	rm -f packaging/chocolatey/gitflow-helper.nuspec.bak packaging/chocolatey/tools/chocolateyinstall.ps1.bak
	@echo "Done. Updated Chocolatey package metadata for $(TAG)."

## publish-all: build locally, upload artifacts to GitHub Releases, and stamp package manifests
publish-all: publish-github publish-homebrew publish-winget publish-choco
	@echo "All publish targets completed for $(TAG)."

## release-snapshot: test goreleaser locally without publishing
release-snapshot:
	goreleaser release --snapshot --clean

## install: build for current OS/arch and install to a user-writable directory (no sudo needed)
## Windows → ~/bin | Linux/macOS → ~/.local/bin
## Override: make install INSTALL_DIR=/custom/path
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
		packaging/winget/novaemx.gitflow-helper.yaml > packaging/winget/novaemx.gitflow-helper.yaml.tmp \
		&& mv packaging/winget/novaemx.gitflow-helper.yaml.tmp packaging/winget/novaemx.gitflow-helper.yaml
	@sed 's|/v[0-9][0-9.]*/|/v$(VERSION)/|g; s|gitflow-[0-9][0-9.]*-|gitflow-$(VERSION)-|g' \
		packaging/winget/novaemx.gitflow-helper.yaml > packaging/winget/novaemx.gitflow-helper.yaml.tmp \
		&& mv packaging/winget/novaemx.gitflow-helper.yaml.tmp packaging/winget/novaemx.gitflow-helper.yaml
	@echo "Done. Validate: winget validate packaging/winget/novaemx.gitflow-helper.yaml"
	@echo "Submit PR to microsoft/winget-pkgs with the updated manifest."

## package-all: build all package formats
package-all: package-homebrew package-choco package-winget
	@echo "All packages built/updated for v$(VERSION)."
