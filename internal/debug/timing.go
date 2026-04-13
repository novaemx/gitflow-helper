package debug

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	enabled  = os.Getenv("GITFLOW_DEBUG") == "1"
	mu       sync.Mutex
	timings  []TimingEntry
	startMu  sync.Mutex
	markers  = make(map[string]time.Time)
)

// TimingEntry represents a timed measurement
type TimingEntry struct {
	Name      string
	Duration  time.Duration
	Timestamp time.Time
}

// Start marks the beginning of a timing block; returns a function to call End
func Start(name string) func() {
	if !enabled {
		return func() {}
	}
	startMu.Lock()
	markers[name] = time.Now()
	startMu.Unlock()
	return func() { End(name) }
}

// End completes a timing block and records it
func End(name string) {
	if !enabled {
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
	if !enabled {
		return
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
}

// Timings returns all recorded timings
func Timings() []TimingEntry {
	mu.Lock()
	defer mu.Unlock()
	return timings
}

// PrintTimings outputs all timings in human-readable format
func PrintTimings() {
	if !enabled {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	
	if len(timings) == 0 {
		return
	}
	
	fmt.Fprintf(os.Stderr, "\n=== TIMING REPORT ===\n")
	var total time.Duration
	for _, t := range timings {
		fmt.Fprintf(os.Stderr, "  %-50s %10.3fms\n", t.Name, t.Duration.Seconds()*1000)
		total += t.Duration
	}
	fmt.Fprintf(os.Stderr, "  %-50s %10.3fms\n", "TOTAL", total.Seconds()*1000)
	fmt.Fprintf(os.Stderr, "======================\n\n")
}

// IsEnabled returns true if debug mode is on
func IsEnabled() bool {
	return enabled
}
