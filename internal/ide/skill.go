package ide

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/gitflow_skill.md
var embeddedSkillFiles embed.FS

var UserHomeDirFunc = os.UserHomeDir

var projectScopedSkillIDEs = map[string]bool{
	IDECursor:     true,
	IDEVSCode:     true,
	IDECopilot:    true,
	IDEClaudeCode: true,
	IDEWindsurf:   true,
	IDECline:      true,
	IDEBoth:       true,
}

func projectSkillPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".agents", "skills", "gitflow", "SKILL.md")
}

func userSkillPath() (string, error) {
	home, err := UserHomeDirFunc()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agents", "skills", "gitflow", "SKILL.md"), nil
}

func skillPathForIDE(projectRoot, ideID string) (string, error) {
	if projectRoot != "" && projectScopedSkillIDEs[ideID] {
		return projectSkillPath(projectRoot), nil
	}
	return userSkillPath()
}

func embeddedSkillContent() (string, error) {
	data, err := embeddedSkillFiles.ReadFile("assets/gitflow_skill.md")
	if err != nil {
		return "", err
	}
	content := string(data)
	content = withVersionHeaderFrontmatter(content)
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content, nil
}

func isDevRepo(projectRoot string) bool {
	marker := filepath.Join(projectRoot, "internal", "ide", "assets", "gitflow_skill.md")
	_, err := os.Stat(marker)
	return err == nil
}

func ensureEmbeddedSkill(projectRoot, ideID string) (string, error) {
	path, err := skillPathForIDE(projectRoot, ideID)
	if err != nil {
		return "", err
	}

	// Skip overwriting the skill file when running inside the gitflow-helper
	// development repository.  The dev repo owns its own SKILL.md; the
	// embedded asset must not clobber local edits.
	if projectScopedSkillIDEs[ideID] && isDevRepo(projectRoot) {
		return "", nil
	}

	content, err := embeddedSkillContent()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}

	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		return "", nil
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}
