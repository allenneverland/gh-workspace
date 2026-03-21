package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderWorkspaceOverlayLayer(baseView string, m Model) string {
	overlay := renderWorkspaceOverlay(m)
	if strings.TrimSpace(overlay) == "" {
		return baseView
	}

	styled := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Render(overlay)
	if m.WindowWidth > 0 {
		styled = lipgloss.PlaceHorizontal(m.WindowWidth, lipgloss.Center, styled)
	}

	if strings.TrimSpace(baseView) == "" {
		return styled
	}
	return baseView + "\n\n" + styled
}

func renderWorkspaceOverlay(m Model) string {
	var out strings.Builder
	out.WriteString("Workspace Overlay")
	out.WriteString("\nmode: " + string(m.Overlay.Mode))
	out.WriteString("\n\n")

	switch m.Overlay.Mode {
	case OverlayModeCreate:
		out.WriteString(renderWorkspaceOverlayCreateContent(m))
	default:
		out.WriteString(renderWorkspaceOverlaySwitchContent(m))
	}

	if status := strings.TrimSpace(workspaceOverlayStatus(m)); status != "" {
		out.WriteString("\n\nstatus: " + status)
	}

	return strings.TrimRight(out.String(), "\n")
}

func renderWorkspaceOverlaySwitchContent(m Model) string {
	var out strings.Builder
	out.WriteString("workspaces:\n")

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

	out.WriteString("\nc: create workspace")
	out.WriteString("\nesc: close")
	return strings.TrimRight(out.String(), "\n")
}

func renderWorkspaceOverlayCreateContent(m Model) string {
	var out strings.Builder
	out.WriteString("name> " + m.Overlay.CreateNameInput)
	out.WriteString("\nscan path> " + m.Overlay.ScanPathInput)
	out.WriteString("\nquery> " + m.Overlay.CandidateQuery)

	out.WriteString("\n\ncandidates:\n")
	visible := FilterCandidates(m.Overlay.Candidates, m.Overlay.CandidateQuery)
	selected := candidateSelectionIndex(m.Overlay.Candidates, m.Overlay.CandidateQuery, m.Overlay.SelectedCandidateIndex)
	if len(visible) == 0 {
		out.WriteString("  (none)\n")
	} else {
		for i, candidate := range visible {
			marker := "  "
			if i == selected {
				marker = "> "
			}
			label := strings.TrimSpace(candidate.Name)
			if label == "" {
				label = strings.TrimSpace(candidate.Path)
			}
			if strings.TrimSpace(candidate.Path) != "" {
				label += " (" + candidate.Path + ")"
			}
			out.WriteString(marker + label + "\n")
		}
	}

	out.WriteString("\nstaged repos:\n")
	if len(m.Overlay.StagedRepos) == 0 {
		out.WriteString("  (none)\n")
	} else {
		for _, repo := range m.Overlay.StagedRepos {
			label := strings.TrimSpace(repo.Name)
			if label == "" {
				label = strings.TrimSpace(repo.Path)
			}
			if strings.TrimSpace(repo.Path) != "" {
				label += " (" + repo.Path + ")"
			}
			out.WriteString("  - " + label + "\n")
		}
	}

	out.WriteString("\nenter: add selected repo")
	out.WriteString("\ntab: next field")
	out.WriteString("\ns: save and switch")
	out.WriteString("\nesc: discard")

	return strings.TrimRight(out.String(), "\n")
}

func workspaceOverlayStatus(m Model) string {
	if strings.TrimSpace(m.Overlay.LastError) != "" {
		return m.Overlay.LastError
	}
	if m.Overlay.SaveInFlight {
		return "saving..."
	}
	if m.Overlay.ScanInFlight {
		return "scanning..."
	}
	return ""
}
