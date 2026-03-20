package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	diffadapter "github.com/allenneverland/gh-workspace/internal/adapters/diff"
	lazygitadapter "github.com/allenneverland/gh-workspace/internal/adapters/lazygit"
	worktreeadapter "github.com/allenneverland/gh-workspace/internal/adapters/worktree"
	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

type Tab string

const (
	TabOverview Tab = "overview"
	TabLazygit  Tab = "lazygit"
	TabDiff     Tab = "diff"
)

type Config struct {
	InitialState workspace.State
}

type WorktreeItem struct {
	ID     string
	Path   string
	Branch string
	Commit string
}

type WorktreeAdapter interface {
	Create(ctx context.Context, repoPath, branch, targetPath string) error
	List(ctx context.Context, repoPath string) ([]WorktreeItem, error)
	ValidateSwitchTarget(ctx context.Context, worktreePath string) error
}

type LazygitSessionHandle = lazygitadapter.SessionHandle
type LazygitFrame = lazygitadapter.Frame

type LazygitSessionManager interface {
	StartSession(repoPath string) (LazygitSessionHandle, error)
	WriteInput(sessionID string, input []byte) error
	Frames() <-chan LazygitFrame
}

type DiffRenderer interface {
	Render(ctx context.Context, repoPath string) (string, error)
}

type WorkspaceState struct {
	Snapshot workspace.State
}

func NewWorkspaceState(st workspace.State) WorkspaceState {
	state := WorkspaceState{Snapshot: cloneWorkspaceState(st)}
	state.ensureSelection()
	return state
}

func cloneWorkspaceState(st workspace.State) workspace.State {
	cloned := st
	cloned.Workspaces = make([]workspace.Workspace, len(st.Workspaces))
	for i := range st.Workspaces {
		ws := st.Workspaces[i]
		ws.Repos = append([]workspace.Repo(nil), ws.Repos...)
		cloned.Workspaces[i] = ws
	}
	return cloned
}

func (s WorkspaceState) CurrentWorkspaceID() string {
	return s.Snapshot.SelectedWorkspaceID
}

func (s WorkspaceState) CurrentWorkspace() (workspace.Workspace, bool) {
	idx := findWorkspaceIndex(s.Snapshot.Workspaces, s.Snapshot.SelectedWorkspaceID)
	if idx < 0 {
		return workspace.Workspace{}, false
	}
	return s.Snapshot.Workspaces[idx], true
}

func (s WorkspaceState) CurrentRepoID() string {
	ws, ok := s.CurrentWorkspace()
	if !ok {
		return ""
	}
	return ws.SelectedRepoID
}

func (s WorkspaceState) CurrentRepo() (workspace.Repo, bool) {
	ws, ok := s.CurrentWorkspace()
	if !ok {
		return workspace.Repo{}, false
	}
	for _, repo := range ws.Repos {
		if repo.ID == ws.SelectedRepoID {
			return repo, true
		}
	}
	return workspace.Repo{}, false
}

func (s *WorkspaceState) SelectWorkspace(workspaceID string) bool {
	idx := findWorkspaceIndex(s.Snapshot.Workspaces, workspaceID)
	if idx < 0 {
		return false
	}
	s.Snapshot.SelectedWorkspaceID = workspaceID
	s.ensureSelection()
	return true
}

func (s *WorkspaceState) SelectRepo(repoID string) bool {
	workspaceIdx := findWorkspaceIndex(s.Snapshot.Workspaces, s.Snapshot.SelectedWorkspaceID)
	if workspaceIdx < 0 {
		return false
	}
	if !containsRepoID(s.Snapshot.Workspaces[workspaceIdx].Repos, repoID) {
		return false
	}

	s.Snapshot.Workspaces[workspaceIdx].SelectedRepoID = repoID
	return true
}

func (s *WorkspaceState) SelectNextWorkspace() bool {
	workspaces := s.Snapshot.Workspaces
	if len(workspaces) == 0 {
		return false
	}

	currentIdx := findWorkspaceIndex(workspaces, s.Snapshot.SelectedWorkspaceID)
	if currentIdx < 0 {
		s.Snapshot.SelectedWorkspaceID = workspaces[0].ID
		s.ensureSelection()
		return true
	}

	nextIdx := (currentIdx + 1) % len(workspaces)
	s.Snapshot.SelectedWorkspaceID = workspaces[nextIdx].ID
	s.ensureSelection()
	return true
}

func (s *WorkspaceState) SelectPrevWorkspace() bool {
	workspaces := s.Snapshot.Workspaces
	if len(workspaces) == 0 {
		return false
	}

	currentIdx := findWorkspaceIndex(workspaces, s.Snapshot.SelectedWorkspaceID)
	if currentIdx < 0 {
		s.Snapshot.SelectedWorkspaceID = workspaces[0].ID
		s.ensureSelection()
		return true
	}

	prevIdx := currentIdx - 1
	if prevIdx < 0 {
		prevIdx = len(workspaces) - 1
	}

	s.Snapshot.SelectedWorkspaceID = workspaces[prevIdx].ID
	s.ensureSelection()
	return true
}

func (s *WorkspaceState) RemoveCurrentRepo() (workspace.Repo, bool) {
	workspaceIdx, repoIdx := s.currentIndexes()
	if workspaceIdx < 0 || repoIdx < 0 {
		return workspace.Repo{}, false
	}

	removed := s.Snapshot.Workspaces[workspaceIdx].Repos[repoIdx]
	s.Snapshot.Workspaces[workspaceIdx].Repos = append(
		s.Snapshot.Workspaces[workspaceIdx].Repos[:repoIdx],
		s.Snapshot.Workspaces[workspaceIdx].Repos[repoIdx+1:]...,
	)
	repos := s.Snapshot.Workspaces[workspaceIdx].Repos
	if len(repos) == 0 {
		s.Snapshot.Workspaces[workspaceIdx].SelectedRepoID = ""
	} else if repoIdx >= len(repos) {
		s.Snapshot.Workspaces[workspaceIdx].SelectedRepoID = repos[len(repos)-1].ID
	} else {
		s.Snapshot.Workspaces[workspaceIdx].SelectedRepoID = repos[repoIdx].ID
	}

	return removed, true
}

func (s *WorkspaceState) SetCurrentRepoHealth(health workspace.RepoHealth) bool {
	workspaceIdx, repoIdx := s.currentIndexes()
	if workspaceIdx < 0 || repoIdx < 0 {
		return false
	}

	s.Snapshot.Workspaces[workspaceIdx].Repos[repoIdx].Health = health
	return true
}

func (s *WorkspaceState) SetRepoSelectedWorktree(workspaceID, repoID, worktreeID, worktreePath string) bool {
	workspaceIdx := findWorkspaceIndex(s.Snapshot.Workspaces, workspaceID)
	if workspaceIdx < 0 {
		return false
	}
	repoIdx := findRepoIndex(s.Snapshot.Workspaces[workspaceIdx].Repos, repoID)
	if repoIdx < 0 {
		return false
	}

	s.Snapshot.Workspaces[workspaceIdx].Repos[repoIdx].SelectedWorktreeID = worktreeID
	s.Snapshot.Workspaces[workspaceIdx].Repos[repoIdx].SelectedWorktreePath = worktreePath
	return true
}

func (s *WorkspaceState) ensureSelection() {
	if len(s.Snapshot.Workspaces) == 0 {
		s.Snapshot.SelectedWorkspaceID = ""
		return
	}

	workspaceIdx := findWorkspaceIndex(s.Snapshot.Workspaces, s.Snapshot.SelectedWorkspaceID)
	if workspaceIdx < 0 {
		workspaceIdx = 0
		s.Snapshot.SelectedWorkspaceID = s.Snapshot.Workspaces[workspaceIdx].ID
	}

	selectedRepoID := s.Snapshot.Workspaces[workspaceIdx].SelectedRepoID
	if selectedRepoID == "" || !containsRepoID(s.Snapshot.Workspaces[workspaceIdx].Repos, selectedRepoID) {
		if len(s.Snapshot.Workspaces[workspaceIdx].Repos) == 0 {
			s.Snapshot.Workspaces[workspaceIdx].SelectedRepoID = ""
		} else {
			s.Snapshot.Workspaces[workspaceIdx].SelectedRepoID = s.Snapshot.Workspaces[workspaceIdx].Repos[0].ID
		}
	}
}

func (s WorkspaceState) currentIndexes() (int, int) {
	workspaceIdx := findWorkspaceIndex(s.Snapshot.Workspaces, s.Snapshot.SelectedWorkspaceID)
	if workspaceIdx < 0 {
		return -1, -1
	}
	selectedRepoID := s.Snapshot.Workspaces[workspaceIdx].SelectedRepoID
	if selectedRepoID == "" {
		return workspaceIdx, -1
	}

	return workspaceIdx, findRepoIndex(s.Snapshot.Workspaces[workspaceIdx].Repos, selectedRepoID)
}

func findWorkspaceIndex(workspaces []workspace.Workspace, workspaceID string) int {
	for i := range workspaces {
		if workspaces[i].ID == workspaceID {
			return i
		}
	}
	return -1
}

func containsRepoID(repos []workspace.Repo, repoID string) bool {
	for _, repo := range repos {
		if repo.ID == repoID {
			return true
		}
	}
	return false
}

func findRepoIndex(repos []workspace.Repo, repoID string) int {
	for i := range repos {
		if repos[i].ID == repoID {
			return i
		}
	}
	return -1
}

type Model struct {
	ActiveTab                    Tab
	LeftPaneWidth                int
	CenterPaneWidth              int
	RightPaneWidth               int
	State                        WorkspaceState
	Keys                         KeyMap
	AddRepoRequested             bool
	StatusMessage                string
	WorktreeAdapter              WorktreeAdapter
	Worktrees                    []WorktreeItem
	LazygitSessionManager        LazygitSessionManager
	LazygitSessionID             string
	LazygitCenterFrameText       string
	DiffRenderer                 DiffRenderer
	DiffOutput                   string
	DiffStatus                   string
	DiffLoading                  bool
	diffRenderRequestID          int
	lazygitFrameListenerInFlight bool
}

func NewModel(config Config) Model {
	return Model{
		ActiveTab:             TabOverview,
		LeftPaneWidth:         30,
		CenterPaneWidth:       80,
		RightPaneWidth:        40,
		State:                 NewWorkspaceState(config.InitialState),
		Keys:                  DefaultKeyMap(),
		WorktreeAdapter:       appWorktreeAdapter{inner: worktreeadapter.NewAdapter(nil)},
		LazygitSessionManager: lazygitadapter.NewSessionManager(),
		DiffRenderer:          diffadapter.NewRenderer(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := updateModel(m.cloneForUpdate(), msg)
	return updated, cmd
}

func (m Model) View() string {
	return renderView(m)
}

func (m Model) cloneForUpdate() Model {
	cloned := m
	cloned.State = WorkspaceState{Snapshot: cloneWorkspaceState(m.State.Snapshot)}
	return cloned
}

type appWorktreeAdapter struct {
	inner *worktreeadapter.Adapter
}

func (a appWorktreeAdapter) Create(ctx context.Context, repoPath, branch, targetPath string) error {
	return a.inner.Create(ctx, repoPath, branch, targetPath)
}

func (a appWorktreeAdapter) List(ctx context.Context, repoPath string) ([]WorktreeItem, error) {
	entries, err := a.inner.List(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	items := make([]WorktreeItem, len(entries))
	for i := range entries {
		items[i] = WorktreeItem{
			ID:     entries[i].ID,
			Path:   entries[i].Path,
			Branch: entries[i].Branch,
			Commit: entries[i].Commit,
		}
	}
	return items, nil
}

func (a appWorktreeAdapter) ValidateSwitchTarget(ctx context.Context, worktreePath string) error {
	return a.inner.ValidateSwitchTarget(ctx, worktreePath)
}
