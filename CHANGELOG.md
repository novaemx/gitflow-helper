# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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