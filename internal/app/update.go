package app

import (
	"context"
	"os"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MsgSelectWorkspace:
		m.State.SelectWorkspace(msg.WorkspaceID)
		if m.ActiveTab == TabLazygit {
			m = ensureLazygitSession(m)
			return scheduleLazygitFrameWait(m)
		}
		if m.ActiveTab == diffTab {
			return m, nil
		}
	case MsgSelectRepo:
		m.State.SelectRepo(msg.RepoID)
		if m.ActiveTab == TabLazygit {
			m = ensureLazygitSession(m)
			return scheduleLazygitFrameWait(m)
		}
		if m.ActiveTab == diffTab {
			return m, nil
		}
	case MsgSetActiveTab:
		m.ActiveTab = msg.Tab
		if m.ActiveTab == TabLazygit {
			m = ensureLazygitSession(m)
			return scheduleLazygitFrameWait(m)
		}
		if m.ActiveTab == diffTab {
			// Diff tab remains read-only in v1 and renders directly from view.
			return m, nil
		}
	case MsgRequestAddRepo:
		m.AddRepoRequested = true
		m.StatusMessage = "add repo requested"
	case MsgCreateWorktree:
		m = createWorktree(m, msg)
	case MsgSwitchWorktree:
		m = switchWorktree(m, msg)
	case MsgLazygitFrame:
		m.lazygitFrameListenerInFlight = false
		if msg.SessionID == m.LazygitSessionID {
			m.LazygitCenterFrameText += string(msg.Data)
		}
		return scheduleLazygitFrameWait(m)
	case MsgLazygitFrameClosed:
		m.lazygitFrameListenerInFlight = false
	case tea.KeyMsg:
		if lazygitOwnsKeys(m) {
			return forwardLazygitInput(m, msg), nil
		}
		switch {
		case key.Matches(msg, m.Keys.AddRepo):
			return m, func() tea.Msg { return MsgRequestAddRepo{} }
		case key.Matches(msg, m.Keys.RemoveRepo):
			if removed, ok := m.State.RemoveCurrentRepo(); ok {
				m.StatusMessage = "removed repo: " + removed.Name
			} else {
				m.StatusMessage = "no selected repo to remove"
			}
		case key.Matches(msg, m.Keys.SelectRepo):
			m = attemptRepoRecovery(m)
		case key.Matches(msg, m.Keys.NextWorkspace):
			m.State.SelectNextWorkspace()
		case key.Matches(msg, m.Keys.PrevWorkspace):
			m.State.SelectPrevWorkspace()
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
