package app

import (
	"strings"
	"time"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func renderView(m Model) string {
	var out strings.Builder
	out.WriteString(m.renderLeftPane())
	out.WriteString("\n\nCenter")
	out.WriteString("\n" + m.renderCenterTabs())
	out.WriteString("\nactive tab: " + string(m.ActiveTab))
	switch m.ActiveTab {
	case TabOverview:
		out.WriteString("\n")
		out.WriteString("overview ready")
	case TabWorktrees:
		out.WriteString("\n")
		out.WriteString(m.renderWorktreesCenter())
	case TabLazygit:
		out.WriteString("\n")
		out.WriteString(m.renderLazygitCenter())
	case TabDiff:
		out.WriteString("\n")
		out.WriteString(m.renderDiffCenter())
	}
	out.WriteString("\n\n")
	out.WriteString(m.renderRightPane())
	return out.String()
}

func (m Model) renderCenterTabs() string {
	labels := make([]string, 0, len(m.CenterTabs()))
	for _, tab := range m.CenterTabs() {
		labels = append(labels, string(tab))
	}
	return "tabs: " + strings.Join(labels, " | ")
}

func (m Model) renderLeftPane() string {
	var out strings.Builder
	selectedWorktreeID := ""
	selectedWorktreePath := ""
	if repo, ok := m.State.CurrentRepo(); ok {
		selectedWorktreeID = repo.SelectedWorktreeID
		selectedWorktreePath = repo.SelectedWorktreePath
	}

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

	out.WriteString("\nWorktrees\n")
	if len(m.Worktrees) == 0 {
		out.WriteString("  (none)\n")
	} else {
		for _, wt := range m.Worktrees {
			marker := "  "
			if wt.ID == selectedWorktreeID || wt.Path == selectedWorktreePath {
				marker = "> "
			}
			label := wt.ID
			if label == "" {
				label = wt.Path
			}
			if wt.Branch != "" {
				label += " (" + wt.Branch + ")"
			}
			out.WriteString(marker + label + "\n")
		}
	}

	if selectedWorktreePath != "" {
		out.WriteString("\nselected worktree: " + selectedWorktreePath)
	}
	out.WriteString("\nworktree actions: create/switch")

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

func (m Model) renderLazygitCenter() string {
	repo, ok := m.State.CurrentRepo()
	if !ok || repo.ID == "" {
		return "請先選擇 repo"
	}
	if m.LazygitCenterFrameText == "" {
		return "Lazygit 啟動中..."
	}
	return m.LazygitCenterFrameText
}

func (m Model) renderDiffCenter() string {
	repo, ok := m.State.CurrentRepo()
	if !ok || repo.ID == "" {
		return "請先選擇 repo"
	}
	if m.DiffLoading {
		return "Loading diff..."
	}
	if m.DiffStatus != "" {
		return m.DiffStatus
	}
	return m.DiffOutput
}

func (m Model) renderWorktreesCenter() string {
	if len(m.Worktrees) == 0 {
		return "No worktrees loaded. Use create/switch actions from left pane."
	}

	selectedID := ""
	selectedPath := ""
	if repo, ok := m.State.CurrentRepo(); ok {
		selectedID = repo.SelectedWorktreeID
		selectedPath = repo.SelectedWorktreePath
	}

	var out strings.Builder
	out.WriteString("Worktrees tab\n")
	for _, wt := range m.Worktrees {
		marker := "  "
		if wt.ID == selectedID || wt.Path == selectedPath {
			marker = "> "
		}
		label := wt.ID
		if label == "" {
			label = wt.Path
		}
		if wt.Branch != "" {
			label += " (" + wt.Branch + ")"
		}
		out.WriteString(marker + label + "\n")
	}
	return strings.TrimSpace(out.String())
}

func (m Model) renderRightPane() string {
	var out strings.Builder
	out.WriteString("Right Pane")

	repo, ok := m.State.CurrentRepo()
	if !ok || repo.ID == "" {
		out.WriteString("\nrepo: (none)")
		out.WriteString("\npr: " + string(workspace.StatusNeutral))
		out.WriteString("\nci: " + string(workspace.StatusNeutral))
		out.WriteString("\nrelease: " + string(workspace.StatusUnconfigured))
		out.WriteString("\nlastSyncedAt: -")
		return out.String()
	}

	snapshot := m.repoStatusSnapshotForCurrentRepo()
	out.WriteString("\nrepo: " + repo.Name)
	out.WriteString("\npr: " + statusLabel(snapshot.PR))
	out.WriteString("\nci: " + statusLabel(snapshot.CI))
	out.WriteString("\nrelease: " + statusLabel(snapshot.Release))

	if snapshot.LastSyncedAt.IsZero() {
		out.WriteString("\nlastSyncedAt: -")
	} else {
		out.WriteString("\nlastSyncedAt: " + snapshot.LastSyncedAt.UTC().Format(time.RFC3339))
	}
	if snapshot.IsStale {
		out.WriteString("\n[stale]")
	}
	if strings.TrimSpace(snapshot.LatestError) != "" {
		out.WriteString("\nerror: " + snapshot.LatestError)
	}

	return out.String()
}

func statusLabel(st workspace.Status) string {
	if st == "" {
		return string(workspace.StatusNeutral)
	}
	return string(st)
}
