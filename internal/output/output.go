package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
)

// ANSI color constants
const (
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Cyan    = "\033[36m"
	Red     = "\033[31m"
	Magenta = "\033[35m"
	Reset   = "\033[0m"
)

var (
	jsonMode bool
	ansiRe   = regexp.MustCompile(`\033\[[0-9;]*m`)
)

func SetJSONMode(enabled bool) { jsonMode = enabled }
func IsJSONMode() bool         { return jsonMode }

func Info(msg string) {
	if jsonMode {
		fmt.Fprintln(os.Stderr, msg)
	} else {
		fmt.Println(msg)
	}
}

func Infof(format string, a ...any) {
	Info(fmt.Sprintf(format, a...))
}

func JSONOutput(data any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

func StripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func Writer() io.Writer {
	if jsonMode {
		return os.Stderr
	}
	return os.Stdout
}
