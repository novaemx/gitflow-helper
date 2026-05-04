package ide

import (
	"os"
	"regexp"
	"strings"
)

const versionHeaderKey = "gitflow-version"

var versionHeaderRegexp = regexp.MustCompile(`gitflow-version:\s*([A-Za-z0-9._+-]+)`)

var currentGeneratorVersion = "dev"

func SetGeneratorVersion(version string) {
	v := strings.TrimSpace(version)
	if v == "" {
		currentGeneratorVersion = "dev"
		return
	}
	currentGeneratorVersion = v
}

func generatorVersion() string {
	v := strings.TrimSpace(currentGeneratorVersion)
	if v == "" {
		return "dev"
	}
	return v
}

func versionHeaderLine() string {
	return "<!-- " + versionHeaderKey + ": " + generatorVersion() + " -->"
}

func hasFrontmatter(content string) bool {
	return strings.HasPrefix(content, "---\n")
}

// parseVersionFromFrontmatter extracts the gitflow_version field from within
// a YAML frontmatter block (--- ... ---). CRLF line endings are normalized.
func parseVersionFromFrontmatter(content string) (string, bool) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !hasFrontmatter(content) {
		return "", false
	}
	rest := content[4:] // skip opening "---\n"
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		return "", false
	}
	for _, line := range strings.Split(rest[:closeIdx], "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "gitflow_version:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "gitflow_version:"))
			val = strings.Trim(val, `"'`)
			if val != "" {
				return val, true
			}
		}
	}
	return "", false
}

// withVersionHeaderFrontmatter injects a gitflow_version field inside the YAML
// frontmatter block as the last field before the closing ---. If the content
// does not start with YAML frontmatter it falls back to withVersionHeader.
// CRLF line endings are normalized to LF.
func withVersionHeaderFrontmatter(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !hasFrontmatter(content) {
		return withVersionHeader(content)
	}
	rest := content[4:] // skip opening "---\n"
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		return withVersionHeader(content)
	}
	lines := strings.Split(rest[:closeIdx], "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "gitflow_version:") {
			filtered = append(filtered, line)
		}
	}
	body := strings.Join(filtered, "\n")
	afterFM := rest[closeIdx+1:] // skip the \n before closing ---
	return "---\n" + body + "\ngitflow_version: \"" + generatorVersion() + "\"\n" + afterFM
}

func hasCurrentVersionHeader(content string) bool {
	var stored string
	var ok bool
	if hasFrontmatter(content) {
		stored, ok = parseVersionFromFrontmatter(content)
	} else {
		stored, ok = parseVersionFromLine(firstLine(content))
	}
	if !ok {
		return false
	}
	return !isOlderVersion(stored, generatorVersion())
}

func withVersionHeader(content string) string {
	if strings.TrimSpace(content) == "" {
		return versionHeaderLine() + "\n"
	}
	first := firstLine(content)
	if _, ok := parseVersionFromLine(first); ok {
		remainder := strings.TrimPrefix(content, first)
		remainder = strings.TrimPrefix(remainder, "\n")
		return versionHeaderLine() + "\n" + remainder
	}
	return versionHeaderLine() + "\n" + content
}

func firstLine(content string) string {
	if idx := strings.IndexByte(content, '\n'); idx >= 0 {
		return content[:idx]
	}
	return content
}

func parseVersionFromLine(line string) (string, bool) {
	m := versionHeaderRegexp.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) != 2 {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

func isOlderVersion(storedVersion, appVersion string) bool {
	stored := strings.TrimSpace(storedVersion)
	running := strings.TrimSpace(appVersion)
	if running == "" {
		return false
	}
	if stored == "" {
		return true
	}

	runningParts, runningOK := parseSemverParts(running)
	storedParts, storedOK := parseSemverParts(stored)
	if !runningOK || !storedOK {
		return stored != running
	}
	for i := 0; i < 3; i++ {
		if storedParts[i] < runningParts[i] {
			return true
		}
		if storedParts[i] > runningParts[i] {
			return false
		}
	}
	return false
}

// fileContentDiffers returns true when the file at path does not exactly match
// expected. Use this for fully-generated files where any content change (body
// or version field) should trigger regeneration.
func fileContentDiffers(path, expected string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	return string(data) != expected
}

// fileMissingHomologationSections reports whether a managed instruction file
// lacks standardized skill-activation/LLM-routing sections.
func fileMissingHomologationSections(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	content := string(data)
	if !strings.Contains(content, "Gitflow Pre-flight Check") && !strings.Contains(content, "Gitflow Enforcement") {
		return false
	}
	if !strings.Contains(content, "Skill Activation (Homologated)") {
		return true
	}
	if !strings.Contains(content, "LLM Activity Routing (Command Selection)") && !strings.Contains(content, "LLM Activity Routing (Compact)") {
		return true
	}
	return false
}

func fileNeedsVersionRefresh(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	content := string(data)
	var stored string
	var ok bool
	if hasFrontmatter(content) {
		stored, ok = parseVersionFromFrontmatter(content)
	} else {
		stored, ok = parseVersionFromLine(firstLine(content))
	}
	if !ok {
		return true
	}
	return isOlderVersion(stored, generatorVersion())
}
