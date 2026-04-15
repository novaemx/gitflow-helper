# Release 0.5.24

**Date:** 2026-04-15

## What's New

- Merge feature 'atomic-commits-remote-branch-check' into develop
- Merge feature 'fix-init-files-on-develop-branch' into develop
- Merge feature 'audit-deep-dives' into develop
- Enhance git command execution and testing capabilities - Introduced GitClient interface and LocalGitClient implementation for better abstraction. - Added auto-initialization of git repository in root command. - Implemented coverage reporting in Makefile and updated README. - Added tests for git command execution and version handling. - Updated .gitignore to exclude coverage files.
- Introduce SplitCommand wrapper and refactor command execution

## Bug Fixes

- Enhance HARD STOP branch check and clarify mandatory pre-flight sequence
- Skip remote branch delete when branch is absent
- Enhance HARD STOP decision tree and clarify pre-flight checks
- Ask AI consent before provisioning IDE rules; TUI A-key 3-state activity panel
- Silent git plumbing, clean output, commit .gitflow.json on develop
- Generate init files on develop branch, AGENTS.md only for IDEs without .agents/
- Resolve executables when PATH missing (fallback to project/home bin)

## Improvements

- Streamline mandatory pre-flight checks and clarify branch creation rules

