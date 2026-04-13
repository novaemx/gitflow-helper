package state

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/version"
)

type BranchInfo struct {
	Name         string `json:"name"`
	BranchType   string `json:"branch_type"`
	ShortName    string `json:"short_name"`
	CommitsAhead int    `json:"commits_ahead"`
	HasRemote    bool   `json:"has_remote"`
}

type MergeState struct {
	InMerge          bool     `json:"in_merge"`
	ConflictedFiles  []string `json:"conflicted_files"`
	MergeHead        string   `json:"merge_head"`
	OperationType    string   `json:"operation_type"`
	OperationVersion string   `json:"operation_version"`
}

type RepoState struct {
	Current            string       `json:"current"`
	Version            string       `json:"version"`
	LastTag            string       `json:"last_tag"`
	Dirty              bool         `json:"dirty"`
	UncommittedCount   int          `json:"uncommitted_count"`
	Features           []BranchInfo `json:"features"`
	Bugfixes           []BranchInfo `json:"bugfixes"`
	Releases           []BranchInfo `json:"releases"`
	Hotfixes           []BranchInfo `json:"hotfixes"`
	HasDevelop         bool         `json:"has_develop"`
	HasMain            bool         `json:"has_main"`
	DevelopAheadOfMain int          `json:"develop_ahead_of_main"`
	MainAheadOfDevelop int          `json:"main_ahead_of_develop"`
	DevelopOnlyFiles   []string     `json:"develop_only_files"`
	MainOnlyFiles      []string     `json:"main_only_files"`
	Merge              MergeState   `json:"merge"`
	GitFlowInitialized bool        `json:"git_flow_initialized"`
	ProjectRoot        string       `json:"project_root"`
}

func DetectMergeState(cfg config.FlowConfig) MergeState {
	ms := MergeState{}
	gitDir := filepath.Join(cfg.ProjectRoot, ".git")
	mergeHeadFile := filepath.Join(gitDir, "MERGE_HEAD")

	data, err := os.ReadFile(mergeHeadFile)
	if err == nil {
		ms.InMerge = true
		head := strings.TrimSpace(string(data))
		if len(head) > 12 {
			head = head[:12]
		}
		ms.MergeHead = head
		ms.ConflictedFiles = git.RunLines("git diff --name-only --diff-filter=U")
	}

	if ms.InMerge {
		allLocal := git.RunLines("git branch --format='%(refname:short)'")
		for _, b := range allLocal {
			if strings.HasPrefix(b, "release/") {
				ms.OperationType = "release"
				v := strings.TrimPrefix(b, "release/v")
				ms.OperationVersion = strings.TrimPrefix(v, "release/")
				break
			}
			if strings.HasPrefix(b, "hotfix/") {
				ms.OperationType = "hotfix"
				v := strings.TrimPrefix(b, "hotfix/v")
				ms.OperationVersion = strings.TrimPrefix(v, "hotfix/")
				break
			}
		}
	}
	return ms
}

func DetectState(cfg config.FlowConfig) RepoState {
	s := RepoState{
		ProjectRoot: cfg.ProjectRoot,
	}
	s.Current = git.CurrentBranch()
	s.Version = version.ReadVersion(cfg)
	s.LastTag = git.LatestTag()
	s.Merge = DetectMergeState(cfg)
	s.GitFlowInitialized = git.IsGitFlowInitialized()

	statusLines := git.RunLines("git status --porcelain")
	s.UncommittedCount = len(statusLines)
	s.Dirty = s.UncommittedCount > 0

	allLocal := git.RunLines("git branch --format='%(refname:short)'")
	allRemote := git.RunLines("git branch -r --format='%(refname:short)'")
	remoteSet := make(map[string]bool)
	for _, r := range allRemote {
		remoteSet[r] = true
	}

	for _, b := range allLocal {
		if b == cfg.DevelopBranch {
			s.HasDevelop = true
		}
		if b == cfg.MainBranch {
			s.HasMain = true
		}
	}

	if s.HasDevelop && s.HasMain && !s.Merge.InMerge {
		ahead := git.RunQuiet("git rev-list --count " + cfg.MainBranch + ".." + cfg.DevelopBranch + " 2>/dev/null")
		s.DevelopAheadOfMain = atoi(ahead)
		behind := git.RunQuiet("git rev-list --count " + cfg.DevelopBranch + ".." + cfg.MainBranch + " 2>/dev/null")
		s.MainAheadOfDevelop = atoi(behind)

		if s.DevelopAheadOfMain > 0 {
			s.DevelopOnlyFiles = git.RunLines("git diff --name-only " + cfg.MainBranch + "..." + cfg.DevelopBranch)
		}
		if s.MainAheadOfDevelop > 0 {
			s.MainOnlyFiles = git.RunLines("git diff --name-only " + cfg.DevelopBranch + "..." + cfg.MainBranch)
		}
	}

	type prefixMapping struct {
		prefix     string
		branchType string
		target     *[]BranchInfo
	}
	mappings := []prefixMapping{
		{"feature/", "feature", &s.Features},
		{"bugfix/", "bugfix", &s.Bugfixes},
		{"release/", "release", &s.Releases},
		{"hotfix/", "hotfix", &s.Hotfixes},
	}

	for _, branch := range allLocal {
		for _, m := range mappings {
			if strings.HasPrefix(branch, m.prefix) {
				short := strings.TrimPrefix(branch, m.prefix)
				hasRemote := remoteSet[cfg.Remote+"/"+branch]
				parent := cfg.DevelopBranch
				if m.branchType == "hotfix" || m.branchType == "release" {
					if m.branchType == "hotfix" {
						parent = cfg.MainBranch
					}
				}
				aheadStr := git.RunQuiet("git rev-list --count " + parent + ".." + branch + " 2>/dev/null")
				aheadN := atoi(aheadStr)
				*m.target = append(*m.target, BranchInfo{
					Name:         branch,
					BranchType:   m.branchType,
					ShortName:    short,
					CommitsAhead: aheadN,
					HasRemote:    hasRemote,
				})
				break
			}
		}
	}

	if s.Features == nil {
		s.Features = []BranchInfo{}
	}
	if s.Bugfixes == nil {
		s.Bugfixes = []BranchInfo{}
	}
	if s.Releases == nil {
		s.Releases = []BranchInfo{}
	}
	if s.Hotfixes == nil {
		s.Hotfixes = []BranchInfo{}
	}
	if s.DevelopOnlyFiles == nil {
		s.DevelopOnlyFiles = []string{}
	}
	if s.MainOnlyFiles == nil {
		s.MainOnlyFiles = []string{}
	}
	if s.Merge.ConflictedFiles == nil {
		s.Merge.ConflictedFiles = []string{}
	}
	return s
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
