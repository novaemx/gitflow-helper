package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
)

type stateStubGitClient struct {
	execResultFn func(args ...string) (int, string, string)
}

func (s stateStubGitClient) Exec(args ...string) error { return nil }
func (s stateStubGitClient) ExecResult(args ...string) (int, string, string) {
	if s.execResultFn != nil {
		return s.execResultFn(args...)
	}
	return 0, "", ""
}
func (s stateStubGitClient) ExecQuiet(args ...string) string {
	_, stdout, _ := s.ExecResult(args...)
	return stdout
}
func (s stateStubGitClient) ExecLines(args ...string) []string {
	_, stdout, _ := s.ExecResult(args...)
	if stdout == "" {
		return nil
	}
	var out []string
	for _, line := range splitLines(stdout) {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s)-1 {
		out = append(out, s[start:])
	}
	return out
}

func TestAtoi(t *testing.T) {
	if atoi("12") != 12 {
		t.Fatal("expected atoi(12)=12")
	}
	if atoi("x") != 0 {
		t.Fatal("expected atoi(x)=0")
	}
}

func TestDefaultRemoteBranch_EmptyRemote(t *testing.T) {
	if defaultRemoteBranch("") != "" {
		t.Fatal("expected empty for empty remote")
	}
}

func TestDefaultRemoteBranch_FromSymbolicRef(t *testing.T) {
	prev := git.DefaultClient()
	defer git.SetDefaultClient(prev)
	git.SetDefaultClient(stateStubGitClient{execResultFn: func(args ...string) (int, string, string) {
		if len(args) >= 3 && args[0] == "symbolic-ref" {
			return 0, "refs/remotes/origin/main", ""
		}
		return 0, "", ""
	}})

	got := defaultRemoteBranch("origin")
	if got != "main" {
		t.Fatalf("expected main, got %q", got)
	}
}

func TestDefaultRemoteBranch_FallbackRemoteShow(t *testing.T) {
	prev := git.DefaultClient()
	defer git.SetDefaultClient(prev)
	git.SetDefaultClient(stateStubGitClient{execResultFn: func(args ...string) (int, string, string) {
		if len(args) >= 4 && args[0] == "symbolic-ref" {
			return 1, "", ""
		}
		if len(args) >= 3 && args[0] == "remote" && args[1] == "show" {
			return 0, "* remote origin\n  HEAD branch: develop\n", ""
		}
		return 0, "", ""
	}})

	got := defaultRemoteBranch("origin")
	if got != "develop" {
		t.Fatalf("expected develop, got %q", got)
	}
}

func TestDetectMergeState_NoMerge(t *testing.T) {
	dir := t.TempDir()
	cfg := config.FlowConfig{ProjectRoot: dir}
	ms := DetectMergeState(cfg)
	if ms.InMerge {
		t.Fatal("expected InMerge=false")
	}
}

func TestDetectMergeState_WithMergeHeadAndReleaseBranch(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), []byte("1234567890abcdef\n"), 0644); err != nil {
		t.Fatalf("write MERGE_HEAD: %v", err)
	}

	prev := git.DefaultClient()
	defer git.SetDefaultClient(prev)
	git.SetDefaultClient(stateStubGitClient{execResultFn: func(args ...string) (int, string, string) {
		if len(args) >= 4 && args[0] == "diff" && args[3] == "--diff-filter=U" {
			return 0, "conflict.txt\n", ""
		}
		if len(args) >= 2 && args[0] == "branch" {
			return 0, "release/1.2.3\nfeature/x\n", ""
		}
		return 0, "", ""
	}})

	cfg := config.FlowConfig{ProjectRoot: dir}
	ms := DetectMergeState(cfg)
	if !ms.InMerge {
		t.Fatal("expected InMerge=true")
	}
	if ms.MergeHead != "1234567890ab" {
		t.Fatalf("expected short merge head, got %q", ms.MergeHead)
	}
	if ms.OperationType != "release" {
		t.Fatalf("expected release operation, got %q", ms.OperationType)
	}
	if ms.OperationVersion != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", ms.OperationVersion)
	}
}

func TestDetectState_Basic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.2.3\n"), 0644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	prev := git.DefaultClient()
	defer git.SetDefaultClient(prev)
	git.SetDefaultClient(stateStubGitClient{execResultFn: func(args ...string) (int, string, string) {
		if len(args) >= 2 && args[0] == "branch" && args[1] == "--show-current" {
			return 0, "feature/abc", ""
		}
		if len(args) >= 2 && args[0] == "describe" {
			return 0, "v1.2.2", ""
		}
		if len(args) >= 2 && args[0] == "status" {
			return 0, "M file.txt\n", ""
		}
		if len(args) >= 1 && args[0] == "remote" && len(args) == 1 {
			return 0, "origin\n", ""
		}
		if len(args) >= 2 && args[0] == "symbolic-ref" {
			return 0, "refs/remotes/origin/main", ""
		}
		if len(args) >= 2 && args[0] == "branch" && args[1] == "--format=%(refname:short)" {
			return 0, "main\ndevelop\nfeature/abc\n", ""
		}
		if len(args) >= 2 && args[0] == "branch" && args[1] == "-r" {
			return 0, "origin/feature/abc\n", ""
		}
		if len(args) >= 3 && args[0] == "rev-list" && args[2] == "main..develop" {
			return 0, "2", ""
		}
		if len(args) >= 3 && args[0] == "rev-list" && args[2] == "develop..main" {
			return 0, "1", ""
		}
		if len(args) >= 3 && args[0] == "diff" && args[2] == "--name-only" {
			return 0, "a.txt\nb.txt\n", ""
		}
		if len(args) >= 3 && args[0] == "rev-list" && args[2] == "develop..feature/abc" {
			return 0, "3", ""
		}
		return 0, "", ""
	}})

	cfg := config.FlowConfig{
		ProjectRoot:   dir,
		MainBranch:    "main",
		DevelopBranch: "develop",
		Remote:        "origin",
		VersionFile:   "VERSION",
	}
	state := DetectState(cfg)
	if state.Current != "feature/abc" {
		t.Fatalf("expected current feature/abc, got %q", state.Current)
	}
	if state.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", state.Version)
	}
	if !state.Dirty || state.UncommittedCount != 1 {
		t.Fatalf("expected dirty with one change, got dirty=%v count=%d", state.Dirty, state.UncommittedCount)
	}
	if !state.HasDefaultRemote || state.DefaultRemoteBranch != "main" {
		t.Fatalf("expected default remote main, got has=%v branch=%q", state.HasDefaultRemote, state.DefaultRemoteBranch)
	}
	if len(state.Features) != 1 || state.Features[0].Name != "feature/abc" {
		t.Fatalf("expected one feature branch, got %+v", state.Features)
	}
	if state.Features[0].CommitsAhead != 3 {
		t.Fatalf("expected feature ahead=3, got %d", state.Features[0].CommitsAhead)
	}
}
