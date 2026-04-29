package mcp

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/novaemx/gitflow-helper/internal/config"
	gflogic "github.com/novaemx/gitflow-helper/internal/gitflow"
)

func setupMCPRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "initial commit")
	run("git", "branch", "develop")
	return dir
}

func startMCPClientServer(t *testing.T) (context.Context, *sdkmcp.ClientSession) {
	t.Helper()
	dir := setupMCPRepo(t)
	gf := gflogic.NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "", TagPrefix: "v", IntegrationMode: config.IntegrationModeLocalMerge})
	server := &Server{
		gf: gf,
		mcp: sdkmcp.NewServer(&sdkmcp.Implementation{
			Name:    "gitflow-test-server",
			Version: "0.1.0",
		}, &sdkmcp.ServerOptions{}),
	}
	server.registerTools()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	serverT, clientT := sdkmcp.NewInMemoryTransports()
	go func() {
		_ = server.mcp.Run(ctx, serverT)
	}()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

func callToolText(t *testing.T, ctx context.Context, cs *sdkmcp.ClientSession, name string, args any) string {
	t.Helper()
	res, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("CallTool(%s): empty content", name)
	}
	text, ok := res.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): first content is not TextContent", name)
	}
	return text.Text
}

func TestMCPTools_ListAndInvokeCore(t *testing.T) {
	ctx, cs := startMCPClientServer(t)

	tools, err := cs.ListTools(ctx, &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("expected non-empty tools list")
	}

	for _, tool := range []string{"status", "init", "pull", "sync", "backmerge", "cleanup", "health", "doctor", "log", "undo", "releasenotes", "switch", "mode"} {
		body := callToolText(t, ctx, cs, tool, map[string]any{})
		if body == "" {
			t.Fatalf("expected non-empty body for %s", tool)
		}
	}
}

func TestMCPTools_InvokeParameterizedAndErrorPaths(t *testing.T) {
	ctx, cs := startMCPClientServer(t)

	body := callToolText(t, ctx, cs, "start", map[string]any{"type": "feature", "name": "mcp-e2e"})
	if !strings.Contains(body, "start") {
		t.Fatalf("expected start response, got %q", body)
	}

	body = callToolText(t, ctx, cs, "finish", map[string]any{"name": "", "squash": false, "rebase": false})
	if !strings.Contains(body, "finish") {
		t.Fatalf("expected finish response, got %q", body)
	}

	body = callToolText(t, ctx, cs, "fast-release", map[string]any{"name": "missing-branch"})
	if !strings.Contains(body, "fast-release") && !strings.Contains(body, "error") {
		t.Fatalf("expected fast-release result, got %q", body)
	}

	body = callToolText(t, ctx, cs, "mode", map[string]any{"mode": "invalid-mode"})
	if !strings.Contains(strings.ToLower(body), "error") {
		t.Fatalf("expected mode error, got %q", body)
	}
}
