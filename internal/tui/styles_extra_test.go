package tui

import "testing"

func TestBranchStyle_AllTypes(t *testing.T) {
	types := []string{"feature", "bugfix", "release", "hotfix", "other"}
	for _, typ := range types {
		rendered := branchStyle(typ).Render("x")
		if rendered == "" {
			t.Fatalf("expected non-empty rendered style for %s", typ)
		}
	}
}
