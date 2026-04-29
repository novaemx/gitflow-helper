package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	fn()

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, rOut)
	_, _ = io.Copy(&errBuf, rErr)
	return outBuf.String(), errBuf.String()
}

func TestSetJSONMode_RoundTrip(t *testing.T) {
	prev := IsJSONMode()
	defer SetJSONMode(prev)

	SetJSONMode(true)
	if !IsJSONMode() {
		t.Fatal("expected json mode enabled")
	}
	SetJSONMode(false)
	if IsJSONMode() {
		t.Fatal("expected json mode disabled")
	}
}

func TestInfo_TextMode_WritesStdout(t *testing.T) {
	prev := IsJSONMode()
	defer SetJSONMode(prev)
	SetJSONMode(false)

	stdout, stderr := captureOutput(t, func() {
		Info("hello")
	})
	if !strings.Contains(stdout, "hello") {
		t.Fatalf("expected stdout to contain message, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestInfo_JSONMode_WritesStderr(t *testing.T) {
	prev := IsJSONMode()
	defer SetJSONMode(prev)
	SetJSONMode(true)

	stdout, stderr := captureOutput(t, func() {
		Info("json-msg")
	})
	if !strings.Contains(stderr, "json-msg") {
		t.Fatalf("expected stderr to contain message, got %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestInfof_FormatsMessage(t *testing.T) {
	prev := IsJSONMode()
	defer SetJSONMode(prev)
	SetJSONMode(false)

	stdout, _ := captureOutput(t, func() {
		Infof("value=%d", 7)
	})
	if !strings.Contains(stdout, "value=7") {
		t.Fatalf("expected formatted output, got %q", stdout)
	}
}

func TestJSONOutput_WritesIndentedJSON(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		JSONOutput(map[string]any{"a": 1, "b": "x"})
	})
	if !strings.Contains(stdout, "\n") || !strings.Contains(stdout, "\"a\": 1") {
		t.Fatalf("expected indented JSON, got %q", stdout)
	}
}

func TestWriter_UsesStdoutInTextMode(t *testing.T) {
	prev := IsJSONMode()
	defer SetJSONMode(prev)
	SetJSONMode(false)
	if Writer() != os.Stdout {
		t.Fatal("expected Writer to return stdout in text mode")
	}
}

func TestWriter_UsesStderrInJSONMode(t *testing.T) {
	prev := IsJSONMode()
	defer SetJSONMode(prev)
	SetJSONMode(true)
	if Writer() != os.Stderr {
		t.Fatal("expected Writer to return stderr in json mode")
	}
}
