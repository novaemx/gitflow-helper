package flow

import (
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
)

// ── bumpPatch ──────────────────────────────────────────────────────────────

func TestBumpPatch_IncrementsPatch(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1.2.3", "1.2.4"},
		{"0.0.0", "0.0.1"},
		{"10.20.99", "10.20.100"},
	}
	for _, tc := range cases {
		got, err := bumpPatch(tc.in)
		if err != nil {
			t.Fatalf("bumpPatch(%q): unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("bumpPatch(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBumpPatch_InvalidVersion(t *testing.T) {
	cases := []string{"abc", "1.2", "1.2.x", ""}
	for _, in := range cases {
		_, err := bumpPatch(in)
		if err == nil {
			t.Fatalf("bumpPatch(%q): expected error, got nil", in)
		}
	}
}

// ── bumpPatchVersion ───────────────────────────────────────────────────────

func TestBumpPatchVersion_IncrementsPatch(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2.5.10", "2.5.11"},
		{"0.0.0", "0.0.1"},
		{"1.0.9", "1.0.10"},
	}
	for _, tc := range cases {
		got, err := bumpPatchVersion(tc.in)
		if err != nil {
			t.Fatalf("bumpPatchVersion(%q): unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("bumpPatchVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBumpPatchVersion_InvalidVersion(t *testing.T) {
	cases := []string{"bad", "1.2", "1.x.3", ""}
	for _, in := range cases {
		_, err := bumpPatchVersion(in)
		if err == nil {
			t.Fatalf("bumpPatchVersion(%q): expected error, got nil", in)
		}
	}
}

// ── classifyCommit ─────────────────────────────────────────────────────────

func TestClassifyCommit_Maintenance(t *testing.T) {
	cases := []string{
		"chore: update deps",
		"chore(ci): fix lint",
		"ci: add workflow",
		"build: update Makefile",
		"deps: bump golang to 1.22",
		"chore: bump to 2.0.0",
		"feat: bump version to 1.2.3",
	}
	for _, s := range cases {
		got := classifyCommit(s)
		if got != "maintenance" {
			t.Fatalf("classifyCommit(%q) = %q, want maintenance", s, got)
		}
	}
}

func TestClassifyCommit_Features(t *testing.T) {
	cases := []string{
		"feat: add new command",
		"add release notes command",
		"new dashboard component",
		"implement retry logic",
		"introduce activity log",
	}
	for _, s := range cases {
		got := classifyCommit(s)
		if got != "features" {
			t.Fatalf("classifyCommit(%q) = %q, want features", s, got)
		}
	}
}

func TestClassifyCommit_Fixes(t *testing.T) {
	cases := []string{
		"fix: resolve nil pointer",
		"bug: handle empty stash",
		"hotfix: critical issue",
		"patch: edge case in parser",
		"resolve conflict in output",
		"correct typo in error message",
	}
	for _, s := range cases {
		got := classifyCommit(s)
		if got != "fixes" {
			t.Fatalf("classifyCommit(%q) = %q, want fixes", s, got)
		}
	}
}

func TestClassifyCommit_Improvements(t *testing.T) {
	cases := []string{
		"improve error messages",
		"enhance performance",
		"refactor status display",
		"perf: reduce allocations",
		"update output format",
		"upgrade dependencies",
		"optim: reduce memory usage",
	}
	for _, s := range cases {
		got := classifyCommit(s)
		if got != "improvements" {
			t.Fatalf("classifyCommit(%q) = %q, want improvements", s, got)
		}
	}
}

func TestClassifyCommit_Other(t *testing.T) {
	got := classifyCommit("some random commit message")
	if got != "other" {
		t.Fatalf("classifyCommit: expected other, got %q", got)
	}
}

// ── isFlowBranch ───────────────────────────────────────────────────────────

func TestIsFlowBranch_MatchesPrefixes(t *testing.T) {
	prefixes := []string{"feature/", "bugfix/", "release/", "hotfix/"}
	cases := []struct {
		name string
		want bool
	}{
		{"feature/foo", true},
		{"bugfix/bar", true},
		{"release/1.0.0", true},
		{"hotfix/1.0.1", true},
		{"main", false},
		{"develop", false},
		{"feat/foo", false},
		{"", false},
	}
	for _, tc := range cases {
		got := isFlowBranch(tc.name, prefixes)
		if got != tc.want {
			t.Fatalf("isFlowBranch(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// ── extractSemver ──────────────────────────────────────────────────────────

func TestExtractSemver_ExtractsSemver(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"release-1.0.0-rc1", "1.0.0"},
		{"abc", ""},
		{"", ""},
		{"v0.0.0", "0.0.0"},
	}
	for _, tc := range cases {
		got := extractSemver(tc.in)
		if got != tc.want {
			t.Fatalf("extractSemver(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ── latestSemverTag ────────────────────────────────────────────────────────

func TestLatestSemverTag_ReturnsFirstSemverTag(t *testing.T) {
	prev := execLinesStart
	defer func() { execLinesStart = prev }()
	execLinesStart = func(args ...string) []string {
		return []string{"build-123", "v2.0.0", "v1.9.0"}
	}
	got := latestSemverTag()
	if got != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %q", got)
	}
}

func TestLatestSemverTag_EmptyWhenNoSemver(t *testing.T) {
	prev := execLinesStart
	defer func() { execLinesStart = prev }()
	execLinesStart = func(args ...string) []string {
		return []string{"build-123", "no-semver-here"}
	}
	got := latestSemverTag()
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestLatestSemverTag_EmptyList(t *testing.T) {
	prev := execLinesStart
	defer func() { execLinesStart = prev }()
	execLinesStart = func(args ...string) []string { return nil }
	got := latestSemverTag()
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// ── ListSwitchableBranches ─────────────────────────────────────────────────

func TestListSwitchableBranches_ExcludesCurrent(t *testing.T) {
	cfg := config.FlowConfig{MainBranch: "main", DevelopBranch: "develop"}
	all := []string{"main", "develop", "feature/foo", "feature/bar"}
	got := ListSwitchableBranches(all, cfg, "main")
	for _, b := range got {
		if b == "main" {
			t.Fatal("current branch 'main' should not appear in switchable list")
		}
	}
}

func TestListSwitchableBranches_IncludesFlowBranches(t *testing.T) {
	cfg := config.FlowConfig{MainBranch: "main", DevelopBranch: "develop"}
	all := []string{"main", "develop", "feature/foo", "bugfix/bar", "release/1.0.0", "hotfix/1.0.1"}
	got := ListSwitchableBranches(all, cfg, "develop")
	found := map[string]bool{}
	for _, b := range got {
		found[b] = true
	}
	for _, expected := range []string{"main", "feature/foo", "bugfix/bar", "release/1.0.0", "hotfix/1.0.1"} {
		if !found[expected] {
			t.Fatalf("expected %q in switchable branches, got: %v", expected, got)
		}
	}
}

func TestListSwitchableBranches_EmptyAllBranches(t *testing.T) {
	cfg := config.FlowConfig{MainBranch: "main", DevelopBranch: "develop"}
	got := ListSwitchableBranches(nil, cfg, "main")
	if len(got) != 0 {
		t.Fatalf("expected empty list, got %v", got)
	}
}

func TestListSwitchableBranches_OnlyFlowBranch(t *testing.T) {
	cfg := config.FlowConfig{MainBranch: "main", DevelopBranch: "develop"}
	all := []string{"feature/abc"}
	got := ListSwitchableBranches(all, cfg, "develop")
	if len(got) != 1 || got[0] != "feature/abc" {
		t.Fatalf("expected [feature/abc], got %v", got)
	}
}
