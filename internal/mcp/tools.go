package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/flow"
)

// Each tool follows the pattern: define args struct, register with mcp.AddTool,
// delegate to gitflow.Logic, record activity, return JSON result.

type emptyArgs struct{}

// ── status ──────────────────────────────────────────────────

func (s *Server) registerStatus() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "status",
		Description: "Show full repository state: branch, version, divergence, merge conflicts, in-flight branches",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		state := s.gf.Status()
		s.record("status", "", "ok", "")
		return textResult(state), nil, nil
	})
}

// ── init ────────────────────────────────────────────────────

func (s *Server) registerInit() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "init",
		Description: "Initialize gitflow structure (main + develop branches)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		ok, msg := s.gf.Init()
		result := map[string]any{"action": "init", "result": msg, "ok": ok}
		status := "ok"
		if !ok {
			status = "error"
		}
		s.record("init", "", status, "")
		return textResult(result), nil, nil
	})
}

// ── pull ────────────────────────────────────────────────────

func (s *Server) registerPull() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "pull",
		Description: "Safe fetch + fast-forward merge (never pushes)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		code, result := s.gf.Pull()
		status := "ok"
		if code != 0 {
			status = "error"
		}
		s.record("pull", "", status, "")
		return textResult(result), nil, nil
	})
}

// ── sync ────────────────────────────────────────────────────

func (s *Server) registerSync() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "sync",
		Description: "Sync current flow branch with its parent (develop or main)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		code, result := s.gf.Sync()
		status := "ok"
		if code != 0 {
			status = "error"
		}
		s.record("sync", "", status, "")
		return textResult(result), nil, nil
	})
}

// ── backmerge ───────────────────────────────────────────────

func (s *Server) registerBackmerge() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "backmerge",
		Description: "Merge main into develop to restore the gitflow invariant (develop must contain all of main)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		code, result := s.gf.Backmerge()
		status := "ok"
		if code != 0 {
			status = "error"
		}
		s.record("backmerge", "", status, "")
		return textResult(result), nil, nil
	})
}

// ── cleanup ─────────────────────────────────────────────────

func (s *Server) registerCleanup() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "cleanup",
		Description: "Delete local branches already merged into develop/main",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		code, result := s.gf.Cleanup()
		status := "ok"
		if code != 0 {
			status = "error"
		}
		s.record("cleanup", "", status, "")
		return textResult(result), nil, nil
	})
}

// ── health ──────────────────────────────────────────────────

func (s *Server) registerHealth() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "health",
		Description: "Comprehensive repo health check: divergence, stale branches, unpushed commits, remote reachability",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		report := s.gf.HealthReport()
		s.record("health", "", "ok", "")
		return textResult(report.ToMap()), nil, nil
	})
}

// ── doctor ──────────────────────────────────────────────────

func (s *Server) registerDoctor() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "doctor",
		Description: "Validate prerequisites: git version, gitflow structure, IDE detection",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		result := s.gf.Doctor()
		s.record("doctor", "", "ok", "")
		return textResult(result), nil, nil
	})
}

// ── log ─────────────────────────────────────────────────────

type logArgs struct {
	Count int `json:"count" jsonschema:"number of log entries to return,default=20"`
}

func (s *Server) registerLog() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "log",
		Description: "Gitflow-aware commit log with release boundaries and branch type annotations",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args logArgs) (*mcp.CallToolResult, any, error) {
		count := args.Count
		if count <= 0 {
			count = 20
		}
		result := s.gf.Log(count)
		s.record("log", "", "ok", "")
		return textResult(result), nil, nil
	})
}

// ── undo ────────────────────────────────────────────────────

func (s *Server) registerUndo() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "undo",
		Description: "Show undoable operations from reflog (does not perform reset — returns candidates for review)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		result := s.gf.Undo()
		s.record("undo", "", "ok", "")
		return textResult(result), nil, nil
	})
}

// ── releasenotes ────────────────────────────────────────────

type releaseNotesArgs struct {
	FromTag string `json:"from_tag" jsonschema:"optional tag to generate notes from (default: latest tag)"`
}

func (s *Server) registerReleaseNotes() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "releasenotes",
		Description: "Generate user-facing release notes from git history (conventional commits format)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args releaseNotesArgs) (*mcp.CallToolResult, any, error) {
		result := s.gf.ReleaseNotes(args.FromTag)
		if result == nil {
			result = map[string]any{"action": "releasenotes", "result": "empty"}
		}
		s.record("releasenotes", args.FromTag, "ok", "")
		return textResult(result), nil, nil
	})
}

// ── start ───────────────────────────────────────────────────

type startArgs struct {
	Type string `json:"type" jsonschema:"branch type: feature, bugfix, release, or hotfix"`
	Name string `json:"name" jsonschema:"branch name or version number"`
}

func (s *Server) registerStart() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "start",
		Description: "Start a new flow branch (feature/bugfix/release/hotfix). Type must be one of: feature, bugfix, release, hotfix",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args startArgs) (*mcp.CallToolResult, any, error) {
		if args.Type == "" || args.Name == "" {
			return errResult("both 'type' and 'name' are required")
		}
		code, result := s.gf.Start(args.Type, args.Name)
		status := "ok"
		errMsg := ""
		if code != 0 {
			status = "error"
			if e, ok := result["error"]; ok {
				errMsg, _ = e.(string)
			}
		}
		s.record("start", args.Type+" "+args.Name, status, errMsg)
		return textResult(result), nil, nil
	})
}

// ── finish ──────────────────────────────────────────────────

type finishArgs struct {
	Name   string `json:"name" jsonschema:"optional branch name (defaults to current branch)"`
	Squash bool   `json:"squash" jsonschema:"squash all branch commits into one commit on develop"`
	Rebase bool   `json:"rebase" jsonschema:"rebase branch onto develop before the final merge"`
}

func (s *Server) registerFinish() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "finish",
		Description: "Finish current flow branch with pre-merge safety check. Supports rebase-first and squash strategies.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args finishArgs) (*mcp.CallToolResult, any, error) {
		var code int
		var result map[string]any

		if args.Squash || args.Rebase {
			opts := flow.FinishOptions{
				Rebase:       args.Rebase,
				Squash:       args.Squash,
				DeleteRemote: true,
			}
			code, result = s.gf.Finish(args.Name, opts)
		} else {
			code, result = s.gf.SmartFinish(args.Name)
		}

		status := "ok"
		errMsg := ""
		if code != 0 {
			status = "error"
			if e, ok := result["error"]; ok {
				errMsg, _ = e.(string)
			}
		}
		s.record("finish", args.Name, status, errMsg)
		return textResult(result), nil, nil
	})
}

// ── fast-release ─────────────────────────────────────────────

type fastReleaseArgs struct {
	Name string `json:"name" jsonschema:"feature or bugfix branch name (with or without prefix)"`
}

func (s *Server) registerFastRelease() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "fast-release",
		Description: "Merge a feature/bugfix branch directly to main (skip the release/ phase). Tags the release with the current version.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args fastReleaseArgs) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return errResult("name is required")
		}
		code, result := s.gf.FastRelease(args.Name)
		status := "ok"
		errMsg := ""
		if code != 0 {
			status = "error"
			if e, ok := result["error"]; ok {
				errMsg, _ = e.(string)
			}
		}
		s.record("fast-release", args.Name, status, errMsg)
		return textResult(result), nil, nil
	})
}

// ── switch ──────────────────────────────────────────────────

type switchArgs struct {
	Branch string `json:"branch" jsonschema:"target branch name or short name (e.g. 'develop' or 'my-feature')"`
}

type modeArgs struct {
	Mode string `json:"mode" jsonschema:"optional mode: local-merge|pull-request|toggle"`
}

func (s *Server) registerSwitch() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "switch",
		Description: "Switch to a gitflow branch with automatic stash/unstash of uncommitted changes",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args switchArgs) (*mcp.CallToolResult, any, error) {
		if args.Branch == "" {
			available := s.gf.ListSwitchable()
			result := map[string]any{"action": "switch", "result": "branch_required", "available": available}
			s.record("switch", "", "branch_required", "")
			return textResult(result), nil, nil
		}
		code, result := s.gf.Switch(args.Branch)
		status := "ok"
		if code != 0 {
			status = "error"
		}
		s.record("switch", args.Branch, status, "")
		return textResult(result), nil, nil
	})
}

// ── mode ───────────────────────────────────────────────────

func (s *Server) registerMode() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "mode",
		Description: "Get or set integration mode: local-merge or pull-request (use toggle to switch)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args modeArgs) (*mcp.CallToolResult, any, error) {
		if args.Mode == "" {
			mode := s.gf.IntegrationMode()
			result := map[string]any{
				"action":              "mode",
				"result":              "ok",
				"integration_mode":    mode,
				"integration_display": config.IntegrationModeDisplay(mode),
			}
			s.record("mode", "get", "ok", "")
			return textResult(result), nil, nil
		}

		next := args.Mode
		if next == "toggle" {
			if s.gf.IntegrationMode() == config.IntegrationModePullRequest {
				next = config.IntegrationModeLocalMerge
			} else {
				next = config.IntegrationModePullRequest
			}
		}

		normalized := config.NormalizeIntegrationMode(next)
		if normalized == "" {
			s.record("mode", args.Mode, "error", "invalid mode")
			return errResult("invalid mode; use local-merge|pull-request|toggle")
		}

		if err := s.gf.SetIntegrationMode(normalized); err != nil {
			s.record("mode", args.Mode, "error", err.Error())
			return errResult(err.Error())
		}

		result := map[string]any{
			"action":              "mode",
			"result":              "ok",
			"integration_mode":    normalized,
			"integration_display": config.IntegrationModeDisplay(normalized),
		}
		s.record("mode", normalized, "ok", "")
		return textResult(result), nil, nil
	})
}
