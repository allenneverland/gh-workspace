package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var (
	ErrStoreNotConfigured   = errors.New("workspace: state store not configured")
	ErrWorkspaceNotFound    = errors.New("workspace: workspace not found")
	ErrWorkspaceNameMissing = errors.New("workspace: workspace name is required")
	ErrRepoNotFound         = errors.New("workspace: repo not found")
	ErrRepoNameMissing      = errors.New("workspace: repo name is required")
	ErrRepoPathMissing      = errors.New("workspace: repo path is required")
)

type RepoHealth string

const (
	RepoHealthy RepoHealth = "healthy"
	RepoInvalid RepoHealth = "invalid"
)

type StateStore interface {
	Load(context.Context) (State, error)
	Save(context.Context, State) error
}

type Service struct {
	store StateStore
}

func NewService(store StateStore) *Service {
	return &Service{store: store}
}

func (s *Service) LoadState() (State, error) {
	if s.store == nil {
		return State{}, ErrStoreNotConfigured
	}
	return s.store.Load(context.Background())
}

func (s *Service) CreateWorkspace(name string) (Workspace, error) {
	if strings.TrimSpace(name) == "" {
		return Workspace{}, ErrWorkspaceNameMissing
	}

	state, err := s.LoadState()
	if err != nil {
		return Workspace{}, err
	}

	now := time.Now().UTC()
	workspace := Workspace{
		ID:        newID("ws"),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	state.Workspaces = append(state.Workspaces, workspace)
	if state.SelectedWorkspaceID == "" {
		state.SelectedWorkspaceID = workspace.ID
	}

	if err := s.store.Save(context.Background(), state); err != nil {
		return Workspace{}, err
	}

	return workspace, nil
}

func (s *Service) AddRepo(workspaceID string, input RepoInput) (Repo, error) {
	if strings.TrimSpace(input.Name) == "" {
		return Repo{}, ErrRepoNameMissing
	}
	if strings.TrimSpace(input.Path) == "" {
		return Repo{}, ErrRepoPathMissing
	}

	state, err := s.LoadState()
	if err != nil {
		return Repo{}, err
	}

	workspaceIdx := findWorkspaceIndex(state.Workspaces, workspaceID)
	if workspaceIdx < 0 {
		return Repo{}, fmt.Errorf("%w: %s", ErrWorkspaceNotFound, workspaceID)
	}

	repo := Repo{
		ID:                 newID("repo"),
		Name:               input.Name,
		Path:               input.Path,
		DefaultBranch:      input.DefaultBranch,
		ReleaseWorkflowRef: input.ReleaseWorkflowRef,
		Health:             RepoHealthy,
	}
	state.Workspaces[workspaceIdx].Repos = append(state.Workspaces[workspaceIdx].Repos, repo)
	state.Workspaces[workspaceIdx].UpdatedAt = time.Now().UTC()

	if err := s.store.Save(context.Background(), state); err != nil {
		return Repo{}, err
	}

	return repo, nil
}

func (s *Service) DeleteWorkspace(workspaceID string) error {
	state, err := s.LoadState()
	if err != nil {
		return err
	}

	workspaceIdx := findWorkspaceIndex(state.Workspaces, workspaceID)
	if workspaceIdx < 0 {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, workspaceID)
	}

	state.Workspaces = append(state.Workspaces[:workspaceIdx], state.Workspaces[workspaceIdx+1:]...)
	switch {
	case len(state.Workspaces) == 0:
		state.SelectedWorkspaceID = ""
	case state.SelectedWorkspaceID == workspaceID:
		state.SelectedWorkspaceID = state.Workspaces[0].ID
	}

	return s.store.Save(context.Background(), state)
}

func (s *Service) SelectRepo(workspaceID, repoID string) error {
	state, err := s.LoadState()
	if err != nil {
		return err
	}

	workspaceIdx := findWorkspaceIndex(state.Workspaces, workspaceID)
	if workspaceIdx < 0 {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, workspaceID)
	}
	if !containsRepo(state.Workspaces[workspaceIdx].Repos, repoID) {
		return fmt.Errorf("%w: %s", ErrRepoNotFound, repoID)
	}

	state.SelectedWorkspaceID = workspaceID
	state.Workspaces[workspaceIdx].SelectedRepoID = repoID
	state.Workspaces[workspaceIdx].UpdatedAt = time.Now().UTC()
	return s.store.Save(context.Background(), state)
}

func (s *Service) MarkRepoInvalid(workspaceID, repoID string) error {
	state, err := s.LoadState()
	if err != nil {
		return err
	}

	workspaceIdx := findWorkspaceIndex(state.Workspaces, workspaceID)
	if workspaceIdx < 0 {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, workspaceID)
	}

	repoIdx := findRepoIndex(state.Workspaces[workspaceIdx].Repos, repoID)
	if repoIdx < 0 {
		return fmt.Errorf("%w: %s", ErrRepoNotFound, repoID)
	}

	state.Workspaces[workspaceIdx].Repos[repoIdx].Health = RepoInvalid
	state.Workspaces[workspaceIdx].UpdatedAt = time.Now().UTC()
	return s.store.Save(context.Background(), state)
}

func (s *Service) UpdateRepoPath(workspaceID, repoID, newPath string) error {
	if strings.TrimSpace(newPath) == "" {
		return ErrRepoPathMissing
	}

	state, err := s.LoadState()
	if err != nil {
		return err
	}

	workspaceIdx := findWorkspaceIndex(state.Workspaces, workspaceID)
	if workspaceIdx < 0 {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, workspaceID)
	}

	repoIdx := findRepoIndex(state.Workspaces[workspaceIdx].Repos, repoID)
	if repoIdx < 0 {
		return fmt.Errorf("%w: %s", ErrRepoNotFound, repoID)
	}

	state.Workspaces[workspaceIdx].Repos[repoIdx].Path = newPath
	state.Workspaces[workspaceIdx].Repos[repoIdx].Health = RepoHealthy
	state.Workspaces[workspaceIdx].UpdatedAt = time.Now().UTC()
	return s.store.Save(context.Background(), state)
}

func (s *Service) RemoveRepo(workspaceID, repoID string) error {
	state, err := s.LoadState()
	if err != nil {
		return err
	}

	workspaceIdx := findWorkspaceIndex(state.Workspaces, workspaceID)
	if workspaceIdx < 0 {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, workspaceID)
	}

	repoIdx := findRepoIndex(state.Workspaces[workspaceIdx].Repos, repoID)
	if repoIdx < 0 {
		return fmt.Errorf("%w: %s", ErrRepoNotFound, repoID)
	}

	state.Workspaces[workspaceIdx].Repos = append(
		state.Workspaces[workspaceIdx].Repos[:repoIdx],
		state.Workspaces[workspaceIdx].Repos[repoIdx+1:]...,
	)
	if state.Workspaces[workspaceIdx].SelectedRepoID == repoID {
		if len(state.Workspaces[workspaceIdx].Repos) == 0 {
			state.Workspaces[workspaceIdx].SelectedRepoID = ""
		} else {
			state.Workspaces[workspaceIdx].SelectedRepoID = state.Workspaces[workspaceIdx].Repos[0].ID
		}
	}
	state.Workspaces[workspaceIdx].UpdatedAt = time.Now().UTC()
	return s.store.Save(context.Background(), state)
}

func findWorkspaceIndex(workspaces []Workspace, workspaceID string) int {
	for i := range workspaces {
		if workspaces[i].ID == workspaceID {
			return i
		}
	}
	return -1
}

func containsRepo(repos []Repo, repoID string) bool {
	for _, repo := range repos {
		if repo.ID == repoID {
			return true
		}
	}
	return false
}

func findRepoIndex(repos []Repo, repoID string) int {
	for i := range repos {
		if repos[i].ID == repoID {
			return i
		}
	}
	return -1
}

var idSequence atomic.Uint64

func newID(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), idSequence.Add(1))
}
