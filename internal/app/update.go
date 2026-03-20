package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type MsgSelectWorkspace struct {
	WorkspaceID string
}

type MsgSelectRepo struct {
	RepoID string
}

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MsgSelectWorkspace:
		m.State.SelectWorkspace(msg.WorkspaceID)
	case MsgSelectRepo:
		m.State.SelectRepo(msg.RepoID)
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.Keys.NextWorkspace):
			m.State.SelectNextWorkspace()
		case key.Matches(msg, m.Keys.PrevWorkspace):
			m.State.SelectPrevWorkspace()
		}
	}

	return m, nil
}
