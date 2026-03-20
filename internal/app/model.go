package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

type Tab string

const (
	TabOverview Tab = "overview"
)

type Config struct {
	InitialState workspace.State
}

type WorkspaceState struct {
	Snapshot workspace.State
}

func NewWorkspaceState(st workspace.State) WorkspaceState {
	state := WorkspaceState{Snapshot: st}
	state.ensureSelection()
	return state
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

type Model struct {
	ActiveTab       Tab
	LeftPaneWidth   int
	CenterPaneWidth int
	RightPaneWidth  int
	State           WorkspaceState
	Keys            KeyMap
}

func NewModel(config Config) Model {
	return Model{
		ActiveTab:       TabOverview,
		LeftPaneWidth:   30,
		CenterPaneWidth: 80,
		RightPaneWidth:  40,
		State:           NewWorkspaceState(config.InitialState),
		Keys:            DefaultKeyMap(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := updateModel(m, msg)
	return updated, cmd
}

func (m Model) View() string {
	return renderView(m)
}
