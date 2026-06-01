package flow

import (
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
)

// stubTagAndBranchExists replaces the package-level git lookups so the
// "first candidate free" / "skips existing" tests don't need a real repo.
func stubTagAndBranchExists(t *testing.T, existingTags, existingBranches map[string]bool) {
	t.Helper()
	prevTag := tagExistsStart
	prevBranch := branchExistsStart
	tagExistsStart = func(name string) bool { return existingTags[name] }
	branchExistsStart = func(name string) bool { return existingBranches[name] }
	t.Cleanup(func() {
		tagExistsStart = prevTag
		branchExistsStart = prevBranch
	})
}

func TestParentForBranchType(t *testing.T) {
	cfg := config.FlowConfig{MainBranch: "main", DevelopBranch: "develop"}
	cases := []struct {
		btype string
		want  string
	}{
		{"hotfix", "main"},
		{"feature", "develop"},
		{"bugfix", "develop"},
		{"release", "develop"},
		{"unknown", "develop"},
		{"", "develop"},
	}
	for _, c := range cases {
		if got := parentForBranchType(cfg, c.btype); got != c.want {
			t.Errorf("parentForBranchType(%q) = %q, want %q", c.btype, got, c.want)
		}
	}
}

func TestBumpPatchVersion_BumpsZeroPatch(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1.2.3", "1.2.4"},
		{"1.2.0", "1.2.1"},
		{"0.0.9", "0.0.10"},
		{"0.0.0", "0.0.1"},
		{"10.20.30", "10.20.31"},
	}
	for _, c := range cases {
		got, err := bumpPatchVersion(c.in)
		if err != nil {
			t.Errorf("bumpPatchVersion(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("bumpPatchVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBumpPatchVersion_InvalidReturnsError(t *testing.T) {
	invalid := []string{"", "1.2", "1.2.3.4", "v1.2.3", "1.x.3", "a.b.c"}
	for _, in := range invalid {
		if _, err := bumpPatchVersion(in); err == nil {
			t.Errorf("bumpPatchVersion(%q) expected error, got nil", in)
		}
	}
}

func TestNextAvailableStartVersion_FirstCandidateFree(t *testing.T) {
	stubTagAndBranchExists(t, map[string]bool{}, map[string]bool{})
	cfg := config.FlowConfig{TagPrefix: "v"}
	got, skipped, err := nextAvailableStartVersion(cfg, "release", "1.2.3")
	if err != nil {
		t.Fatalf("nextAvailableStartVersion: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("expected first candidate 1.2.3, got %q", got)
	}
	if len(skipped) != 0 {
		t.Errorf("expected no skipped, got %v", skipped)
	}
}

func TestNextAvailableStartVersion_SkipsExistingTag(t *testing.T) {
	// v1.2.3 exists as a tag, so the first free candidate is 1.2.4.
	stubTagAndBranchExists(t, map[string]bool{"v1.2.3": true}, map[string]bool{})
	cfg := config.FlowConfig{TagPrefix: "v"}
	got, skipped, err := nextAvailableStartVersion(cfg, "release", "1.2.3")
	if err != nil {
		t.Fatalf("nextAvailableStartVersion: %v", err)
	}
	if got != "1.2.4" {
		t.Errorf("expected 1.2.4 (skipping existing tag v1.2.3), got %q", got)
	}
	if len(skipped) != 1 {
		t.Errorf("expected 1 skipped entry, got %v", skipped)
	}
}

func TestNextAvailableStartVersion_SkipsExistingBranch(t *testing.T) {
	// release/1.2.3 already exists as a branch, no v1.2.3 tag. Next free: 1.2.4.
	stubTagAndBranchExists(t, map[string]bool{}, map[string]bool{"release/1.2.3": true})
	cfg := config.FlowConfig{TagPrefix: "v"}
	got, _, err := nextAvailableStartVersion(cfg, "release", "1.2.3")
	if err != nil {
		t.Fatalf("nextAvailableStartVersion: %v", err)
	}
	if got != "1.2.4" {
		t.Errorf("expected 1.2.4, got %q", got)
	}
}
