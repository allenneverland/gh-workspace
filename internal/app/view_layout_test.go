package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestView_RespectsWindowBounds_HorizontalLayout(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	view := updated.(Model).View()

	assertViewWithinBounds(t, view, 120, 32)
}

func TestView_RespectsWindowBounds_StackedLayout(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	view := updated.(Model).View()

	assertViewWithinBounds(t, view, 90, 24)
}

func assertViewWithinBounds(t *testing.T, view string, width, height int) {
	t.Helper()

	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > height {
		t.Fatalf("expected rendered height <= %d, got %d", height, len(lines))
	}

	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > width {
			t.Fatalf("line %d exceeds width %d (got %d): %q", i+1, width, lineWidth, line)
		}
	}
}
