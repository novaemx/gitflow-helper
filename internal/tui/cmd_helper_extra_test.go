package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExecutable_PathWithSeparator(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "my-local-tool")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write tool: %v", err)
	}
	got := resolveExecutable(bin, "")
	if got != bin {
		t.Fatalf("expected exact path, got %q", got)
	}
}

func TestResolveExecutable_ProjectRootFallback(t *testing.T) {
	root := t.TempDir()
	name := "gitflow-helper-test-tool"
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write tool: %v", err)
	}
	got := resolveExecutable(name, root)
	if got != path {
		t.Fatalf("expected project-root candidate %q, got %q", path, got)
	}
}

func TestResolveExecutable_HomeBinFallback(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir home bin: %v", err)
	}
	name := "gitflow-helper-home-tool"
	path := filepath.Join(binDir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write home tool: %v", err)
	}

	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer os.Setenv("HOME", oldHome)

	got := resolveExecutable(name, "")
	if got != path {
		t.Fatalf("expected home-bin candidate %q, got %q", path, got)
	}
}

func TestResolveExecutable_ReturnsNameWhenMissing(t *testing.T) {
	name := "definitely-missing-gitflow-helper-binary"
	got := resolveExecutable(name, t.TempDir())
	if got != name {
		t.Fatalf("expected unresolved name unchanged, got %q", got)
	}
}
