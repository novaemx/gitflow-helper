package ide

import (
	"fmt"
	"testing"
	"time"
)

func TestDetectPrimary_ReturnsValidResult(t *testing.T) {
	dir := t.TempDir()
	result := DetectPrimary(dir)
	// Environment may have IDE env vars (e.g. running inside Cursor/VSCode),
	// so we can't assert a specific IDE. Just verify it returns a valid result.
	if result.ID == "" {
		t.Error("expected non-empty IDE ID")
	}
	if result.DisplayName == "" {
		t.Error("expected non-empty display name")
	}
}

func TestDetectAll_NoPanic(t *testing.T) {
	dir := t.TempDir()
	all := DetectAll(dir)
	// In a temp dir with no runtime markers, the only possible detection
	// is via parent process or env vars (which we can't control in CI).
	// Just verify it doesn't panic and returns a slice.
	_ = all
}

func TestDetectCursor_EnvVar(t *testing.T) {
	t.Setenv("CURSOR_TRACE_ID", "trace")
	dir := t.TempDir()

	if !detectCursor(dir) {
		t.Error("expected detectCursor to return true with CURSOR_TRACE_ID")
	}
}

func TestDetectCursor_NoDirectoryFallbackInVSCodeTerminal(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "vscode")
	dir := t.TempDir()

	if detectCursor(dir) {
		t.Error("expected detectCursor to return false in VS Code terminal without Cursor runtime signals")
	}
}

func TestDetectCursor_ExplicitEnvWinsOverVSCodeTerminal(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "vscode")
	t.Setenv("CURSOR_TRACE_ID", "trace")
	dir := t.TempDir()

	if !detectCursor(dir) {
		t.Error("expected detectCursor to return true with explicit Cursor env vars")
	}
}

func TestDetectClaudeCode_EnvVar(t *testing.T) {
	t.Setenv("CLAUDE_SESSION", "1")
	dir := t.TempDir()

	if !detectClaudeCode(dir) {
		t.Error("expected detectClaudeCode to return true with CLAUDE_* env var")
	}
}

func TestDetectWindsurf_EnvVar(t *testing.T) {
	t.Setenv("WINDSURF_SESSION", "1")
	dir := t.TempDir()

	if !detectWindsurf(dir) {
		t.Error("expected detectWindsurf to return true with WINDSURF_* env var")
	}
}

func TestDetectWindsurf_CodeiumEnvVar(t *testing.T) {
	t.Setenv("CODEIUM_API_KEY", "x")
	dir := t.TempDir()

	if !detectWindsurf(dir) {
		t.Error("expected detectWindsurf to return true with CODEIUM_* env var")
	}
}

func TestDetectCline_EnvVar(t *testing.T) {
	t.Setenv("CLINE_PROFILE", "default")
	dir := t.TempDir()

	if !detectCline(dir) {
		t.Error("expected detectCline to return true with CLINE_* env var")
	}
}

func TestDetectZed_EnvVar(t *testing.T) {
	t.Setenv("ZED_TERM", "1")
	dir := t.TempDir()

	if !detectZed(dir) {
		t.Error("expected detectZed to return true with ZED_TERM")
	}
}

func TestDetectJetBrains_TERM_PROGRAM(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "JetBrains-JediTerm")
	dir := t.TempDir()

	if !detectJetBrains(dir) {
		t.Error("expected detectJetBrains to return true with JetBrains terminal marker")
	}
}

func TestDetectJetBrains_EnvVar(t *testing.T) {
	t.Setenv("PYCHARM_HOSTED", "1")
	dir := t.TempDir()

	if !detectJetBrains(dir) {
		t.Error("expected detectJetBrains to return true with JetBrains env var")
	}
}

func TestDetectVSCode_EnvVar(t *testing.T) {
	t.Setenv("VSCODE_GIT_ASKPASS_NODE", "/some/path")
	dir := t.TempDir()

	result := detectVSCode(dir)
	if !result {
		t.Error("expected detectVSCode to return true with VSCODE_GIT_ASKPASS_NODE set")
	}
}

func TestDetectVSCode_TERM_PROGRAM(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "vscode")
	dir := t.TempDir()

	if !detectVSCode(dir) {
		t.Error("expected detectVSCode to return true with TERM_PROGRAM=vscode")
	}
}

func TestMatchProcessInAncestry_FindsInChain(t *testing.T) {
	type node struct {
		name string
		ppid int
	}
	chain := map[int]node{
		100: {name: "bash", ppid: 90},
		90:  {name: "code", ppid: 80},
		80:  {name: "init", ppid: 1},
	}

	fetch := func(pid int) (string, int, error) {
		n, ok := chain[pid]
		if !ok {
			return "", 0, fmt.Errorf("pid not found")
		}
		return n.name, n.ppid, nil
	}

	if !matchProcessInAncestry("code", 100, 8, fetch) {
		t.Error("expected ancestry matcher to find process in parent chain")
	}
}

func TestMatchProcessInAncestry_RespectsDepth(t *testing.T) {
	type node struct {
		name string
		ppid int
	}
	chain := map[int]node{
		100: {name: "bash", ppid: 90},
		90:  {name: "code", ppid: 1},
	}

	fetch := func(pid int) (string, int, error) {
		n, ok := chain[pid]
		if !ok {
			return "", 0, fmt.Errorf("pid not found")
		}
		return n.name, n.ppid, nil
	}

	if matchProcessInAncestry("code", 100, 1, fetch) {
		t.Error("expected ancestry matcher not to find process when depth is too small")
	}
}

func TestParseLinuxStatusPPid(t *testing.T) {
	status := "Name:\tbash\nState:\tS (sleeping)\nPPid:\t123\n"
	ppid, err := parseLinuxStatusPPid(status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ppid != 123 {
		t.Fatalf("expected ppid 123, got %d", ppid)
	}
}

func TestParseWindowsProcessLine(t *testing.T) {
	name, ppid, err := parseWindowsProcessLine("Code.exe|4321")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Code.exe" {
		t.Fatalf("expected Code.exe, got %q", name)
	}
	if ppid != 4321 {
		t.Fatalf("expected ppid 4321, got %d", ppid)
	}
}

func TestParseWindowsAncestryOutput(t *testing.T) {
	got := parseWindowsAncestryOutput("Code.exe\nWindowsTerminal.exe\n\n")
	if len(got) != 2 {
		t.Fatalf("expected 2 process names, got %d", len(got))
	}
	if got[0] != "Code.exe" || got[1] != "WindowsTerminal.exe" {
		t.Fatalf("unexpected parsed ancestry: %#v", got)
	}
}

func TestGetProcessAncestry_InvalidInput(t *testing.T) {
	if _, err := getProcessAncestry("windows", 1, 8); err == nil {
		t.Fatal("expected error for invalid start pid")
	}
	if _, err := getProcessAncestry("windows", 100, 0); err == nil {
		t.Fatal("expected error for invalid depth")
	}
}

func TestGenericProcessAncestry_UsesInjectedFetcher(t *testing.T) {
	original := parentProcessInfoFunc
	defer func() { parentProcessInfoFunc = original }()

	type node struct {
		name string
		ppid int
	}
	chain := map[int]node{
		100: {name: "bash", ppid: 90},
		90:  {name: "code", ppid: 1},
	}

	parentProcessInfoFunc = func(pid int, _ string) (string, int, error) {
		n, ok := chain[pid]
		if !ok {
			return "", 0, fmt.Errorf("missing pid")
		}
		return n.name, n.ppid, nil
	}

	got, err := genericProcessAncestry("linux", 100, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "bash" || got[1] != "code" {
		t.Fatalf("unexpected ancestry list: %#v", got)
	}
}

func TestParentProcessInfo_UnsupportedOS(t *testing.T) {
	if _, _, err := parentProcessInfo(100, "plan9"); err == nil {
		t.Fatal("expected error for unsupported os")
	}
}

func TestProcessInfoErrorPaths(t *testing.T) {
	if _, _, err := linuxProcessInfo(-1); err == nil {
		t.Fatal("expected linuxProcessInfo to fail for invalid pid")
	}
	if _, _, err := darwinProcessInfo(-1); err == nil {
		t.Fatal("expected darwinProcessInfo to fail for invalid pid")
	}
	if _, _, err := windowsProcessInfo(-1); err == nil {
		t.Fatal("expected windowsProcessInfo to fail for invalid pid")
	}
}

func TestDetectNeovim_EnvVar(t *testing.T) {
	t.Setenv("NVIM", "/tmp/nvim.sock")
	dir := t.TempDir()

	result := detectNeovim(dir)
	if !result {
		t.Error("expected detectNeovim to return true with NVIM set")
	}
}

func TestDetectPrimary_ReturnsValid(t *testing.T) {
	dir := t.TempDir()
	result := DetectPrimary(dir)
	if result.ID == "" {
		t.Error("expected non-empty IDE ID")
	}
	if result.DisplayName == "" {
		t.Error("expected non-empty display name")
	}
}

func TestDetectPrimary_StopsAfterFirstMatch(t *testing.T) {
	original := ideRegistry
	defer func() { ideRegistry = original }()

	calledSecond := false
	ideRegistry = []struct {
		id      string
		display string
		detect  func(string) bool
	}{
		{IDECursor, "Cursor", func(string) bool { return true }},
		{IDEVSCode, "VS Code", func(string) bool {
			calledSecond = true
			return true
		}},
	}

	got := DetectPrimary(t.TempDir())
	if got.ID != IDECursor {
		t.Fatalf("expected first matching IDE %q, got %q", IDECursor, got.ID)
	}
	if calledSecond {
		t.Fatal("expected DetectPrimary to stop after first match")
	}
}

func TestMatchParentProcessForOS_UsesSingleWindowsFetchPerAncestry(t *testing.T) {
	resetProcessAncestryCacheForTest()

	original := windowsProcessAncestryFunc
	originalNow := ancestryNowFunc
	defer func() {
		windowsProcessAncestryFunc = original
		ancestryNowFunc = originalNow
		resetProcessAncestryCacheForTest()
	}()

	base := time.Unix(1700000000, 0)
	ancestryNowFunc = func() time.Time { return base }

	calls := 0
	windowsProcessAncestryFunc = func(startPID, maxDepth int) ([]string, error) {
		calls++
		return []string{"bash.exe", "Code.exe", "explorer.exe"}, nil
	}

	if !matchParentProcessForOS("code", "windows", 1234, 8) {
		t.Fatal("expected first match to succeed")
	}
	if !matchParentProcessForOS("explorer", "windows", 1234, 8) {
		t.Fatal("expected second match to reuse cached ancestry")
	}
	if calls != 1 {
		t.Fatalf("expected exactly one windows ancestry fetch, got %d", calls)
	}
}

func TestGetProcessAncestry_ExpiresAfterTTL(t *testing.T) {
	resetProcessAncestryCacheForTest()

	originalWin := windowsProcessAncestryFunc
	originalNow := ancestryNowFunc
	originalTTL := processAncestryCacheTTL
	defer func() {
		windowsProcessAncestryFunc = originalWin
		ancestryNowFunc = originalNow
		processAncestryCacheTTL = originalTTL
		resetProcessAncestryCacheForTest()
	}()

	calls := 0
	windowsProcessAncestryFunc = func(startPID, maxDepth int) ([]string, error) {
		calls++
		return []string{"bash.exe", "Code.exe"}, nil
	}

	processAncestryCacheTTL = 2 * time.Second
	current := time.Unix(1700000000, 0)
	ancestryNowFunc = func() time.Time { return current }

	if _, err := getProcessAncestry("windows", 2222, 8); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := getProcessAncestry("windows", 2222, 8); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one call before TTL expiration, got %d", calls)
	}

	current = current.Add(3 * time.Second)
	if _, err := getProcessAncestry("windows", 2222, 8); err != nil {
		t.Fatalf("unexpected error after TTL expiration: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected refresh after TTL expiration, got %d calls", calls)
	}
}

func TestGetProcessAncestry_EvictsOldestWhenMaxEntriesReached(t *testing.T) {
	resetProcessAncestryCacheForTest()

	originalWin := windowsProcessAncestryFunc
	originalNow := ancestryNowFunc
	originalMax := processAncestryCacheMaxEntries
	defer func() {
		windowsProcessAncestryFunc = originalWin
		ancestryNowFunc = originalNow
		processAncestryCacheMaxEntries = originalMax
		resetProcessAncestryCacheForTest()
	}()

	windowsProcessAncestryFunc = func(startPID, maxDepth int) ([]string, error) {
		return []string{fmt.Sprintf("pid-%d", startPID)}, nil
	}

	processAncestryCacheMaxEntries = 2
	base := time.Unix(1700000100, 0)
	step := 0
	ancestryNowFunc = func() time.Time {
		t := base.Add(time.Duration(step) * time.Second)
		step++
		return t
	}

	if _, err := getProcessAncestry("windows", 1_001, 8); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := getProcessAncestry("windows", 1_002, 8); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := getProcessAncestry("windows", 1_003, 8); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	processAncestryCache.mu.Lock()
	defer processAncestryCache.mu.Unlock()
	if len(processAncestryCache.items) != 2 {
		t.Fatalf("expected cache to keep max 2 entries, got %d", len(processAncestryCache.items))
	}
	if _, ok := processAncestryCache.items["windows:1001:8"]; ok {
		t.Fatal("expected oldest cache entry to be evicted")
	}
}

func resetProcessAncestryCacheForTest() {
	processAncestryCache.mu.Lock()
	defer processAncestryCache.mu.Unlock()
	processAncestryCache.items = map[string]cachedAncestry{}
}

func TestIDERegistryCompleteness(t *testing.T) {
	// Verify all IDE constants have entries in ideRuleRegistry
	ides := []string{
		IDECursor, IDEVSCode, IDECopilot, IDEClaudeCode,
		IDEWindsurf, IDECline, IDEZed, IDENeovim, IDEJetBrains,
	}
	for _, id := range ides {
		if _, ok := ideRuleRegistry[id]; !ok {
			t.Errorf("IDE %q missing from ideRuleRegistry", id)
		}
	}
}

func TestIdeRegistryConsistency(t *testing.T) {
	// Verify all entries in ideRegistry have corresponding rule registry entries
	for _, entry := range ideRegistry {
		if entry.id == IDEUnknown || entry.id == IDEBoth {
			continue
		}
		if _, ok := ideRuleRegistry[entry.id]; !ok {
			t.Errorf("IDE %q in ideRegistry but missing from ideRuleRegistry", entry.id)
		}
	}
}
