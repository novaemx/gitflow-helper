package flow

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
)

var (
	currentBranch = git.CurrentBranch
	remoteExists  = git.RemoteExists
	execQuiet     = git.ExecQuiet
	exec          = git.Exec
	readChoice    = readChoiceFromStdin
)

func uniqueNonEmpty(values ...string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		result = append(result, v)
	}
	return result
}

func branchFromMergeRef(ref string) string {
	const prefix = "refs/heads/"
	if strings.HasPrefix(ref, prefix) {
		return strings.TrimPrefix(ref, prefix)
	}
	return strings.TrimSpace(ref)
}

func readChoiceFromStdin() string {
	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	return strings.TrimSpace(choice)
}

func chooseTargetInteractively(current string, candidates []string) (string, bool) {
	if len(candidates) == 0 {
		return "", false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}

	output.Infof("  %sMultiple push targets detected for '%s':%s", output.Yellow, current, output.Reset)
	for i, c := range candidates {
		output.Infof("    %d) %s", i+1, c)
	}
	output.Infof("  Select push target [1-%d]: ", len(candidates))

	choice := readChoice()
	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(candidates) {
		return "", false
	}
	return candidates[idx-1], true
}

func handleBranchMismatch(current, target string) (string, string, bool, map[string]any) {
	if current == target {
		return current, target, false, nil
	}

	if output.IsJSONMode() {
		return current, target, false, map[string]any{
			"action":      "push",
			"result":      "branch_mismatch",
			"branch":      current,
			"target":      target,
			"needs_human": true,
			"options": []string{
				"switch_local_to_target",
				"change_target_to_local_branch",
			},
		}
	}

	output.Infof("  %sTarget branch '%s' does not match local branch '%s'.%s", output.Yellow, target, current, output.Reset)
	output.Infof("  Choose how to continue:")
	output.Infof("    1) Switch local branch to '%s' and push", target)
	output.Infof("    2) Keep local '%s' and push to '%s'", current, current)
	output.Infof("    3) Cancel")
	output.Infof("  Select option [1-3]: ")

	choice := readChoice()
	switch choice {
	case "1":
		if err := exec("checkout", target); err != nil {
			return current, target, false, map[string]any{
				"action": "push",
				"result": "error",
				"detail": "checkout_failed",
				"branch": current,
				"target": target,
			}
		}
		return target, target, false, nil
	case "2":
		return current, current, false, nil
	default:
		return current, target, true, map[string]any{
			"action": "push",
			"result": "cancelled",
			"branch": current,
			"target": target,
		}
	}
}

func Push(cfg config.FlowConfig, target string) (int, map[string]any) {
	branch := currentBranch()
	if branch == "" {
		return 1, map[string]any{"action": "push", "error": "detached HEAD"}
	}

	remote := strings.TrimSpace(cfg.Remote)
	if remote == "" {
		return 1, map[string]any{"action": "push", "result": "no_remote", "message": "no default remote configured"}
	}
	if !remoteExists(remote) {
		return 1, map[string]any{"action": "push", "result": "no_remote", "remote": remote}
	}

	upstreamMerge := branchFromMergeRef(execQuiet("config", "--get", "branch."+branch+".merge"))
	candidates := uniqueNonEmpty(branch, upstreamMerge)

	if strings.TrimSpace(target) == "" {
		if output.IsJSONMode() && len(candidates) > 1 {
			return 1, map[string]any{
				"action":            "push",
				"result":            "target_required",
				"branch":            branch,
				"available_targets": candidates,
			}
		}

		selected, ok := chooseTargetInteractively(branch, candidates)
		if !ok {
			return 1, map[string]any{
				"action":            "push",
				"result":            "invalid_target_selection",
				"branch":            branch,
				"available_targets": candidates,
			}
		}
		target = selected
	}

	branch, target, cancelled, mismatchResult := handleBranchMismatch(branch, target)
	if mismatchResult != nil {
		if cancelled {
			return 1, mismatchResult
		}
		if mismatchResult["result"] == "branch_mismatch" {
			return 1, mismatchResult
		}
		return 1, mismatchResult
	}

	output.Infof("\n  %sPushing '%s' to %s/%s...%s", output.Bold, branch, remote, target, output.Reset)
	if err := exec("push", "-u", remote, branch+":"+target); err != nil {
		return 1, map[string]any{
			"action": "push",
			"result": "error",
			"branch": branch,
			"target": target,
			"remote": remote,
		}
	}

	return 0, map[string]any{
		"action": "push",
		"result": "ok",
		"branch": branch,
		"target": target,
		"remote": remote,
	}
}
