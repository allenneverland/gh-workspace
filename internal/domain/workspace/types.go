package workspace

import "time"

type State struct {
	SelectedWorkspaceID string      `json:"selected_workspace_id"`
	Workspaces          []Workspace `json:"workspaces"`
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
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Path               string     `json:"path"`
	DefaultBranch      string     `json:"default_branch"`
	ReleaseWorkflowRef string     `json:"release_workflow_ref"`
	Health             RepoHealth `json:"health"`
}

type RepoInput struct {
	Name               string
	Path               string
	DefaultBranch      string
	ReleaseWorkflowRef string
}
