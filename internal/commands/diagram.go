package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/state"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newDiagramCmd() *cobra.Command {
	var outputPath string
	var direction string

	cmd := &cobra.Command{
		Use:   "diagram",
		Short: "Render the current gitflow topology as a Mermaid diagram",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := strings.ToUpper(strings.TrimSpace(direction))
			if dir == "" {
				dir = "LR"
			}
			if dir != "LR" && dir != "TB" {
				return fmt.Errorf("invalid direction %q (expected LR or TB)", direction)
			}

			s := GF.Status()
			diagram := buildMermaidDiagram(s, GF.Config.MainBranch, GF.Config.DevelopBranch, dir)

			if outputPath != "" {
				if err := os.WriteFile(outputPath, []byte(diagram+"\n"), 0644); err != nil {
					return fmt.Errorf("write output file: %w", err)
				}
			}

			if output.IsJSONMode() {
				result := map[string]any{
					"action":    "diagram",
					"result":    "ok",
					"format":    "mermaid",
					"direction": dir,
					"diagram":   diagram,
				}
				if outputPath != "" {
					result["file"] = outputPath
				}
				output.JSONOutput(result)
				return nil
			}

			if outputPath != "" {
				output.Infof("  %s✓%s Mermaid diagram written to %s", output.Green, output.Reset, outputPath)
				return nil
			}

			output.Info(diagram)
			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Write Mermaid output to file")
	cmd.Flags().StringVar(&direction, "direction", "LR", "Diagram direction: LR or TB")
	return cmd
}

func buildMermaidDiagram(s state.RepoState, mainBranch, developBranch, direction string) string {
	var b strings.Builder
	b.WriteString("%% gitflow-helper topology diagram\n")
	b.WriteString("flowchart ")
	b.WriteString(direction)
	b.WriteString("\n")

	b.WriteString("classDef main fill:#0f766e,stroke:#134e4a,stroke-width:2px,color:#ffffff;\n")
	b.WriteString("classDef develop fill:#0f4c81,stroke:#0b2f52,stroke-width:2px,color:#ffffff;\n")
	b.WriteString("classDef feature fill:#1f2937,stroke:#0f172a,stroke-width:1.6px,color:#e5e7eb;\n")
	b.WriteString("classDef bugfix fill:#b45309,stroke:#7c2d12,stroke-width:1.6px,color:#fff7ed;\n")
	b.WriteString("classDef release fill:#047857,stroke:#065f46,stroke-width:1.6px,color:#ecfdf5;\n")
	b.WriteString("classDef hotfix fill:#b91c1c,stroke:#7f1d1d,stroke-width:1.6px,color:#fef2f2;\n")
	b.WriteString("classDef warn fill:#78350f,stroke:#451a03,stroke-width:2px,color:#fef3c7;\n")
	b.WriteString("classDef current stroke:#f59e0b,stroke-width:3px;\n")

	b.WriteString("main[")
	b.WriteString(quotedLabel("🛡️ " + mainBranch))
	b.WriteString("]\n")
	b.WriteString("develop[")
	b.WriteString(quotedLabel("🧪 " + developBranch))
	b.WriteString("]\n")
	b.WriteString("main -->|backmerge| develop\n")
	b.WriteString("develop -->|release| main\n")
	b.WriteString("class main main;\n")
	b.WriteString("class develop develop;\n")

	addBranchGroup(&b, "feature", "✨", s.Features, "develop", "feature")
	addBranchGroup(&b, "bugfix", "🔧", s.Bugfixes, "develop", "bugfix")
	addBranchGroup(&b, "release", "📦", s.Releases, "develop", "release")
	addBranchGroup(&b, "hotfix", "🚑", s.Hotfixes, "main", "hotfix")

	if s.Merge.InMerge {
		b.WriteString("merge_conflict[")
		b.WriteString(quotedLabel(fmt.Sprintf("⚠️ merge conflict\\n%d file(s)", len(s.Merge.ConflictedFiles))))
		b.WriteString("]\n")
		b.WriteString("develop -.blocked.-> merge_conflict\n")
		b.WriteString("main -.blocked.-> merge_conflict\n")
		b.WriteString("class merge_conflict warn;\n")
	}

	if s.Current != "" {
		currentID := nodeID(s.Current)
		target := currentID
		if s.Current == mainBranch {
			target = "main"
		} else if s.Current == developBranch {
			target = "develop"
		}
		b.WriteString("class ")
		b.WriteString(target)
		b.WriteString(" current;\n")
	}

	b.WriteString("%% Eye-candy links\n")
	b.WriteString("linkStyle default interpolate basis;\n")
	return b.String()
}

func addBranchGroup(b *strings.Builder, kind, icon string, branches []state.BranchInfo, parentID, className string) {
	if len(branches) == 0 {
		return
	}

	b.WriteString("subgraph ")
	b.WriteString(strings.ToUpper(kind))
	b.WriteString("[")
	b.WriteString(quotedLabel(strings.ToUpper(kind[:1]) + kind[1:] + " lane"))
	b.WriteString("]\n")

	sorted := make([]state.BranchInfo, len(branches))
	copy(sorted, branches)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	for _, br := range sorted {
		id := nodeID(br.Name)
		label := fmt.Sprintf("%s %s\\n+%d commits", icon, br.Name, br.CommitsAhead)
		b.WriteString(id)
		b.WriteString("[")
		b.WriteString(quotedLabel(label))
		b.WriteString("]\n")
		b.WriteString(parentID)
		b.WriteString(" --> ")
		b.WriteString(id)
		b.WriteString("\n")
		if kind == "release" || kind == "hotfix" {
			b.WriteString(id)
			b.WriteString(" --> main\n")
		} else {
			b.WriteString(id)
			b.WriteString(" --> develop\n")
		}
		b.WriteString("class ")
		b.WriteString(id)
		b.WriteString(" ")
		b.WriteString(className)
		b.WriteString(";\n")
	}
	b.WriteString("end\n")
}

func nodeID(name string) string {
	var out strings.Builder
	out.WriteString("n_")
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			continue
		}
		out.WriteRune('_')
	}
	return out.String()
}

func quotedLabel(s string) string {
	return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
}