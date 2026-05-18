package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Level int

const (
	LevelOff Level = iota
	LevelLog
	LevelDebug
)

const (
	logDirName         = ".gitflow"
	logFilePrefix      = "log"
	logFileExt         = ".txt"
	logFileTimePattern = "20060102-150405"
)

var (
	level          = envLevel()
	mu             sync.Mutex
	timings        []TimingEntry
	startMu        sync.Mutex
	markers        = make(map[string]time.Time)
	logFile        *os.File
	logFilePath    string
	configuredRoot string
	buildVersion   = "unknown"
	buildDate      = "unknown"
	buildCommit    = "unknown"
)

func normalizeBuildValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

// SetBuildInfo stores build/runtime metadata that is emitted in log headers.
func SetBuildInfo(version, compiledAt, commit string) {
	mu.Lock()
	defer mu.Unlock()
	buildVersion = normalizeBuildValue(version)
	buildDate = normalizeBuildValue(compiledAt)
	buildCommit = normalizeBuildValue(commit)
}

func formatBytes(bytes uint64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.2f GiB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.2f MiB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.2f KiB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func logHeader(now time.Time, currentLevel Level) string {
	ramText := "unknown"
	if ramBytes, ok := totalSystemRAM(); ok {
		ramText = formatBytes(ramBytes)
	}

	return fmt.Sprintf(
		"\n===== Log capture started at %s (level=%s) =====\n"+
			"gitflow_version=%s\n"+
			"gitflow_build_date=%s\n"+
			"gitflow_build_commit=%s\n"+
			"os=%s arch=%s\n"+
			"cpu_cores=%d\n"+
			"ram_total=%s\n",
		now.Format(time.RFC3339),
		currentLevel.String(),
		buildVersion,
		buildDate,
		buildCommit,
		runtime.GOOS,
		runtime.GOARCH,
		runtime.NumCPU(),
		ramText,
	)
}

func envLevel() Level {
	if os.Getenv("GITFLOW_DEBUG") == "1" {
		return LevelDebug
	}
	if os.Getenv("GITFLOW_LOG") == "1" {
		return LevelLog
	}
	return LevelOff
}

func Configure(projectRoot string, logEnabled, debugEnabled bool) {
	mu.Lock()
	defer mu.Unlock()

	switch {
	case debugEnabled:
		level = LevelDebug
	case logEnabled:
		level = LevelLog
	default:
		level = envLevel()
	}

	if level < LevelLog {
		closeLogFileLocked()
		configuredRoot = ""
		return
	}

	root := projectRoot
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			closeLogFileLocked()
			configuredRoot = ""
			return
		}
		root = cwd
	}

	if logFile != nil && configuredRoot == root {
		configuredRoot = root
		return
	}

	now := time.Now()
	fileName := fmt.Sprintf("%s-%s%s", logFilePrefix, now.Format(logFileTimePattern), logFileExt)
	path := filepath.Join(root, logDirName, fileName)

	closeLogFileLocked()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		configuredRoot = root
		logFilePath = path
		return
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		configuredRoot = root
		logFilePath = path
		return
	}

	logFile = file
	logFilePath = path
	configuredRoot = root
	_, _ = logFile.WriteString(logHeader(now, level))
}

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelLog:
		return "log"
	default:
		return "off"
	}
}

func closeLogFileLocked() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
	logFilePath = ""
}

// TimingEntry represents a timed measurement
type TimingEntry struct {
	Name      string
	Duration  time.Duration
	Timestamp time.Time
}

// Start marks the beginning of a timing block; returns a function to call End
func Start(name string) func() {
	if !IsDebugEnabled() {
		return func() {}
	}
	startMu.Lock()
	markers[name] = time.Now()
	startMu.Unlock()
	return func() { End(name) }
}

// End completes a timing block and records it
func End(name string) {
	if !IsDebugEnabled() {
		return
	}
	startMu.Lock()
	start, ok := markers[name]
	delete(markers, name)
	startMu.Unlock()

	if !ok {
		return
	}

	duration := time.Since(start)
	mu.Lock()
	timings = append(timings, TimingEntry{
		Name:      name,
		Duration:  duration,
		Timestamp: start,
	})
	mu.Unlock()
}

// Printf logs debug messages if enabled
func Printf(format string, args ...any) {
	writeLine(LevelDebug, "DEBUG", format, args...)
}

// Logf logs high-level troubleshooting messages.
func Logf(format string, args ...any) {
	writeLine(LevelLog, "LOG", format, args...)
}

func writeLine(minLevel Level, prefix, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] %s\n", prefix, message)

	mu.Lock()
	enabled := level >= minLevel
	file := logFile
	mu.Unlock()

	if !enabled {
		return
	}

	if file != nil {
		mu.Lock()
		if logFile != nil {
			_, _ = logFile.WriteString(fmt.Sprintf("%s %s", time.Now().Format(time.RFC3339), line))
		}
		mu.Unlock()
	}
}

// Timings returns all recorded timings
func Timings() []TimingEntry {
	mu.Lock()
	defer mu.Unlock()
	return timings
}

// PrintTimings outputs all timings in human-readable format
func PrintTimings() {
	if !IsDebugEnabled() {
		return
	}
	mu.Lock()
	if len(timings) == 0 {
		mu.Unlock()
		return
	}
	entries := append([]TimingEntry(nil), timings...)
	file := logFile
	mu.Unlock()

	report := "\n=== TIMING REPORT ===\n"
	var total time.Duration
	for _, t := range entries {
		report += fmt.Sprintf("  %-50s %10.3fms\n", t.Name, t.Duration.Seconds()*1000)
		total += t.Duration
	}
	report += fmt.Sprintf("  %-50s %10.3fms\n", "TOTAL", total.Seconds()*1000)
	report += "======================\n\n"

	if file != nil {
		mu.Lock()
		if logFile != nil {
			_, _ = logFile.WriteString(fmt.Sprintf("%s %s", time.Now().Format(time.RFC3339), report))
		}
		mu.Unlock()
	}
}

// IsEnabled returns true if debug mode is on
func IsEnabled() bool {
	return IsDebugEnabled()
}

func IsLogEnabled() bool {
	mu.Lock()
	defer mu.Unlock()
	return level >= LevelLog
}

func IsDebugEnabled() bool {
	mu.Lock()
	defer mu.Unlock()
	return level >= LevelDebug
}
