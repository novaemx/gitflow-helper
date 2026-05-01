# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.5.52] - 2026-04-30

### TL;DR
This release makes `gitflow finish` safer for agent-driven workflows by adding an explicit test-gated finish path, fixes Cursor setup so AI integration consent is asked and version-stamped during explicit setup, and aligns the tracked Homebrew formula with the confirmed `gitflow.rb` tap format.

### Added
- Added a `gitflow finish --run-tests` flow that auto-detects the project test command, runs the suite, and only finishes feature, bugfix, and hotfix branches when tests pass.
- Added regression coverage for explicit setup prompting, version stamping, and legacy AI integration config loading.

### Changed
- Updated embedded gitflow skill and instruction assets to direct feature, bugfix, and hotfix finishes through the new test-gated finish flow.
- Updated Homebrew packaging references to use the tracked `packaging/homebrew/gitflow.rb` formula path and the confirmed tap formula name.

### Fixed
- Fixed `gitflow setup` so explicit setup in Cursor now goes through the AI consent flow instead of bypassing onboarding.
- Fixed AI integration choice loading to recognize legacy consent data stored directly in `.gitflow/config.json`.

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