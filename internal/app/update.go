package app

import (
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

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MsgSelectWorkspace:
		m.State.SelectWorkspace(msg.WorkspaceID)
	case MsgSelectRepo:
		m.State.SelectRepo(msg.RepoID)
	case MsgRequestAddRepo:
		m.AddRepoRequested = true
		m.StatusMessage = "add repo requested"
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
		case key.Matches(msg, m.Keys.FixRepoPath):
			m = attemptRepoRecovery(m)
		case key.Matches(msg, m.Keys.NextWorkspace):
			m.State.SelectNextWorkspace()
		case key.Matches(msg, m.Keys.PrevWorkspace):
			m.State.SelectPrevWorkspace()
		}
	}

	return m, nil
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
