package workspace

import (
	"strings"
	"time"
)

type State struct {
	SelectedWorkspaceID string                        `json:"selected_workspace_id"`
	Workspaces          []Workspace                   `json:"workspaces"`
	RepoStatusSnapshots map[string]RepoStatusSnapshot `json:"repo_status_snapshots,omitempty"`
}

type Workspace struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Repos          []Repo    `json:"repos"`
	SelectedRepoID string    `json:"selected_repo_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Repo struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	Path                 string     `json:"path"`
	DefaultBranch        string     `json:"default_branch"`
	SelectedWorktreeID   string     `json:"selected_worktree_id"`
	SelectedWorktreePath string     `json:"selected_worktree_path"`
	ReleaseWorkflowRef   string     `json:"release_workflow_ref"`
	Health               RepoHealth `json:"health"`
}

type RepoInput struct {
	Name               string
	Path               string
	DefaultBranch      string
	ReleaseWorkflowRef string
}

type Status string

const (
	StatusSuccess      Status = "success"
	StatusFailure      Status = "failure"
	StatusNeutral      Status = "neutral"
	StatusInProgress   Status = "in_progress"
	StatusUnconfigured Status = "unconfigured"
)

type RepoStatus struct {
	PR      Status `json:"pr"`
	CI      Status `json:"ci"`
	Release Status `json:"release"`
}

type RepoStatusSnapshot struct {
	PR           Status    `json:"pr"`
	CI           Status    `json:"ci"`
	Release      Status    `json:"release"`
	LastSyncedAt time.Time `json:"last_synced_at"`
	IsStale      bool      `json:"is_stale"`
	LatestError  string    `json:"latest_error,omitempty"`
}

func StatusFromGitHubRun(status, conclusion string) Status {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "in_progress", "queued":
		return StatusInProgress
	case "success":
		return StatusSuccess
	case "failure", "timed_out", "action_required", "stale":
		return StatusFailure
	case "cancelled", "skipped":
		return StatusNeutral
	}

	switch strings.ToLower(strings.TrimSpace(conclusion)) {
	case "success":
		return StatusSuccess
	case "failure", "timed_out", "action_required", "stale":
		return StatusFailure
	case "cancelled", "skipped":
		return StatusNeutral
	}

	return StatusNeutral
}
