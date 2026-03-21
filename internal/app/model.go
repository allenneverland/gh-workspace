package app

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	diffadapter "github.com/allenneverland/gh-workspace/internal/adapters/diff"
	lazygitadapter "github.com/allenneverland/gh-workspace/internal/adapters/lazygit"
	worktreeadapter "github.com/allenneverland/gh-workspace/internal/adapters/worktree"
	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	storepkg "github.com/allenneverland/gh-workspace/internal/store"
	syncengine "github.com/allenneverland/gh-workspace/internal/sync"
)

type Tab string

const (
	TabOverview  Tab = "overview"
	TabWorktrees Tab = "worktrees"
	TabLazygit   Tab = "lazygit"
	TabDiff      Tab = "diff"
)

type UIMode string

const (
	ModeWorkspace UIMode = "workspace"
	ModeFolder    UIMode = "folder"
)

type Config struct {
	InitialState          workspace.State
	InitialUIMode         UIMode
	WorktreeAdapter       WorktreeAdapter
	LazygitSessionManager LazygitSessionManager
	DiffRenderer          DiffRenderer
	SyncEngine            SyncEngine
	StateStore            storepkg.Store
	SyncStatePublisher    SyncStatePublisher
	RepoPathSubmitter     RepoPathSubmitter
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
	ResizeSession(sessionID string, cols, rows int) error
	Frames() <-chan LazygitFrame
}

type DiffRenderer interface {
	Render(ctx context.Context, repoPath string) (string, error)
}

type SyncEngine interface {
	SetSelection(workspaceID, repoID string)
	RefreshNow(ctx context.Context) (workspace.RepoStatus, error)
	OnTick(ctx context.Context) (workspace.RepoStatus, error)
	OnSelectionChanged(ctx context.Context, workspaceID, repoID string) (workspace.RepoStatus, error)
	Start(ctx context.Context) tea.Cmd
	SetAutoPolling(enabled bool)
	AutoPollingEnabled() bool
}

type SyncStatePublisher interface {
	SetState(workspace.State)
}

type RepoPathSubmissionResult struct {
	State         workspace.State
	StatusMessage string
}

type RepoPathSubmitter interface {
	SubmitRepoPath(ctx context.Context, path string) (RepoPathSubmissionResult, error)
}

type WorkspaceState struct {
	Snapshot workspace.State
}

type RepoStatusSnapshot = workspace.RepoStatusSnapshot

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
	cloned.RepoStatusSnapshots = cloneRepoStatusSnapshots(st.RepoStatusSnapshots)
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

func isSystemWorkspaceEntry(ws workspace.Workspace) bool {
	return ws.ID == workspace.LocalWorkspaceID || ws.Name == workspace.LocalWorkspaceName
}

func userWorkspaceIDs(workspaces []workspace.Workspace) []string {
	ids := make([]string, 0, len(workspaces))
	for _, ws := range workspaces {
		if isSystemWorkspaceEntry(ws) {
			continue
		}
		ids = append(ids, ws.ID)
	}
	return ids
}

func normalizeWorkspaceModeState(state *WorkspaceState) bool {
	if state == nil {
		return false
	}

	userIDs := userWorkspaceIDs(state.Snapshot.Workspaces)
	if len(userIDs) == 0 {
		if state.Snapshot.SelectedWorkspaceID == "" {
			return false
		}
		state.Snapshot.SelectedWorkspaceID = ""
		return true
	}

	for _, id := range userIDs {
		if id == state.Snapshot.SelectedWorkspaceID {
			return false
		}
	}

	state.Snapshot.SelectedWorkspaceID = userIDs[0]
	state.ensureSelection()
	return true
}

type Model struct {
	ActiveTab                    Tab
	UIMode                       UIMode
	LeftPaneWidth                int
	CenterPaneWidth              int
	RightPaneWidth               int
	WindowWidth                  int
	WindowHeight                 int
	State                        WorkspaceState
	Keys                         KeyMap
	AddRepoRequested             bool
	RepoPathInput                RepoPathInput
	RepoPathInputActive          bool
	StatusMessage                string
	StateStore                   storepkg.Store
	SyncStatePublisher           SyncStatePublisher
	RepoPathSubmitter            RepoPathSubmitter
	WorktreeAdapter              WorktreeAdapter
	Worktrees                    []WorktreeItem
	LazygitSessionManager        LazygitSessionManager
	LazygitSessionID             string
	LazygitCenterFrameText       string
	DiffRenderer                 DiffRenderer
	SyncEngine                   SyncEngine
	DiffOutput                   string
	DiffStatus                   string
	DiffLoading                  bool
	diffRenderRequestID          int
	diffRenderInFlight           bool
	diffRenderPending            bool
	syncCommandInFlight          bool
	syncRefreshPending           bool
	syncTickPending              bool
	lazygitFrameListenerInFlight bool
}

func NewModel(config Config) Model {
	state := cloneWorkspaceState(config.InitialState)
	if state.RepoStatusSnapshots == nil {
		state.RepoStatusSnapshots = make(map[string]workspace.RepoStatusSnapshot)
	}

	mode := config.InitialUIMode
	if mode == "" {
		mode = ModeWorkspace
	}

	m := Model{
		ActiveTab:             TabOverview,
		UIMode:                mode,
		LeftPaneWidth:         30,
		CenterPaneWidth:       80,
		RightPaneWidth:        40,
		State:                 NewWorkspaceState(state),
		Keys:                  DefaultKeyMap(),
		RepoPathInput:         newRepoPathInput(),
		StateStore:            config.StateStore,
		SyncStatePublisher:    config.SyncStatePublisher,
		RepoPathSubmitter:     config.RepoPathSubmitter,
		WorktreeAdapter:       config.WorktreeAdapter,
		LazygitSessionManager: config.LazygitSessionManager,
		DiffRenderer:          config.DiffRenderer,
		SyncEngine:            config.SyncEngine,
	}
	if m.WorktreeAdapter == nil {
		m.WorktreeAdapter = appWorktreeAdapter{inner: worktreeadapter.NewAdapter(nil)}
	}
	if m.LazygitSessionManager == nil {
		m.LazygitSessionManager = lazygitadapter.NewSessionManager()
	}
	if m.DiffRenderer == nil {
		m.DiffRenderer = diffadapter.NewRenderer()
	}
	if m.SyncEngine == nil {
		m.SyncEngine = syncengine.NewEngine(syncengine.NoopSelectedRepoStatusFetcher{})
	}
	if m.UIMode == ModeWorkspace {
		_ = normalizeWorkspaceModeState(&m.State)
	}
	return publishSyncState(m)
}

func (m Model) Init() tea.Cmd {
	if m.SyncEngine == nil {
		return nil
	}
	return func() tea.Msg {
		return MsgSyncStartup{}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := updateModel(m.cloneForUpdate(), msg)
	return updated, cmd
}

func (m Model) View() string {
	return renderView(m)
}

func (m Model) CenterTabs() []Tab {
	return []Tab{TabOverview, TabWorktrees, TabLazygit, TabDiff}
}

func (m Model) cloneForUpdate() Model {
	cloned := m
	cloned.State = WorkspaceState{Snapshot: cloneWorkspaceState(m.State.Snapshot)}
	return cloned
}

func cloneRepoStatusSnapshots(src map[string]RepoStatusSnapshot) map[string]RepoStatusSnapshot {
	if src == nil {
		return nil
	}
	cloned := make(map[string]RepoStatusSnapshot, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func repoStatusSnapshotKey(workspaceID, repoID string) string {
	return strings.TrimSpace(workspaceID) + "/" + strings.TrimSpace(repoID)
}

func (m Model) repoStatusSnapshotForCurrentRepo() RepoStatusSnapshot {
	repo, ok := m.State.CurrentRepo()
	if !ok {
		return RepoStatusSnapshot{
			PR:      workspace.StatusNeutral,
			CI:      workspace.StatusNeutral,
			Release: workspace.StatusUnconfigured,
		}
	}
	hasReleaseWorkflow := strings.TrimSpace(repo.ReleaseWorkflowRef) != ""

	snapshot := RepoStatusSnapshot{
		PR:         workspace.StatusNeutral,
		CI:         workspace.StatusNeutral,
		Release:    workspace.StatusNeutral,
		ReleaseRun: workspace.ReleaseRun{},
	}

	key := repoStatusSnapshotKey(m.State.CurrentWorkspaceID(), repo.ID)
	stored, ok := m.State.Snapshot.RepoStatusSnapshots[key]
	if !ok {
		if !hasReleaseWorkflow {
			snapshot.Release = workspace.StatusUnconfigured
		}
		return snapshot
	}

	if stored.PR != "" {
		snapshot.PR = stored.PR
	}
	if stored.CI != "" {
		snapshot.CI = stored.CI
	}
	if stored.Release != "" {
		snapshot.Release = stored.Release
	}
	snapshot.ReleaseRun = stored.ReleaseRun
	snapshot.LastSyncedAt = stored.LastSyncedAt
	snapshot.IsStale = stored.IsStale
	snapshot.LatestError = stored.LatestError
	if !hasReleaseWorkflow {
		snapshot.Release = workspace.StatusUnconfigured
		snapshot.ReleaseRun = workspace.ReleaseRun{}
	}
	return snapshot
}

func (m Model) repoStatusSnapshotByKey(workspaceID, repoID string) RepoStatusSnapshot {
	if m.State.Snapshot.RepoStatusSnapshots == nil {
		return RepoStatusSnapshot{}
	}
	return m.State.Snapshot.RepoStatusSnapshots[repoStatusSnapshotKey(workspaceID, repoID)]
}

func (m *Model) setRepoStatusSnapshot(workspaceID, repoID string, snapshot RepoStatusSnapshot) {
	if m == nil {
		return
	}
	if m.State.Snapshot.RepoStatusSnapshots == nil {
		m.State.Snapshot.RepoStatusSnapshots = make(map[string]workspace.RepoStatusSnapshot)
	}
	m.State.Snapshot.RepoStatusSnapshots[repoStatusSnapshotKey(workspaceID, repoID)] = snapshot
}

func publishSyncState(m Model) Model {
	if m.SyncStatePublisher == nil {
		return m
	}
	m.SyncStatePublisher.SetState(cloneWorkspaceState(m.State.Snapshot))
	return m
}

func persistWorkspaceState(m Model) Model {
	if m.StateStore == nil {
		return m
	}
	if err := m.StateStore.Save(context.Background(), cloneWorkspaceState(m.State.Snapshot)); err != nil {
		m.StatusMessage = "failed to persist state: " + err.Error()
	}
	return m
}

func persistAndPublishState(m Model) Model {
	m = publishSyncState(m)
	return persistWorkspaceState(m)
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
