package commands

import (
	"github.com/luis-lozano/gitflow-helper/internal/tui"
)

func runTUI() error {
	return tui.Run(Cfg)
}
