package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	gflogic "github.com/novaemx/gitflow-helper/internal/gitflow"
)

// ActivityEntry records an MCP tool invocation for TUI display.
type ActivityEntry struct {
	Tool      string `json:"tool"`
	Args      string `json:"args,omitempty"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"`
	Source    string `json:"source,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// Server wraps the MCP server and gitflow logic together.
type Server struct {
	mcp *mcp.Server
	gf  *gflogic.Logic

	mu       sync.Mutex
	activity []ActivityEntry
}

// NewServer creates an MCP server exposing all gitflow tools.
func NewServer(gf *gflogic.Logic) *Server {
	s := &Server{
		gf: gf,
		mcp: mcp.NewServer(&mcp.Implementation{
			Name:    "gitflow",
			Version: "0.1.0",
		}, &mcp.ServerOptions{
			Instructions: "Gitflow workflow helper. Use these tools to manage git-flow branches, merges, and releases.",
		}),
	}
	s.registerTools()
	return s
}

// Run starts the MCP server on stdio transport (blocks until client disconnects).
func (s *Server) Run(ctx context.Context) error {
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}

// Activity returns a snapshot of recent MCP tool invocations.
func (s *Server) Activity() []ActivityEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]ActivityEntry, len(s.activity))
	copy(cp, s.activity)
	return cp
}

func (s *Server) record(tool, args, result, errMsg string) {
	entry := ActivityEntry{
		Tool:      tool,
		Args:      args,
		Result:    result,
		Error:     errMsg,
		Source:    "mcp",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.activity = append(s.activity, entry)
	if len(s.activity) > 100 {
		s.activity = s.activity[len(s.activity)-100:]
	}
	s.writeActivityLog(entry)
}

// ActivityLogPath returns the path to the shared activity log file.
func ActivityLogPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".git", "gitflow-mcp-activity.jsonl")
}

func (s *Server) writeActivityLog(entry ActivityEntry) {
	_ = AppendActivityLog(s.gf.Config.ProjectRoot, entry)
}

func jsonText(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func textResult(v any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: jsonText(v)},
		},
	}
}

func errResult(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(`{"error": %q}`, msg)},
		},
		IsError: true,
	}, nil, nil
}

func (s *Server) registerTools() {
	s.registerStatus()
	s.registerInit()
	s.registerPull()
	s.registerSync()
	s.registerBackmerge()
	s.registerCleanup()
	s.registerHealth()
	s.registerDoctor()
	s.registerLog()
	s.registerUndo()
	s.registerReleaseNotes()
	s.registerStart()
	s.registerFinish()
	s.registerSwitch()
}
