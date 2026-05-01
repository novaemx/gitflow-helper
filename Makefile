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
HOMEBREW_TAP_FORMULA ?= ../homebrew-tap/Formula/gitflow.rb
HOMEBREW_TAP_GITHUB_TOKEN ?=
export HOMEBREW_TAP_GITHUB_TOKEN
WINDOWS_ARCHIVE := $(DIST)/$(BINARY)-$(VERSION)-windows-amd64.zip
LINUX_ARCHIVE   := $(DIST)/$(BINARY)-$(VERSION)-linux-amd64.tar.gz
LINUX_ARM64_ARCHIVE := $(DIST)/$(BINARY)-$(VERSION)-linux-aarch64.tar.gz
DARWIN_ARCHIVE  := $(DIST)/$(BINARY)-$(VERSION)-darwin-universal.tar.gz
CHECKSUMS_FILE  := $(DIST)/checksums.txt
COVER_DIR := test
LINUX_REPO_DIST_DIR := $(DIST)/linux-repo
LINUX_REPO_ASSET_FILES := $(LINUX_REPO_DIST_DIR)/apt/Packages $(LINUX_REPO_DIST_DIR)/apt/Packages.gz $(LINUX_REPO_DIST_DIR)/apt/Release $(LINUX_REPO_DIST_DIR)/apt/gitflow-helper.sources $(LINUX_REPO_DIST_DIR)/yum/gitflow-helper-rocky.repo $(LINUX_REPO_DIST_DIR)/arch/x86_64/gitflow-helper.db $(LINUX_REPO_DIST_DIR)/arch/aarch64/gitflow-helper.db $(LINUX_REPO_DIST_DIR)/arch/x86_64/gitflow-helper.db.tar.gz $(LINUX_REPO_DIST_DIR)/arch/aarch64/gitflow-helper.db.tar.gz

.PHONY: build build-all universal clean test vet lint release install uninstall
.PHONY: release-local release-local-github
.PHONY: package-homebrew package-winget package-all
.PHONY: publish-github publish-homebrew publish-winget publish-linux publish-all
.PHONY: push-winget
.PHONY: upload-release-assets cleanup-release-assets validate-release-assets validate-linux-packages
.PHONY: generate-linux-release-assets generate-linux-repo-metadata
.PHONY: require-gh validate-publish-context

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

## release: run goreleaser (requires goreleaser installed)
release:
	goreleaser release --clean

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

## release-local: build release artifacts locally only (no GitHub Actions)
release-local: $(CHECKSUMS_FILE)

require-gh:
	@command -v gh >/dev/null 2>&1 || { echo "GitHub CLI (gh) is required"; exit 1; }

## validate-publish-context: enforce gitflow release prerequisites before publish
## Best practice: publish only from a finished release/hotfix tag that exists on origin and points into origin/main.
validate-publish-context:
	@test -n "$(TAG)" || (echo "TAG is required" && exit 1)
	@tag_commit=$$(git rev-parse --verify --quiet "refs/tags/$(TAG)^{commit}"); \
	if [ -z "$$tag_commit" ]; then \
		echo "Publish blocked: local tag $(TAG) does not exist."; \
		echo "Hint: finish release/hotfix first (gitflow finish), then push the tag."; \
		exit 1; \
	fi
	@remote_tag=$$(git ls-remote --tags origin "refs/tags/$(TAG)" "refs/tags/$(TAG)^{}" 2>/dev/null); \
	if [ -z "$$remote_tag" ]; then \
		echo "→ Remote tag $(TAG) not found on origin; pushing it automatically..."; \
		if ! git push origin "$(TAG)" >/dev/null; then \
			echo "Publish blocked: failed to push tag $(TAG) to origin."; \
			echo "Hint: verify permissions/auth and run: git push origin $(TAG)"; \
			exit 1; \
		fi; \
		remote_tag=$$(git ls-remote --tags origin "refs/tags/$(TAG)" "refs/tags/$(TAG)^{}" 2>/dev/null); \
		if [ -z "$$remote_tag" ]; then \
			echo "Publish blocked: tag $(TAG) is still missing on origin after push attempt."; \
			exit 1; \
		fi; \
	fi
	@branch=$$(git branch --show-current 2>/dev/null || true); \
	if [ -n "$$branch" ] && ! echo "$$branch" | grep -Eq '^(release|hotfix)/|^main$$'; then \
		echo "→ Publish note: running from '$$branch'; validation will use tag ancestry on origin/main."; \
	fi
	@git fetch origin main --quiet >/dev/null 2>&1 || true
	@if ! git branch -r --contains "$$(git rev-parse --verify "refs/tags/$(TAG)^{commit}")" | grep -q 'origin/main'; then \
		echo "Publish blocked: $(TAG) is not reachable from origin/main."; \
		echo "Hint: publish only after finishing release/hotfix into main and pushing main + tag."; \
		exit 1; \
	fi

validate-release-assets:
	@test -f "$(CHECKSUMS_FILE)" || (echo "Missing $(CHECKSUMS_FILE). Run make $(CHECKSUMS_FILE) first." && exit 1)
	@for artifact in \
		gitflow-$(RELEASE_VERSION)-darwin-universal.tar.gz \
		gitflow-$(RELEASE_VERSION)-windows-amd64.zip \
		gitflow-$(RELEASE_VERSION)-linux-amd64.tar.gz \
		gitflow-$(RELEASE_VERSION)-linux-aarch64.tar.gz; do \
		awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q "^$$artifact$$" || { echo "Missing checksum entry for $$artifact"; exit 1; }; \
		file="$(DIST)/$$artifact"; \
		[ -f "$$file" ] || { echo "Missing artifact file $$file"; exit 1; }; \
		expected_sha=$$(awk -v f="$$artifact" '$$2 == f { print $$1 }' "$(CHECKSUMS_FILE)"); \
		[ -n "$$expected_sha" ] || { echo "Missing expected sha for $$artifact"; exit 1; }; \
		if command -v sha256sum >/dev/null 2>&1; then \
			actual_sha=$$(sha256sum "$$file" | awk '{print $$1}'); \
		elif command -v shasum >/dev/null 2>&1; then \
			actual_sha=$$(shasum -a 256 "$$file" | awk '{print $$1}'); \
		else \
			echo "No SHA256 tool found (sha256sum or shasum required)"; \
			exit 1; \
		fi; \
		if [ "$$actual_sha" != "$$expected_sha" ]; then \
			echo "Checksum mismatch for $$artifact"; \
			echo "  expected: $$expected_sha"; \
			echo "  actual:   $$actual_sha"; \
			exit 1; \
		fi; \
	done

validate-linux-packages:
	@test -f "$(CHECKSUMS_FILE)" || (echo "Missing $(CHECKSUMS_FILE). Run make $(CHECKSUMS_FILE) first." && exit 1)
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '_linux_amd64\\.deb$$' || { echo "Missing amd64 .deb package in checksums"; exit 1; }
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '_linux_arm64\\.deb$$' || { echo "Missing arm64 .deb package in checksums"; exit 1; }
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '_linux_amd64\\.rpm$$' || { echo "Missing amd64 .rpm package in checksums"; exit 1; }
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '_linux_arm64\\.rpm$$' || { echo "Missing arm64 .rpm package in checksums"; exit 1; }
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '_linux_amd64\\.pkg\\.tar\\.zst$$' || { echo "Missing amd64 Arch package (.pkg.tar.zst) in checksums"; exit 1; }
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | grep -q '_linux_arm64\\.pkg\\.tar\\.zst$$' || { echo "Missing arm64 Arch package (.pkg.tar.zst) in checksums"; exit 1; }

generate-linux-release-assets:
	@test -f "$(CHECKSUMS_FILE)" || (echo "Missing $(CHECKSUMS_FILE). Run make $(CHECKSUMS_FILE) first." && exit 1)
	@./scripts/generate-linux-repo-metadata.sh \
		--version "$(RELEASE_VERSION)" \
		--repo "$(GITHUB_REPO)" \
		--dist "$(DIST)" \
		--apt-assets-dir "$(LINUX_REPO_DIST_DIR)/apt" \
		--yum-repo-file "$(LINUX_REPO_DIST_DIR)/yum/gitflow-helper-rocky.repo" \
		--arch-root "$(LINUX_REPO_DIST_DIR)/arch"

generate-linux-repo-metadata:
	@test -f "$(CHECKSUMS_FILE)" || (echo "Missing $(CHECKSUMS_FILE). Run make $(CHECKSUMS_FILE) first." && exit 1)
	@./scripts/generate-linux-repo-metadata.sh \
		--version "$(RELEASE_VERSION)" \
		--repo "$(GITHUB_REPO)" \
		--dist "$(DIST)" \
		--apt-source-file "packaging/linux/apt/gitflow-helper.sources" \
		--yum-repo-file "packaging/linux/yum/gitflow-helper-rocky.repo" \
		--yum-root "packaging/linux/yum/rocky/9" \
		--arch-pkgbuild "packaging/linux/arch/PKGBUILD" \
		--arch-conf-file "packaging/linux/arch/gitflow-helper-arch.conf" \
		--arch-root "$(LINUX_REPO_DIST_DIR)/arch"

cleanup-release-assets:
	@test -n "$(RELEASE_TAG)" || (echo "RELEASE_TAG is required" && exit 1)
	@test -f "$(CHECKSUMS_FILE)" || (echo "Missing $(CHECKSUMS_FILE). Run make $(CHECKSUMS_FILE) first." && exit 1)
	@echo "→ Removing previously published assets from $(RELEASE_TAG) (if present)..."
	@{ awk '{print $$2}' "$(CHECKSUMS_FILE)"; echo "checksums.txt"; printf '%s\n' $(notdir $(LINUX_REPO_ASSET_FILES)); } | while read -r asset; do \
		[ -n "$$asset" ] || continue; \
		gh release delete-asset "$(RELEASE_TAG)" "$$asset" --repo "$(GITHUB_REPO)" --yes >/dev/null 2>&1 || true; \
	done

upload-release-assets:
	@test -n "$(RELEASE_TAG)" || (echo "RELEASE_TAG is required" && exit 1)
	@test -f "$(CHECKSUMS_FILE)" || (echo "Missing $(CHECKSUMS_FILE). Run make $(CHECKSUMS_FILE) first." && exit 1)
	@$(MAKE) validate-release-assets RELEASE_VERSION="$(RELEASE_VERSION)"
	@$(MAKE) generate-linux-release-assets RELEASE_VERSION="$(RELEASE_VERSION)"
	@echo "→ Uploading release assets from $(CHECKSUMS_FILE) to $(RELEASE_TAG)..."
	@awk '{print $$2}' "$(CHECKSUMS_FILE)" | while read -r asset; do \
		[ -n "$$asset" ] || continue; \
		file="$(DIST)/$$asset"; \
		[ -f "$$file" ] || { echo "Missing artifact: $$file"; exit 1; }; \
		upload_ok=0; \
		for attempt in 1 2 3; do \
			if GH_PAGER=cat gh release upload "$(RELEASE_TAG)" "$$file" --repo "$(GITHUB_REPO)" --clobber; then \
				upload_ok=1; \
				break; \
			fi; \
			echo "  upload failed ($$attempt/3): $$asset"; \
			sleep 1; \
		done; \
		[ $$upload_ok -eq 1 ] || { echo "Failed to upload $$asset after 3 attempts"; exit 1; }; \
	done
	@checksums_ok=0; \
	for attempt in 1 2 3; do \
		if GH_PAGER=cat gh release upload "$(RELEASE_TAG)" "$(CHECKSUMS_FILE)" --repo "$(GITHUB_REPO)" --clobber; then \
			checksums_ok=1; \
			break; \
		fi; \
		echo "  upload failed ($$attempt/3): $(CHECKSUMS_FILE)"; \
		sleep 1; \
	done; \
	[ $$checksums_ok -eq 1 ] || { echo "Failed to upload $(CHECKSUMS_FILE) after 3 attempts"; exit 1; }
	@for file in $(LINUX_REPO_ASSET_FILES); do \
		asset=$$(basename "$$file"); \
		[ -f "$$file" ] || { echo "Missing Linux repo asset: $$file"; exit 1; }; \
		asset_ok=0; \
		for attempt in 1 2 3; do \
			if GH_PAGER=cat gh release upload "$(RELEASE_TAG)" "$$file" --repo "$(GITHUB_REPO)" --clobber; then \
				asset_ok=1; \
				break; \
			fi; \
			echo "  upload failed ($$attempt/3): $$asset"; \
			sleep 1; \
		done; \
		[ $$asset_ok -eq 1 ] || { echo "Failed to upload $$asset after 3 attempts"; exit 1; }; \
	done
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
	@$(MAKE) validate-publish-context
	@$(MAKE) -B $(CHECKSUMS_FILE)
	@$(MAKE) validate-release-assets RELEASE_VERSION="$(RELEASE_VERSION)"
	@echo "→ Ensuring GitHub release $(TAG) exists with changelog executive summary..."
	@set -e; \
	notes_file=$$(mktemp 2>/dev/null || mktemp -t gitflow-release-notes.XXXXXX); \
	: >| "$$notes_file"; \
	awk -v version="$(RELEASE_VERSION)" ' \
		BEGIN { section_header = "## [" version "] - " } \
		index($$0, section_header) == 1 { in_version=1; next } \
		in_version && index($$0, "## [") == 1 { in_version=0 } \
		in_version && /^### TL;DR/ { in_tldr=1; next } \
		in_tldr && /^### / { in_tldr=0 } \
		in_tldr { print } \
	' CHANGELOG.md >| "$$notes_file"; \
	if [ ! -s "$$notes_file" ]; then \
		echo "→ Missing TL;DR in CHANGELOG.md for $(RELEASE_VERSION); generating notes from commits between release versions..."; \
		current_ref="$(TAG)"; \
		if ! git rev-parse -q --verify "$$current_ref^{commit}" >/dev/null 2>&1; then \
			current_ref=$$(git rev-parse HEAD); \
		fi; \
		prev_tag=$$(git tag --sort=-version:refname | awk -v cur="$(TAG)" '$$0 != cur { print; exit }'); \
		if [ -n "$$prev_tag" ]; then \
			git log --no-merges --pretty='- %s' "$$prev_tag..$$current_ref" >| "$$notes_file"; \
			echo >> "$$notes_file"; \
			echo "Range: $$prev_tag..$$current_ref" >> "$$notes_file"; \
		else \
			git log --no-merges --pretty='- %s' "$$current_ref" >| "$$notes_file"; \
			echo >> "$$notes_file"; \
			echo "Range: initial..$$current_ref" >> "$$notes_file"; \
		fi; \
		if [ ! -s "$$notes_file" ]; then \
			echo "Release $(TAG)" >| "$$notes_file"; \
			echo >> "$$notes_file"; \
			echo "No commit descriptions were found for the selected range." >> "$$notes_file"; \
		fi; \
	fi; \
	if ! gh release view "$(TAG)" --repo "$(GITHUB_REPO)" >/dev/null 2>&1; then \
		target_commit=$$(git rev-parse --verify "refs/tags/$(TAG)^{commit}"); \
		gh release create "$(TAG)" --repo "$(GITHUB_REPO)" --target "$$target_commit" --title "$(TAG)" --notes-file "$$notes_file"; \
	else \
		gh release edit "$(TAG)" --repo "$(GITHUB_REPO)" --title "$(TAG)" --notes-file "$$notes_file"; \
	fi; \
	rm -f "$$notes_file"
	@$(MAKE) cleanup-release-assets RELEASE_TAG="$(TAG)"
	@$(MAKE) upload-release-assets RELEASE_TAG="$(TAG)" RELEASE_VERSION="$(RELEASE_VERSION)"
	@echo "Done. GitHub release $(TAG) now hosts locally-built artifacts."

## publish-homebrew: upload artifacts first, then update local Homebrew formula to point at the current GitHub release
publish-homebrew: publish-github
	@darwin_sha=$$(awk '/gitflow-$(RELEASE_VERSION)-darwin-universal.tar.gz/ {print $$1}' $(CHECKSUMS_FILE)); \
	[ -n "$$darwin_sha" ] || { echo "Missing darwin checksum"; exit 1; }; \
	branch=$$(git branch --show-current 2>/dev/null || true); \
	update_tracked=1; \
	if [ -n "$$branch" ] && ! echo "$$branch" | grep -Eq '^(release|hotfix)/'; then \
		update_tracked=0; \
		echo "→ Skipping tracked Homebrew manifest update on '$$branch' (protected release metadata)."; \
		echo "  Tracked metadata is only updated from release/hotfix branches."; \
		echo "  Continuing with tap formula sync only."; \
	fi; \
	if [ $$update_tracked -eq 1 ]; then \
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
		' packaging/homebrew/gitflow.rb > packaging/homebrew/gitflow.rb.tmp; \
		mv packaging/homebrew/gitflow.rb.tmp packaging/homebrew/gitflow.rb; \
	fi; \
	[ -f "$(HOMEBREW_TAP_FORMULA)" ] || { echo "Missing Homebrew tap formula at $(HOMEBREW_TAP_FORMULA)"; exit 1; }; \
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
	' "$(HOMEBREW_TAP_FORMULA)" > "$(HOMEBREW_TAP_FORMULA).tmp"; \
	mv "$(HOMEBREW_TAP_FORMULA).tmp" "$(HOMEBREW_TAP_FORMULA)"; \
	echo "Done. Updated Homebrew formula targets for $(TAG):"; \
	if [ $$update_tracked -eq 1 ]; then echo "  - packaging/homebrew/gitflow.rb"; fi; \
	echo "  - $(HOMEBREW_TAP_FORMULA)"

## publish-winget: upload artifacts first, then update local Winget manifests to point at the current GitHub release artifact and checksum
publish-winget: publish-github
	@branch=$$(git branch --show-current 2>/dev/null || true); \
	if [ -n "$$branch" ] && ! echo "$$branch" | grep -Eq '^(release|hotfix)/'; then \
		echo "→ Skipping local Winget manifest updates on '$$branch' (protected release metadata)."; \
		echo "  Run this target from release/hotfix if you want tracked packaging files updated."; \
		exit 0; \
	fi; \
	windows_sha=$$(awk '/gitflow-$(RELEASE_VERSION)-windows-amd64.zip/ {print $$1}' $(CHECKSUMS_FILE)); \
	[ -n "$$windows_sha" ] || { echo "Missing windows checksum"; exit 1; }; \
	sed -i.bak 's|PackageVersion: .*|PackageVersion: $(RELEASE_VERSION)|' packaging/winget/novaemx.gitflow-helper.yaml; \
	sed -i.bak 's|PackageVersion: .*|PackageVersion: $(RELEASE_VERSION)|' packaging/winget/novaemx.gitflow-helper.installer.yaml; \
	sed -i.bak 's|InstallerUrl: .*|InstallerUrl: https://github.com/$(GITHUB_REPO)/releases/download/$(TAG)/gitflow-$(RELEASE_VERSION)-windows-amd64.zip|' packaging/winget/novaemx.gitflow-helper.installer.yaml; \
	sed -i.bak 's|InstallerSha256: .*|InstallerSha256: '"$$windows_sha"'|' packaging/winget/novaemx.gitflow-helper.installer.yaml; \
	sed -i.bak 's|PackageVersion: .*|PackageVersion: $(RELEASE_VERSION)|' packaging/winget/novaemx.gitflow-helper.version.yaml; \
	rm -f packaging/winget/novaemx.gitflow-helper.yaml.bak packaging/winget/novaemx.gitflow-helper.installer.yaml.bak packaging/winget/novaemx.gitflow-helper.version.yaml.bak; \
	echo "Done. Updated Winget manifests for $(TAG)."

## push-winget: submit the current version to the winget community repository via wingetcreate
push-winget: publish-winget
	wingetcreate update \
		--version $(RELEASE_VERSION) \
		--urls https://github.com/$(GITHUB_REPO)/releases/download/$(TAG)/gitflow-$(RELEASE_VERSION)-windows-amd64.zip \
		--submit \
		NovaeMX.gitflow-helper
	@echo "Winget submission done for $(TAG)."

## publish-linux: validate linux package artifacts for release channels (.deb/.rpm/.pkg.tar.zst) on amd64 + arm64, including Arch/CachyOS
publish-linux: publish-github
	@branch=$$(git branch --show-current 2>/dev/null || true); \
	if [ -n "$$branch" ] && ! echo "$$branch" | grep -Eq '^(release|hotfix)/'; then \
		echo "→ Skipping local Linux repository metadata updates on '$$branch' (protected release metadata)."; \
		echo "  Run this target from release/hotfix if you want tracked packaging files updated."; \
		exit 0; \
	fi; \
	$(MAKE) validate-linux-packages; \
	$(MAKE) generate-linux-repo-metadata RELEASE_VERSION="$(RELEASE_VERSION)"; \
	echo "Done. Linux amd64+arm64 package artifacts and Debian/Rocky/Arch repo metadata are ready for $(TAG)."

## publish-all: build locally, upload artifacts to GitHub Releases, and stamp package manifests
publish-all: require-gh publish-homebrew publish-winget publish-linux
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

## package-homebrew: build snapshot and prepare Homebrew formula artifacts via goreleaser
package-homebrew:
	@echo "→ Building Homebrew snapshot..."
	goreleaser release --snapshot --clean
	@echo "Homebrew snapshot generated in dist/."
	@echo "Test locally: brew install --formula ./packaging/homebrew/gitflow.rb"

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
