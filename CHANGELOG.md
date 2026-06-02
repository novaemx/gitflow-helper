# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.7.0] - 2026-06-02

### TL;DR
Release pipeline is now CI-only and produces binaries tuned for maximum per-platform performance. The build matrix is reduced to `linux/amd64`, `linux/arm64`, `windows/amd64`, and `darwin/arm64`; macOS Intel is discontinued. Linux and Windows x86_64 binaries target the `GOAMD64=v3` micro-architecture (AVX2 + FMA, Haswell 2013+). Local `make publish-*` targets that pushed binaries via the `gh` CLI have been removed — the GitHub Actions release workflow is the sole publisher.

### Changed
- Cut the release matrix to 4 platforms: `linux/amd64`, `linux/arm64`, `windows/amd64`, `darwin/arm64`. Removed `darwin/amd64` (Intel Mac discontinued) and the derived `darwin-universal` archive.
- Updated the Homebrew cask to download `gitflow-<ver>-darwin-arm64.tar.gz` instead of the old `darwin-universal.tar.gz`. The CI `formula` job now stamps the darwin-arm64 SHA into `packaging/homebrew/gitflow.rb` and `novaemx/homebrew-tap`.
- Added per-platform max-performance build flags to `.goreleaser.yml`: `-trimpath`, `-buildid=none`, `-ldflags="-s -w"`, and `-pgo=auto` on every build. `-pgo=auto` is a no-op until a `default.pgo` profile is committed; once present it activates PGO with zero further config changes (typical 5–14% speedup on hot paths).
- `linux/amd64` and `windows/amd64` now build with `GOAMD64=v3`, enabling AVX2 + FMA + BMI2 instructions. **Tradeoff:** these binaries require a Haswell (2013) or newer CPU. Pre-Haswell hosts must rebuild from source with `GOAMD64=v2` (or unset the env var). The CLI is a developer tool — modern hardware is the assumed baseline.
- `darwin/arm64` keeps the standard Go 1.21+ Apple Silicon codegen (LSE atomics, NEON FP). There is no `GOARM64` micro-arch level; PGO is the main remaining lever.

### Removed
- Removed local `gh release upload` paths from the Makefile: `require-gh`, `validate-publish-context`, `validate-release-assets`, `validate-linux-packages`, `generate-linux-release-assets`, `generate-linux-repo-metadata`, `cleanup-release-assets`, `upload-release-assets`, `release-local-github`, `publish-github`, `publish-homebrew`, `publish-winget`, `push-winget`, `publish-linux`, `publish-all`, and the `release` (goreleaser-publish) target. The remaining `make release-local` and `make release-snapshot` only build artifacts into `dist/`; they never touch the network.
- Removed the now-unused `HOMEBREW_TAP_FORMULA`, `HOMEBREW_TAP_GITHUB_TOKEN`, `LINUX_REPO_DIST_DIR`, and `LINUX_REPO_ASSET_FILES` Makefile variables.
- Removed the `publish-all` aggregator target. The CI workflow (`.github/workflows/release.yml`) is now the only path that creates the GitHub Release and uploads binaries.

### Compatibility
- `gitflow-0.x.y-darwin-universal.tar.gz` is no longer published. Anyone relying on the universal archive should switch to `gitflow-0.x.y-darwin-arm64.tar.gz` (Apple Silicon).
- `gitflow-0.x.y-windows-amd64.zip`, `gitflow-0.x.y-linux-amd64.tar.gz`, and `gitflow-0.x.y-linux-aarch64.tar.gz` URLs are unchanged.
- Linux/Windows x86_64 binaries now require a Haswell (2013)+ CPU. See Changed section above.

## [0.6.7] - 2026-06-01

### TL;DR
This release turns `gitflow setup` into a complete, agent-friendly installation command and moves binary compilation from local-only builds into a deterministic GitHub Actions pipeline. Four new flags (`--yes`, `--force`, `--check`, `--uninstall`) round out the setup UX for non-interactive agents, and the new CI pipeline builds all 5 OS/arch targets plus nfpms on every `v*` tag push with a per-package coverage gate.

### Added
- Added `gitflow setup --yes` (`-y`) to auto-accept the AI integration consent dialog for non-interactive agents and CI runs.
- Added `gitflow setup --force` (`-f`) to re-write rule files and skills even when on-disk content already matches the expected byte stream.
- Added `gitflow setup --check` dry-run mode that reports what would be created or updated without writing to disk.
- Added `gitflow setup --uninstall` to remove all installed artifacts and clear the consent entry from `.gitflow/config.json`.
- Added `RemoveRulesForIDE` (companion-aware inverse of `EnsureRulesForIDE`) in `internal/ide/detect.go` for the uninstall path.
- Added `RemoveAIIntegrationChoice` in `internal/config` to clear the consent entry during uninstall.
- Added `SetForceRuleWrite` in `internal/ide/detect.go` to scope the `--force` flag to a single setup invocation.
- Added GitHub Actions release pipeline at `.github/workflows/release.yml` that builds all 5 OS/arch targets + `darwin-universal` + nfpms via GoReleaser on `v*` tag push, smoke-tests the linux/amd64 binary, creates the GitHub Release, and auto-bumps `packaging/homebrew/gitflow.rb` and the `novaemx/homebrew-tap` formula.
- Added per-package coverage gate (currently `>=60%`, target `80%`) to the release workflow, with per-package `::error::` annotations on regression.
- Added auto-install of Claude Code companion artifacts (`CLAUDE.md`, `.claude/mcp.json`) when Cursor / VS Code / Copilot is the primary IDE and Claude Code is detected in the same environment.

### Changed
- Rewrote the README "Local-Only Release Policy" section as a "Release Pipeline" section documenting the new CI flow and keeping the local `make publish-*` targets as documented fallback.
- Replaced the workflow's total-coverage gate with a per-package gate, emitting a tabular coverage report and `::error::` lines per failing package.
- Updated the README IDE matrix to document the new Cursor + Claude Code companion install behavior.
- Extended the `gitflow setup` command's `--help` text to document the four new flags.

### Fixed
- Renamed the Homebrew formula from `packaging/homebrew/gitflow-helper.rb` to `packaging/homebrew/gitflow.rb` to match the binary name and Homebrew naming convention.

## [0.6.6] - 2026-05-07

### TL;DR
This release adds a first-class branch topology diagram experience: a new `gitflow diagram` command and a full-screen horizontal timeline view directly inside the TUI, including vertical and horizontal scrolling for large repositories.

### Added
- Added new CLI command `gitflow diagram` to generate Mermaid topology output from live gitflow state.
- Added comprehensive tests for diagram command output and timeline rendering behavior.

### Changed
- Integrated a dedicated full-screen diagram mode into the TUI action flow.
- Updated TUI diagram rendering to a horizontal timeline layout with branch lanes and current-branch highlights.
- Added horizontal timeline navigation controls in TUI (`h/l` and arrow keys) plus updated status hints.
- Updated command references in README and embedded IDE templates to include diagram functionality.

### Fixed
- Restored project-root discovery helper compatibility in config tests and aligned root resolution behavior with CWD-first expectations.
- Restored support for legacy root-shaped AI integration config payloads.

## [0.5.48] - 2026-04-29

### TL;DR
This release adds Linux repository onboarding for native package installation. Debian and Ubuntu can now use a ready-to-install `.sources` file backed by GitHub Release `.deb` assets, and Rocky Linux now has a dedicated YUM/DNF `.repo` plus tracked `repodata` for `x86_64` and `aarch64`.

### Added
- Added Linux repository metadata generator script to build APT and Rocky YUM repository files from release artifacts.
- Added tracked Debian/Ubuntu source definition at `packaging/linux/apt/gitflow-helper.sources`.
- Added tracked Rocky Linux repository definition at `packaging/linux/yum/gitflow-helper-rocky.repo`.
- Added tracked Rocky Linux `repodata` files for `x86_64` and `aarch64` under `packaging/linux/yum/rocky/9/`.

### Changed
- Extended the Makefile publish pipeline to generate Linux repository assets (`Packages`, `Packages.gz`, `Release`, `.sources`, `.repo`) as part of release uploads.
- Updated README installation guidance with Debian/Ubuntu and Rocky Linux repository setup and architecture-specific commands.

## [0.5.47] - 2026-04-29

### TL;DR
This release hardens the publish pipeline so releases can be rebuilt and republished more reliably. Homebrew token handling is now safe when optional credentials are absent, and GitHub asset uploads automatically retry transient failures instead of aborting the whole publish flow.

### Changed
- Defaulted and exported `HOMEBREW_TAP_GITHUB_TOKEN` in the Makefile so publish jobs always evaluate with a predictable environment.
- Switched Homebrew cask token and upload gating templates to safe `index .Env ...` lookups in `.goreleaser.yml`.

### Fixed
- Added retry logic for release asset uploads so transient `gh release upload` failures no longer break the full publish step.
- Improved upload diagnostics with per-asset retry messages and explicit failure reporting after the final attempt.

## [0.5.41] - 2026-04-22

### TL;DR
Implemented changelog TL;DR guardrail for release/hotfix branches to auto-populate missing changelog templates, ensuring publish-github never fails on missing summaries. Enhanced Makefile with clean-tree validation and ephemeral build clones for untagged release versions.

### Added
- Added `ensureChangelogTLDR()` guardrail function that auto-creates CHANGELOG.md sections with TL;DR templates during `gitflow start release/hotfix`.
- Implemented automatic CHANGELOG.md section header creation (with today's date) for new releases.

### Changed
- Enhanced release branch startup to guarantee CHANGELOG.md has proper structure before publish phase.
- Modified Makefile publish workflow to validate and ensure changelog consistency.

### Fixed
- Resolved "Missing TL;DR in CHANGELOG.md" errors during release publishing.
- Guardrail prevents incomplete changelog entries from blocking CI/CD pipelines.

## [0.5.40] - 2026-04-19

### TL;DR
Migrated GoReleaser config to remove deprecated keys and updated packaging targets and Makefile messaging; validated with `goreleaser check` and a snapshot build.

### Added
- Added `homebrew_casks` and explicit `ids` for affected archives in `.goreleaser.yml`.

### Changed
- Replaced deprecated `archives.*.builds` → `ids` and `archives.*.format` → `formats`.
- Replaced `nfpms.builds` → `nfpms.ids`.
- Replaced legacy `brews` → `homebrew_casks` and updated Makefile messaging for Homebrew casks.
- Validated configuration with `goreleaser check` and ran a `--snapshot` build locally.

### Fixed
- Removed GoReleaser deprecation warnings.
- Corrected Makefile Homebrew test message to reference casks.


## [0.5.39] - 2026-04-19

### TL;DR
This release expands Linux distribution support by adding native `aarch64` artifacts and package outputs, and strengthens release validation to ensure all required Linux artifacts are present before publishing.

### Added
- Added Linux `arm64` build target in GoReleaser and published `linux-aarch64` tarball output.
- Added Linux `arm64` native packages for `.deb`, `.rpm`, and Arch (`.pkg.tar.zst`) channels.

### Changed
- Updated release validation rules in `Makefile` to require both Linux tarballs (`amd64` and `aarch64`).
- Updated Linux package validation checks to enforce both `amd64` and `arm64` package artifacts across `.deb`, `.rpm`, and Arch formats.
- Refined publish messaging to explicitly report Linux dual-architecture package validation.

### Fixed
- Reduced risk of partial Linux release payloads by failing publish validation when any required architecture-specific package is missing.