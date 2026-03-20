package app

import (
	"strings"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func renderView(m Model) string {
	return m.renderLeftPane()
}

func (m Model) renderLeftPane() string {
	var out strings.Builder

	out.WriteString("Workspaces\n")
	for _, ws := range m.State.Snapshot.Workspaces {
		marker := "  "
		if ws.ID == m.State.Snapshot.SelectedWorkspaceID {
			marker = "> "
		}
		out.WriteString(marker + ws.Name + "\n")
	}

	out.WriteString("\nRepos\n")
	if ws, ok := m.State.CurrentWorkspace(); ok {
		for _, repo := range ws.Repos {
			marker := "  "
			if repo.ID == ws.SelectedRepoID {
				marker = "* "
			}
			label := repo.Name
			if repo.Health == workspace.RepoInvalid {
				label += " [invalid]"
			}
			out.WriteString(marker + label + "\n")
		}
	}

	out.WriteString("\na: add repo path")
	if repo, ok := m.State.CurrentRepo(); ok && repo.Health == workspace.RepoInvalid {
		out.WriteString("\nenter: fix path")
		out.WriteString("\nx: remove repo")
	}
	if m.StatusMessage != "" {
		out.WriteString("\n\nstatus: " + m.StatusMessage)
	}

	return out.String()
}
