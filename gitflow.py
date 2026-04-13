#!/usr/bin/env python3
"""
Generic Git Flow helper — interactive TUI + CLI subcommands.

Works with any project that follows the git-flow branching model.
Project-specific settings (version file, bump commands, branch names)
are read from an optional .gitflow.json in the project root.

Usage:
    python scripts/gitflow.py                  # interactive TUI (default)
    python scripts/gitflow.py status [--json]  # show repo state
    python scripts/gitflow.py pull  [--json]   # safe fetch + ff-merge/rebase
    python scripts/gitflow.py start <type> <name> [--json]
    python scripts/gitflow.py finish [<name>]  [--json]
    python scripts/gitflow.py init             [--json]
    python scripts/gitflow.py sync             [--json]
    python scripts/gitflow.py switch [<branch>] [--json]
    python scripts/gitflow.py backmerge        [--json]
    python scripts/gitflow.py cleanup          [--json]
    python scripts/gitflow.py health           [--json]
    python scripts/gitflow.py doctor           [--json]
    python scripts/gitflow.py log [-n N]       [--json]
    python scripts/gitflow.py undo             [--json]
    python scripts/gitflow.py releasenotes [<from-tag>] [--json]
    python scripts/gitflow.py interactive

Exit codes: 0=success  1=error  2=conflict-needs-human
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any, Optional


def _removeprefix(s: str, prefix: str) -> str:
    """str.removeprefix() backport for Python < 3.9."""
    if s.startswith(prefix):
        return s[len(prefix):]
    return s

# ── project root detection ────────────────────────────────────────────────────

def _find_project_root() -> Path:
    """Walk up from cwd or script location to find the nearest .git directory."""
    candidates = [Path.cwd(), Path(__file__).resolve().parent.parent]
    for start in candidates:
        p = start
        while p != p.parent:
            if (p / ".git").exists():
                return p
            p = p.parent
    return Path.cwd()


PROJECT_ROOT = _find_project_root()

# ── configuration ─────────────────────────────────────────────────────────────

@dataclass
class FlowConfig:
    remote: str = "origin"
    main_branch: str = "main"
    develop_branch: str = "develop"
    version_file: Optional[str] = None
    version_pattern: str = r'(?:__version__|"version")\s*[:=]\s*"([^"]+)"'
    bump_command: Optional[str] = None
    build_bump_command: Optional[str] = None
    tag_prefix: str = "v"


def load_config() -> FlowConfig:
    cfg = FlowConfig()
    config_path = PROJECT_ROOT / ".gitflow.json"
    if config_path.exists():
        try:
            data = json.loads(config_path.read_text())
            for k, v in data.items():
                if hasattr(cfg, k):
                    setattr(cfg, k, v)
        except (json.JSONDecodeError, OSError):
            pass
    if cfg.version_file is None:
        cfg.version_file = _detect_version_file()
    return cfg


_VERSION_FILE_CANDIDATES = [
    "core/version.py", "version.py", "src/version.py",
    "package.json", "pyproject.toml", "setup.cfg",
    "Cargo.toml", "VERSION", "internal/version/version.go",
]


def _detect_version_file() -> Optional[str]:
    for candidate in _VERSION_FILE_CANDIDATES:
        if (PROJECT_ROOT / candidate).exists():
            return candidate
    return None

# ── ANSI colors ───────────────────────────────────────────────────────────────

C_BOLD = "\033[1m"
C_DIM = "\033[2m"
C_GREEN = "\033[32m"
C_YELLOW = "\033[33m"
C_CYAN = "\033[36m"
C_RED = "\033[31m"
C_MAGENTA = "\033[35m"
C_RESET = "\033[0m"

# ── output helpers ────────────────────────────────────────────────────────────

_JSON_MODE = False


def set_json_mode(enabled: bool) -> None:
    global _JSON_MODE
    _JSON_MODE = enabled


def info(msg: str) -> None:
    if _JSON_MODE:
        print(msg, file=sys.stderr)
    else:
        print(msg)


def json_output(data: Any) -> None:
    print(json.dumps(data, indent=2, default=str))

# ── git helpers ───────────────────────────────────────────────────────────────

def run(cmd: str, *, check: bool = True, cwd: Optional[Path] = None) -> subprocess.CompletedProcess:
    info(f"  {C_DIM}→ {cmd}{C_RESET}")
    return subprocess.run(
        cmd, shell=True, check=check, text=True,
        cwd=cwd or PROJECT_ROOT,
    )


def run_quiet(cmd: str, cwd: Optional[Path] = None) -> str:
    r = subprocess.run(
        cmd, shell=True, capture_output=True, text=True,
        cwd=cwd or PROJECT_ROOT,
    )
    return r.stdout.strip()


def run_result(cmd: str, cwd: Optional[Path] = None) -> subprocess.CompletedProcess:
    return subprocess.run(
        cmd, shell=True, capture_output=True, text=True,
        cwd=cwd or PROJECT_ROOT,
    )


def run_lines(cmd: str) -> list[str]:
    out = run_quiet(cmd)
    return [l.strip() for l in out.splitlines() if l.strip()]


def current_branch() -> str:
    return run_quiet("git branch --show-current")


def read_version(cfg: FlowConfig) -> str:
    if not cfg.version_file:
        tag = latest_tag()
        return tag.lstrip("v") if tag and tag != "none" else "0.0.0"
    vf = PROJECT_ROOT / cfg.version_file
    if not vf.exists():
        return "0.0.0"
    content = vf.read_text()
    m = re.search(cfg.version_pattern, content)
    return m.group(1) if m else "0.0.0"


def latest_tag() -> str:
    return run_quiet("git describe --tags --abbrev=0 2>/dev/null") or "none"


def is_git_flow_initialized() -> bool:
    r = run_result("git config --get gitflow.branch.master")
    return r.returncode == 0


def has_uncommitted_changes() -> bool:
    return bool(run_lines("git status --porcelain"))


def active_release_branches() -> list[str]:
    """Return list of local release/* branches, if any."""
    all_local = run_lines("git branch --format='%(refname:short)'")
    return [b for b in all_local if b.startswith("release/")]

# ── interactive helpers ───────────────────────────────────────────────────────

def ask(prompt, default=""):
    # type: (str, str) -> str
    suffix = " [{}]".format(default) if default else ""
    try:
        answer = input("\n  {}{}: ".format(prompt, suffix)).strip()
    except (EOFError, KeyboardInterrupt):
        return default
    return answer or default


def confirm(prompt, default_yes=True):
    # type: (str, bool) -> bool
    hint = "Y/n" if default_yes else "y/N"
    try:
        answer = input("\n  {} ({}): ".format(prompt, hint)).strip().lower()
    except (EOFError, KeyboardInterrupt):
        return default_yes
    if not answer:
        return default_yes
    return answer in ("y", "yes")


def pick(title, options):
    # type: (str, list) -> int
    info("\n  {}{}{}\n".format(C_BOLD, title, C_RESET))
    for i, opt in enumerate(options, 1):
        info("    {}{}){} {}".format(C_CYAN, i, C_RESET, opt))
    while True:
        try:
            choice = input("\n  Choose [1-{}]: ".format(len(options))).strip()
        except (EOFError, KeyboardInterrupt):
            return 0
        if choice.isdigit() and 1 <= int(choice) <= len(options):
            return int(choice) - 1
        info("  Invalid choice, try again.")

# ── version helpers ───────────────────────────────────────────────────────────

def suggest_version(cfg: FlowConfig, bump_type: str) -> str:
    ver = read_version(cfg)
    m = re.match(r"(\d+)\.(\d+)\.(\d+)", ver)
    if not m:
        return "0.0.1"
    major, minor, patch = map(int, m.groups())
    if bump_type == "major":
        major += 1; minor = 0; patch = 0
    elif bump_type == "minor":
        minor += 1; patch = 0
    else:
        patch += 1
    return f"{major}.{minor}.{patch}"


def run_bump_command(cfg: FlowConfig, part: str = "patch") -> None:
    if not cfg.bump_command:
        return
    cmd = cfg.bump_command
    if "{part}" in cmd:
        cmd = cmd.replace("{part}", part)
    elif part != "patch":
        cmd = f"{cmd} --{part}"
    run(cmd)


def run_build_bump_command(cfg: FlowConfig) -> None:
    if not cfg.build_bump_command:
        return
    import platform as _plat
    plat = "win" if _plat.system() == "Windows" else "mac"
    cmd = cfg.build_bump_command.replace("{platform}", plat)
    run(cmd)


def maybe_push(cfg: FlowConfig, branch: str | None = None) -> None:
    target = branch or current_branch()
    if _JSON_MODE:
        return
    if confirm(f"Push '{target}' to {cfg.remote}?"):
        run(f"git push {cfg.remote} {target}")


def maybe_push_tags(cfg: FlowConfig) -> None:
    if _JSON_MODE:
        return
    if confirm(f"Push tags to {cfg.remote}?"):
        run(f"git push {cfg.remote} --tags")

# ── state detection ───────────────────────────────────────────────────────────

@dataclass
class BranchInfo:
    name: str
    branch_type: str
    short_name: str
    commits_ahead: int = 0
    has_remote: bool = False


@dataclass
class MergeState:
    in_merge: bool = False
    conflicted_files: list[str] = field(default_factory=list)
    merge_head: str = ""
    operation_type: str = ""
    operation_version: str = ""


@dataclass
class RepoState:
    current: str = ""
    version: str = ""
    last_tag: str = ""
    dirty: bool = False
    uncommitted_count: int = 0
    features: list[BranchInfo] = field(default_factory=list)
    bugfixes: list[BranchInfo] = field(default_factory=list)
    releases: list[BranchInfo] = field(default_factory=list)
    hotfixes: list[BranchInfo] = field(default_factory=list)
    has_develop: bool = False
    has_main: bool = False
    develop_ahead_of_main: int = 0
    main_ahead_of_develop: int = 0
    develop_only_files: list[str] = field(default_factory=list)
    main_only_files: list[str] = field(default_factory=list)
    merge: MergeState = field(default_factory=MergeState)
    git_flow_initialized: bool = False


def detect_merge_state(cfg: FlowConfig) -> MergeState:
    ms = MergeState()
    git_dir = PROJECT_ROOT / ".git"
    merge_head_file = git_dir / "MERGE_HEAD"
    if merge_head_file.exists():
        ms.in_merge = True
        ms.merge_head = merge_head_file.read_text().strip()[:12]
        ms.conflicted_files = run_lines("git diff --name-only --diff-filter=U")

    if ms.in_merge:
        all_local = run_lines("git branch --format='%(refname:short)'")
        for b in all_local:
            if b.startswith("release/"):
                ms.operation_type = "release"
                ms.operation_version = _removeprefix(_removeprefix(b, "release/v"), "release/")
                break
            if b.startswith("hotfix/"):
                ms.operation_type = "hotfix"
                ms.operation_version = _removeprefix(_removeprefix(b, "hotfix/v"), "hotfix/")
                break
    return ms


def detect_state(cfg: FlowConfig) -> RepoState:
    state = RepoState()
    state.current = current_branch()
    state.version = read_version(cfg)
    state.last_tag = latest_tag()
    state.merge = detect_merge_state(cfg)
    state.git_flow_initialized = is_git_flow_initialized()

    status_lines = run_lines("git status --porcelain")
    state.uncommitted_count = len(status_lines)
    state.dirty = state.uncommitted_count > 0

    all_local = run_lines("git branch --format='%(refname:short)'")
    all_remote = run_lines("git branch -r --format='%(refname:short)'")
    remote_set = set(all_remote)

    state.has_develop = cfg.develop_branch in all_local
    state.has_main = cfg.main_branch in all_local

    if state.has_develop and state.has_main and not state.merge.in_merge:
        ahead = run_quiet(f"git rev-list --count {cfg.main_branch}..{cfg.develop_branch} 2>/dev/null")
        state.develop_ahead_of_main = int(ahead) if ahead.isdigit() else 0
        behind = run_quiet(f"git rev-list --count {cfg.develop_branch}..{cfg.main_branch} 2>/dev/null")
        state.main_ahead_of_develop = int(behind) if behind.isdigit() else 0
        if state.develop_ahead_of_main > 0:
            state.develop_only_files = run_lines(
                f"git diff --name-only {cfg.main_branch}...{cfg.develop_branch}")
        if state.main_ahead_of_develop > 0:
            state.main_only_files = run_lines(
                f"git diff --name-only {cfg.develop_branch}...{cfg.main_branch}")

    prefix_map = {
        "feature/": ("feature", state.features),
        "bugfix/":  ("bugfix",  state.bugfixes),
        "release/": ("release", state.releases),
        "hotfix/":  ("hotfix",  state.hotfixes),
    }

    for branch in all_local:
        for prefix, (btype, target_list) in prefix_map.items():
            if branch.startswith(prefix):
                short = branch[len(prefix):]
                has_remote = f"{cfg.remote}/{branch}" in remote_set
                parent = cfg.develop_branch if btype in ("feature", "bugfix") else cfg.main_branch
                ahead_str = run_quiet(f"git rev-list --count {parent}..{branch} 2>/dev/null")
                ahead = int(ahead_str) if ahead_str.isdigit() else 0
                target_list.append(BranchInfo(
                    name=branch, branch_type=btype,
                    short_name=short, commits_ahead=ahead,
                    has_remote=has_remote,
                ))
                break
    return state

# ── conflict resolution ───────────────────────────────────────────────────────

def resolve_conflicts(state: RepoState, cfg: FlowConfig) -> None:
    ms = state.merge
    n = len(ms.conflicted_files)

    if n == 0:
        info(f"  {C_GREEN}No conflicts found.{C_RESET}")
        return

    info(f"\n  {C_RED}{C_BOLD}Merge conflict detected — {n} file(s) need resolution:{C_RESET}\n")
    for f in ms.conflicted_files:
        info(f"    {C_RED}✗{C_RESET} {f}")

    op_label = ""
    if ms.operation_type == "release":
        op_label = f"release v{ms.operation_version}"
    elif ms.operation_type == "hotfix":
        op_label = f"hotfix v{ms.operation_version}"

    if op_label:
        info(f"\n  {C_DIM}This conflict happened while finishing {op_label}.{C_RESET}")
        info(f"  {C_DIM}The incoming branch has the newer code you developed.{C_RESET}")

    idx = pick("How do you want to resolve?", [
        f"{C_GREEN}Accept incoming (theirs){C_RESET} — keep all changes from the {op_label or 'source'} branch "
        f"{C_DIM}(most common for releases/hotfixes){C_RESET}",
        f"{C_YELLOW}Accept current (ours){C_RESET} — keep what's on {state.current}, discard incoming",
        f"{C_MAGENTA}Resolve manually{C_RESET} — open each file yourself, then come back",
        f"{C_RED}Abort the merge{C_RESET} — cancel everything and go back to before",
    ])

    if idx == 0:
        _resolve_all_theirs(ms, cfg)
    elif idx == 1:
        _resolve_all_ours(ms, cfg)
    elif idx == 2:
        _resolve_manually(ms, cfg)
    elif idx == 3:
        _abort_merge(ms)


def _resolve_all_theirs(ms: MergeState, cfg: FlowConfig) -> None:
    files = " ".join(f'"{f}"' for f in ms.conflicted_files)
    run(f"git checkout --theirs {files}")
    run(f"git add {files}")
    info(f"\n  {C_GREEN}All {len(ms.conflicted_files)} conflict(s) resolved — accepted incoming version.{C_RESET}")
    _offer_continue(ms, cfg)


def _resolve_all_ours(ms: MergeState, cfg: FlowConfig) -> None:
    files = " ".join(f'"{f}"' for f in ms.conflicted_files)
    run(f"git checkout --ours {files}")
    run(f"git add {files}")
    info(f"\n  {C_GREEN}All {len(ms.conflicted_files)} conflict(s) resolved — kept current version.{C_RESET}")
    _offer_continue(ms, cfg)


def _resolve_manually(ms: MergeState, cfg: FlowConfig) -> None:
    info(f"\n  {C_YELLOW}Manual resolution mode.{C_RESET}")
    info(f"  For each file, choose how to resolve:\n")

    for filepath in ms.conflicted_files:
        idx = pick(f"Resolve '{filepath}':", [
            "Accept incoming (theirs)",
            "Accept current (ours)",
            "Skip — I'll edit this file myself later",
        ])
        if idx == 0:
            run(f'git checkout --theirs "{filepath}" && git add "{filepath}"')
        elif idx == 1:
            run(f'git checkout --ours "{filepath}" && git add "{filepath}"')
        else:
            info(f"  {C_DIM}Skipped {filepath} — resolve it manually and run 'git add {filepath}'{C_RESET}")

    remaining = run_lines("git diff --name-only --diff-filter=U")
    if remaining:
        info(f"\n  {C_YELLOW}{len(remaining)} file(s) still need manual resolution:{C_RESET}")
        for f in remaining:
            info(f"    {C_RED}✗{C_RESET} {f}")
        info(f"\n  {C_DIM}Edit those files, remove conflict markers (<<<< ==== >>>>), then:{C_RESET}")
        info(f"  {C_DIM}  git add <file>  ... for each one{C_RESET}")
        info(f"  {C_DIM}  then run this script again to continue.{C_RESET}")
    else:
        info(f"\n  {C_GREEN}All conflicts resolved!{C_RESET}")
        _offer_continue(ms, cfg)


def _abort_merge(ms: MergeState) -> None:
    if ms.operation_type:
        if confirm(f"Abort the {ms.operation_type} finish? This reverts to before the merge.", default_yes=False):
            run(f"git merge --abort", check=False)
            info(f"\n  {C_YELLOW}Merge aborted.{C_RESET} You're back on '{current_branch()}'.")
    else:
        if confirm("Abort the merge?", default_yes=False):
            run("git merge --abort", check=False)
            info(f"\n  {C_YELLOW}Merge aborted.{C_RESET}")


def _offer_continue(ms: MergeState, cfg: FlowConfig) -> None:
    if not ms.operation_type or not ms.operation_version:
        info(f"\n  {C_DIM}Run 'git commit' to finalize the merge, then continue your workflow.{C_RESET}")
        return

    if confirm(f"Continue the {ms.operation_type} finish for v{ms.operation_version}?"):
        run(f"git flow {ms.operation_type} finish {ms.operation_version}", check=False)
        if ms.operation_type in ("release", "hotfix"):
            maybe_push(cfg, cfg.main_branch)
            maybe_push(cfg, cfg.develop_branch)
            maybe_push_tags(cfg)
        info(f"\n  {C_GREEN}{ms.operation_type.title()} v{ms.operation_version} finished successfully!{C_RESET}")

# ── branch diff viewer ────────────────────────────────────────────────────────

def show_branch_diff(cfg: FlowConfig, source: str, target: str) -> None:
    """Show files and optionally full diffs between two branches."""
    files = run_lines(f"git diff --name-only {target}...{source}")
    if not files:
        info(f"  {C_GREEN}No differences between {source} and {target}.{C_RESET}")
        return

    info(f"\n  {C_BOLD}Files changed in {source} vs {target} ({len(files)} file(s)):{C_RESET}\n")
    stat_output = run_quiet(f"git diff --stat {target}...{source}")
    if stat_output:
        for line in stat_output.splitlines():
            info(f"    {line}")

    info("")
    if confirm("View full diff?", default_yes=False):
        diff_output = run_quiet(f"git diff {target}...{source}")
        if diff_output:
            _print_colorized_diff(diff_output)
        else:
            info(f"  {C_DIM}(empty diff){C_RESET}")


def _print_colorized_diff(diff_text: str) -> None:
    """Print a unified diff with ANSI coloring."""
    for line in diff_text.splitlines():
        if line.startswith("+++") or line.startswith("---"):
            info(f"  {C_BOLD}{line}{C_RESET}")
        elif line.startswith("@@"):
            info(f"  {C_CYAN}{line}{C_RESET}")
        elif line.startswith("+"):
            info(f"  {C_GREEN}{line}{C_RESET}")
        elif line.startswith("-"):
            info(f"  {C_RED}{line}{C_RESET}")
        else:
            info(f"  {line}")


# ── dashboard ─────────────────────────────────────────────────────────────────

def branch_type_of(name: str) -> str:
    for prefix in ("feature/", "bugfix/", "release/", "hotfix/"):
        if name.startswith(prefix):
            return prefix.rstrip("/")
    if name in ("develop", "main", "master"):
        return "base"
    return "other"


def _project_name() -> str:
    return PROJECT_ROOT.name


def print_dashboard(state: RepoState, cfg: FlowConfig) -> None:
    btype = branch_type_of(state.current)
    tag_display = state.last_tag if state.last_tag != "none" else "no tags yet"
    pname = _project_name()

    info(f"""
  {C_BOLD}╔═══════════════════════════════════════════════════╗
  ║       Git Flow Helper — {pname:<25s}║
  ╚═══════════════════════════════════════════════════╝{C_RESET}

  {C_BOLD}Current state:{C_RESET}
    Branch:    {C_CYAN}{state.current}{C_RESET}
    Version:   {C_GREEN}{state.version}{C_RESET}
    Last tag:  {tag_display}
    Git Flow:  {"initialized" if state.git_flow_initialized else C_YELLOW + "not initialized" + C_RESET}
    Uncommitted files: {C_YELLOW if state.dirty else C_GREEN}{state.uncommitted_count}{C_RESET}""")

    if state.develop_ahead_of_main > 0:
        info(f"    {cfg.develop_branch} is {C_YELLOW}{state.develop_ahead_of_main}{C_RESET} commit(s) ahead of {cfg.main_branch}")
        if state.develop_only_files:
            info(f"    {C_DIM}Files changed in {cfg.develop_branch} (not yet in {cfg.main_branch}):{C_RESET}")
            for f in state.develop_only_files[:10]:
                info(f"      {C_CYAN}+{C_RESET} {f}")
            if len(state.develop_only_files) > 10:
                info(f"      {C_DIM}... and {len(state.develop_only_files) - 10} more{C_RESET}")
    if state.main_ahead_of_develop > 0:
        info(f"    {C_RED}{cfg.main_branch} is {state.main_ahead_of_develop} commit(s) ahead of {cfg.develop_branch}{C_RESET}")
        info(f"    {C_RED}⚠  Branch divergence detected!{C_RESET} {cfg.main_branch} has changes not in {cfg.develop_branch}.")
        if state.main_only_files:
            info(f"    {C_DIM}Files in {cfg.main_branch} missing from {cfg.develop_branch}:{C_RESET}")
            for f in state.main_only_files[:10]:
                info(f"      {C_RED}!{C_RESET} {f}")
            if len(state.main_only_files) > 10:
                info(f"      {C_DIM}... and {len(state.main_only_files) - 10} more{C_RESET}")
        info(f"    {C_DIM}Fix: merge {cfg.main_branch} into {cfg.develop_branch} (run 'backmerge' below).{C_RESET}")

    ms = state.merge
    if ms.in_merge:
        n = len(ms.conflicted_files)
        info(f"""
  {C_RED}{C_BOLD}╔═══════════════════════════════════════════════════╗
  ║  ⚠  MERGE CONFLICT — {n:>2d} file(s) need resolution   ║
  ╚═══════════════════════════════════════════════════╝{C_RESET}""")
        if ms.operation_type:
            info(f"  {C_DIM}This happened during: git flow {ms.operation_type} finish v{ms.operation_version}{C_RESET}")
        files_to_show = ms.conflicted_files[:10] if n <= 10 else ms.conflicted_files[:8]
        for f in files_to_show:
            info(f"    {C_RED}✗{C_RESET} {f}")
        if n > 10:
            info(f"    {C_DIM}... and {n - 8} more{C_RESET}")

    any_inflight = False
    for label, branches, color in [
        ("Features",  state.features,  C_CYAN),
        ("Bugfixes",  state.bugfixes,  C_YELLOW),
        ("Releases",  state.releases,  C_GREEN),
        ("Hotfixes",  state.hotfixes,  C_RED),
    ]:
        if branches:
            any_inflight = True
            info(f"\n  {C_BOLD}{label} in flight:{C_RESET}")
            for b in branches:
                remote_tag = f" {C_DIM}(pushed){C_RESET}" if b.has_remote else ""
                commits = f"{b.commits_ahead} commit(s)" if b.commits_ahead else "no new commits"
                info(f"    {color}●{C_RESET} {b.name}  —  {commits}{remote_tag}")

    if not any_inflight and not ms.in_merge:
        info(f"\n  {C_DIM}No feature, bugfix, release, or hotfix branches in flight.{C_RESET}")

    info(f"\n  {C_BOLD}{'─' * 51}{C_RESET}")
    info(f"  {C_BOLD}Phase analysis:{C_RESET}\n")

    if ms.in_merge and ms.conflicted_files:
        _explain_conflict_phase(state, cfg)
    elif btype == "feature":
        _explain_feature_phase(state)
    elif btype == "bugfix":
        _explain_bugfix_phase(state)
    elif btype == "release":
        _explain_release_phase(state)
    elif btype == "hotfix":
        _explain_hotfix_phase(state)
    elif state.current == cfg.develop_branch:
        _explain_develop_phase(state, cfg)
    elif state.current == cfg.main_branch:
        _explain_main_phase(state, cfg)
    else:
        info(f"    You are on {C_DIM}'{state.current}'{C_RESET}, not a standard git-flow branch.")
        info(f"    {C_DIM}Consider switching to {cfg.develop_branch} or finishing an in-flight branch.{C_RESET}")

    if state.releases and btype != "release" and not ms.in_merge:
        info(f"\n  {C_YELLOW}⚠  Release '{state.releases[0].name}' is open.{C_RESET} Finish it before starting new features.")
    if state.hotfixes and btype != "hotfix" and not ms.in_merge:
        info(f"\n  {C_RED}⚠  Hotfix '{state.hotfixes[0].name}' is open.{C_RESET} Hotfixes should be finished quickly.")
    if len(state.features) > 2 and not ms.in_merge:
        info(f"\n  {C_YELLOW}⚠  {len(state.features)} features in flight.{C_RESET} Consider finishing some before starting more.")
    if state.main_ahead_of_develop > 0 and not ms.in_merge:
        info(f"\n  {C_RED}⚠  BRANCH DIVERGENCE: {cfg.main_branch} has {state.main_ahead_of_develop} commit(s) not in {cfg.develop_branch}.{C_RESET}")
        info(f"  {C_DIM}Run 'backmerge' to merge {cfg.main_branch} into {cfg.develop_branch} and restore gitflow invariant.{C_RESET}")
    info("")


def _explain_conflict_phase(state: RepoState, cfg: FlowConfig) -> None:
    ms = state.merge
    n = len(ms.conflicted_files)
    if ms.operation_type:
        info(f"    {C_RED}You are in a merge conflict{C_RESET} during {ms.operation_type} finish v{ms.operation_version}.")
        info(f"    The {ms.operation_type} branch has your latest code; '{state.current}' has the older version.")
        info(f"    For most cases, accepting the incoming ({ms.operation_type}) branch resolves everything.")
    else:
        info(f"    {C_RED}You are in a merge conflict{C_RESET} with {n} file(s) to resolve.")
    info(f"\n    {C_GREEN}Next → Resolve the conflicts below, then continue the operation.{C_RESET}")


def _explain_feature_phase(state: RepoState) -> None:
    name = _removeprefix(state.current, "feature/")
    bi = next((b for b in state.features if b.short_name == name), None)
    commits = bi.commits_ahead if bi else 0
    info(f"    You are on feature {C_CYAN}'{name}'{C_RESET}.")
    if state.dirty:
        info(f"    {C_YELLOW}You have uncommitted changes.{C_RESET} Commit or stash before finishing.")
    if commits == 0:
        info(f"    No new commits yet — you just started.")
        info(f"    {C_DIM}Next → Write code, commit, then finish the feature.{C_RESET}")
    else:
        info(f"    {C_GREEN}{commits}{C_RESET} commit(s) ready.")
        info(f"    {C_GREEN}Next → Finish the feature{C_RESET} to merge into develop.")


def _explain_bugfix_phase(state: RepoState) -> None:
    name = _removeprefix(state.current, "bugfix/")
    bi = next((b for b in state.bugfixes if b.short_name == name), None)
    commits = bi.commits_ahead if bi else 0
    info(f"    You are on bugfix {C_YELLOW}'{name}'{C_RESET}.")
    if state.dirty:
        info(f"    {C_YELLOW}Uncommitted changes.{C_RESET} Commit or stash before finishing.")
    if commits == 0:
        info(f"    No commits yet — apply your fix.")
    else:
        info(f"    {commits} commit(s) ready. {C_GREEN}Next → Finish the bugfix.{C_RESET}")


def _explain_release_phase(state: RepoState) -> None:
    ver = _removeprefix(_removeprefix(state.current, "release/v"), "release/")
    info(f"    You are on release branch {C_GREEN}v{ver}{C_RESET}.")
    info(f"    This is the stabilization phase before tagging and merging to main.")
    if state.dirty:
        info(f"    {C_YELLOW}Uncommitted changes.{C_RESET} Commit last fixes first.")
        info(f"    {C_DIM}Next → Commit final adjustments, then finish the release.{C_RESET}")
    else:
        info(f"    {C_GREEN}Next → Finish the release{C_RESET} to tag v{ver}, merge to main & develop.")


def _explain_hotfix_phase(state: RepoState) -> None:
    ver = _removeprefix(_removeprefix(state.current, "hotfix/v"), "hotfix/")
    info(f"    You are on hotfix {C_RED}v{ver}{C_RESET}.")
    info(f"    Hotfixes are urgent patches applied directly from main.")
    if state.dirty:
        info(f"    {C_YELLOW}Uncommitted changes.{C_RESET} Commit your fix first.")
    else:
        info(f"    {C_GREEN}Next → Finish the hotfix{C_RESET} to tag and merge to main & develop.")


def _explain_develop_phase(state: RepoState, cfg: FlowConfig) -> None:
    info(f"    You are on {C_CYAN}{cfg.develop_branch}{C_RESET} — the integration branch.")
    if state.main_ahead_of_develop > 0:
        info(f"    {C_RED}CRITICAL:{C_RESET} {cfg.main_branch} has {state.main_ahead_of_develop} commit(s) not in {cfg.develop_branch}.")
        info(f"    Per gitflow, {cfg.develop_branch} must always be a superset of {cfg.main_branch}.")
        info(f"    {C_GREEN}Next → Back-merge {cfg.main_branch} into {cfg.develop_branch} before any other work.{C_RESET}")
    elif state.releases:
        rel = state.releases[0]
        info(f"    {C_YELLOW}Warning:{C_RESET} release/{rel.short_name} exists. Switch there to finish it.")
    elif state.develop_ahead_of_main > 0:
        info(f"    {cfg.develop_branch} has {state.develop_ahead_of_main} unreleased commit(s).")
        info(f"    {C_GREEN}Next → Start a new feature, bugfix, or start a release.{C_RESET}")
    else:
        info(f"    {cfg.develop_branch} is up to date with {cfg.main_branch}.")
        info(f"    {C_GREEN}Next → Start a new feature or bugfix.{C_RESET}")


def _explain_main_phase(state: RepoState, cfg: FlowConfig) -> None:
    info(f"    You are on {C_BOLD}{cfg.main_branch}{C_RESET} — the production branch.")
    if state.main_ahead_of_develop > 0:
        info(f"    {C_RED}CRITICAL:{C_RESET} {cfg.main_branch} has {state.main_ahead_of_develop} commit(s) not in {cfg.develop_branch}.")
        info(f"    {C_GREEN}Next → Switch to {cfg.develop_branch} and back-merge {cfg.main_branch}.{C_RESET}")
    elif state.hotfixes:
        hf = state.hotfixes[0]
        info(f"    {C_YELLOW}Hotfix '{hf.short_name}' is in progress.{C_RESET} Switch there to continue.")
    elif state.merge.in_merge:
        pass
    else:
        info(f"    {C_DIM}Normally you work from {cfg.develop_branch}. Switch there to start work.{C_RESET}")
        info(f"    {C_GREEN}Next → git checkout {cfg.develop_branch}, or start a hotfix if urgent.{C_RESET}")

# ── smart menu ────────────────────────────────────────────────────────────────

@dataclass
class Action:
    label: str
    fn: object
    recommended: bool = False
    tag: str = ""


def build_actions(state: RepoState, cfg: FlowConfig) -> list[Action]:
    btype = branch_type_of(state.current)
    ms = state.merge
    actions: list[Action] = []

    if ms.in_merge and ms.conflicted_files:
        actions.append(Action(
            f"{C_RED}Resolve {len(ms.conflicted_files)} merge conflict(s){C_RESET}",
            lambda: resolve_conflicts(state, cfg),
            recommended=True, tag="resolve"))
        actions.append(Action(
            f"Abort the {'%s finish' % ms.operation_type if ms.operation_type else 'merge'}",
            lambda: _abort_merge(ms), tag="abort"))
        actions.append(Action("Exit", None, tag="exit"))
        return actions

    if ms.in_merge and not ms.conflicted_files and ms.operation_type:
        actions.append(Action(
            f"{C_GREEN}Continue {ms.operation_type} finish v{ms.operation_version}{C_RESET}",
            lambda: _offer_continue(ms, cfg),
            recommended=True, tag="continue"))
        actions.append(Action("Exit", None, tag="exit"))
        return actions

    # Always offer pull/sync near the top
    actions.append(Action("Pull latest (safe fetch + merge)", lambda: cmd_pull(cfg), tag="pull"))

    if state.main_ahead_of_develop > 0:
        actions.append(Action(
            f"{C_RED}Back-merge {cfg.main_branch} into {cfg.develop_branch} "
            f"({state.main_ahead_of_develop} commit(s) behind){C_RESET}",
            lambda: cmd_backmerge(cfg),
            recommended=True, tag="backmerge"))
        actions.append(Action(
            f"View diff: {cfg.main_branch} vs {cfg.develop_branch} "
            f"({len(state.main_only_files)} file(s))",
            lambda: show_branch_diff(cfg, cfg.main_branch, cfg.develop_branch),
            tag="diff"))

    if state.develop_ahead_of_main > 0:
        actions.append(Action(
            f"View diff: {cfg.develop_branch} vs {cfg.main_branch} "
            f"({len(state.develop_only_files)} file(s))",
            lambda: show_branch_diff(cfg, cfg.develop_branch, cfg.main_branch),
            tag="diff"))

    if not state.git_flow_initialized:
        actions.append(Action(
            f"{C_YELLOW}Initialize git-flow{C_RESET}",
            lambda: cmd_init(cfg),
            recommended=True, tag="init"))

    if btype == "feature":
        name = _removeprefix(state.current, "feature/")
        bi = next((b for b in state.features if b.short_name == name), None)
        has_work = bi and bi.commits_ahead > 0 and not state.dirty
        actions.append(Action(
            f"Finish feature '{name}'", lambda: finish_feature(cfg),
            recommended=has_work, tag="finish"))
        actions.append(Action(
            f"Sync with {cfg.develop_branch}", lambda: cmd_sync(cfg), tag="sync"))
        actions.append(Action("Start a new feature", lambda: start_feature(cfg), tag="start"))

    elif btype == "bugfix":
        name = _removeprefix(state.current, "bugfix/")
        bi = next((b for b in state.bugfixes if b.short_name == name), None)
        has_work = bi and bi.commits_ahead > 0 and not state.dirty
        actions.append(Action(
            f"Finish bugfix '{name}'", lambda: finish_bugfix(cfg),
            recommended=has_work, tag="finish"))
        actions.append(Action(
            f"Sync with {cfg.develop_branch}", lambda: cmd_sync(cfg), tag="sync"))

    elif btype == "release":
        ver = _removeprefix(_removeprefix(state.current, "release/v"), "release/")
        actions.append(Action(
            f"Finish release v{ver}", lambda: finish_release(cfg),
            recommended=not state.dirty, tag="finish"))

    elif btype == "hotfix":
        ver = _removeprefix(_removeprefix(state.current, "hotfix/v"), "hotfix/")
        actions.append(Action(
            f"Finish hotfix v{ver}", lambda: finish_hotfix(cfg),
            recommended=not state.dirty, tag="finish"))

    elif state.current == cfg.develop_branch:
        if state.releases:
            rel = state.releases[0]
            ver = rel.short_name.lstrip("v")
            actions.append(Action(
                f"Switch to release '{rel.name}' and finish it",
                lambda: _switch_and_finish_release(rel.name, cfg),
                recommended=True, tag="finish"))
        actions.append(Action("Start a new feature", lambda: start_feature(cfg),
                              recommended=not state.releases, tag="start"))
        actions.append(Action("Start a bugfix", lambda: start_bugfix(cfg), tag="start"))
        if state.develop_ahead_of_main > 0 and not state.releases:
            actions.append(Action("Start a release", lambda: start_release(cfg),
                                  recommended=True, tag="release"))

    elif state.current == cfg.main_branch:
        actions.append(Action("Start a hotfix (urgent)", lambda: start_hotfix(cfg), tag="hotfix"))
        actions.append(Action(
            f"Switch to {cfg.develop_branch}",
            lambda: run(f"git checkout {cfg.develop_branch}"),
            recommended=True, tag="switch"))

    # Switch actions for all reachable branches
    if state.current != cfg.develop_branch:
        if not any(a.tag == "switch" and cfg.develop_branch in a.label for a in actions):
            actions.append(Action(
                f"Switch to {cfg.develop_branch}",
                lambda: run(f"git checkout {cfg.develop_branch}"), tag="switch"))
    if state.current != cfg.main_branch:
        actions.append(Action(
            f"Switch to {cfg.main_branch}",
            lambda: run(f"git checkout {cfg.main_branch}"), tag="switch"))
    for b in state.features:
        if b.name != state.current:
            actions.append(Action(
                f"Switch to feature '{b.short_name}'",
                lambda br=b.name: run(f"git checkout {br}"), tag="switch"))
    for b in state.bugfixes:
        if b.name != state.current:
            actions.append(Action(
                f"Switch to bugfix '{b.short_name}'",
                lambda br=b.name: run(f"git checkout {br}"), tag="switch"))
    for b in state.releases:
        if b.name != state.current:
            actions.append(Action(
                f"Switch to release '{b.short_name}'",
                lambda br=b.name: run(f"git checkout {br}"), tag="switch"))
    for b in state.hotfixes:
        if b.name != state.current:
            actions.append(Action(
                f"Switch to hotfix '{b.short_name}'",
                lambda br=b.name: run(f"git checkout {br}"), tag="switch"))

    if btype not in ("release", "hotfix") and not state.releases:
        if not any(a.tag == "release" for a in actions):
            actions.append(Action("Start a release", lambda: start_release(cfg), tag="release"))
    if btype != "hotfix" and not state.hotfixes:
        if not any(a.tag == "hotfix" for a in actions):
            actions.append(Action("Start a hotfix (urgent)", lambda: start_hotfix(cfg), tag="hotfix"))
    if not any(a.tag == "start" and "feature" in a.label for a in actions):
        actions.append(Action("Start a new feature", lambda: start_feature(cfg), tag="start"))

    actions.append(Action("Clean up merged branches", lambda: cmd_cleanup(cfg), tag="cleanup"))
    actions.append(Action("View commit log", lambda: cmd_log(cfg), tag="log"))
    actions.append(Action("Repo health check", lambda: cmd_health(cfg), tag="health"))
    actions.append(Action(f"{C_YELLOW}Undo last operation{C_RESET}", lambda: cmd_undo(cfg), tag="undo"))
    actions.append(Action("Exit", None, tag="exit"))
    return actions


def _switch_and_finish_release(branch: str, cfg: FlowConfig) -> None:
    run(f"git checkout {branch}")
    finish_release(cfg)

# ── workflows ─────────────────────────────────────────────────────────────────

def _flow_version(version: str) -> str:
    """Strip leading 'v' to avoid double-prefix with git-flow's tag_prefix."""
    return version.lstrip("v")


def start_feature(cfg: FlowConfig) -> None:
    name = ask("Feature name (e.g. add-csv-export)")
    if not name:
        info("  Aborted."); return
    run(f"git flow feature start {name}")
    info(f"\n  {C_GREEN}Feature branch 'feature/{name}' created. Happy coding!{C_RESET}")


def finish_feature(cfg: FlowConfig) -> None:
    branch = current_branch()
    if not branch.startswith("feature/"):
        branch_name = ask("Feature name to finish")
    else:
        branch_name = _removeprefix(branch, "feature/")
    if not branch_name:
        info("  Aborted."); return
    run(f"git flow feature finish {branch_name}")
    maybe_push(cfg, cfg.develop_branch)


def start_bugfix(cfg: FlowConfig) -> None:
    name = ask("Bugfix name (e.g. fix-cluster-filter)")
    if not name:
        info("  Aborted."); return
    run(f"git flow bugfix start {name}")
    info(f"\n  {C_GREEN}Bugfix branch 'bugfix/{name}' created.{C_RESET}")


def finish_bugfix(cfg: FlowConfig) -> None:
    branch = current_branch()
    if not branch.startswith("bugfix/"):
        branch_name = ask("Bugfix name to finish")
    else:
        branch_name = _removeprefix(branch, "bugfix/")
    if not branch_name:
        info("  Aborted."); return
    run(f"git flow bugfix finish {branch_name}")
    maybe_push(cfg, cfg.develop_branch)


def start_release(cfg: FlowConfig) -> None:
    current_ver = read_version(cfg)
    suggested = suggest_version(cfg, "minor")
    info(f"\n  Current version: {current_ver}")
    info(f"  Suggested next:  {suggested}")
    version = ask("Release version", default=suggested)
    if not version:
        info("  Aborted."); return

    if cfg.bump_command:
        idx = pick("Bump version file now?", [
            f"Yes, bump to {version} (minor)",
            "Yes, bump patch",
            "Yes, bump major",
            "No, I'll bump manually",
        ])
        if idx == 0:
            run_bump_command(cfg, "minor")
        elif idx == 1:
            run_bump_command(cfg, "patch")
        elif idx == 2:
            run_bump_command(cfg, "major")

        run_build_bump_command(cfg)

        if cfg.version_file:
            run(f'git add "{cfg.version_file}"')
        run(f'git commit -m "chore: bump version to {version}"', check=False)

    run(f"git flow release start {_flow_version(version)}")
    info(f"\n  {C_GREEN}Release branch created.{C_RESET}")
    info(f"  Make any final adjustments, then run this script again to finish.")


def finish_release(cfg: FlowConfig) -> None:
    branch = current_branch()
    if not branch.startswith("release/"):
        version = ask("Release version to finish (e.g. 0.5.0)")
    else:
        version = _removeprefix(_removeprefix(branch, "release/v"), "release/")
    if not version:
        info("  Aborted."); return

    version = _flow_version(version)
    info(f"\n  Will finish release v{version} and create tag.")
    if not confirm("Proceed?"):
        info("  Aborted."); return

    result = run(f"git flow release finish {version}", check=False)
    if result.returncode != 0:
        _handle_flow_failure("release", version)
        return

    info(f"\n  {C_GREEN}Release v{version} finished!{C_RESET}")
    info(f"\n  Generating release notes...")
    meta = _write_release_notes(cfg)
    if meta:
        _print_release_notes(meta)
    else:
        info(f"  {C_YELLOW}Could not generate release notes (no commit range found).{C_RESET}")

    maybe_push(cfg, cfg.main_branch)
    maybe_push(cfg, cfg.develop_branch)
    maybe_push_tags(cfg)


def start_hotfix(cfg: FlowConfig) -> None:
    current_ver = read_version(cfg)
    suggested = suggest_version(cfg, "patch")
    info(f"\n  Current version: {current_ver}")
    info(f"  Suggested hotfix: {suggested}")
    version = ask("Hotfix version", default=suggested)
    if not version:
        info("  Aborted."); return

    if cfg.bump_command:
        run_bump_command(cfg, "patch")
        run_build_bump_command(cfg)
        if cfg.version_file:
            run(f'git add "{cfg.version_file}"')
        run(f'git commit -m "chore: bump version to {version} (hotfix)"', check=False)

    run(f"git flow hotfix start {_flow_version(version)}")
    info(f"\n  {C_GREEN}Hotfix branch created. Apply your fix, then finish.{C_RESET}")


def finish_hotfix(cfg: FlowConfig) -> None:
    branch = current_branch()
    if not branch.startswith("hotfix/"):
        version = ask("Hotfix version to finish (e.g. 0.5.1)")
    else:
        version = _removeprefix(_removeprefix(branch, "hotfix/v"), "hotfix/")
    if not version:
        info("  Aborted."); return

    version = _flow_version(version)

    releases = active_release_branches()
    if releases:
        rel = releases[0]
        info(f"\n  {C_YELLOW}Note:{C_RESET} Release branch '{rel}' is active.")
        info(f"  Per gitflow best practices, the hotfix will also be merged into")
        info(f"  the release branch instead of (or in addition to) {cfg.develop_branch}.")

    info(f"\n  Will finish hotfix v{version} and create tag.")
    if not confirm("Proceed?"):
        info("  Aborted."); return

    result = run(f"git flow hotfix finish {version}", check=False)
    if result.returncode != 0:
        _handle_flow_failure("hotfix", version)
        return

    maybe_push(cfg, cfg.main_branch)
    maybe_push(cfg, cfg.develop_branch)
    maybe_push_tags(cfg)
    info(f"\n  {C_GREEN}Hotfix v{version} finished!{C_RESET}")


def _handle_flow_failure(operation: str, version: str) -> None:
    conflicts = run_lines("git diff --name-only --diff-filter=U")
    if conflicts:
        info(f"\n  {C_RED}Merge conflict detected during {operation} finish.{C_RESET}")
        info(f"  {C_DIM}Run this script again — it will detect the conflict and guide you.{C_RESET}")
    else:
        info(f"\n  {C_RED}The {operation} finish failed.{C_RESET} Check the git output above.")
        info(f"  {C_DIM}You may need to resolve the issue manually, then run:{C_RESET}")
        info(f"  {C_DIM}  git flow {operation} finish {version}{C_RESET}")

# ── CLI subcommand: status ────────────────────────────────────────────────────

def cmd_status(cfg: FlowConfig) -> int:
    state = detect_state(cfg)
    if _JSON_MODE:
        json_output(_state_to_dict(state))
    else:
        print_dashboard(state, cfg)
    return 0


def _state_to_dict(state: RepoState) -> dict:
    d = asdict(state)
    d["project_root"] = str(PROJECT_ROOT)
    return d

# ── CLI subcommand: pull ──────────────────────────────────────────────────────

def cmd_pull(cfg: FlowConfig) -> int:
    """Safe pull: fetch, then fast-forward merge. Never pushes."""
    branch = current_branch()
    if not branch:
        info(f"  {C_RED}Not on any branch (detached HEAD?). Aborting pull.{C_RESET}")
        return 1

    had_changes = has_uncommitted_changes()
    stashed = False
    if had_changes:
        info(f"  {C_YELLOW}Stashing uncommitted changes before pull...{C_RESET}")
        run("git stash push -m 'gitflow-auto-stash'")
        stashed = True

    info(f"\n  {C_BOLD}Fetching from all remotes...{C_RESET}")
    run(f"git fetch --all --prune", check=False)

    remote_branch = run_quiet(f"git config --get branch.{branch}.remote")
    if not remote_branch:
        remote_branch = cfg.remote

    merge_branch = run_quiet(f"git config --get branch.{branch}.merge")
    if not merge_branch:
        tracking_ref = f"{remote_branch}/{branch}"
        check = run_result(f"git rev-parse --verify {tracking_ref}")
        if check.returncode != 0:
            info(f"  {C_DIM}No upstream tracking for '{branch}'. Nothing to pull.{C_RESET}")
            if stashed:
                run("git stash pop", check=False)
            if _JSON_MODE:
                json_output({"action": "pull", "branch": branch, "result": "no_upstream"})
            return 0

    result = run_result(f"git merge --ff-only {remote_branch}/{branch}")
    if result.returncode == 0:
        info(f"  {C_GREEN}Fast-forward merge successful for '{branch}'.{C_RESET}")
        if stashed:
            pop = run_result("git stash pop")
            if pop.returncode != 0:
                info(f"  {C_YELLOW}Stash pop had conflicts. Resolve manually with 'git stash show -p | git apply'.{C_RESET}")
                if _JSON_MODE:
                    json_output({"action": "pull", "branch": branch, "result": "ok_stash_conflict"})
                return 2
        if _JSON_MODE:
            json_output({"action": "pull", "branch": branch, "result": "fast_forward"})
        return 0

    info(f"  {C_YELLOW}Fast-forward not possible — branches have diverged.{C_RESET}")

    if _JSON_MODE:
        info(f"  {C_DIM}Attempting rebase (CLI mode)...{C_RESET}")
        do_rebase = True
    else:
        do_rebase = confirm("Rebase your local commits on top of remote?", default_yes=True)

    if do_rebase:
        rebase_result = run_result(f"git rebase {remote_branch}/{branch}")
        if rebase_result.returncode == 0:
            info(f"  {C_GREEN}Rebase successful for '{branch}'.{C_RESET}")
            if stashed:
                pop = run_result("git stash pop")
                if pop.returncode != 0:
                    info(f"  {C_YELLOW}Stash pop had conflicts.{C_RESET}")
                    if _JSON_MODE:
                        json_output({"action": "pull", "branch": branch, "result": "rebase_ok_stash_conflict"})
                    return 2
            if _JSON_MODE:
                json_output({"action": "pull", "branch": branch, "result": "rebased"})
            return 0
        else:
            info(f"  {C_RED}Rebase has conflicts. Aborting to preserve your code.{C_RESET}")
            run("git rebase --abort", check=False)
            if stashed:
                run("git stash pop", check=False)
            if _JSON_MODE:
                json_output({"action": "pull", "branch": branch, "result": "conflict", "needs_human": True})
            return 2
    else:
        info(f"  {C_DIM}Pull skipped. Your branch is behind remote.{C_RESET}")
        if stashed:
            run("git stash pop", check=False)
        if _JSON_MODE:
            json_output({"action": "pull", "branch": branch, "result": "skipped"})
        return 0

# ── CLI subcommand: start ─────────────────────────────────────────────────────

def cmd_start(cfg: FlowConfig, branch_type: str, name: str) -> int:
    valid_types = ("feature", "bugfix", "release", "hotfix")
    if branch_type not in valid_types:
        info(f"  {C_RED}Invalid type '{branch_type}'. Must be one of: {', '.join(valid_types)}{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "start", "error": f"invalid type: {branch_type}"})
        return 1

    branch = current_branch()
    expected_parent = cfg.main_branch if branch_type == "hotfix" else cfg.develop_branch

    if branch != expected_parent:
        hint = f"switch to {expected_parent} first"
        info(f"  {C_YELLOW}Warning:{C_RESET} '{branch_type}' branches should start from '{expected_parent}' (currently on '{branch}').")
        if _JSON_MODE:
            info(f"  {C_DIM}git-flow will handle the checkout, but verify you are on the right base.{C_RESET}")
        else:
            if confirm(f"Checkout '{expected_parent}' before starting?", default_yes=True):
                run(f"git checkout {expected_parent}", check=False)

    if branch_type in ("feature", "bugfix"):
        releases = active_release_branches()
        if releases:
            info(f"  {C_YELLOW}Warning:{C_RESET} Release branch '{releases[0]}' is in progress.")
            info(f"  Per gitflow best practices, no new features should start until the release is finished.")
            if _JSON_MODE:
                pass  # non-blocking warning, included in output below
            elif not confirm("Start anyway?", default_yes=False):
                info("  Aborted.")
                return 1

    if branch_type in ("release", "hotfix"):
        version = _flow_version(name)
        result = run(f"git flow {branch_type} start {version}", check=False)
    else:
        result = run(f"git flow {branch_type} start {name}", check=False)

    if result.returncode != 0:
        if _JSON_MODE:
            json_output({"action": "start", "type": branch_type, "name": name, "result": "error"})
        return 1

    branch_name = f"{branch_type}/{name}"
    info(f"\n  {C_GREEN}Branch '{branch_name}' created.{C_RESET}")
    if _JSON_MODE:
        output: dict[str, Any] = {
            "action": "start", "type": branch_type, "name": name,
            "branch": branch_name, "result": "ok",
        }
        if branch != expected_parent:
            output["hint"] = f"switch to {expected_parent} first"
        releases = active_release_branches()
        if releases and branch_type in ("feature", "bugfix"):
            output["warning"] = "release_in_progress"
        json_output(output)
    return 0

# ── CLI subcommand: finish ────────────────────────────────────────────────────

def cmd_finish(cfg: FlowConfig, name: Optional[str] = None) -> int:
    branch = current_branch()
    btype = branch_type_of(branch)

    if btype not in ("feature", "bugfix", "release", "hotfix"):
        if name:
            for prefix in ("feature/", "bugfix/", "release/", "hotfix/"):
                if name.startswith(prefix):
                    btype = prefix.rstrip("/")
                    name = name[len(prefix):]
                    break
        if btype not in ("feature", "bugfix", "release", "hotfix"):
            info(f"  {C_RED}Not on a git-flow branch and no valid name provided.{C_RESET}")
            if _JSON_MODE:
                json_output({"action": "finish", "error": "not on flow branch"})
            return 1

    if name is None:
        prefixes = {"feature": "feature/", "bugfix": "bugfix/", "release": "release/", "hotfix": "hotfix/"}
        prefix = prefixes.get(btype, "")
        name = _removeprefix(branch, prefix).lstrip("v")

    if btype in ("release", "hotfix"):
        name = _flow_version(name)

    if _JSON_MODE:
        output: dict[str, Any] = {"action": "finish", "type": btype, "name": name}
        if btype == "hotfix":
            releases = active_release_branches()
            if releases:
                output["release_branch_active"] = releases[0]
                info(f"  {C_YELLOW}Note:{C_RESET} Hotfix will also merge into {releases[0]} (nvie rule).")
        result = run(f"git flow {btype} finish {name}", check=False)
        success = result.returncode == 0
        output["result"] = "ok" if success else "error"
        if success and btype == "release":
            meta = _write_release_notes(cfg)
            if meta:
                output["release_notes"] = meta
        json_output(output)
        return 0 if success else 1
    else:
        if btype == "feature":
            finish_feature(cfg)
        elif btype == "bugfix":
            finish_bugfix(cfg)
        elif btype == "release":
            finish_release(cfg)
        elif btype == "hotfix":
            finish_hotfix(cfg)
        return 0

# ── CLI subcommand: init ──────────────────────────────────────────────────────

def cmd_init(cfg: FlowConfig) -> int:
    if is_git_flow_initialized():
        info(f"  {C_GREEN}Git flow is already initialized.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "init", "result": "already_initialized"})
        return 0

    result = run(
        f"git flow init -d "
        f"--defaults",
        check=False,
    )

    if result.returncode != 0:
        info(f"  {C_YELLOW}Default init failed, trying with explicit branches...{C_RESET}")
        run(f"git flow init -f", check=False)

    ok = is_git_flow_initialized()
    if ok:
        info(f"  {C_GREEN}Git flow initialized successfully.{C_RESET}")
    else:
        info(f"  {C_RED}Git flow initialization failed.{C_RESET}")

    if _JSON_MODE:
        json_output({"action": "init", "result": "ok" if ok else "error"})
    return 0 if ok else 1

# ── CLI subcommand: sync ──────────────────────────────────────────────────────

def cmd_sync(cfg: FlowConfig) -> int:
    """Sync current feature/bugfix branch with develop, or hotfix with main."""
    branch = current_branch()
    btype = branch_type_of(branch)

    if btype in ("feature", "bugfix"):
        parent = cfg.develop_branch
    elif btype == "hotfix":
        parent = cfg.main_branch
    elif btype == "release":
        parent = cfg.develop_branch
    else:
        info(f"  {C_DIM}Sync is for feature/bugfix/release/hotfix branches.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "sync", "error": "not on flow branch"})
        return 1

    info(f"\n  {C_BOLD}Syncing '{branch}' with '{parent}'...{C_RESET}")
    run(f"git fetch {cfg.remote} {parent}", check=False)

    result = run_result(f"git merge --no-ff {cfg.remote}/{parent}")
    if result.returncode == 0:
        info(f"  {C_GREEN}Sync successful.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "sync", "branch": branch, "parent": parent, "result": "ok"})
        return 0

    conflicts = run_lines("git diff --name-only --diff-filter=U")
    if conflicts:
        info(f"  {C_RED}Merge conflicts during sync:{C_RESET}")
        for f in conflicts:
            info(f"    {C_RED}✗{C_RESET} {f}")

        if _JSON_MODE:
            info(f"  {C_DIM}Aborting merge to preserve code.{C_RESET}")
            run("git merge --abort", check=False)
            json_output({"action": "sync", "branch": branch, "parent": parent,
                         "result": "conflict", "files": conflicts, "needs_human": True})
            return 2
        else:
            if not confirm("Resolve conflicts now?", default_yes=False):
                run("git merge --abort", check=False)
                info(f"  {C_YELLOW}Sync aborted.{C_RESET}")
                return 1
    else:
        info(f"  {C_RED}Merge failed (not a conflict).{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "sync", "branch": branch, "result": "error"})
        return 1

    return 0

# ── CLI subcommand: backmerge ─────────────────────────────────────────────────

def cmd_backmerge(cfg: FlowConfig) -> int:
    """Merge main into develop to restore the gitflow invariant (develop >= main)."""
    behind = run_quiet(f"git rev-list --count {cfg.develop_branch}..{cfg.main_branch} 2>/dev/null")
    behind_count = int(behind) if behind.isdigit() else 0

    if behind_count == 0:
        info(f"  {C_GREEN}{cfg.develop_branch} already contains all commits from {cfg.main_branch}.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "backmerge", "result": "up_to_date"})
        return 0

    info(f"\n  {C_BOLD}Back-merging {cfg.main_branch} into {cfg.develop_branch}...{C_RESET}")
    info(f"  {cfg.main_branch} has {C_YELLOW}{behind_count}{C_RESET} commit(s) not in {cfg.develop_branch}.")

    changed_files = run_lines(f"git diff --name-only {cfg.develop_branch}...{cfg.main_branch}")
    if changed_files:
        info(f"\n  {C_BOLD}Files that will be merged into {cfg.develop_branch}:{C_RESET}\n")
        stat_output = run_quiet(f"git diff --stat {cfg.develop_branch}...{cfg.main_branch}")
        if stat_output:
            for line in stat_output.splitlines():
                info(f"    {line}")
        info("")
        if _JSON_MODE:
            pass  # file list included in JSON output below
        elif confirm("View full diff before merging?", default_yes=False):
            diff_output = run_quiet(f"git diff {cfg.develop_branch}...{cfg.main_branch}")
            if diff_output:
                _print_colorized_diff(diff_output)
            info("")
        if not _JSON_MODE and not confirm("Proceed with back-merge?", default_yes=True):
            info("  Aborted.")
            return 0

    original_branch = current_branch()
    if original_branch != cfg.develop_branch:
        info(f"  Switching to {cfg.develop_branch}...")
        result = run(f"git checkout {cfg.develop_branch}", check=False)
        if result.returncode != 0:
            info(f"  {C_RED}Failed to checkout {cfg.develop_branch}.{C_RESET}")
            if _JSON_MODE:
                json_output({"action": "backmerge", "result": "error", "detail": "checkout_failed"})
            return 1

    run(f"git fetch {cfg.remote} {cfg.main_branch}", check=False)
    result = run_result(f"git merge --no-ff {cfg.main_branch} -m \"Merge {cfg.main_branch} into {cfg.develop_branch} (backmerge)\"")

    if result.returncode == 0:
        info(f"  {C_GREEN}Back-merge successful. {cfg.develop_branch} now contains all of {cfg.main_branch}.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "backmerge", "result": "ok",
                         "commits_merged": behind_count,
                         "files_merged": changed_files})
        return 0

    conflicts = run_lines("git diff --name-only --diff-filter=U")
    if conflicts:
        info(f"  {C_RED}Merge conflicts during back-merge:{C_RESET}")
        for f in conflicts:
            info(f"    {C_RED}✗{C_RESET} {f}")
        if _JSON_MODE:
            run("git merge --abort", check=False)
            json_output({"action": "backmerge", "result": "conflict",
                         "files": conflicts, "needs_human": True})
            return 2
        else:
            info(f"\n  {C_DIM}Resolve conflicts, then 'git add' and 'git commit' to complete.{C_RESET}")
            return 2
    else:
        info(f"  {C_RED}Merge failed.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "backmerge", "result": "error"})
        return 1

# ── CLI subcommand: switch ────────────────────────────────────────────────────

def cmd_switch(cfg: FlowConfig, target: Optional[str] = None) -> int:
    """Switch to a gitflow branch by name or pick interactively."""
    cur = current_branch()

    all_local = run_lines("git branch --format='%(refname:short)'")
    flow_branches = _list_switchable_branches(all_local, cfg, cur)

    if not flow_branches:
        info(f"  {C_YELLOW}No other branches to switch to.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "switch", "result": "no_branches"})
        return 1

    if target:
        matches = [b for b in flow_branches if b == target or b.endswith("/" + target)]
        if not matches:
            info(f"  {C_RED}Branch '{target}' not found.{C_RESET}")
            if _JSON_MODE:
                json_output({"action": "switch", "result": "not_found",
                             "target": target,
                             "available": flow_branches})
            return 1
        chosen = matches[0]
    elif _JSON_MODE:
        json_output({"action": "switch", "result": "branch_required",
                     "available": flow_branches})
        return 1
    else:
        info(f"\n  {C_BOLD}Switch branch (current: {C_CYAN}{cur}{C_RESET}{C_BOLD}){C_RESET}\n")
        idx = pick("Switch to:", flow_branches)
        chosen = flow_branches[idx]

    stashed = False
    if state_is_dirty():
        if _JSON_MODE:
            json_output({"action": "switch", "result": "dirty",
                         "detail": "uncommitted changes — stash or commit first"})
            return 1
        if not confirm("You have uncommitted changes. Auto-stash and switch?"):
            info("  Aborted.")
            return 0
        stashed = _smart_stash_save(cur)

    if _JSON_MODE:
        result = run_result(f"git checkout {chosen}")
    else:
        result = run(f"git checkout {chosen}", check=False)
    if result.returncode != 0:
        info(f"  {C_RED}Failed to switch to '{chosen}'.{C_RESET}")
        if stashed:
            _smart_stash_pop(cur)
        if _JSON_MODE:
            json_output({"action": "switch", "result": "error", "target": chosen})
        return 1

    # Try to restore any previously stashed changes for the target branch
    _smart_stash_pop(chosen)

    info(f"  {C_GREEN}Switched to '{chosen}'.{C_RESET}")
    if _JSON_MODE:
        json_output({"action": "switch", "result": "ok", "branch": chosen,
                     "previous": cur})
    return 0


def state_is_dirty() -> bool:
    return len(run_lines("git status --porcelain")) > 0


def _list_switchable_branches(all_local: list, cfg: FlowConfig, current: str) -> list:
    """Build an ordered list of branches available for switching."""
    result = []
    permanent = [cfg.main_branch, cfg.develop_branch]
    for b in permanent:
        if b in all_local and b != current:
            result.append(b)

    prefixes = ("feature/", "bugfix/", "release/", "hotfix/")
    for prefix in prefixes:
        for b in sorted(all_local):
            if b.startswith(prefix) and b != current:
                result.append(b)
    return result


# ── CLI subcommand: cleanup ───────────────────────────────────────────────────

def cmd_cleanup(cfg: FlowConfig) -> int:
    """Delete local branches that have been fully merged into develop/main."""
    info(f"\n  {C_BOLD}Cleaning up merged branches...{C_RESET}")

    merged_into_develop = set()
    merged_into_main = set()

    if run_result(f"git rev-parse --verify {cfg.develop_branch}").returncode == 0:
        merged_into_develop = set(run_lines(f"git branch --merged {cfg.develop_branch} --format='%(refname:short)'"))
    if run_result(f"git rev-parse --verify {cfg.main_branch}").returncode == 0:
        merged_into_main = set(run_lines(f"git branch --merged {cfg.main_branch} --format='%(refname:short)'"))

    all_merged = merged_into_develop | merged_into_main
    protected = {cfg.main_branch, cfg.develop_branch, "master", current_branch()}
    to_delete = [b for b in all_merged if b not in protected and
                 any(b.startswith(p) for p in ("feature/", "bugfix/", "release/", "hotfix/"))]

    if not to_delete:
        info(f"  {C_GREEN}No merged branches to clean up.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "cleanup", "deleted": [], "result": "nothing_to_clean"})
        return 0

    info(f"\n  Branches to delete:")
    for b in to_delete:
        info(f"    {C_DIM}●{C_RESET} {b}")

    if _JSON_MODE:
        for b in to_delete:
            run(f"git branch -d {b}", check=False)
        json_output({"action": "cleanup", "deleted": to_delete, "result": "ok"})
        return 0

    if confirm(f"Delete {len(to_delete)} merged branch(es)?"):
        for b in to_delete:
            run(f"git branch -d {b}", check=False)
        info(f"  {C_GREEN}Cleanup complete.{C_RESET}")
    else:
        info(f"  Cleanup skipped.")
    return 0

# ── CLI subcommand: health ────────────────────────────────────────────────────

def cmd_health(cfg: FlowConfig) -> int:
    """Comprehensive repo health check — like an experienced dev reviewing the repo."""
    issues = []
    warnings = []
    ok_items = []

    # 1. Git & git-flow installed
    git_ver = run_quiet("git --version")
    if not git_ver:
        issues.append("git is not installed or not in PATH")
    else:
        ok_items.append(f"git: {git_ver.replace('git version ', '')}")

    gf_check = run_result("git flow version")
    if gf_check.returncode != 0:
        issues.append("git-flow extensions not installed (run: brew install git-flow / apt install git-flow)")
    else:
        ok_items.append(f"git-flow: {gf_check.stdout.strip()}")

    # 2. Git-flow initialized
    if not is_git_flow_initialized():
        issues.append("git-flow not initialized (run: gitflow.py init)")

    # 3. Branch existence
    all_local = run_lines("git branch --format='%(refname:short)'")
    if cfg.develop_branch not in all_local:
        issues.append(f"'{cfg.develop_branch}' branch missing")
    if cfg.main_branch not in all_local:
        issues.append(f"'{cfg.main_branch}' branch missing")

    # 4. Remote reachable
    fetch_result = run_result(f"git ls-remote --exit-code {cfg.remote} HEAD")
    if fetch_result.returncode != 0:
        warnings.append(f"Remote '{cfg.remote}' unreachable (offline or misconfigured)")
    else:
        ok_items.append(f"remote '{cfg.remote}' reachable")

    # 5. Branch divergence
    if cfg.develop_branch in all_local and cfg.main_branch in all_local:
        main_ahead = run_quiet(f"git rev-list --count {cfg.develop_branch}..{cfg.main_branch}")
        main_ahead_n = int(main_ahead) if main_ahead.isdigit() else 0
        if main_ahead_n > 0:
            files = run_lines(f"git diff --name-only {cfg.develop_branch}...{cfg.main_branch}")
            issues.append(
                f"{cfg.main_branch} is {main_ahead_n} commit(s) ahead of {cfg.develop_branch} "
                f"({len(files)} file(s)) — run backmerge")
        else:
            ok_items.append(f"{cfg.develop_branch} contains all of {cfg.main_branch}")

    # 6. Unpushed commits on permanent branches
    for branch in [cfg.main_branch, cfg.develop_branch]:
        if branch in all_local:
            unpushed = run_quiet(f"git rev-list --count {cfg.remote}/{branch}..{branch} 2>/dev/null")
            n = int(unpushed) if unpushed.isdigit() else 0
            if n > 0:
                warnings.append(f"'{branch}' has {n} unpushed commit(s)")
            else:
                ok_items.append(f"'{branch}' up to date with remote")

    # 7. Stale branches (no commits in > 30 days)
    stale = []
    for b in all_local:
        if any(b.startswith(p) for p in ("feature/", "bugfix/", "release/", "hotfix/")):
            last_commit_ts = run_quiet(f"git log -1 --format=%ct {b} 2>/dev/null")
            if last_commit_ts.isdigit():
                import time
                age_days = (time.time() - int(last_commit_ts)) / 86400
                if age_days > 30:
                    stale.append(f"{b} (inactive {int(age_days)} days)")
    if stale:
        for s in stale:
            warnings.append(f"stale branch: {s}")

    # 8. Orphan flow branches (parent deleted from under them)
    for b in all_local:
        if b.startswith("feature/") or b.startswith("bugfix/"):
            parent = cfg.develop_branch
        elif b.startswith("hotfix/"):
            parent = cfg.main_branch
        elif b.startswith("release/"):
            parent = cfg.develop_branch
        else:
            continue
        if parent not in all_local:
            issues.append(f"'{b}' parent '{parent}' does not exist")

    # 9. Tag on main HEAD
    main_head = run_quiet(f"git rev-parse {cfg.main_branch} 2>/dev/null")
    tag_at_main = run_quiet(f"git tag --points-at {main_head} 2>/dev/null") if main_head else ""
    if main_head and not tag_at_main:
        warnings.append(f"{cfg.main_branch} HEAD has no tag (every main merge should be tagged)")
    elif tag_at_main:
        ok_items.append(f"{cfg.main_branch} HEAD tagged: {tag_at_main.splitlines()[0]}")

    # 10. Uncommitted changes
    dirty_count = len(run_lines("git status --porcelain"))
    if dirty_count > 0:
        warnings.append(f"{dirty_count} uncommitted file(s) in working tree")

    if _JSON_MODE:
        json_output({
            "action": "health",
            "issues": issues,
            "warnings": warnings,
            "ok": ok_items,
            "healthy": len(issues) == 0,
        })
        return 0 if not issues else 1

    info(f"\n  {C_BOLD}╔═══════════════════════════════════════════════════╗")
    info(f"  ║              Repository Health Check              ║")
    info(f"  ╚═══════════════════════════════════════════════════╝{C_RESET}\n")

    for item in ok_items:
        info(f"    {C_GREEN}✓{C_RESET} {item}")
    for w in warnings:
        info(f"    {C_YELLOW}⚠{C_RESET} {w}")
    for iss in issues:
        info(f"    {C_RED}✗{C_RESET} {iss}")

    if not issues and not warnings:
        info(f"\n  {C_GREEN}{C_BOLD}All checks passed — repo is healthy!{C_RESET}")
    elif not issues:
        info(f"\n  {C_YELLOW}No critical issues, {len(warnings)} warning(s).{C_RESET}")
    else:
        info(f"\n  {C_RED}{len(issues)} issue(s) need attention.{C_RESET}")

    return 0 if not issues else 1


# ── CLI subcommand: undo ─────────────────────────────────────────────────────

def cmd_undo(cfg: FlowConfig) -> int:
    """Undo the last gitflow operation using reflog-based recovery."""
    cur = current_branch()

    reflog = run_lines("git reflog --format='%H %gs' -n 20")
    if not reflog:
        info(f"  {C_RED}No reflog entries found.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "undo", "result": "no_reflog"})
        return 1

    # Find the last significant gitflow-related operation
    undo_candidates = []
    for entry in reflog:
        parts = entry.split(" ", 1)
        if len(parts) < 2:
            continue
        sha, desc = parts[0], parts[1]
        desc_lower = desc.lower()
        if any(kw in desc_lower for kw in (
            "merge", "checkout: moving", "commit", "finish", "start",
        )):
            undo_candidates.append((sha, desc))
        if len(undo_candidates) >= 10:
            break

    if not undo_candidates:
        info(f"  {C_YELLOW}No undoable operations found in recent history.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "undo", "result": "nothing_to_undo"})
        return 1

    if _JSON_MODE:
        json_output({
            "action": "undo",
            "result": "candidates",
            "current_branch": cur,
            "entries": [{"sha": s[:12], "description": d} for s, d in undo_candidates],
        })
        return 0

    info(f"\n  {C_BOLD}Recent operations (newest first):{C_RESET}\n")
    labels = []
    for sha, desc in undo_candidates:
        labels.append(f"{C_CYAN}{sha[:10]}{C_RESET}  {desc}")
    labels.append(f"{C_DIM}Cancel — don't undo anything{C_RESET}")

    idx = pick("Reset HEAD to which point?", labels)
    if idx == len(undo_candidates):
        info("  Aborted.")
        return 0

    target_sha, target_desc = undo_candidates[idx]
    info(f"\n  Will reset {C_CYAN}{cur}{C_RESET} to {C_YELLOW}{target_sha[:10]}{C_RESET}")
    info(f"  {C_DIM}({target_desc}){C_RESET}")
    info(f"\n  {C_YELLOW}This is a soft reset — your files stay as they are,")
    info(f"  changes become uncommitted so you can review them.{C_RESET}")

    if not confirm("Proceed with undo?", default_yes=False):
        info("  Aborted.")
        return 0

    result = run(f"git reset --soft {target_sha}", check=False)
    if result.returncode == 0:
        info(f"\n  {C_GREEN}Undo successful.{C_RESET} HEAD moved to {target_sha[:10]}.")
        info(f"  {C_DIM}Use 'git status' to see staged changes, 'git reset HEAD' to unstage.{C_RESET}")
        return 0
    else:
        info(f"  {C_RED}Reset failed.{C_RESET}")
        return 1


# ── CLI subcommand: log ──────────────────────────────────────────────────────

def cmd_log(cfg: FlowConfig, count: int = 20) -> int:
    """Show gitflow-aware commit history with release boundaries."""
    fmt = "%H|%h|%s|%an|%ar|%D"
    entries = run_lines(f"git log --all --format='{fmt}' -n {count}")
    if not entries:
        info(f"  {C_DIM}No commits found.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "log", "entries": []})
        return 0

    parsed = []
    for entry in entries:
        parts = entry.split("|", 5)
        if len(parts) < 6:
            continue
        full_sha, short_sha, subject, author, rel_date, refs = parts
        # Determine if this is a release boundary (tagged merge into main)
        is_release = False
        tag = ""
        branch_refs = ""
        if refs.strip():
            branch_refs = refs.strip()
            if "tag:" in refs:
                tag_match = re.search(r"tag:\s*([^\s,)]+)", refs)
                if tag_match:
                    tag = tag_match.group(1)
                    is_release = True

        parsed.append({
            "sha": short_sha, "full_sha": full_sha,
            "subject": subject, "author": author,
            "date": rel_date, "refs": branch_refs,
            "tag": tag, "is_release": is_release,
        })

    if _JSON_MODE:
        json_output({"action": "log", "entries": parsed})
        return 0

    info(f"\n  {C_BOLD}Gitflow commit log (last {count}):{C_RESET}\n")
    for p in parsed:
        sha_col = f"{C_YELLOW}{p['sha']}{C_RESET}"
        subj = p["subject"]

        # Color-code by branch type detected from subject
        if p["is_release"]:
            info(f"  {'─' * 55}")
            info(f"  {C_GREEN}{C_BOLD}▼ RELEASE {p['tag']}{C_RESET}")

        prefix = ""
        subj_lower = subj.lower()
        if "merge branch 'feature/" in subj_lower or subj_lower.startswith("feature"):
            prefix = f"{C_CYAN}[feature]{C_RESET} "
        elif "merge branch 'bugfix/" in subj_lower or subj_lower.startswith("bugfix") or subj_lower.startswith("fix"):
            prefix = f"{C_YELLOW}[bugfix]{C_RESET}  "
        elif "merge branch 'hotfix/" in subj_lower or subj_lower.startswith("hotfix"):
            prefix = f"{C_RED}[hotfix]{C_RESET}  "
        elif "merge branch 'release/" in subj_lower:
            prefix = f"{C_GREEN}[release]{C_RESET} "
        elif "backmerge" in subj_lower:
            prefix = f"{C_MAGENTA}[sync]{C_RESET}    "

        refs_display = ""
        if p["refs"]:
            refs_display = f" {C_DIM}({p['refs']}){C_RESET}"

        info(f"  {sha_col} {prefix}{subj}{refs_display}")
        info(f"         {C_DIM}{p['author']} · {p['date']}{C_RESET}")

    return 0


# ── CLI subcommand: doctor (prerequisite validation) ─────────────────────────

def cmd_doctor(cfg: FlowConfig) -> int:
    """Validate all prerequisites for running gitflow."""
    checks = []

    # Python version
    py_ver = f"{sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}"
    checks.append(("Python", py_ver, True))

    # Git
    git_ver = run_quiet("git --version")
    checks.append(("git", git_ver.replace("git version ", "") if git_ver else "NOT FOUND", bool(git_ver)))

    # Git-flow
    gf = run_result("git flow version")
    gf_ver = gf.stdout.strip() if gf.returncode == 0 else "NOT INSTALLED"
    checks.append(("git-flow", gf_ver, gf.returncode == 0))

    # Inside a git repo
    in_repo = (PROJECT_ROOT / ".git").exists()
    checks.append(("git repo", str(PROJECT_ROOT) if in_repo else "NOT A REPO", in_repo))

    # Remote configured
    remotes = run_lines("git remote")
    has_remote = cfg.remote in remotes
    checks.append(("remote", cfg.remote if has_remote else f"'{cfg.remote}' not found", has_remote))

    # Branches
    all_branches = run_lines("git branch --format='%(refname:short)'")
    has_main = cfg.main_branch in all_branches
    has_dev = cfg.develop_branch in all_branches
    checks.append((cfg.main_branch, "exists" if has_main else "MISSING", has_main))
    checks.append((cfg.develop_branch, "exists" if has_dev else "MISSING", has_dev))

    # Git-flow initialized
    gf_init = is_git_flow_initialized()
    checks.append(("gitflow init", "yes" if gf_init else "NOT INITIALIZED", gf_init))

    all_ok = all(ok for _, _, ok in checks)

    if _JSON_MODE:
        json_output({
            "action": "doctor",
            "checks": [{"name": n, "value": v, "ok": o} for n, v, o in checks],
            "all_ok": all_ok,
        })
        return 0 if all_ok else 1

    info(f"\n  {C_BOLD}Gitflow Doctor — prerequisite check{C_RESET}\n")
    for name, value, ok in checks:
        icon = f"{C_GREEN}✓{C_RESET}" if ok else f"{C_RED}✗{C_RESET}"
        info(f"    {icon} {name}: {value}")

    if all_ok:
        info(f"\n  {C_GREEN}All prerequisites met.{C_RESET}")
    else:
        info(f"\n  {C_RED}Some prerequisites are missing. Fix them before proceeding.{C_RESET}")

    return 0 if all_ok else 1


# ── Smart stash helpers ──────────────────────────────────────────────────────

def _smart_stash_save(branch: str) -> bool:
    """Auto-stash with a named message tied to the branch. Returns True if stashed."""
    if not has_uncommitted_changes():
        return False
    msg = f"gitflow-auto: {branch}"
    result = run(f'git stash push -m "{msg}"', check=False)
    return result.returncode == 0


def _smart_stash_pop(branch: str) -> bool:
    """Pop the most recent stash that matches the branch name."""
    stashes = run_lines("git stash list")
    for i, line in enumerate(stashes):
        if f"gitflow-auto: {branch}" in line:
            result = run(f"git stash pop stash@{{{i}}}", check=False)
            if result.returncode == 0:
                info(f"  {C_GREEN}Restored stashed changes for '{branch}'.{C_RESET}")
                return True
            else:
                info(f"  {C_YELLOW}Stash pop had conflicts. Resolve manually.{C_RESET}")
                return False
    return False


# ── CLI subcommand: releasenotes ──────────────────────────────────────────────

def _get_previous_tag(current_tag: str) -> str:
    """Find the tag immediately before *current_tag* in chronological order."""
    tags = run_lines("git tag --sort=-version:refname")
    found = False
    for t in tags:
        if found:
            return t
        if t == current_tag:
            found = True
    return ""


def _classify_commit(subject: str) -> str:
    """Classify a commit subject line into a release-note category."""
    s = subject.lower()
    if any(s.startswith(p) for p in ("chore:", "chore(", "ci:", "ci(", "build:", "build(", "deps:")):
        return "maintenance"
    if any(kw in s for kw in ("bump version", "bump to")):
        return "maintenance"
    if any(kw in s for kw in ("feat", "add ", "new ", "implement", "introduce")):
        return "features"
    if any(kw in s for kw in ("fix", "bug", "patch", "hotfix", "resolve", "correct")):
        return "fixes"
    if any(kw in s for kw in ("improve", "enhance", "optim", "refactor", "perf", "update", "upgrade")):
        return "improvements"
    return "other"


def _generate_release_notes(cfg: FlowConfig, from_ref: str, to_ref: str, version: str) -> str:
    """Build user-facing markdown release notes between two refs."""
    fmt = "%s"
    entries = run_lines(f"git log --format='{fmt}' {from_ref}..{to_ref}")

    groups = {
        "features": [],
        "fixes": [],
        "improvements": [],
        "maintenance": [],
        "other": [],
    }

    for subject in entries:
        if not subject.strip():
            continue
        cat = _classify_commit(subject)
        clean = re.sub(r"^(feat|fix|chore|refactor|perf|docs|style|test|ci|build|improvement)(\(.*?\))?:\s*", "", subject, flags=re.IGNORECASE)
        clean = clean.strip().capitalize()
        if clean and clean not in groups[cat]:
            groups[cat].append(clean)

    lines = []
    lines.append(f"# Release {version}\n")

    import datetime
    lines.append(f"**Date:** {datetime.date.today().isoformat()}\n")

    has_content = False

    if groups["features"]:
        has_content = True
        lines.append("## What's New\n")
        for item in groups["features"]:
            lines.append(f"- {item}")
        lines.append("")

    if groups["fixes"]:
        has_content = True
        lines.append("## Bug Fixes\n")
        for item in groups["fixes"]:
            lines.append(f"- {item}")
        lines.append("")

    if groups["improvements"]:
        has_content = True
        lines.append("## Improvements\n")
        for item in groups["improvements"]:
            lines.append(f"- {item}")
        lines.append("")

    if not has_content:
        if groups["maintenance"] or groups["other"]:
            lines.append("## Changes\n")
            for item in groups["maintenance"] + groups["other"]:
                lines.append(f"- {item}")
            lines.append("")
        else:
            lines.append("_No user-facing changes in this release._\n")

    return "\n".join(lines)


def _write_release_notes(cfg: FlowConfig, from_tag: str = "") -> Optional[dict]:
    """Generate notes, write file, return metadata dict or None on failure."""
    current_ver = read_version(cfg)
    tag = latest_tag()

    if from_tag:
        from_ref = from_tag
    elif tag and tag != "none":
        prev = _get_previous_tag(tag)
        from_ref = prev if prev else tag
    else:
        from_ref = ""

    if not from_ref:
        entries = run_lines("git log --format='%s' -n 50")
        if not entries:
            return None
        from_ref = run_quiet("git rev-list --max-parents=0 HEAD")

    to_ref = "HEAD"
    version = current_ver if current_ver != "0.0.0" else (
        tag.lstrip("v") if tag and tag != "none" else "unreleased"
    )

    content = _generate_release_notes(cfg, from_ref, to_ref, version)

    notes_path = PROJECT_ROOT / "RELEASE_NOTES.md"
    notes_path.write_text(content + "\n")

    return {
        "version": version,
        "from_ref": from_ref,
        "to_ref": to_ref,
        "file": str(notes_path),
        "content": content,
    }


def _print_release_notes(meta: dict) -> None:
    """Pretty-print release notes to stderr/stdout."""
    info(f"\n  {C_BOLD}Release Notes for {meta['version']}{C_RESET}\n")
    info(f"  {C_DIM}Range: {meta['from_ref']}..{meta['to_ref']}{C_RESET}\n")
    for line in meta["content"].splitlines():
        if line.startswith("# "):
            info(f"  {C_BOLD}{C_GREEN}{line}{C_RESET}")
        elif line.startswith("## "):
            info(f"  {C_BOLD}{C_CYAN}{line}{C_RESET}")
        elif line.startswith("- "):
            info(f"    {line}")
        else:
            info(f"  {line}")
    info(f"\n  {C_GREEN}Written to:{C_RESET} {meta['file']}")


def cmd_releasenotes(cfg: FlowConfig, from_tag: str = "") -> int:
    """Generate user-facing release notes from git history between tags."""
    meta = _write_release_notes(cfg, from_tag)
    if meta is None:
        info(f"  {C_YELLOW}No commits found for release notes.{C_RESET}")
        if _JSON_MODE:
            json_output({"action": "releasenotes", "result": "empty"})
        return 1

    if _JSON_MODE:
        json_output({"action": "releasenotes", "result": "ok", **meta})
    else:
        _print_release_notes(meta)

    return 0


# ── Enhanced state detection ─────────────────────────────────────────────────

def _detect_unpushed(cfg: FlowConfig, branch: str) -> int:
    """Count commits on branch not pushed to remote."""
    out = run_quiet(f"git rev-list --count {cfg.remote}/{branch}..{branch} 2>/dev/null")
    return int(out) if out.isdigit() else 0


def _detect_remote_ahead(cfg: FlowConfig, branch: str) -> int:
    """Count commits on remote not yet in local branch."""
    run_result(f"git fetch {cfg.remote} {branch} --quiet")
    out = run_quiet(f"git rev-list --count {branch}..{cfg.remote}/{branch} 2>/dev/null")
    return int(out) if out.isdigit() else 0


# ── TUI engine (OpenCode-style full-screen interface) ────────────────────────

try:
    import curses
    _HAS_CURSES = True
except ImportError:
    _HAS_CURSES = False

import signal

# Curses color pair IDs
_CP_NORMAL = 0
_CP_TITLE = 1
_CP_BRANCH_FEATURE = 2
_CP_BRANCH_BUGFIX = 3
_CP_BRANCH_RELEASE = 4
_CP_BRANCH_HOTFIX = 5
_CP_STATUS = 6
_CP_SELECTED = 7
_CP_DIM = 8
_CP_ERROR = 9
_CP_RECOMMENDED = 10
_CP_DIVERGENCE = 11
_CP_SECTION = 12

# Key constants
K_UP = "up"
K_DOWN = "down"
K_ENTER = "enter"
K_ESCAPE = "escape"
K_BACKSPACE = "backspace"
K_QUIT = "q"
K_HELP = "?"
K_SEARCH = "/"
K_REFRESH = "r"
K_HOME = "g"
K_END = "G"
K_TAB = "tab"


def _strip_ansi(text):
    # type: (str) -> str
    """Remove ANSI escape sequences from a string."""
    return re.sub(r'\033\[[0-9;]*m', '', text)


def _trunc(text, width):
    # type: (str, int) -> str
    """Truncate text to width (using visible length, not byte length)."""
    plain = _strip_ansi(text)
    if len(plain) <= width:
        return text
    return plain[:max(0, width - 1)] + "\u2026"


class _CursesBackend(object):
    """Full-screen TUI backend using curses."""

    def __init__(self, stdscr):
        self.scr = stdscr
        self.h = 0
        self.w = 0
        self._init_colors()
        curses.curs_set(0)
        self.scr.keypad(True)
        self.scr.timeout(-1)
        self._resize()

    def _init_colors(self):
        curses.start_color()
        curses.use_default_colors()
        curses.init_pair(_CP_TITLE, curses.COLOR_BLACK, curses.COLOR_CYAN)
        curses.init_pair(_CP_BRANCH_FEATURE, curses.COLOR_CYAN, -1)
        curses.init_pair(_CP_BRANCH_BUGFIX, curses.COLOR_YELLOW, -1)
        curses.init_pair(_CP_BRANCH_RELEASE, curses.COLOR_GREEN, -1)
        curses.init_pair(_CP_BRANCH_HOTFIX, curses.COLOR_RED, -1)
        curses.init_pair(_CP_STATUS, curses.COLOR_BLACK, curses.COLOR_WHITE)
        curses.init_pair(_CP_SELECTED, curses.COLOR_BLACK, curses.COLOR_CYAN)
        curses.init_pair(_CP_DIM, curses.COLOR_WHITE, -1)
        curses.init_pair(_CP_ERROR, curses.COLOR_RED, -1)
        curses.init_pair(_CP_RECOMMENDED, curses.COLOR_GREEN, -1)
        curses.init_pair(_CP_DIVERGENCE, curses.COLOR_RED, -1)
        curses.init_pair(_CP_SECTION, curses.COLOR_MAGENTA, -1)

    def _resize(self):
        self.h, self.w = self.scr.getmaxyx()

    def clear(self):
        self.scr.erase()

    def refresh(self):
        self.scr.noutrefresh()
        curses.doupdate()

    def addstr(self, row, col, text, attr=0):
        # type: (int, int, str, int) -> None
        if row < 0 or row >= self.h or col >= self.w:
            return
        max_len = self.w - col
        if max_len <= 0:
            return
        t = text[:max_len]
        try:
            self.scr.addstr(row, col, t, attr)
        except curses.error:
            pass

    def fill_row(self, row, char=" ", attr=0):
        # type: (int, str, int) -> None
        self.addstr(row, 0, char * self.w, attr)

    def get_key(self):
        # type: () -> str
        try:
            ch = self.scr.getch()
        except curses.error:
            return ""
        if ch == curses.KEY_UP or ch == ord('k'):
            return K_UP
        if ch == curses.KEY_DOWN or ch == ord('j'):
            return K_DOWN
        if ch in (curses.KEY_ENTER, 10, 13):
            return K_ENTER
        if ch == 27:
            self.scr.nodelay(True)
            ch2 = self.scr.getch()
            self.scr.nodelay(False)
            if ch2 == -1:
                return K_ESCAPE
            return ""
        if ch in (curses.KEY_BACKSPACE, 127, 8):
            return K_BACKSPACE
        if ch == 9:
            return K_TAB
        if ch == curses.KEY_HOME:
            return K_HOME
        if ch == curses.KEY_END:
            return K_END
        if ch == curses.KEY_RESIZE:
            self._resize()
            return "resize"
        if 0 <= ch <= 255:
            return chr(ch)
        return ""

    def color(self, pair_id):
        # type: (int) -> int
        return curses.color_pair(pair_id)

    def bold(self):
        return curses.A_BOLD

    def dim(self):
        return curses.A_DIM

    def reverse(self):
        return curses.A_REVERSE


class _AnsiBackend(object):
    """Fallback TUI backend using ANSI escape codes + raw terminal I/O."""

    def __init__(self):
        self.h = 24
        self.w = 80
        self._buffer = []  # type: list
        self._resize()
        self._setup_raw()

    def _resize(self):
        try:
            sz = os.get_terminal_size()
            self.w = sz.columns
            self.h = sz.lines
        except (OSError, ValueError):
            pass

    def _setup_raw(self):
        try:
            import tty as _tty
            import termios as _termios
            self._fd = sys.stdin.fileno()
            self._old_settings = _termios.tcgetattr(self._fd)
            _tty.setcbreak(self._fd)
            self._raw = True
        except (ImportError, OSError, _termios.error if 'termios' in dir() else Exception):
            self._raw = False

    def cleanup(self):
        if getattr(self, '_raw', False):
            import termios as _termios
            _termios.tcsetattr(self._fd, _termios.TCSADRAIN, self._old_settings)
        sys.stdout.write("\033[?25h")
        sys.stdout.flush()

    def clear(self):
        self._buffer = [" " * self.w for _ in range(self.h)]
        sys.stdout.write("\033[?25l")
        sys.stdout.flush()

    def refresh(self):
        self._resize()
        sys.stdout.write("\033[H")
        for i, line in enumerate(self._buffer):
            if i < self.h:
                sys.stdout.write(line[:self.w])
                if i < self.h - 1:
                    sys.stdout.write("\n")
        sys.stdout.flush()

    def addstr(self, row, col, text, attr=0):
        # type: (int, int, str, int) -> None
        if row < 0 or row >= self.h or col >= self.w:
            return
        while len(self._buffer) <= row:
            self._buffer.append(" " * self.w)
        line = self._buffer[row]
        max_len = self.w - col
        t = text[:max_len]
        prefix = ""
        suffix = "\033[0m"
        if attr & 0x1:
            prefix += "\033[1m"
        if attr & 0x2:
            prefix += "\033[7m"
        if attr & 0x100:
            prefix += "\033[36m"
        elif attr & 0x200:
            prefix += "\033[33m"
        elif attr & 0x400:
            prefix += "\033[32m"
        elif attr & 0x800:
            prefix += "\033[31m"
        padded = line[:col] + prefix + t + suffix + line[col + len(t):]
        self._buffer[row] = padded

    def fill_row(self, row, char=" ", attr=0):
        # type: (int, str, int) -> None
        self.addstr(row, 0, char * self.w, attr)

    def get_key(self):
        # type: () -> str
        if not getattr(self, '_raw', False):
            try:
                ch = input()
                if not ch:
                    return K_ENTER
                return ch[0]
            except (EOFError, KeyboardInterrupt):
                return K_QUIT
        ch = sys.stdin.read(1)
        if not ch:
            return ""
        if ch == "\x1b":
            ch2 = sys.stdin.read(1) if self._raw else ""
            if ch2 == "[":
                ch3 = sys.stdin.read(1) if self._raw else ""
                if ch3 == "A":
                    return K_UP
                if ch3 == "B":
                    return K_DOWN
                if ch3 == "H":
                    return K_HOME
                if ch3 == "F":
                    return K_END
                return ""
            return K_ESCAPE
        if ch in ("\n", "\r"):
            return K_ENTER
        if ch in ("\x7f", "\x08"):
            return K_BACKSPACE
        if ch == "\t":
            return K_TAB
        if ch == "\x03":
            return K_QUIT
        return ch

    def color(self, pair_id):
        # type: (int) -> int
        mapping = {
            _CP_TITLE: 0x100 | 0x1,
            _CP_BRANCH_FEATURE: 0x100,
            _CP_BRANCH_BUGFIX: 0x200,
            _CP_BRANCH_RELEASE: 0x400,
            _CP_BRANCH_HOTFIX: 0x800,
            _CP_STATUS: 0x2,
            _CP_SELECTED: 0x2 | 0x100,
            _CP_DIM: 0,
            _CP_ERROR: 0x800,
            _CP_RECOMMENDED: 0x400,
            _CP_DIVERGENCE: 0x800,
            _CP_SECTION: 0x100,
        }
        return mapping.get(pair_id, 0)

    def bold(self):
        return 0x1

    def dim(self):
        return 0

    def reverse(self):
        return 0x2


# ── TUI panels ──────────────────────────────────────────────────────────────

def _branch_color_pair(btype):
    # type: (str) -> int
    return {
        "feature": _CP_BRANCH_FEATURE,
        "bugfix": _CP_BRANCH_BUGFIX,
        "release": _CP_BRANCH_RELEASE,
        "hotfix": _CP_BRANCH_HOTFIX,
    }.get(btype, _CP_DIM)


def _render_title_bar(be, state, cfg):
    """Draw the title bar at row 0."""
    be.fill_row(0, attr=be.color(_CP_TITLE) | be.bold())
    pname = PROJECT_ROOT.name
    btype = branch_type_of(state.current)
    tag_display = state.last_tag if state.last_tag != "none" else ""

    left = " {} ".format(pname)
    be.addstr(0, 0, left, be.color(_CP_TITLE) | be.bold())

    sep = " \u2502 "
    col = len(left)

    be.addstr(0, col, sep, be.color(_CP_TITLE))
    col += len(sep)

    branch_label = " {} ".format(state.current)
    be.addstr(0, col, branch_label, be.color(_branch_color_pair(btype)) | be.bold())
    col += len(branch_label)

    be.addstr(0, col, sep, be.color(_CP_TITLE))
    col += len(sep)

    ver_label = "v{}".format(state.version)
    be.addstr(0, col, ver_label, be.color(_CP_TITLE))
    col += len(ver_label)

    if tag_display:
        be.addstr(0, col, sep, be.color(_CP_TITLE))
        col += len(sep)
        be.addstr(0, col, tag_display, be.color(_CP_TITLE))
        col += len(tag_display)

    if state.dirty:
        be.addstr(0, col, sep, be.color(_CP_TITLE))
        col += len(sep)
        be.addstr(0, col, "\u25cf dirty", be.color(_CP_ERROR) | be.bold())
        col += 7

    if state.git_flow_initialized:
        right = " git-flow "
        be.addstr(0, max(0, be.w - len(right)), right, be.color(_CP_TITLE))


def _render_status_bar(be, hint_text):
    """Draw the status bar at the last row."""
    row = be.h - 1
    be.fill_row(row, attr=be.color(_CP_STATUS))
    be.addstr(row, 1, hint_text, be.color(_CP_STATUS))


def _render_dashboard_lines(state, cfg, width):
    # type: (RepoState, FlowConfig, int) -> list
    """Build dashboard content as a list of (text, color_pair_id) tuples."""
    lines = []  # type: list

    # Divergence warnings
    if state.main_ahead_of_develop > 0:
        lines.append(("", _CP_NORMAL))
        lines.append(
            (" \u26a0  BRANCH DIVERGENCE: {} has {} commit(s) not in {}".format(
                cfg.main_branch, state.main_ahead_of_develop, cfg.develop_branch),
             _CP_DIVERGENCE))
        lines.append(
            ("    Run backmerge to restore the gitflow invariant.", _CP_DIM))
        if state.main_only_files:
            lines.append(("", _CP_NORMAL))
            lines.append(("    Files in {} missing from {}:".format(
                cfg.main_branch, cfg.develop_branch), _CP_DIM))
            for f in state.main_only_files[:8]:
                lines.append(("      ! {}".format(f), _CP_ERROR))
            if len(state.main_only_files) > 8:
                lines.append(("      ... and {} more".format(
                    len(state.main_only_files) - 8), _CP_DIM))

    # Merge conflict
    ms = state.merge
    if ms.in_merge:
        n = len(ms.conflicted_files)
        lines.append(("", _CP_NORMAL))
        lines.append(
            (" \u26a0  MERGE CONFLICT \u2014 {} file(s) need resolution".format(n),
             _CP_ERROR))
        if ms.operation_type:
            lines.append(
                ("    During: git flow {} finish v{}".format(
                    ms.operation_type, ms.operation_version), _CP_DIM))
        for f in ms.conflicted_files[:8]:
            lines.append(("      \u2717 {}".format(f), _CP_ERROR))

    # Develop ahead of main
    if state.develop_ahead_of_main > 0 and state.main_ahead_of_develop == 0:
        lines.append(("", _CP_NORMAL))
        lines.append(
            (" {} is {} commit(s) ahead of {}".format(
                cfg.develop_branch, state.develop_ahead_of_main, cfg.main_branch),
             _CP_BRANCH_FEATURE))
        if state.develop_only_files:
            lines.append(("    Unreleased files:", _CP_DIM))
            for f in state.develop_only_files[:6]:
                lines.append(("      + {}".format(f), _CP_BRANCH_FEATURE))
            if len(state.develop_only_files) > 6:
                lines.append(("      ... and {} more".format(
                    len(state.develop_only_files) - 6), _CP_DIM))

    # In-flight branches
    branch_groups = [
        ("Features", state.features, _CP_BRANCH_FEATURE),
        ("Bugfixes", state.bugfixes, _CP_BRANCH_BUGFIX),
        ("Releases", state.releases, _CP_BRANCH_RELEASE),
        ("Hotfixes", state.hotfixes, _CP_BRANCH_HOTFIX),
    ]
    any_inflight = False
    for label, branches, cp in branch_groups:
        if branches:
            any_inflight = True
            lines.append(("", _CP_NORMAL))
            lines.append((" {} in flight:".format(label), _CP_SECTION))
            for b in branches:
                remote_tag = " (pushed)" if b.has_remote else ""
                commits = "{} commit(s)".format(b.commits_ahead) if b.commits_ahead else "no commits"
                lines.append(("   \u25cf {} \u2014 {}{}".format(
                    b.name, commits, remote_tag), cp))

    if not any_inflight and not ms.in_merge:
        lines.append(("", _CP_NORMAL))
        lines.append((" No feature, bugfix, release, or hotfix branches in flight.", _CP_DIM))

    # Phase analysis
    lines.append(("", _CP_NORMAL))
    lines.append(("\u2500" * min(width - 2, 55), _CP_DIM))
    lines.append((" Phase analysis:", _CP_SECTION))
    lines.append(("", _CP_NORMAL))

    btype = branch_type_of(state.current)
    if ms.in_merge and ms.conflicted_files:
        if ms.operation_type:
            lines.append(("   Merge conflict during {} finish v{}.".format(
                ms.operation_type, ms.operation_version), _CP_ERROR))
            lines.append(("   Accept incoming changes or resolve manually.", _CP_DIM))
        else:
            lines.append(("   Merge conflict with {} file(s).".format(
                len(ms.conflicted_files)), _CP_ERROR))
    elif btype == "feature":
        name = _removeprefix(state.current, "feature/")
        bi = next((b for b in state.features if b.short_name == name), None)
        commits = bi.commits_ahead if bi else 0
        lines.append(("   Feature: {}".format(name), _CP_BRANCH_FEATURE))
        if commits == 0:
            lines.append(("   No new commits yet.", _CP_DIM))
        else:
            lines.append(("   {} commit(s) ready to merge.".format(commits), _CP_RECOMMENDED))
    elif btype == "bugfix":
        name = _removeprefix(state.current, "bugfix/")
        bi = next((b for b in state.bugfixes if b.short_name == name), None)
        commits = bi.commits_ahead if bi else 0
        lines.append(("   Bugfix: {}".format(name), _CP_BRANCH_BUGFIX))
        if commits > 0:
            lines.append(("   {} commit(s) ready.".format(commits), _CP_RECOMMENDED))
    elif btype == "release":
        ver = _removeprefix(_removeprefix(state.current, "release/v"), "release/")
        lines.append(("   Release v{} \u2014 stabilization phase.".format(ver), _CP_BRANCH_RELEASE))
        if state.dirty:
            lines.append(("   Uncommitted changes. Commit before finishing.", _CP_BRANCH_BUGFIX))
        else:
            lines.append(("   Ready to finish. This tags and merges to main + develop.", _CP_RECOMMENDED))
    elif btype == "hotfix":
        ver = _removeprefix(_removeprefix(state.current, "hotfix/v"), "hotfix/")
        lines.append(("   Hotfix v{} \u2014 urgent production fix.".format(ver), _CP_BRANCH_HOTFIX))
        if state.dirty:
            lines.append(("   Uncommitted changes. Commit your fix first.", _CP_BRANCH_BUGFIX))
        else:
            lines.append(("   Ready to finish.", _CP_RECOMMENDED))
    elif state.current == cfg.develop_branch:
        lines.append(("   Integration branch (develop).", _CP_BRANCH_FEATURE))
        if state.main_ahead_of_develop > 0:
            lines.append(("   CRITICAL: backmerge required before any work.", _CP_ERROR))
        elif state.develop_ahead_of_main > 0:
            lines.append(("   {} unreleased commit(s). Consider a release.".format(
                state.develop_ahead_of_main), _CP_RECOMMENDED))
        else:
            lines.append(("   Up to date with {}.".format(cfg.main_branch), _CP_DIM))
    elif state.current == cfg.main_branch:
        lines.append(("   Production branch (main). Do not commit directly.", _CP_BRANCH_HOTFIX))
        lines.append(("   Switch to develop to start work.", _CP_DIM))
    else:
        lines.append(("   Branch '{}' is not a standard gitflow branch.".format(
            state.current), _CP_DIM))

    # Warnings
    if state.releases and btype != "release" and not ms.in_merge:
        lines.append(("", _CP_NORMAL))
        lines.append((" \u26a0  Release '{}' is open. Finish it first.".format(
            state.releases[0].name), _CP_BRANCH_BUGFIX))
    if state.hotfixes and btype != "hotfix" and not ms.in_merge:
        lines.append((" \u26a0  Hotfix '{}' is open. Finish it quickly.".format(
            state.hotfixes[0].name), _CP_BRANCH_HOTFIX))

    return lines


def _render_action_item(be, row, col, action_text, selected, recommended, max_width):
    """Render a single action menu item."""
    plain = _strip_ansi(action_text)
    display = plain[:max(0, max_width - 4)]

    if selected:
        indicator = " \u25b8 "
        be.addstr(row, col, indicator, be.color(_CP_SELECTED) | be.bold())
        be.addstr(row, col + 3, display, be.color(_CP_SELECTED) | be.bold())
        remaining = max_width - 3 - len(display)
        if remaining > 0:
            be.addstr(row, col + 3 + len(display), " " * remaining, be.color(_CP_SELECTED))
    else:
        indicator = "   "
        be.addstr(row, col, indicator, 0)
        if recommended:
            be.addstr(row, col + 3, display, be.color(_CP_RECOMMENDED))
            tag = " \u2190 recommended"
            space = max_width - 3 - len(display)
            if space > len(tag):
                be.addstr(row, col + 3 + len(display), tag, be.color(_CP_DIM))
        else:
            be.addstr(row, col + 3, display, 0)


# ── Overlay rendering ───────────────────────────────────────────────────────

def _draw_overlay_box(be, title, lines, width, height):
    """Draw a centered overlay box. Returns (top_row, left_col)."""
    box_w = min(width, be.w - 4)
    box_h = min(height, be.h - 4)
    top = max(0, (be.h - box_h) // 2)
    left = max(0, (be.w - box_w) // 2)

    border_h = "\u2500"
    border_v = "\u2502"
    corner_tl = "\u250c"
    corner_tr = "\u2510"
    corner_bl = "\u2514"
    corner_br = "\u2518"

    # Top border
    top_border = corner_tl + border_h * (box_w - 2) + corner_tr
    be.addstr(top, left, top_border, be.color(_CP_SECTION))

    # Title
    if title:
        t = " {} ".format(title[:box_w - 6])
        be.addstr(top, left + 2, t, be.color(_CP_SECTION) | be.bold())

    # Content rows
    for i, line in enumerate(lines[:box_h - 2]):
        row = top + 1 + i
        content = " " + line[:box_w - 4] + " "
        content = content.ljust(box_w - 2)
        be.addstr(row, left, border_v, be.color(_CP_SECTION))
        be.addstr(row, left + 1, content, 0)
        be.addstr(row, left + box_w - 1, border_v, be.color(_CP_SECTION))

    # Fill remaining rows
    for i in range(len(lines), box_h - 2):
        row = top + 1 + i
        be.addstr(row, left, border_v, be.color(_CP_SECTION))
        be.addstr(row, left + 1, " " * (box_w - 2), 0)
        be.addstr(row, left + box_w - 1, border_v, be.color(_CP_SECTION))

    # Bottom border
    bot_border = corner_bl + border_h * (box_w - 2) + corner_br
    be.addstr(top + box_h - 1, left, bot_border, be.color(_CP_SECTION))

    return top, left, box_w, box_h


def _overlay_confirm(be, prompt, default_yes=True):
    # type: (...) -> bool
    """Show a centered confirmation dialog. Returns True/False."""
    hint = "[Y/n]" if default_yes else "[y/N]"
    lines = [
        "",
        prompt,
        "",
        hint,
        "",
    ]
    width = max(len(prompt) + 8, 40)
    _draw_overlay_box(be, "Confirm", lines, width, len(lines) + 2)
    be.refresh()

    while True:
        k = be.get_key()
        if k in ("y", "Y"):
            return True
        if k in ("n", "N"):
            return False
        if k == K_ENTER:
            return default_yes
        if k == K_ESCAPE:
            return not default_yes


def _overlay_input(be, prompt, default=""):
    # type: (...) -> str
    """Show a centered input dialog. Returns user text or empty string."""
    value = default
    hint = "Enter: confirm  Esc: cancel"

    while True:
        display_val = value if value else "(empty)"
        lines = [
            "",
            prompt,
            "",
            "> {}".format(display_val),
            "",
            hint,
            "",
        ]
        width = max(len(prompt) + 8, 50)
        _draw_overlay_box(be, "Input", lines, width, len(lines) + 2)
        be.refresh()

        k = be.get_key()
        if k == K_ENTER:
            return value if value else default
        if k == K_ESCAPE:
            return ""
        if k == K_BACKSPACE:
            value = value[:-1]
        elif len(k) == 1 and k.isprintable():
            value += k


def _overlay_help(be):
    """Show the help overlay with all keybindings."""
    lines = [
        "",
        "  Navigation",
        "  \u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500",
        "  j / \u2193        Move down",
        "  k / \u2191        Move up",
        "  g / Home     First item",
        "  G / End      Last item",
        "  Enter        Execute selected action",
        "",
        "  Commands",
        "  \u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500",
        "  /            Search / filter actions",
        "  r            Refresh dashboard",
        "  ?            Toggle this help",
        "  q / Ctrl+C   Quit",
        "",
    ]
    _draw_overlay_box(be, "Help", lines, 48, len(lines) + 2)
    be.refresh()
    while True:
        k = be.get_key()
        if k in (K_HELP, K_ESCAPE, K_ENTER, K_QUIT):
            return


def _overlay_command_palette(be, actions):
    # type: (...) -> int
    """Show a filterable command palette. Returns action index or -1."""
    query = ""
    selected = 0

    while True:
        filtered = []  # type: list
        for i, a in enumerate(actions):
            plain = _strip_ansi(a.label)
            if query.lower() in plain.lower():
                filtered.append((i, a))

        if selected >= len(filtered):
            selected = max(0, len(filtered) - 1)

        lines = [" > {}".format(query if query else "type to filter...")]
        lines.append("")
        for fi, (orig_idx, a) in enumerate(filtered):
            plain = _strip_ansi(a.label)
            marker = "\u25b8 " if fi == selected else "  "
            rec = " \u2190" if a.recommended else ""
            lines.append(" {}{}{}".format(marker, plain, rec))

        width = max(55, be.w // 2)
        height = min(len(lines) + 3, be.h - 4)
        _draw_overlay_box(be, "Command Palette (/)", lines[:height - 2], width, height)
        be.refresh()

        k = be.get_key()
        if k == K_ESCAPE or (k == K_SEARCH and query == ""):
            return -1
        if k == K_UP:
            selected = max(0, selected - 1)
        elif k == K_DOWN:
            selected = min(len(filtered) - 1, selected + 1)
        elif k == K_ENTER:
            if filtered:
                return filtered[selected][0]
            return -1
        elif k == K_BACKSPACE:
            query = query[:-1]
            selected = 0
        elif len(k) == 1 and k.isprintable():
            query += k
            selected = 0


def _overlay_diff_viewer(be, cfg, source, target):
    """Show a scrollable diff viewer overlay."""
    files = run_lines("git diff --name-only {}...{}".format(target, source))
    if not files:
        lines = ["  No differences between {} and {}.".format(source, target)]
    else:
        stat_output = run_quiet("git diff --stat {}...{}".format(target, source))
        diff_output = run_quiet("git diff {}...{}".format(target, source))
        lines = ["  {} file(s) changed in {} vs {}".format(len(files), source, target)]
        lines.append("")
        if stat_output:
            for l in stat_output.splitlines():
                lines.append("  " + l)
        lines.append("")
        if diff_output:
            for l in diff_output.splitlines():
                lines.append("  " + l)

    scroll = 0
    while True:
        view_h = be.h - 6
        view_w = be.w - 6
        visible = lines[scroll:scroll + view_h]
        truncated = [l[:view_w - 2] for l in visible]

        title = "Diff: {} vs {}".format(source, target)
        hint = " [j/k] scroll  [q/Esc] close"
        _draw_overlay_box(be, title, truncated, view_w + 4, view_h + 2)
        be.addstr(be.h - 3, (be.w - len(hint)) // 2, hint, be.color(_CP_DIM))
        be.refresh()

        k = be.get_key()
        if k in (K_QUIT, K_ESCAPE):
            return
        if k == K_DOWN or k == "j":
            if scroll < max(0, len(lines) - view_h):
                scroll += 1
        elif k == K_UP or k == "k":
            scroll = max(0, scroll - 1)
        elif k == K_HOME or k == "g":
            scroll = 0
        elif k in (K_END, "G"):
            scroll = max(0, len(lines) - view_h)
        elif k == " ":
            scroll = min(scroll + view_h, max(0, len(lines) - view_h))


# ── TUI application ─────────────────────────────────────────────────────────

_TUI_ACTIVE = False


def _tui_pick(title, options):
    """Replacement for pick() when TUI is NOT active (subprocess actions)."""
    print("\n  {}{}{}\n".format(C_BOLD, title, C_RESET))
    for i, opt in enumerate(options, 1):
        print("    {}{}){}  {}".format(C_CYAN, i, C_RESET, opt))
    while True:
        try:
            choice = input("\n  Choose [1-{}]: ".format(len(options))).strip()
        except (EOFError, KeyboardInterrupt):
            return 0
        if choice.isdigit() and 1 <= int(choice) <= len(options):
            return int(choice) - 1
        print("  Invalid choice, try again.")


def _tui_ask(prompt, default=""):
    """Replacement for ask() when TUI is NOT active."""
    suffix = " [{}]".format(default) if default else ""
    try:
        answer = input("\n  {}{}: ".format(prompt, suffix)).strip()
    except (EOFError, KeyboardInterrupt):
        return default
    return answer or default


def _tui_confirm(prompt, default_yes=True):
    """Replacement for confirm() when TUI is NOT active."""
    hint = "Y/n" if default_yes else "y/N"
    try:
        answer = input("\n  {} ({}): ".format(prompt, hint)).strip().lower()
    except (EOFError, KeyboardInterrupt):
        return default_yes
    if not answer:
        return default_yes
    return answer in ("y", "yes")


class TUIApp(object):
    """Full-screen TUI application inspired by OpenCode."""

    def __init__(self, cfg, backend):
        # type: (FlowConfig, Any) -> None
        self.cfg = cfg
        self.be = backend
        self.state = None  # type: Optional[RepoState]
        self.actions = []  # type: list
        self.selected = 0
        self.scroll = 0
        self.status_msg = ""
        self.running = True
        self._action_result_lines = []  # type: list

    def _detect(self):
        self.state = detect_state(self.cfg)
        self.actions = build_actions(self.state, self.cfg)
        self.selected = 0
        self.scroll = 0
        # Auto-select first recommended
        for i, a in enumerate(self.actions):
            if a.recommended:
                self.selected = i
                break

    def _render(self):
        be = self.be
        be.clear()

        if self.state is None:
            be.addstr(be.h // 2, be.w // 2 - 6, "Loading...", be.bold())
            be.refresh()
            return

        # Title bar (row 0)
        _render_title_bar(be, self.state, self.cfg)

        # Dashboard lines (rows 2..n) + action menu
        dash_lines = _render_dashboard_lines(self.state, self.cfg, be.w)
        action_start = len(dash_lines) + 2
        total_content = action_start + len(self.actions)

        content_area_h = be.h - 3  # rows 2..h-2

        # Auto-scroll to keep selected action visible
        sel_row = action_start + self.selected
        if sel_row - self.scroll >= content_area_h:
            self.scroll = sel_row - content_area_h + 1
        if sel_row - self.scroll < 0:
            self.scroll = sel_row

        # Render dashboard lines
        for i, (text, cp) in enumerate(dash_lines):
            row = 2 + i - self.scroll
            if 2 <= row < be.h - 1:
                be.addstr(row, 1, text[:be.w - 2], be.color(cp))

        # Separator + "Actions:" header
        sep_row = 2 + len(dash_lines) - self.scroll
        if 2 <= sep_row < be.h - 1:
            be.addstr(sep_row, 1, "", 0)

        header_row = 2 + len(dash_lines) + 1 - self.scroll
        if 2 <= header_row < be.h - 1:
            be.addstr(header_row, 1, " Actions:", be.color(_CP_SECTION) | be.bold())

        # Render action items
        for i, a in enumerate(self.actions):
            row = 2 + action_start + 1 + i - self.scroll
            if 2 <= row < be.h - 1:
                plain = _strip_ansi(a.label)
                _render_action_item(
                    be, row, 1, plain,
                    selected=(i == self.selected),
                    recommended=a.recommended,
                    max_width=be.w - 3,
                )

        # Scrollbar indicator
        if total_content + 2 > content_area_h:
            bar_h = max(1, content_area_h * content_area_h // max(1, total_content + 2))
            bar_pos = 2 + (self.scroll * content_area_h // max(1, total_content + 2))
            for r in range(bar_pos, min(bar_pos + bar_h, be.h - 1)):
                be.addstr(r, be.w - 1, "\u2588", be.color(_CP_DIM))

        # Status bar
        hint = " [j/k] move  [Enter] select  [/] search  [?] help  [r] refresh  [q] quit"
        if self.status_msg:
            hint = " " + self.status_msg + "  |" + hint
        _render_status_bar(be, hint[:be.w - 2])

        be.refresh()

    def _execute_action(self, action):
        """Execute an action, temporarily yielding the screen."""
        if action.fn is None:
            self.running = False
            return

        be = self.be

        # Temporarily leave curses mode for the subprocess
        if _HAS_CURSES and hasattr(be, 'scr'):
            curses.def_prog_mode()
            curses.endwin()

        global _TUI_ACTIVE
        _TUI_ACTIVE = False
        print("")
        try:
            action.fn()
        except subprocess.CalledProcessError as e:
            print("\n  {}Command failed (exit {}).{} Check the output above.".format(
                C_RED, e.returncode, C_RESET))
        except KeyboardInterrupt:
            print("\n  Interrupted.")

        print("\n  {}Press Enter to return to the dashboard...{}".format(C_DIM, C_RESET))
        try:
            input()
        except (EOFError, KeyboardInterrupt):
            pass

        _TUI_ACTIVE = True

        # Re-enter curses mode
        if _HAS_CURSES and hasattr(be, 'scr'):
            curses.reset_prog_mode()
            be.scr.refresh()
            be._resize()

    def _handle_key(self, key):
        # type: (str) -> None
        if key == K_QUIT or key == "\x03":
            self.running = False
        elif key == K_UP:
            self.selected = max(0, self.selected - 1)
        elif key == K_DOWN:
            self.selected = min(len(self.actions) - 1, self.selected + 1)
        elif key == K_HOME:
            self.selected = 0
            self.scroll = 0
        elif key == K_END or key == "G":
            self.selected = len(self.actions) - 1
        elif key == K_ENTER:
            if self.actions:
                self._execute_action(self.actions[self.selected])
                self._detect()
        elif key == K_SEARCH:
            idx = _overlay_command_palette(self.be, self.actions)
            if idx >= 0:
                self._execute_action(self.actions[idx])
                self._detect()
        elif key == K_HELP:
            _overlay_help(self.be)
        elif key == K_REFRESH or key == "r":
            self.status_msg = "Refreshing..."
            self._render()
            self._detect()
            self.status_msg = ""
        elif key == "resize":
            pass

    def run(self):
        global _TUI_ACTIVE
        _TUI_ACTIVE = True
        self._detect()
        while self.running:
            self._render()
            key = self.be.get_key()
            if key:
                self._handle_key(key)
        _TUI_ACTIVE = False


def _run_tui_curses(cfg):
    """Launch TUI with curses backend."""
    def _main(stdscr):
        be = _CursesBackend(stdscr)
        app = TUIApp(cfg, be)
        app.run()
    curses.wrapper(_main)


def _run_tui_ansi(cfg):
    """Launch TUI with ANSI fallback backend."""
    be = _AnsiBackend()
    try:
        app = TUIApp(cfg, be)
        app.run()
    finally:
        be.cleanup()
        sys.stdout.write("\033[2J\033[H\033[?25h")
        sys.stdout.flush()


# ── interactive main loop ─────────────────────────────────────────────────────

def interactive_loop(cfg):
    # type: (FlowConfig) -> None
    os.chdir(PROJECT_ROOT)
    if _HAS_CURSES:
        _run_tui_curses(cfg)
    else:
        _run_tui_ansi(cfg)

# ── argument parser ───────────────────────────────────────────────────────────

def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="gitflow",
        description="Generic Git Flow helper — interactive TUI + CLI subcommands.",
    )
    p.add_argument("--json", action="store_true", help="Machine-readable JSON output (for agents)")
    p.add_argument("--dry-run", action="store_true", help="Preview actions without executing")

    sub = p.add_subparsers(dest="command")

    sub.add_parser("interactive", help="Launch interactive TUI (default)")
    sub.add_parser("status", help="Show repository state")
    sub.add_parser("pull", help="Safe fetch + fast-forward merge (never pushes)")
    sub.add_parser("init", help="Initialize git-flow on this repository")
    sub.add_parser("sync", help="Sync current branch with its parent")
    sub.add_parser("cleanup", help="Delete local branches merged into develop/main")
    sub.add_parser("backmerge", help="Merge main into develop (restore gitflow invariant)")
    sub.add_parser("health", help="Comprehensive repo health check")
    sub.add_parser("undo", help="Undo last gitflow operation (soft reset via reflog)")
    sub.add_parser("doctor", help="Validate prerequisites (git, git-flow, branches)")

    sp_rn = sub.add_parser("releasenotes", help="Generate user-facing release notes from git history")
    sp_rn.add_argument("from_tag", nargs="?", default="", help="Starting tag (default: auto-detect previous tag)")

    sp_log = sub.add_parser("log", help="Gitflow-aware commit log with release boundaries")
    sp_log.add_argument("-n", "--count", type=int, default=20, help="Number of entries (default: 20)")

    sp_switch = sub.add_parser("switch", help="Switch to a gitflow branch")
    sp_switch.add_argument("target", nargs="?", default=None,
                           help="Branch name to switch to (optional — picks interactively if omitted)")

    sp_start = sub.add_parser("start", help="Start a feature/bugfix/release/hotfix")
    sp_start.add_argument("type", choices=["feature", "bugfix", "release", "hotfix"])
    sp_start.add_argument("name", help="Branch name or version number")

    sp_finish = sub.add_parser("finish", help="Finish current or named branch")
    sp_finish.add_argument("name", nargs="?", default=None, help="Branch name (optional if on flow branch)")

    return p


def main() -> None:
    parser = build_parser()
    args = parser.parse_args()

    if args.json:
        set_json_mode(True)

    cfg = load_config()

    if args.command is None or args.command == "interactive":
        if args.json:
            info("  --json is not supported in interactive mode, switching to status.")
            sys.exit(cmd_status(cfg))
        interactive_loop(cfg)
    elif args.command == "status":
        sys.exit(cmd_status(cfg))
    elif args.command == "pull":
        sys.exit(cmd_pull(cfg))
    elif args.command == "init":
        sys.exit(cmd_init(cfg))
    elif args.command == "sync":
        sys.exit(cmd_sync(cfg))
    elif args.command == "cleanup":
        sys.exit(cmd_cleanup(cfg))
    elif args.command == "backmerge":
        sys.exit(cmd_backmerge(cfg))
    elif args.command == "health":
        sys.exit(cmd_health(cfg))
    elif args.command == "undo":
        sys.exit(cmd_undo(cfg))
    elif args.command == "doctor":
        sys.exit(cmd_doctor(cfg))
    elif args.command == "releasenotes":
        sys.exit(cmd_releasenotes(cfg, args.from_tag))
    elif args.command == "log":
        sys.exit(cmd_log(cfg, args.count))
    elif args.command == "switch":
        sys.exit(cmd_switch(cfg, args.target))
    elif args.command == "start":
        sys.exit(cmd_start(cfg, args.type, args.name))
    elif args.command == "finish":
        sys.exit(cmd_finish(cfg, args.name))
    else:
        parser.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()
