BINARY   := gitflow
MODULE   := github.com/novaemx/gitflow-helper
VERSION  ?= $(shell cat VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.version=$(VERSION)
BUILD    := CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)'
DIST     := dist
TAG      ?= v$(VERSION)
RELEASE_VERSION ?= $(patsubst v%,%,$(TAG))
LATEST_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo v$(VERSION))
GITHUB_REPO ?= novaemx/gitflow-helper
WINDOWS_ARCHIVE := $(DIST)/$(BINARY)-$(VERSION)-windows-amd64.zip
LINUX_ARCHIVE   := $(DIST)/$(BINARY)-$(VERSION)-linux-amd64.tar.gz
DARWIN_ARCHIVE  := $(DIST)/$(BINARY)-$(VERSION)-darwin-universal.tar.gz
CHECKSUMS_FILE  := $(DIST)/checksums.txt
COVER_DIR := test

.PHONY: build build-all universal clean test vet lint release install uninstall
.PHONY: release-local release-local-github
.PHONY: package-homebrew package-choco package-winget package-all
.PHONY: publish-github publish-homebrew publish-winget publish-choco publish-linux publish-all
.PHONY: upload-release-assets cleanup-release-assets validate-release-assets validate-linux-packages
.PHONY: require-gh

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

## release: run goreleaser (requires goreleaser installed)
release:
	goreleaser release --clean

$(CHECKSUMS_FILE):
	@echo "→ Building release artifacts locally (no cloud build)..."
	@if [ -n "$(BUILD_REF)" ]; then \
		_build_ref="$(BUILD_REF)"; \
		echo "→ Building from requested ref $$_build_ref..."; \
		if git describe --exact-match --tags HEAD 2>/dev/null | grep -qx "$$_build_ref"; then \
			goreleaser release --clean --skip=publish; \
		else \
			_worktree=$$(mktemp -d 2>/dev/null || mktemp -d -t gitflow-release.XXXXXX); \
			git worktree add --detach "$$_worktree" "$$_build_ref" >/dev/null; \
			( cd "$$_worktree" && goreleaser release --clean --skip=publish --config "$(CURDIR)/.goreleaser.yml" ); \
			_exit=$$?; \
			if [ $$_exit -eq 0 ]; then \
				rm -rf "$(DIST)"; \
				cp -R "$$_worktree/$(DIST)" "$(DIST)"; \
			fi; \
			git worktree remove "$$_worktree" --force >/dev/null 2>&1 || true; \
			rm -rf "$$_worktree"; \
			exit $$_exit; \
		fi; \
	elif git describe --exact-match --tags HEAD >/dev/null 2>&1; then \
		goreleaser release --clean --skip=publish; \
	else \
		_build_tag=$$(git describe --tags --abbrev=0 2>/dev/null); \
		[ -n "$$_build_tag" ] || { echo "No git tag found to build release artifacts"; exit 1; }; \
		_worktree=$$(mktemp -d 2>/dev/null || mktemp -d -t gitflow-release.XXXXXX); \
		echo "→ HEAD is not tagged. Building from temporary worktree at $$_build_tag..."; \
		git worktree add --detach "$$_worktree" "$$_build_tag" >/dev/null; \
		( cd "$$_worktree" && goreleaser release --clean --skip=publish --config "$(CURDIR)/.goreleaser.yml" ); \
		_exit=$$?; \
		if [ $$_exit -eq 0 ]; then \
			rm -rf "$(DIST)"; \
			cp -R "$$_worktree/$(DIST)" "$(DIST)"; \
		fi; \
		git worktree remove "$$_worktree" --force >/dev/null 2>&1 || true; \
		rm -rf "$$_worktree"; \
		exit $$_exit; \
	fi
	@test -f "$(CHECKSUMS_FILE)" || (echo "Expected $(CHECKSUMS_FILE) was not generated" && exit 1)
	@echo "Done. Artifacts and checksums in $(DIST)/"

## release-local: build release artifacts locally only (no GitHub Actions)
release-local: $(CHECKSUMS_FILE)

require-gh:
	@command -v gh >/dev/null 2>&1 || { echo "GitHub CLI (gh) is required"; exit 1; }

validate-release-assets:
	@test -f "$(CHECKSUMS_FILE)" || (echo "Missing $(CHECKSUMS_FILE). Run make $(CHECKSUMS_FILE) first." && exit 1)
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '^gitflow-$(RELEASE_VERSION)-darwin-universal.tar.gz$$' || { echo "Missing darwin checksum for $(RELEASE_VERSION)"; exit 1; }
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '^gitflow-$(RELEASE_VERSION)-windows-amd64.zip$$' || { echo "Missing windows checksum for $(RELEASE_VERSION)"; exit 1; }
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '^gitflow-$(RELEASE_VERSION)-linux-amd64.tar.gz$$' || { echo "Missing linux tarball checksum for $(RELEASE_VERSION)"; exit 1; }

validate-linux-packages:
	@test -f "$(CHECKSUMS_FILE)" || (echo "Missing $(CHECKSUMS_FILE). Run make $(CHECKSUMS_FILE) first." && exit 1)
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '\\.deb$$' || { echo "Missing .deb package in checksums"; exit 1; }
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '\\.rpm$$' || { echo "Missing .rpm package in checksums"; exit 1; }
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '\\.pkg\\.tar\\.zst$$' || { echo "Missing Arch package (.pkg.tar.zst) in checksums"; exit 1; }

cleanup-release-assets:
	@test -n "$(RELEASE_TAG)" || (echo "RELEASE_TAG is required" && exit 1)
	@test -f "$(CHECKSUMS_FILE)" || (echo "Missing $(CHECKSUMS_FILE). Run make $(CHECKSUMS_FILE) first." && exit 1)
	@echo "→ Removing previously published assets from $(RELEASE_TAG) (if present)..."
	@{ awk '{print $$2}' "$(CHECKSUMS_FILE)"; echo "checksums.txt"; } | while read -r asset; do \
		[ -n "$$asset" ] || continue; \
		gh release delete-asset "$(RELEASE_TAG)" "$$asset" --repo "$(GITHUB_REPO)" --yes >/dev/null 2>&1 || true; \
	done

upload-release-assets:
	@test -n "$(RELEASE_TAG)" || (echo "RELEASE_TAG is required" && exit 1)
	@test -f "$(CHECKSUMS_FILE)" || (echo "Missing $(CHECKSUMS_FILE). Run make $(CHECKSUMS_FILE) first." && exit 1)
	@$(MAKE) validate-release-assets RELEASE_VERSION="$(RELEASE_VERSION)"
	@echo "→ Uploading release assets from $(CHECKSUMS_FILE) to $(RELEASE_TAG)..."
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | while read -r asset; do \
		[ -n "$$asset" ] || continue; \
		file="$(DIST)/$$asset"; \
		[ -f "$$file" ] || { echo "Missing artifact: $$file"; exit 1; }; \
		gh release upload "$(RELEASE_TAG)" "$$file" --repo "$(GITHUB_REPO)" --clobber; \
	done
	@gh release upload "$(RELEASE_TAG)" "$(CHECKSUMS_FILE)" --repo "$(GITHUB_REPO)" --clobber
	@echo "Done. Uploaded archives + checksums to $(RELEASE_TAG)."

## release-local-github: build locally and upload artifacts to the latest GitHub release tag
## Usage: make release-local-github
release-local-github:
	@$(MAKE) require-gh
	@$(MAKE) -B $(CHECKSUMS_FILE) BUILD_REF="$(LATEST_TAG)"
	@$(MAKE) cleanup-release-assets RELEASE_TAG="$(LATEST_TAG)"
	@$(MAKE) upload-release-assets RELEASE_TAG="$(LATEST_TAG)"
	@echo "Done. Local-built artifacts uploaded to GitHub release $(LATEST_TAG)."

## publish-github: create/update GitHub Release and upload locally-built artifacts
## Usage: make publish-github TAG=v0.5.12
publish-github:
	@test -n "$(TAG)" || (echo "TAG is required. Example: make publish-github TAG=v0.5.12" && exit 1)
	@$(MAKE) require-gh
	@$(MAKE) -B $(CHECKSUMS_FILE) BUILD_REF="$(TAG)"
	@$(MAKE) validate-release-assets RELEASE_VERSION="$(RELEASE_VERSION)"
	@echo "→ Ensuring GitHub release $(TAG) exists..."
	@if ! gh release view "$(TAG)" --repo "$(GITHUB_REPO)" >/dev/null 2>&1; then \
		gh release create "$(TAG)" --repo "$(GITHUB_REPO)" --title "$(TAG)" --notes "Local build artifacts"; \
	fi
	@$(MAKE) cleanup-release-assets RELEASE_TAG="$(TAG)"
	@$(MAKE) upload-release-assets RELEASE_TAG="$(TAG)" RELEASE_VERSION="$(RELEASE_VERSION)"
	@echo "Done. GitHub release $(TAG) now hosts locally-built artifacts."

## publish-homebrew: upload artifacts first, then update local Homebrew formula to point at the current GitHub release
publish-homebrew: publish-github
	@darwin_sha=$$(awk '/gitflow-$(RELEASE_VERSION)-darwin-universal.tar.gz/ {print $$1}' $(CHECKSUMS_FILE)); \
	[ -n "$$darwin_sha" ] || { echo "Missing darwin checksum"; exit 1; }; \
	awk -v version="$(RELEASE_VERSION)" -v tag="$(TAG)" -v darwin_sha="$$darwin_sha" ' \
		BEGIN { target_sha = "" } \
		/^[[:space:]]*version "/ { sub(/version ".*"/, "version \"" version "\"") } \
		/releases\/download\/v[^\/]*\/gitflow-[^"]*-darwin-universal.tar.gz/ { sub(/releases\/download\/v[^\/]*\/gitflow-[^"]*-darwin-universal.tar.gz/, "releases/download/" tag "/gitflow-" version "-darwin-universal.tar.gz") } \
		/gitflow-[^"]*-darwin-universal.tar.gz/ { target_sha = darwin_sha } \
		/^[[:space:]]*sha256 "/ { \
			if (target_sha != "") { \
				sub(/sha256 ".*"/, "sha256 \"" target_sha "\""); \
				target_sha = ""; \
			} \
		} \
		{ print } \
	' packaging/homebrew/gitflow-helper.rb > packaging/homebrew/gitflow-helper.rb.tmp; \
	mv packaging/homebrew/gitflow-helper.rb.tmp packaging/homebrew/gitflow-helper.rb
	@echo "Done. Updated packaging/homebrew/gitflow-helper.rb for $(TAG)."

## publish-winget: upload artifacts first, then update local Winget manifest to point at the current GitHub release artifact and checksum
publish-winget: publish-github
	@windows_sha=$$(awk '/gitflow-$(RELEASE_VERSION)-windows-amd64.zip/ {print $$1}' $(CHECKSUMS_FILE)); \
	[ -n "$$windows_sha" ] || { echo "Missing windows checksum"; exit 1; }; \
	sed -i.bak 's|PackageVersion: .*|PackageVersion: $(RELEASE_VERSION)|' packaging/winget/novaemx.gitflow-helper.yaml; \
	sed -i.bak 's|InstallerUrl: .*|InstallerUrl: https://github.com/$(GITHUB_REPO)/releases/download/$(TAG)/gitflow-$(RELEASE_VERSION)-windows-amd64.zip|' packaging/winget/novaemx.gitflow-helper.yaml; \
	sed -i.bak 's|InstallerSha256: .*|InstallerSha256: '"$$windows_sha"'|' packaging/winget/novaemx.gitflow-helper.yaml; \
	rm -f packaging/winget/novaemx.gitflow-helper.yaml.bak
	@echo "Done. Updated Winget manifest for $(TAG)."

## publish-choco: upload artifacts first, then update Chocolatey metadata to point at the current GitHub release artifact and checksum
publish-choco: publish-github
	@windows_sha=$$(awk '/gitflow-$(RELEASE_VERSION)-windows-amd64.zip/ {print $$1}' $(CHECKSUMS_FILE)); \
	[ -n "$$windows_sha" ] || { echo "Missing windows checksum"; exit 1; }; \
	sed -i.bak 's|<version>.*</version>|<version>$(RELEASE_VERSION)</version>|' packaging/chocolatey/gitflow-helper.nuspec; \
	sed -i.bak "s|\$$version     = '.*'|\$$version     = '$(RELEASE_VERSION)'|" packaging/chocolatey/tools/chocolateyinstall.ps1; \
	sed -i.bak "s|\$$url      = \".*\"|\$$url      = \"https://github.com/$(GITHUB_REPO)/releases/download/$(TAG)/gitflow-$(RELEASE_VERSION)-windows-amd64.zip\"|" packaging/chocolatey/tools/chocolateyinstall.ps1; \
	sed -i.bak "s|\$$checksum = '.*'|\$$checksum = '$$windows_sha'|" packaging/chocolatey/tools/chocolateyinstall.ps1; \
	rm -f packaging/chocolatey/gitflow-helper.nuspec.bak packaging/chocolatey/tools/chocolateyinstall.ps1.bak
	@echo "Done. Updated Chocolatey package metadata for $(TAG)."

## publish-linux: validate linux package artifacts for release channels (.deb/.rpm/.pkg.tar.zst)
publish-linux: publish-github
	@$(MAKE) validate-linux-packages
	@echo "Done. Linux package artifacts (.deb/.rpm/.pkg.tar.zst) are present for $(TAG)."

## publish-all: build locally, upload artifacts to GitHub Releases, and stamp package manifests
publish-all: require-gh publish-homebrew publish-winget publish-choco publish-linux
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
