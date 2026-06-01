package debug

import "github.com/pbnjay/memory"

func totalSystemRAM() (uint64, bool) {
	total := memory.TotalMemory()
	if total == 0 {
		return 0, false
	}
	return total, true
}
