# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.6.5] - 2026-05-04

### TL;DR
All generated AI instructions are now homologated to use the gitflow SKILL consistently, with standardized command-routing guidance by LLM interaction intent. Existing managed instruction files now self-refresh when homologation sections are missing, even if the version stamp has not changed.

### Changed
- Standardized shared instruction templates (`full` and `compact`) with:
  - `Skill Activation (Homologated)`
  - `LLM Activity Routing` command mapping
- Added missing-section refresh logic for managed compact instruction files so minimal template updates are applied automatically.
- Extended reprovision checks to trigger when homologation sections are missing (not only on version mismatch).
- Updated tracked instruction artifacts to the new homologated format.

### Tests
- Added regression coverage to ensure existing Copilot/AGENTS instruction files receive homologation sections on refresh.
- Updated generation tests to assert homologated sections in Cursor and Copilot outputs.

## [0.6.4] - 2026-05-04

### TL;DR
Cursor rules (`.cursor/rules/gitflow-preflight.mdc` and `semver.mdc`) are now refreshed whenever their on-disk content differs from what the current binary would generate — not just when the stored version is older. This means template fixes, new sections, or any body change trigger a refresh even without a version bump.

### Changed
- Cursor rule generators (`generateCursorRule`, `generateSemverCursorRule`) are now **idempotent**: they skip the write when on-disk content already matches the expected output. No spurious writes on every startup.
- `EnsureRulesForIDE`: Cursor rules are now always passed through the (idempotent) generator instead of being gated on a version stamp comparison.
- `needsReprovisionFromFileVersions`: Cursor and semver rules are now checked via **content equality** (`fileContentDiffers`) so any template or body change triggers reprovision, regardless of whether the version field changed.
- Added `fileContentDiffers(path, expected)` helper to `version_stamp.go`.
- Exposed `cursorRuleContent()` and `semverCursorRuleContent()` so the expected output for the running version is available without side effects.

### Tests
- Added `TestGenerateCursorRule_Idempotent`: second call returns `""` (no write) when content matches.
- Added `TestGenerateCursorRule_RefreshesOnContentChange`: rule is regenerated when body differs even with same version.
- Added `TestGenerateSemverCursorRule_Idempotent`.

## [0.6.3] - 2026-05-04

### TL;DR
Fixes improper version stamp placement in generated Cursor `.mdc` rules and `SKILL.md` files. Version is now injected as a proper YAML field (`gitflow_version: "X.Y.Z"`) inside the frontmatter block instead of corrupting the frontmatter opening delimiter or prepending an HTML comment before `---`. Also removes the redundant `# Gitflow Pre-flight Check` heading from the cursor rule (the template already provides `## Gitflow Pre-flight Check`).

### Fixed
- Cursor rule and semver `.mdc` files: version stamp is now placed as `gitflow_version: "X.Y.Z"` inside the YAML frontmatter block, not as an inline comment on `---` which produced invalid YAML.
- `SKILL.md`: version stamp is now placed as `gitflow_version: "X.Y.Z"` inside the frontmatter block, not as an HTML comment prepended before `---` which broke frontmatter parsing.
- Removed duplicate `# Gitflow Pre-flight Check` heading from cursor preflight rule.
- CRLF line endings in embedded Windows assets are normalized before frontmatter injection.
- Version detection (`fileNeedsVersionRefresh`, `hasCurrentVersionHeader`) now reads `gitflow_version:` from within frontmatter for `.mdc`/SKILL files and the HTML comment first-line for plain markdown files.

## [0.6.2] - 2026-05-04

### TL;DR
This release hardens AI rule provisioning by stamping generated rules/skills/agents with the running gitflow version on the first line, and auto-refreshing outdated files when gitflow starts. Logging now creates timestamped files under `.gitflow` (for example `log-20260504-153210.txt`) while preserving capture-start timestamps in file content. Homebrew formula sync is aligned to `gitflow.rb` in both tracked packaging and tap sync paths.

### Added
- Added first-line version stamp to generated AI integration artifacts (rules/skills/agents) so generated file provenance can be compared against the running gitflow binary version.
- Added startup-time stale-file detection that reprovisions IDE artifacts when their stamped version is missing or older than the running app version.

### Changed
- Updated generated debug log filename pattern from fixed `log.txt` to timestamped `log-YYYYMMDD-HHMMSS.txt` in `.gitflow`.
- Updated Homebrew publish path to use `gitflow.rb` in both local tracked packaging and `../homebrew-tap/Formula` sync.
- Updated `--log`/`--debug` flag help text to reflect timestamped log filename behavior.

### Tests
- Added coverage for version stamp parsing, header refresh, and stale-header reprovision behavior in IDE onboarding/generation.
- Updated existing IDE generation tests to assert first-line version headers in generated files.

## [0.6.1] - 2026-05-04

### TL;DR
Bugfix: `--log` flag was writing output to stderr (polluting the TUI) instead of a file. Log output now routes exclusively to `.gitflow/log.txt` with a timestamped capture-start header. The `setup` command now routes directly through `EnsureRulesWithAIConsent`. Homebrew formula renamed to `gitflow-helper.rb`.

### Fixed
- Fixed `--log` flag: log output now writes to `<project>/.gitflow/log.txt` only, never to stderr. Stderr output is reserved for `--debug` level only.
- Added timestamped capture-start header to the log file on every new logging session.
- Removed `EnsureRulesForSetup` indirection; `gitflow setup` now calls `EnsureRulesWithAIConsent` directly.

### Changed
- Homebrew tracked formula renamed from `gitflow.rb` to `gitflow-helper.rb` in Makefile and README.

### Tests
- Added `TestLogf_LogOnly_WritesFileNotStderr` — regression guard ensuring log-only mode produces zero stderr output and writes to `.gitflow/log.txt`.

## [0.6.0] - 2026-05-01

### TL;DR
Critical bugfix: running `gitflow` in a fresh empty directory now correctly initializes git and creates all files in the user's directory. Previously, when installed via Homebrew, the tool would mistake `/opt/homebrew` (Homebrew's own git repo) as the project root, leaving the user's directory completely empty. Arch Linux / CachyOS repo support is also added with a full pacman database generator, PKGBUILD template, and `.conf` snippet.

### Fixed
- Fixed `FindProjectRoot()` to only consider the current working directory ancestry when looking for a `.git` directory. The binary's own install location (e.g. `/opt/homebrew/bin/gitflow`) was previously used as a fallback candidate, causing Homebrew-installed binaries to detect the Homebrew git repository as the project root for any fresh empty directory. All git operations (init, VERSION, IDE rules, workspace commit) now run in the user's actual directory.

### Added
- Added Arch Linux / CachyOS custom pacman repository support:
  - `packaging/linux/arch/PKGBUILD` — AUR-compatible build file, auto-updated by the release script with current version and SHA256 checksums.
  - `packaging/linux/arch/gitflow-helper-arch.conf` — pacman repo config snippet for `/etc/pacman.conf`.
  - `scripts/generate-linux-repo-metadata.sh` gains `--arch-pkgbuild`, `--arch-conf-file`, and `--arch-root` flags to generate a minimal pacman `.db.tar.gz` without requiring `repo-add` (pure bash, cross-platform).
  - `make publish-linux` now generates Arch pacman database files for `x86_64` and `aarch64` alongside existing Debian/Rocky artifacts.

### Tests
- Added `TestFindProjectRootFrom_ReturnsGitRoot` — unit test for CWD ancestry walk-up.
- Added `TestFindProjectRootFrom_NoGitReturnsEmpty` — unit test for empty dir fallback.
- Added `TestFindProjectRoot_NeverFallsBackToExePath` — regression guard for the Homebrew bug.
- Added `TestInitGitFlow_FreshDirectory` — integration test verifying VERSION is created in the user's actual directory after a full fresh-dir init.

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