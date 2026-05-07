package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/novaemx/gitflow-helper/internal/state"
)

func TestBuildMermaidDiagram_IncludesCoreLayoutAndStyles(t *testing.T) {
	s := state.RepoState{
		Current: "feature/ui-polish",
		Features: []state.BranchInfo{
			{Name: "feature/ui-polish", CommitsAhead: 3},
		},
		Bugfixes: []state.BranchInfo{
			{Name: "bugfix/merge-crash", CommitsAhead: 1},
		},
		Merge: state.MergeState{InMerge: true, ConflictedFiles: []string{"README.md", "go.mod"}},
	}

	got := buildMermaidDiagram(s, "main", "develop", "LR")

	mustContain := []string{
		"flowchart LR",
		"classDef feature",
		"classDef current",
		"main[\"🛡️ main\"]",
		"develop[\"🧪 develop\"]",
		"Feature lane",
		"Bugfix lane",
		"merge_conflict",
		"class n_feature_ui_polish current;",
	}
	for _, item := range mustContain {
		if !strings.Contains(got, item) {
			t.Fatalf("diagram should contain %q\n%s", item, got)
		}
	}
}

func TestNewDiagramCmd_WritesOutputFile(t *testing.T) {
	dir := setupCommandsRepo(t)
	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", TagPrefix: "v"})

	prevJSON := output.IsJSONMode()
	output.SetJSONMode(false)
	defer output.SetJSONMode(prevJSON)

	outFile := filepath.Join(dir, "branch-diagram.mmd")
	cmd := newDiagramCmd()
	if err := cmd.ParseFlags([]string{"--output", outFile, "--direction", "TB"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("run diagram cmd: %v", err)
	}

	b, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(b), "flowchart TB") {
		t.Fatalf("expected TB flowchart in output, got:\n%s", string(b))
	}
}

func TestNewDiagramCmd_RejectsInvalidDirection(t *testing.T) {
	cmd := newDiagramCmd()
	if err := cmd.ParseFlags([]string{"--direction", "LEFT"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := cmd.RunE(cmd, []string{}); err == nil {
		t.Fatal("expected invalid direction error")
	}
}
