package app

import (
	"context"
	"errors"
	"os"
	"strings"

	diffadapter "github.com/allenneverland/gh-workspace/internal/adapters/diff"
	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	syncengine "github.com/allenneverland/gh-workspace/internal/sync"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MsgSelectWorkspace:
		m.State.SelectWorkspace(msg.WorkspaceID)
		syncCmd := syncOnSelectionChangedCmd(m)
		if m.ActiveTab == TabLazygit {
			m = ensureLazygitSession(m)
			next, lazygitCmd := scheduleLazygitFrameWait(m)
			return next, tea.Batch(syncCmd, lazygitCmd)
		}
		if m.ActiveTab == TabDiff {
			next, diffCmd := requestDiffRender(m)
			return next, tea.Batch(syncCmd, diffCmd)
		}
		return m, syncCmd
	case MsgSelectRepo:
		m.State.SelectRepo(msg.RepoID)
		syncCmd := syncOnSelectionChangedCmd(m)
		if m.ActiveTab == TabLazygit {
			m = ensureLazygitSession(m)
			next, lazygitCmd := scheduleLazygitFrameWait(m)
			return next, tea.Batch(syncCmd, lazygitCmd)
		}
		if m.ActiveTab == TabDiff {
			next, diffCmd := requestDiffRender(m)
			return next, tea.Batch(syncCmd, diffCmd)
		}
		return m, syncCmd
	case MsgSyncStartup:
		syncSetSelection(m)
		return m, tea.Batch(syncRefreshNowCmd(m), scheduleSyncPolling(m))
	case MsgRefreshSelectedRepo:
		syncSetSelection(m)
		return m, syncRefreshNowCmd(m)
	case MsgToggleAutoPolling:
		return toggleAutoPolling(m)
	case MsgSyncRefreshCompleted:
		if msg.Err != nil {
			m.StatusMessage = "failed to refresh selected repo: " + msg.Err.Error()
		}
	case MsgSetActiveTab:
		m.ActiveTab = msg.Tab
		if m.ActiveTab == TabLazygit {
			m = ensureLazygitSession(m)
			return scheduleLazygitFrameWait(m)
		}
		if m.ActiveTab == TabDiff {
			return requestDiffRender(m)
		}
	case MsgRefreshDiff:
		if m.ActiveTab == TabDiff {
			return requestDiffRender(m)
		}
	case MsgRequestAddRepo:
		if m.ActiveTab == TabDiff {
			return m, nil
		}
		m.AddRepoRequested = true
		m.StatusMessage = "add repo requested"
	case MsgCreateWorktree:
		if m.ActiveTab == TabDiff {
			return m, nil
		}
		m = createWorktree(m, msg)
	case MsgSwitchWorktree:
		if m.ActiveTab == TabDiff {
			return m, nil
		}
		m = switchWorktree(m, msg)
	case MsgDiffRendered:
		m = applyDiffRenderResult(m, msg)
	case MsgLazygitFrame:
		m.lazygitFrameListenerInFlight = false
		if msg.SessionID == m.LazygitSessionID {
			m.LazygitCenterFrameText += string(msg.Data)
		}
		return scheduleLazygitFrameWait(m)
	case MsgLazygitFrameClosed:
		m.lazygitFrameListenerInFlight = false
	case syncengine.MsgTick:
		return m, tea.Batch(syncOnTickCmd(m), scheduleSyncPolling(m))
	case tea.KeyMsg:
		if lazygitOwnsKeys(m) {
			return forwardLazygitInput(m, msg), nil
		}
		if diffTabBlocksMutatingKeys(m, msg) {
			return m, nil
		}
		switch {
		case key.Matches(msg, m.Keys.AddRepo):
			return m, func() tea.Msg { return MsgRequestAddRepo{} }
		case key.Matches(msg, m.Keys.RemoveRepo):
			if removed, ok := m.State.RemoveCurrentRepo(); ok {
				m.StatusMessage = "removed repo: " + removed.Name
				return m, syncOnSelectionChangedCmd(m)
			} else {
				m.StatusMessage = "no selected repo to remove"
			}
		case key.Matches(msg, m.Keys.SelectRepo):
			m = attemptRepoRecovery(m)
		case key.Matches(msg, m.Keys.ManualRefresh):
			return m, func() tea.Msg { return MsgRefreshSelectedRepo{} }
		case key.Matches(msg, m.Keys.TogglePolling):
			return m, func() tea.Msg { return MsgToggleAutoPolling{} }
		case key.Matches(msg, m.Keys.NextWorkspace):
			m.State.SelectNextWorkspace()
			return m, syncOnSelectionChangedCmd(m)
		case key.Matches(msg, m.Keys.PrevWorkspace):
			m.State.SelectPrevWorkspace()
			return m, syncOnSelectionChangedCmd(m)
		}
	}

	return m, nil
}

func ensureLazygitSession(m Model) Model {
	repo, ok := m.State.CurrentRepo()
	if !ok {
		m = clearLazygitSessionState(m)
		m.StatusMessage = "請先選擇 repo"
		return m
	}
	if m.LazygitSessionManager == nil {
		m = clearLazygitSessionState(m)
		m.StatusMessage = "lazygit session manager unavailable"
		return m
	}

	handle, err := m.LazygitSessionManager.StartSession(repo.Path)
	if err != nil {
		m = clearLazygitSessionState(m)
		m.StatusMessage = "failed to start lazygit session: " + err.Error()
		return m
	}

	if m.LazygitSessionID != handle.ID {
		m.LazygitCenterFrameText = ""
	}
	m.LazygitSessionID = handle.ID
	return m
}

func clearLazygitSessionState(m Model) Model {
	m.LazygitSessionID = ""
	m.LazygitCenterFrameText = ""
	return m
}

func scheduleLazygitFrameWait(m Model) (Model, tea.Cmd) {
	if m.ActiveTab != TabLazygit || m.LazygitSessionManager == nil || m.LazygitSessionID == "" {
		return m, nil
	}
	if m.lazygitFrameListenerInFlight {
		return m, nil
	}

	cmd := waitForLazygitFrame(m.LazygitSessionManager)
	if cmd == nil {
		return m, nil
	}
	m.lazygitFrameListenerInFlight = true
	return m, cmd
}

func lazygitOwnsKeys(m Model) bool {
	return m.ActiveTab == TabLazygit && m.LazygitSessionManager != nil && m.LazygitSessionID != ""
}

func diffTabBlocksMutatingKeys(m Model, msg tea.KeyMsg) bool {
	if m.ActiveTab != TabDiff {
		return false
	}
	return key.Matches(msg, m.Keys.AddRepo) ||
		key.Matches(msg, m.Keys.RemoveRepo) ||
		key.Matches(msg, m.Keys.SelectRepo) ||
		key.Matches(msg, m.Keys.NextWorkspace) ||
		key.Matches(msg, m.Keys.PrevWorkspace)
}

func syncOnSelectionChangedCmd(m Model) tea.Cmd {
	syncSetSelection(m)
	return syncRefreshNowCmd(m)
}

func syncSetSelection(m Model) {
	if m.SyncEngine == nil {
		return
	}
	m.SyncEngine.SetSelection(m.State.CurrentWorkspaceID(), m.State.CurrentRepoID())
}

func syncRefreshNowCmd(m Model) tea.Cmd {
	if m.SyncEngine == nil {
		return nil
	}

	return func() tea.Msg {
		_, err := m.SyncEngine.RefreshNow(context.Background())
		return MsgSyncRefreshCompleted{Err: err}
	}
}

func syncOnTickCmd(m Model) tea.Cmd {
	if m.SyncEngine == nil {
		return nil
	}

	return func() tea.Msg {
		_, err := m.SyncEngine.OnTick(context.Background())
		return MsgSyncRefreshCompleted{Err: err}
	}
}

func scheduleSyncPolling(m Model) tea.Cmd {
	if m.SyncEngine == nil || !m.SyncEngine.AutoPollingEnabled() {
		return nil
	}
	return m.SyncEngine.Start(context.Background())
}

func toggleAutoPolling(m Model) (Model, tea.Cmd) {
	if m.SyncEngine == nil {
		return m, nil
	}

	enabled := !m.SyncEngine.AutoPollingEnabled()
	m.SyncEngine.SetAutoPolling(enabled)
	if !enabled {
		return m, nil
	}
	return m, scheduleSyncPolling(m)
}

func requestDiffRender(m Model) (Model, tea.Cmd) {
	if m.ActiveTab != TabDiff {
		return m, nil
	}

	repo, ok := m.State.CurrentRepo()
	if !ok || strings.TrimSpace(repo.Path) == "" {
		m.DiffLoading = false
		m.DiffOutput = ""
		m.DiffStatus = ""
		return m, nil
	}
	if m.DiffRenderer == nil {
		m.DiffLoading = false
		m.DiffOutput = ""
		m.DiffStatus = "diff renderer unavailable"
		return m, nil
	}

	m.diffRenderRequestID++
	requestID := m.diffRenderRequestID
	repoPath := repo.Path
	m.DiffLoading = true
	m.DiffStatus = ""

	return m, func() tea.Msg {
		out, err := m.DiffRenderer.Render(context.Background(), repoPath)
		return MsgDiffRendered{
			RequestID: requestID,
			Output:    out,
			Err:       err,
		}
	}
}

func applyDiffRenderResult(m Model, msg MsgDiffRendered) Model {
	if msg.RequestID != m.diffRenderRequestID {
		return m
	}

	m.DiffLoading = false
	if msg.Err != nil {
		m.DiffOutput = ""
		if errors.Is(msg.Err, diffadapter.ErrDeltaNotFound) {
			m.DiffStatus = "delta not found; install delta to use Diff tab"
			return m
		}
		m.DiffStatus = "failed to render diff: " + msg.Err.Error()
		return m
	}

	m.DiffOutput = msg.Output
	if strings.TrimSpace(msg.Output) == "" {
		m.DiffStatus = "(no changes)"
		return m
	}

	m.DiffStatus = ""
	return m
}

func waitForLazygitFrame(manager LazygitSessionManager) tea.Cmd {
	if manager == nil {
		return nil
	}
	frames := manager.Frames()
	if frames == nil {
		return nil
	}

	return func() tea.Msg {
		frame, ok := <-frames
		if !ok {
			return MsgLazygitFrameClosed{}
		}
		return MsgLazygitFrame{
			SessionID: frame.SessionID,
			Data:      frame.Data,
		}
	}
}

func forwardLazygitInput(m Model, msg tea.KeyMsg) Model {
	if m.ActiveTab != TabLazygit {
		return m
	}
	if m.LazygitSessionManager == nil || m.LazygitSessionID == "" {
		return m
	}

	input, ok := keyMsgToBytes(msg)
	if !ok {
		return m
	}

	if err := m.LazygitSessionManager.WriteInput(m.LazygitSessionID, input); err != nil {
		m.StatusMessage = "failed to write lazygit input: " + err.Error()
	}
	return m
}

func keyMsgToBytes(msg tea.KeyMsg) ([]byte, bool) {
	switch msg.Type {
	case tea.KeyRunes:
		return []byte(string(msg.Runes)), true
	case tea.KeyEnter:
		return []byte{'\r'}, true
	case tea.KeyBackspace:
		return []byte{0x7f}, true
	case tea.KeySpace:
		return []byte{' '}, true
	case tea.KeyTab:
		return []byte{'\t'}, true
	case tea.KeyUp:
		return []byte("\x1b[A"), true
	case tea.KeyDown:
		return []byte("\x1b[B"), true
	case tea.KeyRight:
		return []byte("\x1b[C"), true
	case tea.KeyLeft:
		return []byte("\x1b[D"), true
	default:
		return nil, false
	}
}

func createWorktree(m Model, msg MsgCreateWorktree) Model {
	repo, ok := m.State.CurrentRepo()
	if !ok {
		m.StatusMessage = "no selected repo for worktree create"
		return m
	}
	if m.WorktreeAdapter == nil {
		m.StatusMessage = "worktree adapter unavailable"
		return m
	}

	if err := m.WorktreeAdapter.Create(context.Background(), repo.Path, msg.Branch, msg.Path); err != nil {
		m.StatusMessage = "failed to create worktree: " + err.Error()
		return m
	}

	worktrees, err := m.WorktreeAdapter.List(context.Background(), repo.Path)
	if err != nil {
		m.StatusMessage = "failed to refresh worktrees after create: " + err.Error()
		return m
	}
	m.Worktrees = worktrees

	m.StatusMessage = "created worktree: " + msg.Path
	return m
}

func switchWorktree(m Model, msg MsgSwitchWorktree) Model {
	repo, ok := m.State.CurrentRepo()
	if !ok {
		m.StatusMessage = "no selected repo for worktree switch"
		return m
	}
	if m.WorktreeAdapter == nil {
		m.StatusMessage = "worktree adapter unavailable"
		return m
	}

	worktrees, err := m.WorktreeAdapter.List(context.Background(), repo.Path)
	if err != nil {
		m.StatusMessage = "failed to list worktrees: " + err.Error()
		return m
	}
	m.Worktrees = worktrees

	selected, exists := findWorktreeByPath(worktrees, msg.WorktreePath)
	if !exists {
		m.StatusMessage = "worktree not found in list: " + msg.WorktreePath
		return m
	}

	if err := m.WorktreeAdapter.ValidateSwitchTarget(context.Background(), msg.WorktreePath); err != nil {
		m.StatusMessage = "failed to validate worktree target: " + err.Error()
		return m
	}

	if !m.State.SetRepoSelectedWorktree(m.State.CurrentWorkspaceID(), repo.ID, selected.ID, selected.Path) {
		m.StatusMessage = "failed to persist selected worktree"
		return m
	}
	m.StatusMessage = "switched worktree: " + selected.Path
	return m
}

func findWorktreeByPath(worktrees []WorktreeItem, path string) (WorktreeItem, bool) {
	for _, wt := range worktrees {
		if wt.Path == path {
			return wt, true
		}
	}
	return WorktreeItem{}, false
}

func attemptRepoRecovery(m Model) Model {
	repo, ok := m.State.CurrentRepo()
	if !ok {
		m.StatusMessage = "no selected repo to recover"
		return m
	}

	if repo.Health != workspace.RepoInvalid {
		m.StatusMessage = "selected repo is already healthy"
		return m
	}

	if pathExists(repo.Path) {
		m.State.SetCurrentRepoHealth(workspace.RepoHealthy)
		m.StatusMessage = "repo path recovered: " + repo.Path
		return m
	}

	m.StatusMessage = "repo path still invalid: " + repo.Path
	return m
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
