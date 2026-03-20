package sync

import (
	"context"
	"strings"
	"sync"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

type RepoStatusFetcher interface {
	FetchRepoStatus(ctx context.Context, repo workspace.Repo) (workspace.RepoStatus, error)
}

type StateBackedFetcher struct {
	mu      sync.RWMutex
	fetcher RepoStatusFetcher
	state   workspace.State
}

func NewStateBackedFetcher(fetcher RepoStatusFetcher) *StateBackedFetcher {
	return &StateBackedFetcher{fetcher: fetcher}
}

func (f *StateBackedFetcher) SetState(st workspace.State) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.state = cloneWorkspaceState(st)
	f.mu.Unlock()
}

func (f *StateBackedFetcher) FetchSelectedRepoStatus(ctx context.Context, workspaceID, repoID string) (workspace.RepoStatus, error) {
	if f == nil {
		return workspace.RepoStatus{}, nil
	}

	workspaceID = strings.TrimSpace(workspaceID)
	repoID = strings.TrimSpace(repoID)
	if workspaceID == "" || repoID == "" {
		return workspace.RepoStatus{}, nil
	}

	f.mu.RLock()
	fetcher := f.fetcher
	repo, found := repoFromState(f.state, workspaceID, repoID)
	f.mu.RUnlock()

	if fetcher == nil || !found {
		return workspace.RepoStatus{}, nil
	}
	return fetcher.FetchRepoStatus(ctx, repo)
}

func repoFromState(st workspace.State, workspaceID, repoID string) (workspace.Repo, bool) {
	for i := range st.Workspaces {
		ws := st.Workspaces[i]
		if ws.ID != workspaceID {
			continue
		}
		for j := range ws.Repos {
			if ws.Repos[j].ID == repoID {
				return ws.Repos[j], true
			}
		}
		return workspace.Repo{}, false
	}
	return workspace.Repo{}, false
}

func cloneWorkspaceState(st workspace.State) workspace.State {
	cloned := st
	cloned.Workspaces = make([]workspace.Workspace, len(st.Workspaces))
	for i := range st.Workspaces {
		ws := st.Workspaces[i]
		ws.Repos = append([]workspace.Repo(nil), ws.Repos...)
		cloned.Workspaces[i] = ws
	}
	cloned.RepoStatusSnapshots = cloneRepoStatusSnapshots(st.RepoStatusSnapshots)
	return cloned
}

func cloneRepoStatusSnapshots(src map[string]workspace.RepoStatusSnapshot) map[string]workspace.RepoStatusSnapshot {
	if src == nil {
		return nil
	}
	cloned := make(map[string]workspace.RepoStatusSnapshot, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}
