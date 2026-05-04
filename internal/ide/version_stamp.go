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

func hasCurrentVersionHeader(content string) bool {
	line := firstLine(content)
	stored, ok := parseVersionFromLine(line)
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

func fileNeedsVersionRefresh(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	line := firstLine(string(data))
	stored, ok := parseVersionFromLine(line)
	if !ok {
		return true
	}
	return isOlderVersion(stored, generatorVersion())
}
