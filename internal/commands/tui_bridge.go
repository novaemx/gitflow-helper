package commands

import (
	"github.com/novaemx/gitflow-helper/internal/tui"
)

func runTUI() error {
	return tui.Run(GF)
}
