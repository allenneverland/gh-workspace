package app

import (
	"context"
	"os"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type MsgSelectWorkspace struct {
	WorkspaceID string
}

type MsgSelectRepo struct {
	RepoID string
}

type MsgRequestAddRepo struct{}

type MsgCreateWorktree struct {
	Branch string
	Path   string
}

type MsgSwitchWorktree struct {
	WorktreePath string
}

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MsgSelectWorkspace:
		m.State.SelectWorkspace(msg.WorkspaceID)
	case MsgSelectRepo:
		m.State.SelectRepo(msg.RepoID)
	case MsgRequestAddRepo:
		m.AddRepoRequested = true
		m.StatusMessage = "add repo requested"
	case MsgCreateWorktree:
		m = createWorktree(m, msg)
	case MsgSwitchWorktree:
		m = switchWorktree(m, msg)
	case tea.KeyMsg:
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
