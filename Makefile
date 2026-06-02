BINARY   := gitflow
MODULE   := github.com/novaemx/gitflow-helper
VERSION  ?= $(shell cat VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo "unknown")
LDFLAGS  := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)
BUILD    := CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)'
DIST     := dist
TAG      ?= $(shell \
	branch=$$(git branch --show-current 2>/dev/null || true); \
	if echo "$$branch" | grep -Eq '^(release|hotfix)/'; then \
		ver=$${branch#*/}; \
		ver=$${ver#v}; \
		echo "v$$ver"; \
	else \
		head_tag=$$(git describe --exact-match --tags HEAD 2>/dev/null || true); \
		if [ -n "$$head_tag" ]; then \
			echo "$$head_tag"; \
		else \
			latest_tag=$$(git tag --sort=-version:refname | head -1); \
			if [ -n "$$latest_tag" ]; then \
				echo "$$latest_tag"; \
			else \
				echo "v$(VERSION)"; \
			fi; \
		fi; \
	fi)
RELEASE_VERSION ?= $(patsubst v%,%,$(TAG))
LATEST_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo v$(VERSION))
GITHUB_REPO ?= novaemx/gitflow-helper
WINDOWS_ARCHIVE := $(DIST)/$(BINARY)-$(VERSION)-windows-amd64.zip
LINUX_ARCHIVE   := $(DIST)/$(BINARY)-$(VERSION)-linux-amd64.tar.gz
LINUX_ARM64_ARCHIVE := $(DIST)/$(BINARY)-$(VERSION)-linux-aarch64.tar.gz
DARWIN_ARCHIVE  := $(DIST)/$(BINARY)-$(VERSION)-darwin-arm64.tar.gz
CHECKSUMS_FILE  := $(DIST)/checksums.txt
COVER_DIR := test

.PHONY: build build-all universal clean test vet lint install uninstall
.PHONY: release-local release-snapshot
.PHONY: package-homebrew package-winget package-all

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

# On Windows the binary needs a .exe suffix
ifeq ($(HOST_OS),windows)
	EXE_SUFFIX := .exe
else
	EXE_SUFFIX :=
endif

# Full binary name including platform suffix when needed
BINARY_FULL := $(BINARY)$(EXE_SUFFIX)

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
	$(BUILD) -o $(BINARY_FULL) ./cmd/gitflow

## build-all: cross-compile all targets
build-all: clean
	@mkdir -p $(DIST)
	@echo "→ Linux amd64"
	GOOS=linux   GOARCH=amd64 $(BUILD) -o $(DIST)/$(BINARY)-linux-amd64       ./cmd/gitflow
	@echo "→ Linux arm64 (aarch64)"
	GOOS=linux   GOARCH=arm64 $(BUILD) -o $(DIST)/$(BINARY)-linux-aarch64      ./cmd/gitflow
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
	rm -rf $(DIST) $(BINARY) $(BINARY).exe
	@# Remove common test/coverage/debug and packaging temp artifacts across the repo.
	find . -type f \( -name "*.out" -o -name "*.test" -o -name "*.prof" -o -name "cover.out" -o -name "cover.html" -o -name "__debug_bin*" -o -name "*.bak" -o -name "*.tmp" -o -name ".DS_Store" \) -not -path "./.git/*" -delete
	@# Remove coverage artifacts generated into $(COVER_DIR)
	@mkdir -p $(COVER_DIR) 2>/dev/null || true
	@rm -f $(COVER_DIR)/*.cov $(COVER_DIR)/*.out $(COVER_DIR)/cover.* 2>/dev/null || true

## test: run all tests
test:
	go test ./... -v

## cover: run all tests and write a coverage profile into $(COVER_DIR)/coverage.out
cover:
	@mkdir -p $(COVER_DIR)
	go test ./... -covermode=atomic -coverprofile=$(COVER_DIR)/coverage.out

## cover-package: run tests for a single package and write profile into $(COVER_DIR)/<pkg>.cov
## Usage: make cover-package PKG=./internal/commands
cover-package:
	@mkdir -p $(COVER_DIR)
	@test -n "$(PKG)" || (echo "PKG is required. Example: make cover-package PKG=./internal/commands" && exit 1)
	go test $(PKG) -v -covermode=atomic -coverprofile=$(COVER_DIR)/$(notdir $(PKG)).cov

## vet: run go vet
vet:
	go vet ./...

## lint: run staticcheck (install with go install honnef.co/go/tools/cmd/staticcheck@latest)
lint:
	@command -v staticcheck >/dev/null 2>&1 || { echo "install staticcheck first"; exit 1; }
	staticcheck ./...

## release-local: build release artifacts locally only (no GitHub Actions)
## NOTE: this only BUILDS, never publishes. Binaries are pushed to the GitHub
## Release exclusively by .github/workflows/release.yml on a `v*` tag push.
release-local: $(CHECKSUMS_FILE)

$(CHECKSUMS_FILE):
	@echo "→ Building release artifacts locally (no cloud build)..."
	@_build_ref="$(BUILD_REF)"; \
	if [ -n "$$_build_ref" ]; then \
		echo "→ Building from requested ref $$_build_ref..."; \
	else \
		_current_branch=$$(git branch --show-current 2>/dev/null || true); \
		if git describe --exact-match --tags HEAD >/dev/null 2>&1; then \
			_build_ref="HEAD"; \
			echo "→ Auto BUILD_REF: HEAD is already tagged."; \
		elif [ "$$_current_branch" = "release/$(RELEASE_VERSION)" ] || [ "$$_current_branch" = "hotfix/$(RELEASE_VERSION)" ]; then \
			_build_ref="HEAD"; \
			echo "→ Auto BUILD_REF: using current branch HEAD ($$_current_branch) for $(TAG)."; \
		elif git rev-parse --verify --quiet "refs/tags/$(TAG)^{commit}" >/dev/null; then \
			_build_ref="$(TAG)"; \
			echo "→ Auto BUILD_REF: using existing tag $(TAG)."; \
		else \
			_build_ref=$$(git describe --tags --abbrev=0 2>/dev/null || true); \
			[ -n "$$_build_ref" ] || { echo "No git tag found to build release artifacts"; exit 1; }; \
			echo "→ Auto BUILD_REF: falling back to latest tag $$_build_ref."; \
		fi; \
	fi; \
	if [ "$$_build_ref" = "HEAD" ]; then \
		_current_branch=$$(git branch --show-current 2>/dev/null || true); \
		if [ "$$_current_branch" = "release/$(RELEASE_VERSION)" ] || [ "$$_current_branch" = "hotfix/$(RELEASE_VERSION)" ]; then \
			if ! git diff --quiet || ! git diff --cached --quiet; then \
				echo "Working tree is dirty; commit/stash changes before publishing $(TAG)."; \
				exit 1; \
			fi; \
			if git rev-parse --verify --quiet "refs/tags/$(TAG)^{commit}" >/dev/null; then \
				_tag_commit=$$(git rev-list -n 1 "$(TAG)"); \
				_head_commit=$$(git rev-parse HEAD); \
				if [ "$$_tag_commit" != "$$_head_commit" ]; then \
					echo "→ Auto BUILD_REF: $(TAG) exists but points to a different commit; building from HEAD in an ephemeral clone with local tag override."; \
					echo "  Tag commit:  $$_tag_commit"; \
					echo "  HEAD commit: $$_head_commit"; \
					_clone=$$(mktemp -d 2>/dev/null || mktemp -d -t gitflow-release.XXXXXX); \
					if ! git clone --quiet --no-hardlinks . "$$_clone"; then \
						echo "Failed to create ephemeral clone for release build."; \
						rm -rf "$$_clone"; \
						exit 1; \
					fi; \
					( cd "$$_clone" && git checkout --detach HEAD >/dev/null && git tag -f "$(TAG)" HEAD >/dev/null && goreleaser release --clean --skip=publish --config "$(CURDIR)/.goreleaser.yml" ); \
					_exit=$$?; \
					if [ $$_exit -eq 0 ]; then \
						rm -rf "$(DIST)"; \
						cp -R "$$_clone/$(DIST)" "$(DIST)"; \
					fi; \
					rm -rf "$$_clone"; \
					exit $$_exit; \
				fi; \
				goreleaser release --clean --skip=publish; \
			else \
				_clone=$$(mktemp -d 2>/dev/null || mktemp -d -t gitflow-release.XXXXXX); \
				echo "→ Auto BUILD_REF: no $(TAG) tag yet; building from an ephemeral clone tagged at HEAD."; \
				if ! git clone --quiet --no-hardlinks . "$$_clone"; then \
					echo "Failed to create ephemeral clone for release build."; \
					rm -rf "$$_clone"; \
					exit 1; \
				fi; \
				( cd "$$_clone" && git checkout --detach HEAD >/dev/null && git tag "$(TAG)" HEAD && goreleaser release --clean --skip=publish --config "$(CURDIR)/.goreleaser.yml" ); \
				_exit=$$?; \
				if [ $$_exit -eq 0 ]; then \
					rm -rf "$(DIST)"; \
					cp -R "$$_clone/$(DIST)" "$(DIST)"; \
				fi; \
				rm -rf "$$_clone"; \
				exit $$_exit; \
			fi; \
		else \
			goreleaser release --clean --skip=publish; \
		fi; \
	elif git describe --exact-match --tags HEAD 2>/dev/null | grep -qx "$$_build_ref"; then \
		goreleaser release --clean --skip=publish; \
	elif git rev-parse --verify --quiet "$$_build_ref^{commit}" >/dev/null; then \
		_worktree=$$(mktemp -d 2>/dev/null || mktemp -d -t gitflow-release.XXXXXX); \
		if ! git worktree add --detach "$$_worktree" "$$_build_ref" >/dev/null; then \
			echo "Failed to create worktree for ref '$$_build_ref'."; \
			rm -rf "$$_worktree"; \
			exit 1; \
		fi; \
		( cd "$$_worktree" && goreleaser release --clean --skip=publish --config "$(CURDIR)/.goreleaser.yml" ); \
		_exit=$$?; \
		if [ $$_exit -eq 0 ]; then \
			rm -rf "$(DIST)"; \
			cp -R "$$_worktree/$(DIST)" "$(DIST)"; \
		fi; \
		git worktree remove "$$_worktree" --force >/dev/null 2>&1 || true; \
		rm -rf "$$_worktree"; \
		exit $$_exit; \
	else \
		echo "Resolved BUILD_REF '$$_build_ref' does not exist (tag/branch/commit)."; \
		echo "Hint: pass BUILD_REF=<valid-ref> explicitly to override auto-inference."; \
		exit 1; \
	fi
	@test -f "$(CHECKSUMS_FILE)" || (echo "Expected $(CHECKSUMS_FILE) was not generated" && exit 1)
	@echo "Done. Artifacts and checksums in $(DIST)/"

## release-snapshot: test goreleaser locally without publishing
release-snapshot:
	goreleaser release --snapshot --clean

## install: build for current OS/arch and install to a user-writable directory (no sudo needed)
## Windows → ~/bin | Linux/macOS → ~/.local/bin
## Override: make install INSTALL_DIR=/custom/path
install: build
	@echo "→ Installing $(BINARY) to $(INSTALL_DIR) ($(HOST_OS)/$(HOST_ARCH))"
	@mkdir -p $(INSTALL_DIR) 2>/dev/null || { echo "Cannot create $(INSTALL_DIR). Try: sudo make install"; exit 1; }
	@install -m 755 $(BINARY_FULL) $(INSTALL_DIR)/$(BINARY_FULL) 2>/dev/null \
		|| { echo "Permission denied on $(INSTALL_DIR). Try: sudo make install"; exit 1; }
	@# Also ensure a local copy exists in the project root with the OS suffix
	@cp -f $(BINARY_FULL) . 2>/dev/null || true
	@echo "Installed: $(INSTALL_DIR)/$(BINARY_FULL) v$(VERSION)"
	@echo "Verify:    $(BINARY_FULL) --version"

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

## package-homebrew: build snapshot and prepare Homebrew formula artifacts via goreleaser
package-homebrew:
	@echo "→ Building Homebrew snapshot..."
	goreleaser release --snapshot --clean
	@echo "Homebrew cask generated in dist/."
	@echo "Test locally: brew install --cask dist/homebrew/Casks/gitflow-helper.rb"

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
