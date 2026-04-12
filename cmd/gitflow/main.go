package main

import (
	"os"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/commands"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	if version == "dev" {
		if data, err := os.ReadFile("VERSION"); err == nil {
			if v := strings.TrimSpace(string(data)); v != "" {
				version = v
			}
		}
	}

	root := commands.NewRootCmd(version)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
