# Release 0.5.23

**Date:** 2026-04-15

## What's New

- Merge feature 'audit-deep-dives' into develop
- Enhance git command execution and testing capabilities - Introduced GitClient interface and LocalGitClient implementation for better abstraction. - Added auto-initialization of git repository in root command. - Implemented coverage reporting in Makefile and updated README. - Added tests for git command execution and version handling. - Updated .gitignore to exclude coverage files.
- Introduce SplitCommand wrapper and refactor command execution
- Merge feature 'gitflow-cd-smart-finish-improvements' into develop
- Implement fast-release command and enhance finish command with rebase and squash options
- Merge feature 'tui-selector-activity-toggle' into develop
- Implement integration mode toggle and enhance TUI with mode display
- Enhance TUI with activity panel toggle and improve commit warning diagnostics
- Merge feature 'enforce-base-branch-guard' into develop
- Enforce protection on base branches with uncommitted changes and provide TUI guidance
- Merge feature 'tui-push-activity-panel' into develop
- Add activity logging for CLI commands and enhance TUI with push action

## Bug Fixes

- Resolve executables when PATH missing (fallback to project/home bin)
- Correct formatting of error message in invariant check test

