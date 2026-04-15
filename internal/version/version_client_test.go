package version

import (
	"sync"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	igit "github.com/novaemx/gitflow-helper/internal/git"
)

type mockGitClient struct {
	mu    sync.Mutex
	calls [][]string
}

func (m *mockGitClient) Exec(args ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, append([]string(nil), args...))
	return nil
}

func (m *mockGitClient) ExecResult(args ...string) (int, string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, append([]string(nil), args...))
	return 0, "", ""
}

func (m *mockGitClient) ExecQuiet(args ...string) string {
	m.ExecResult(args...)
	return ""
}

func (m *mockGitClient) ExecLines(args ...string) []string {
	return nil
}

func TestWriteVersionFile_UsesDefaultClient(t *testing.T) {
	dir := t.TempDir()
	cfg := config.FlowConfig{ProjectRoot: dir, VersionFile: "VERSION"}

	mock := &mockGitClient{}
	old := igit.DefaultClient()
	igit.SetDefaultClient(mock)
	defer igit.SetDefaultClient(old)

	WriteVersionFile(cfg, "1.2.3")

	foundAdd := false
	foundCommit := false
	for _, c := range mock.calls {
		if len(c) >= 2 && c[0] == "add" && c[1] == "VERSION" {
			foundAdd = true
		}
		if len(c) >= 2 && c[0] == "commit" {
			foundCommit = true
		}
	}
	if !foundAdd || !foundCommit {
		t.Fatalf("expected add and commit calls, got %v", mock.calls)
	}
}
