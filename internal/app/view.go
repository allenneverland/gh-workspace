package app

import (
	"strings"
	"time"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func renderView(m Model) string {
	shouldTrim := m.WindowWidth > 0
	if m.WindowWidth > 0 && m.WindowWidth < 100 {
		stackedHeights := splitHeights(m.WindowHeight, 3)
		width := m.WindowWidth
		if width < 1 {
			width = 1
		}

		left := renderPane(
			m.renderLeftPane(),
			width,
			stackedHeights[0],
			shouldTrim,
		)
		center := renderPane(
			m.renderCenterPane(),
			width,
			stackedHeights[1],
			shouldTrim,
		)
		right := renderPane(
			m.renderRightPane(),
			width,
			stackedHeights[2],
			shouldTrim,
		)
		return lipgloss.JoinVertical(lipgloss.Left, left, center, right)
	}

	left := renderPane(
		m.renderLeftPane(),
		m.LeftPaneWidth,
		m.WindowHeight,
		shouldTrim,
	)
	center := renderPane(
		m.renderCenterPane(),
		m.CenterPaneWidth,
		m.WindowHeight,
		shouldTrim,
	)
	right := renderPane(
		m.renderRightPane(),
		m.RightPaneWidth,
		m.WindowHeight,
		shouldTrim,
	)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
}

func (m Model) renderCenterTabs() string {
	labels := make([]string, 0, len(m.CenterTabs()))
	for _, tab := range m.CenterTabs() {
		labels = append(labels, string(tab))
	}
	return "tabs: " + strings.Join(labels, " | ")
}

func (m Model) renderCenterPane() string {
	var out strings.Builder
	out.WriteString("Center")
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
	return out.String()
}

func (m Model) renderLeftPane() string {
	var out strings.Builder
	selectedWorktreeID := ""
	selectedWorktreePath := ""
	if repo, ok := m.State.CurrentRepo(); ok {
		selectedWorktreeID = repo.SelectedWorktreeID
		selectedWorktreePath = repo.SelectedWorktreePath
	}

	if m.UIMode == ModeWorkspace {
		out.WriteString("Workspaces\n")
		rendered := 0
		for _, ws := range m.State.Snapshot.Workspaces {
			if isSystemWorkspaceEntry(ws) {
				continue
			}
			marker := "  "
			if ws.ID == m.State.Snapshot.SelectedWorkspaceID {
				marker = "> "
			}
			out.WriteString(marker + ws.Name + "\n")
			rendered++
		}
		if rendered == 0 {
			out.WriteString("  (none)\n")
		}
		out.WriteString("\n")
	}

	out.WriteString("Repos\n")
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
	if m.UIMode == ModeFolder && m.RepoPathInputActive {
		out.WriteString("\nrepo path> " + m.RepoPathInput.Render())
	}
	if repo, ok := m.State.CurrentRepo(); ok && repo.Health == workspace.RepoInvalid {
		out.WriteString("\nenter: fix path")
		out.WriteString("\nx: remove repo")
	}

	statusMessage := strings.TrimSpace(m.StatusMessage)
	if m.UIMode == ModeFolder && m.State.CurrentRepoID() == "" && statusMessage == "" {
		statusMessage = "current folder is not a git repo"
	}
	if statusMessage != "" {
		out.WriteString("\n\nstatus: " + statusMessage)
	}
	if m.UIMode == ModeFolder && m.State.CurrentRepoID() == "" {
		out.WriteString("\npress a to add repo path")
		out.WriteString("\nlaunch workspace mode with -w <name>")
	}

	return out.String()
}

func (m Model) renderLazygitCenter() string {
	repo, ok := m.State.CurrentRepo()
	if !ok || repo.ID == "" {
		return "請先選擇 repo"
	}
	if m.LazygitSessionID == "" {
		status := strings.TrimSpace(m.StatusMessage)
		if strings.Contains(strings.ToLower(status), "lazygit") {
			return status
		}
		return "Lazygit 尚未啟動"
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
		out.WriteString("\npublishRun: -")
		out.WriteString("\npublishEvent: -")
		out.WriteString("\npublishUpdatedAt: -")
		out.WriteString("\npublishUrl: -")
		out.WriteString("\nlastSyncedAt: -")
		return out.String()
	}

	snapshot := m.repoStatusSnapshotForCurrentRepo()
	out.WriteString("\nrepo: " + repo.Name)
	out.WriteString("\npr: " + statusLabel(snapshot.PR))
	out.WriteString("\nci: " + statusLabel(snapshot.CI))
	out.WriteString("\nrelease: " + statusLabel(snapshot.Release))
	if strings.TrimSpace(snapshot.ReleaseRun.Name) == "" {
		out.WriteString("\npublishRun: -")
	} else {
		out.WriteString("\npublishRun: " + snapshot.ReleaseRun.Name)
	}
	if strings.TrimSpace(snapshot.ReleaseRun.Event) == "" {
		out.WriteString("\npublishEvent: -")
	} else {
		out.WriteString("\npublishEvent: " + snapshot.ReleaseRun.Event)
	}
	if snapshot.ReleaseRun.UpdatedAt.IsZero() {
		out.WriteString("\npublishUpdatedAt: -")
	} else {
		out.WriteString("\npublishUpdatedAt: " + snapshot.ReleaseRun.UpdatedAt.UTC().Format(time.RFC3339))
	}
	if strings.TrimSpace(snapshot.ReleaseRun.URL) == "" {
		out.WriteString("\npublishUrl: -")
	} else {
		out.WriteString("\npublishUrl: " + snapshot.ReleaseRun.URL)
	}

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

func renderPane(content string, width, height int, trim bool) string {
	if width <= 0 {
		width = 1
	}

	contentWidth := width - 2 // compensate border width
	if contentWidth < 1 {
		contentWidth = 1
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(contentWidth)
	contentHeight := 0
	if height > 0 {
		contentHeight = height - 2 // compensate border height
		if contentHeight < 1 {
			contentHeight = 1
		}
		style = style.Height(contentHeight)
	}

	textWidth := contentWidth - 2 // horizontal padding
	if textWidth < 1 {
		textWidth = 1
	}
	if trim {
		content = trimLines(content, textWidth)
		if contentHeight > 0 {
			content = trimLineCount(content, contentHeight)
		}
	}
	return style.Render(content)
}

func splitHeights(total, parts int) []int {
	if parts <= 0 {
		return nil
	}

	heights := make([]int, parts)
	if total <= 0 {
		return heights
	}

	base := total / parts
	rem := total % parts
	for i := 0; i < parts; i++ {
		heights[i] = base
		if i < rem {
			heights[i]++
		}
	}
	return heights
}

func trimLines(content string, maxWidth int) string {
	if maxWidth < 1 {
		return ""
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = trimLine(line, maxWidth)
	}
	return strings.Join(lines, "\n")
}

func trimLine(line string, maxWidth int) string {
	if ansi.StringWidth(line) <= maxWidth {
		return line
	}
	if maxWidth <= 3 {
		return ansi.Truncate(line, maxWidth, "")
	}
	return ansi.Truncate(line, maxWidth, "...")
}

func trimLineCount(content string, maxLines int) string {
	if maxLines < 1 {
		return ""
	}

	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n")
}

func statusLabel(st workspace.Status) string {
	if st == "" {
		return string(workspace.StatusNeutral)
	}
	return string(st)
}
