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
	// Check environment and TERM_PROGRAM signals first (fast)
	{IDECopilot, "VS Code + Copilot", detectCopilot},
	{IDEVSCode, "VS Code", detectVSCode},
	// Then terminal-specific signals (medium)
	{IDECursor, "Cursor", detectCursor},
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
	exists   func(string) bool
	generate func(string) (string, error)
}

var ideRuleRegistry = map[string]ideRuleSpec{
	IDECursor:     {cursorRuleExists, generateCursorRule},
	IDEVSCode:     {copilotRuleExists, generateCopilotInstructions},
	IDECopilot:    {copilotRuleExists, generateCopilotInstructions},
	IDEClaudeCode: {claudeCodeRuleExists, generateClaudeCodeRule},
	IDEWindsurf:   {windsurfRuleExists, generateWindsurfRule},
	IDECline:      {clineRuleExists, generateClineRule},
	IDEZed:        {zedRuleExists, generateZedRule},
	IDENeovim:     {neovimRuleExists, generateNeovimRule},
	IDEJetBrains:  {jetbrainsRuleExists, generateJetBrainsRule},
}

// EnsureRulesForIDE checks if rules exist for the detected IDE.
// If missing, it creates them. Also ensures AGENTS.md is present as a
// universal fallback, and MCP config for IDEs that support it.
// Returns list of newly created files (empty if all exist).
func EnsureRulesForIDE(projectRoot string, detected DetectedIDE) ([]string, error) {
	var created []string

	// Generate IDE-specific rules
	if spec, ok := ideRuleRegistry[detected.ID]; ok {
		if !spec.exists(projectRoot) {
			path, err := spec.generate(projectRoot)
			if err != nil {
				return created, err
			}
			created = append(created, path)
		}
	}

	// Always ensure AGENTS.md as universal fallback
	if !agentsRuleExists(projectRoot) {
		path, err := generateAgentsMD(projectRoot)
		if err != nil {
			return created, err
		}
		created = append(created, path)
	}

	// Auto-provision MCP config for IDEs that support it
	if MCPSupportedIDEs[detected.ID] && !MCPConfigExists(projectRoot, detected.ID) {
		path, err := EnsureMCPConfig(projectRoot, detected.ID)
		if err == nil && path != "" {
			created = append(created, path)
		}
	}

	return created, nil
}

// --- Individual IDE detectors ---

func detectCursor(projectRoot string) bool {
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

func detectVSCode(projectRoot string) bool {
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
// For explicit setup: always generates for the specified IDE + AGENTS.md + MCP config.
func Generate(projectRoot, ideType string) ([]string, error) {
	var files []string

	if spec, ok := ideRuleRegistry[ideType]; ok {
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
