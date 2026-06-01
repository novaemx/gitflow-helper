package ide

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/novaemx/gitflow-helper/internal/debug"
)

// IDE type constants
const (
	IDECursor     = "cursor"
	IDEVSCode     = "vscode"
	IDECopilot    = "copilot" // vscode + copilot
	IDEClaudeCode = "claude-code"
	IDEWindsurf   = "windsurf"
	IDECline      = "cline"
	IDEZed        = "zed"
	IDENeovim     = "neovim"
	IDEJetBrains  = "jetbrains"
	IDEUnknown    = "unknown"
	IDEBoth       = "both" // legacy: cursor + copilot
)

// DetectedIDE holds the result of IDE detection with display-friendly name.
type DetectedIDE struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

var ideRegistry = []struct {
	id      string
	display string
	detect  func(string) bool
}{
	// Check most specific IDE terminals first to avoid VS Code-compatible
	// env vars masking Cursor when both apps are running.
	{IDECursor, "Cursor", detectCursor},
	{IDECopilot, "VS Code + Copilot", detectCopilot},
	{IDEVSCode, "VS Code", detectVSCode},
	{IDEClaudeCode, "Claude Code", detectClaudeCode},
	{IDEWindsurf, "Windsurf", detectWindsurf},
	{IDECline, "Cline", detectCline},
	{IDEZed, "Zed", detectZed},
	{IDENeovim, "Neovim", detectNeovim},
	{IDEJetBrains, "JetBrains", detectJetBrains},
}

// DetectAll returns all detected IDEs (there may be multiple signals).
func DetectAll(projectRoot string) []DetectedIDE {
	var found []DetectedIDE
	for _, entry := range ideRegistry {
		if entry.detect(projectRoot) {
			found = append(found, DetectedIDE{ID: entry.id, DisplayName: entry.display})
		}
	}
	return found
}

// DetectPrimary returns the most specific IDE detected, or "unknown".
func DetectPrimary(projectRoot string) DetectedIDE {
	deferEnd := debug.Start("DetectPrimary.total")
	defer deferEnd()

	for _, entry := range ideRegistry {
		deferEntry := debug.Start(fmt.Sprintf("DetectPrimary.%s", entry.id))
		if entry.detect(projectRoot) {
			deferEntry()
			debug.Printf("IDE detected: %s", entry.id)
			return DetectedIDE{ID: entry.id, DisplayName: entry.display}
		}
		deferEntry()
	}
	debug.Printf("No IDE detected, returning 'Terminal'")
	return DetectedIDE{ID: IDEUnknown, DisplayName: "Terminal"}
}

// ideRuleSpec maps an IDE to its existence-check and generator functions.
type ideRuleSpec struct {
	path     func(string) string
	exists   func(string) bool
	generate func(string) (string, error)
}

// forceRuleWrite is set during a `--force` run. The cursor rule generator
// (the only fully-regenerated rule) checks this var to skip the
// content-equality idempotency check. The non-cursor rules are append-style
// so the var does not affect them; for those, --force is implemented by
// deleting the file before regeneration.
var forceRuleWrite bool

// SetForceRuleWrite toggles the package-level force flag. The setup command
// uses this with a defer to scope the flag to a single invocation.
func SetForceRuleWrite(v bool) { forceRuleWrite = v }

var ideRuleRegistry = map[string]ideRuleSpec{
	IDECursor:     {cursorRulePath, cursorRuleExists, generateCursorRule},
	IDEVSCode:     {copilotPath, copilotRuleExists, generateCopilotInstructions},
	IDECopilot:    {copilotPath, copilotRuleExists, generateCopilotInstructions},
	IDEClaudeCode: {claudeCodePath, claudeCodeRuleExists, generateClaudeCodeRule},
	IDEWindsurf:   {windsurfRulePath, windsurfRuleExists, generateWindsurfRule},
	IDECline:      {clineRulePath, clineRuleExists, generateClineRule},
	IDEZed:        {zedRulePath, zedRuleExists, generateZedRule},
	IDENeovim:     {neovimRulePath, neovimRuleExists, generateNeovimRule},
	IDEJetBrains:  {jetbrainsRulePath, jetbrainsRuleExists, generateJetBrainsRule},
}

// companionDisplayName maps an IDE id to its human-readable display name.
var companionDisplayName = map[string]string{
	IDEClaudeCode: "Claude Code",
}

// companionIDEs returns additional IDEs that should be auto-provisioned
// alongside the primary IDE when they are also detected in the same
// environment. The list is empty when no companion is detected.
//
// The matrix is intentionally narrow: today, only Cursor-family primaries
// (Cursor, VSCode, Copilot, the legacy "both") companion-install Claude
// Code, because real-world usage commonly pairs Cursor with Claude Code.
// Other IDEs (Windsurf, Cline, Zed, ...) live in different ecosystems
// and do not auto-companion-install anything.
func companionIDEs(primary, projectRoot string) []string {
	switch primary {
	case IDECursor, IDEBoth, IDEVSCode, IDECopilot:
		if detectClaudeCode(projectRoot) {
			return []string{IDEClaudeCode}
		}
	}
	return nil
}

// EnsureRulesForIDE checks if rules exist for the detected IDE and any
// detected companion IDEs. For the primary it creates rules if missing,
// ensures the embedded gitflow skill, AGENTS.md (for non-project-skill
// IDEs), and MCP config (where supported). After the primary it
// recursively provisions any IDE returned by companionIDEs() whose
// detectXxx() returns true in the same environment.
//
// Recursion is bounded by a processed-set; a companion whose id equals
// the primary is skipped.
//
// Returns list of newly created files (empty if all exist or on second
// idempotent run).
func EnsureRulesForIDE(projectRoot string, detected DetectedIDE) ([]string, error) {
	processed := map[string]bool{}
	return ensureRulesRecursive(projectRoot, detected, processed)
}

func ensureRulesRecursive(projectRoot string, detected DetectedIDE, processed map[string]bool) ([]string, error) {
	if processed[detected.ID] {
		return nil, nil
	}
	processed[detected.ID] = true

	created, err := ensureRulesForSingleIDE(projectRoot, detected)
	if err != nil {
		return created, err
	}

	for _, companionID := range companionIDEs(detected.ID, projectRoot) {
		if processed[companionID] {
			continue
		}
		display := companionDisplayName[companionID]
		if display == "" {
			display = companionID
		}
		more, err := ensureRulesRecursive(projectRoot,
			DetectedIDE{ID: companionID, DisplayName: display},
			processed)
		if err != nil {
			return append(created, more...), err
		}
		created = append(created, more...)
	}
	return created, nil
}

// ensureRulesForSingleIDE contains the original EnsureRulesForIDE body:
// generates the IDE-specific rule, installs the embedded skill, creates
// AGENTS.md (when not redundant), provisions MCP config, and emits the
// semver section for Cursor/Copilot families. No companion logic.
func ensureRulesForSingleIDE(projectRoot string, detected DetectedIDE) ([]string, error) {
	var created []string

	// Generate IDE-specific rules.
	// Cursor rules are fully-generated: always call generate (idempotent — skips
	// write when content matches). Other IDEs use append-style generation gated
	// on existence + version stamp.
	if spec, ok := ideRuleRegistry[detected.ID]; ok {
		var path string
		var err error
		if detected.ID == IDECursor {
			// Idempotent: writes only when content differs from expected.
			path, err = spec.generate(projectRoot)
		} else {
			rulePath := spec.path(projectRoot)
			if !spec.exists(projectRoot) || fileNeedsVersionRefresh(rulePath) || fileMissingHomologationSections(rulePath) {
				path, err = spec.generate(projectRoot)
			}
		}
		if err != nil {
			return created, err
		}
		if path != "" {
			created = append(created, path)
		}
	}

	if skillPath, err := ensureEmbeddedSkill(projectRoot, detected.ID); err != nil {
		return created, err
	} else if skillPath != "" {
		created = append(created, skillPath)
	}

	// Create AGENTS.md only for IDEs that do not support .agents/ or ~/.agents/
	// (i.e. IDEs not in projectScopedSkillIDEs). For those IDEs the embedded
	// skill already lands in .agents/skills/gitflow/SKILL.md, making AGENTS.md
	// redundant.
	if !projectScopedSkillIDEs[detected.ID] && (!agentsRuleExists(projectRoot) || fileNeedsVersionRefresh(agentsPath(projectRoot)) || fileMissingHomologationSections(agentsPath(projectRoot))) {
		path, err := generateAgentsMD(projectRoot)
		if err != nil {
			return created, err
		}
		if path != "" {
			created = append(created, path)
		}
	}

	// Auto-provision MCP config for IDEs that support it
	if MCPSupportedIDEs[detected.ID] && !MCPConfigExists(projectRoot, detected.ID) {
		path, err := EnsureMCPConfig(projectRoot, detected.ID)
		if err == nil && path != "" {
			created = append(created, path)
		}
	}

	// Provision conventional-commits / semver rule:
	//   Cursor        → .cursor/rules/semver.mdc (fully-generated, idempotent)
	//   VSCode/Copilot → appended section in .github/copilot-instructions.md
	switch detected.ID {
	case IDECursor, IDEBoth:
		// Idempotent: writes only when content differs from expected.
		path, err := generateSemverCursorRule(projectRoot)
		if err != nil {
			return created, err
		}
		if path != "" {
			created = append(created, path)
		}
	case IDEVSCode, IDECopilot:
		if !semverCopilotSectionExists(projectRoot) {
			path, err := generateSemverCopilotSection(projectRoot)
			if err != nil {
				return created, err
			}
			if path != "" {
				created = append(created, path)
			}
		}
	}

	return created, nil
}

// planEnsureRulesForIDE returns the list of file paths that
// EnsureRulesForIDE would create/modify for the detected IDE chain, without
// writing to projectRoot. Implementation: runs EnsureRulesForIDE in a
// throwaway temp dir and returns that file list. Used by `gitflow setup
// --check` for dry-run reports.
func planEnsureRulesForIDE(projectRoot string, detected DetectedIDE) ([]string, error) {
	tmpDir, err := os.MkdirTemp("", "gitflow-check-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	return EnsureRulesForIDE(tmpDir, detected)
}

// RemoveRulesForIDE is the inverse of EnsureRulesForIDE: it deletes the
// IDE-specific rule file, MCP config, project-scoped skill, and (for
// non-project-skill IDEs) the AGENTS.md universal fallback. It mirrors the
// companion-aware recursion so uninstalling the primary also removes any
// companion artifacts (e.g. Cursor uninstall removes Claude Code companion
// files too). Returns the list of paths that were removed.
func RemoveRulesForIDE(projectRoot string, detected DetectedIDE) ([]string, error) {
	processed := map[string]bool{}
	return removeRulesRecursive(projectRoot, detected, processed)
}

func removeRulesRecursive(projectRoot string, detected DetectedIDE, processed map[string]bool) ([]string, error) {
	if processed[detected.ID] {
		return nil, nil
	}
	processed[detected.ID] = true

	var removed []string

	// 1. Remove IDE-specific rule file (e.g. .cursor/rules/gitflow-preflight.mdc)
	if spec, ok := ideRuleRegistry[detected.ID]; ok {
		rulePath := spec.path(projectRoot)
		if _, err := os.Stat(rulePath); err == nil {
			if rmErr := os.Remove(rulePath); rmErr == nil {
				removed = append(removed, rulePath)
			}
		}
	}

	// 2. Remove MCP config (e.g. .cursor/mcp.json)
	if MCPSupportedIDEs[detected.ID] {
		mcpPath := mcpConfigPath(projectRoot, detected.ID)
		if mcpPath != "" {
			if _, err := os.Stat(mcpPath); err == nil {
				if rmErr := os.Remove(mcpPath); rmErr == nil {
					removed = append(removed, mcpPath)
				}
			}
		}
	}

	// 3. Remove project-scoped skill (only for projectScopedSkillIDEs)
	if projectScopedSkillIDEs[detected.ID] {
		skillPath := projectSkillPath(projectRoot)
		if _, err := os.Stat(skillPath); err == nil {
			if rmErr := os.Remove(skillPath); rmErr == nil {
				removed = append(removed, skillPath)
			}
		}
		// Also try to remove the now-empty parent dirs.
		_ = os.Remove(filepath.Dir(skillPath))
		_ = os.Remove(filepath.Dir(filepath.Dir(skillPath)))
	}

	// 4. Remove AGENTS.md universal fallback (for non-project-skill IDEs)
	if !projectScopedSkillIDEs[detected.ID] {
		agentsMD := agentsPath(projectRoot)
		if _, err := os.Stat(agentsMD); err == nil {
			if rmErr := os.Remove(agentsMD); rmErr == nil {
				removed = append(removed, agentsMD)
			}
		}
	}

	// 5. Recurse into companion IDEs (e.g. uninstalling Cursor also removes
	//    Claude Code companion files).
	for _, companionID := range companionIDEs(detected.ID, projectRoot) {
		if processed[companionID] {
			continue
		}
		display := companionDisplayName[companionID]
		if display == "" {
			display = companionID
		}
		more, err := removeRulesRecursive(projectRoot,
			DetectedIDE{ID: companionID, DisplayName: display},
			processed)
		if err != nil {
			return append(removed, more...), err
		}
		removed = append(removed, more...)
	}

	return removed, nil
}

// --- Individual IDE detectors ---

func detectCursor(projectRoot string) bool {
	if hasCursorRuntimeMarkers() {
		return true
	}

	envVars := []string{"CURSOR_TRACE_ID", "CURSOR_SESSION", "CURSOR_CHANNEL"}
	for _, v := range envVars {
		if os.Getenv(v) != "" {
			return true
		}
	}

	// Cursor is a VSCode extension, only check process ancestry if we're in VSCode terminal
	// This avoids expensive process lookups on non-VSCode terminals
	if !isVSCodeTerminal() {
		return false
	}

	return matchParentProcess("cursor")
}

func hasCursorRuntimeMarkers() bool {
	for _, key := range []string{"GIT_ASKPASS", "VSCODE_GIT_ASKPASS_MAIN", "VSCODE_GIT_ASKPASS_NODE", "BUNDLED_DEBUGPY_PATH", "VSCODE_DEBUGPY_ADAPTER_ENDPOINTS", "TERM_PROGRAM_VERSION", "XPC_SERVICE_NAME", "__CFBundleIdentifier"} {
		v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if v == "" {
			continue
		}
		if strings.Contains(v, "cursor") || strings.Contains(v, ".cursor/") || strings.Contains(v, "com.todesktop") {
			return true
		}
	}
	return false
}

func detectVSCode(projectRoot string) bool {
	if hasCursorRuntimeMarkers() {
		return false
	}

	envVars := []string{"VSCODE_GIT_ASKPASS_NODE", "VSCODE_GIT_ASKPASS_MAIN", "VSCODE_IPC_HOOK", "VSCODE_CWD"}
	for _, v := range envVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	if isVSCodeTerminal() {
		return true
	}
	return matchParentProcess("code")
}

func isVSCodeTerminal() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("TERM_PROGRAM")))
	if v == "vscode" {
		return true
	}

	for _, key := range []string{"VSCODE_IPC_HOOK", "VSCODE_CWD", "VSCODE_GIT_ASKPASS_NODE", "VSCODE_GIT_ASKPASS_MAIN"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

func detectCopilot(projectRoot string) bool {
	if !detectVSCode(projectRoot) {
		return false
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GITHUB_COPILOT_") {
			return true
		}
	}
	return false
}

func detectClaudeCode(projectRoot string) bool {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDE_") || strings.HasPrefix(e, "ANTHROPIC_") {
			return true
		}
	}
	return matchParentProcess("claude")
}

func detectWindsurf(projectRoot string) bool {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "WINDSURF_") || strings.HasPrefix(e, "CODEIUM_") {
			return true
		}
	}
	return matchParentProcess("windsurf")
}

func detectCline(projectRoot string) bool {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLINE_") {
			return true
		}
	}
	return matchParentProcess("cline")
}

func detectZed(projectRoot string) bool {
	if os.Getenv("ZED_TERM") != "" {
		return true
	}
	return matchParentProcess("zed")
}

func detectNeovim(projectRoot string) bool {
	if os.Getenv("NVIM") != "" || os.Getenv("NVIM_LISTEN_ADDRESS") != "" {
		return true
	}
	return matchParentProcess("nvim")
}

func detectJetBrains(projectRoot string) bool {
	if isJetBrainsTerminal() {
		return true
	}
	jetbrainsProcesses := []string{"idea", "pycharm", "webstorm", "goland", "clion", "rider", "phpstorm", "rubymine", "datagrip"}
	for _, p := range jetbrainsProcesses {
		if matchParentProcess(p) {
			return true
		}
	}
	return false
}

func isJetBrainsTerminal() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("TERM_PROGRAM")))
	if strings.Contains(v, "jetbrains") || strings.Contains(v, "jediterm") {
		return true
	}

	for _, key := range []string{"IDEA_INITIAL_DIRECTORY", "PYCHARM_HOSTED", "WEBIDE_INITIAL_DIRECTORY"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

// matchParentProcess checks if a process name appears in the parent chain.
// Uses reduced depth (5 instead of 8) for Windows performance.
func matchParentProcess(name string) bool {
	ppid := os.Getppid()
	if ppid <= 1 {
		return false
	}
	// Use maxDepth=5 instead of 8 for better Windows performance
	return matchParentProcessForOS(name, runtime.GOOS, ppid, 5)
}

func matchParentProcessForOS(name, goos string, startPID, maxDepth int) bool {
	ancestry, err := getProcessAncestry(goos, startPID, maxDepth)
	if err != nil {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(name))
	for _, procName := range ancestry {
		if strings.Contains(strings.ToLower(procName), target) {
			return true
		}
	}
	return false
}

var processAncestryCache = struct {
	mu    sync.Mutex
	items map[string]cachedAncestry
}{items: map[string]cachedAncestry{}}

type cachedAncestry struct {
	names     []string
	createdAt time.Time
}

var processAncestryCacheTTL = 5 * time.Second
var processAncestryCacheMaxEntries = 64
var ancestryNowFunc = time.Now

var windowsProcessAncestryFunc = windowsProcessAncestry
var parentProcessInfoFunc = parentProcessInfo

func getProcessAncestry(goos string, startPID, maxDepth int) ([]string, error) {
	if startPID <= 1 || maxDepth <= 0 {
		return nil, fmt.Errorf("invalid ancestry input")
	}
	key := fmt.Sprintf("%s:%d:%d", goos, startPID, maxDepth)
	now := ancestryNowFunc()

	processAncestryCache.mu.Lock()
	if cached, ok := processAncestryCache.items[key]; ok {
		if now.Sub(cached.createdAt) <= processAncestryCacheTTL {
			processAncestryCache.mu.Unlock()
			return cached.names, nil
		}
		delete(processAncestryCache.items, key)
	}
	processAncestryCache.mu.Unlock()

	var names []string
	var err error
	if goos == "windows" {
		names, err = windowsProcessAncestryFunc(startPID, maxDepth)
	} else {
		names, err = genericProcessAncestry(goos, startPID, maxDepth)
	}
	if err != nil {
		return nil, err
	}

	processAncestryCache.mu.Lock()
	if len(processAncestryCache.items) >= processAncestryCacheMaxEntries {
		evictOldestAncestryEntry(processAncestryCache.items)
	}
	processAncestryCache.items[key] = cachedAncestry{names: names, createdAt: now}
	processAncestryCache.mu.Unlock()
	return names, nil
}

func evictOldestAncestryEntry(items map[string]cachedAncestry) {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range items {
		if first || v.createdAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.createdAt
			first = false
		}
	}
	if !first {
		delete(items, oldestKey)
	}
}

func genericProcessAncestry(goos string, startPID, maxDepth int) ([]string, error) {
	var names []string
	pid := startPID
	for depth := 0; depth < maxDepth && pid > 1; depth++ {
		procName, ppid, err := parentProcessInfoFunc(pid, goos)
		if err != nil {
			return nil, err
		}
		names = append(names, procName)
		if ppid <= 1 || ppid == pid {
			break
		}
		pid = ppid
	}
	return names, nil
}

func matchProcessInAncestry(name string, startPID, maxDepth int, fetch func(int) (string, int, error)) bool {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" || startPID <= 1 || maxDepth <= 0 {
		return false
	}

	pid := startPID
	for depth := 0; depth < maxDepth && pid > 1; depth++ {
		procName, ppid, err := fetch(pid)
		if err != nil {
			return false
		}
		if strings.Contains(strings.ToLower(procName), target) {
			return true
		}
		if ppid <= 1 || ppid == pid {
			break
		}
		pid = ppid
	}
	return false
}

func parentProcessInfo(pid int, goos string) (string, int, error) {
	switch goos {
	case "linux":
		return linuxProcessInfo(pid)
	case "darwin":
		return darwinProcessInfo(pid)
	case "windows":
		return windowsProcessInfo(pid)
	default:
		return "", 0, fmt.Errorf("unsupported os: %s", goos)
	}
}

func linuxProcessInfo(pid int) (string, int, error) {
	commBytes, err := os.ReadFile(filepath.Join("/proc", fmt.Sprintf("%d", pid), "comm"))
	if err != nil {
		return "", 0, err
	}
	statusBytes, err := os.ReadFile(filepath.Join("/proc", fmt.Sprintf("%d", pid), "status"))
	if err != nil {
		return "", 0, err
	}
	ppid, err := parseLinuxStatusPPid(string(statusBytes))
	if err != nil {
		return "", 0, err
	}
	return strings.TrimSpace(string(commBytes)), ppid, nil
}

func darwinProcessInfo(pid int) (string, int, error) {
	nameOut, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=").Output()
	if err != nil {
		return "", 0, err
	}
	ppidOut, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "ppid=").Output()
	if err != nil {
		return "", 0, err
	}
	ppid, err := strconv.Atoi(strings.TrimSpace(string(ppidOut)))
	if err != nil {
		return "", 0, err
	}
	return strings.TrimSpace(string(nameOut)), ppid, nil
}

func windowsProcessInfo(pid int) (string, int, error) {
	query := fmt.Sprintf(`$p=Get-CimInstance Win32_Process -Filter "ProcessId = %d"; if ($p) { Write-Output ($p.Name + "|" + $p.ParentProcessId) }`, pid)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", query).Output()
	if err != nil {
		return "", 0, err
	}
	return parseWindowsProcessLine(string(out))
}

func windowsProcessAncestry(startPID, maxDepth int) ([]string, error) {
	query := fmt.Sprintf(`$pidValue=%d; $depth=%d; $current=Get-CimInstance Win32_Process -Filter ("ProcessId = " + $pidValue); for($i=0; $i -lt $depth -and $current; $i++){ Write-Output $current.Name; if ($current.ParentProcessId -le 1 -or $current.ParentProcessId -eq $current.ProcessId) { break }; $current=Get-CimInstance Win32_Process -Filter ("ProcessId = " + $current.ParentProcessId) }`, startPID, maxDepth)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", query).Output()
	if err != nil {
		return nil, err
	}
	return parseWindowsAncestryOutput(string(out)), nil
}

func parseLinuxStatusPPid(status string) (int, error) {
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("invalid PPid line")
			}
			return strconv.Atoi(fields[1])
		}
	}
	return 0, fmt.Errorf("PPid not found")
}

func parseWindowsProcessLine(raw string) (string, int, error) {
	line := strings.TrimSpace(raw)
	if line == "" {
		return "", 0, fmt.Errorf("empty process line")
	}
	parts := strings.Split(line, "|")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid process line")
	}
	ppid, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return "", 0, err
	}
	return strings.TrimSpace(parts[0]), ppid, nil
}

func parseWindowsAncestryOutput(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// Generate dispatches to the appropriate rule/instruction file generators.
// For explicit setup: always generates for the specified IDE, installs the
// embedded skill, ensures AGENTS.md, and writes MCP config when supported.
//
// For Cursor-family primaries (cursor, vscode, copilot), Generate also
// installs any companion IDEs returned by companionIDEs() (e.g. Claude
// Code when detected in the same environment). The "both" and "unknown"
// branches keep their original multi-IDE fan-out behavior.
func Generate(projectRoot, ideType string) ([]string, error) {
	var files []string

	if spec, ok := ideRuleRegistry[ideType]; ok {

		if skillPath, err := ensureEmbeddedSkill(projectRoot, ideType); err != nil {
			return nil, err
		} else if skillPath != "" {
			files = append(files, skillPath)
		}
		f, err := spec.generate(projectRoot)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	// For "both" or unknown, generate Cursor + Copilot + AGENTS.md
	if ideType == IDEBoth || ideType == IDEUnknown {
		for _, id := range []string{IDECursor, IDECopilot} {
			if spec, ok := ideRuleRegistry[id]; ok {
				f, err := spec.generate(projectRoot)
				if err != nil {
					return nil, err
				}
				files = append(files, f)
			}
		}
	}

	// For Cursor-family primaries, also install any companion IDEs that
	// are detected in the same environment. Mirrors the behavior of
	// EnsureRulesForIDE so that `gitflow setup --ide cursor` and the
	// auto-detect path stay consistent.
	switch ideType {
	case IDECursor, IDEVSCode, IDECopilot:
		for _, companion := range companionIDEs(ideType, projectRoot) {
			cspec, ok := ideRuleRegistry[companion]
			if !ok {
				continue
			}
			if skillPath, err := ensureEmbeddedSkill(projectRoot, companion); err != nil {
				return nil, err
			} else if skillPath != "" {
				files = append(files, skillPath)
			}
			cf, err := cspec.generate(projectRoot)
			if err != nil {
				return nil, err
			}
			files = append(files, cf)
			if MCPSupportedIDEs[companion] {
				if p, err := EnsureMCPConfig(projectRoot, companion); err == nil && p != "" {
					files = append(files, p)
				}
			}
		}
	}

	// Always generate AGENTS.md as universal fallback
	f, err := generateAgentsMD(projectRoot)
	if err != nil {
		return nil, err
	}
	files = append(files, f)

	// Generate MCP config for supported IDEs
	mcpTargets := []string{ideType}
	if ideType == IDEBoth || ideType == IDEUnknown {
		mcpTargets = []string{IDECursor, IDECopilot}
	}
	for _, id := range mcpTargets {
		if MCPSupportedIDEs[id] {
			p, err := EnsureMCPConfig(projectRoot, id)
			if err == nil && p != "" {
				files = append(files, p)
			}
		}
	}

	return files, nil
}
