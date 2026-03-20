package app

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestNewModel_InitialLayout(t *testing.T) {
	m := NewModel(Config{})
	if m.ActiveTab != TabOverview {
		t.Fatalf("expected default tab %q, got %q", TabOverview, m.ActiveTab)
	}
	if m.LeftPaneWidth != 30 {
		t.Fatalf("expected left pane width %d, got %d", 30, m.LeftPaneWidth)
	}
	if m.CenterPaneWidth != 80 {
		t.Fatalf("expected center pane width %d, got %d", 80, m.CenterPaneWidth)
	}
	if m.RightPaneWidth != 40 {
		t.Fatalf("expected right pane width %d, got %d", 40, m.RightPaneWidth)
	}
}

func TestNewModel_InitialStateIsolation_DoesNotMutateInputState(t *testing.T) {
	initial := workspace.State{
		SelectedWorkspaceID: "ws-1",
		Workspaces: []workspace.Workspace{
			{
				ID:             "ws-1",
				Name:           "alpha",
				SelectedRepoID: "repo-1",
				Repos: []workspace.Repo{
					{ID: "repo-1", Name: "api", Path: "/tmp/api", Health: workspace.RepoHealthy},
					{ID: "repo-2", Name: "web", Path: "/tmp/web", Health: workspace.RepoHealthy},
				},
			},
		},
	}

	m := NewModel(Config{InitialState: initial})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := updated.(Model)

	ws, ok := got.State.CurrentWorkspace()
	if !ok {
		t.Fatal("expected model workspace")
	}
	if len(ws.Repos) != 1 {
		t.Fatalf("expected model workspace repo count %d, got %d", 1, len(ws.Repos))
	}
	if len(initial.Workspaces[0].Repos) != 2 {
		t.Fatalf("expected original input repo count %d, got %d", 2, len(initial.Workspaces[0].Repos))
	}
	if initial.Workspaces[0].SelectedRepoID != "repo-1" {
		t.Fatalf("expected original selected repo %q, got %q", "repo-1", initial.Workspaces[0].SelectedRepoID)
	}
}

func TestModel_CenterTabs_IncludeWorktrees(t *testing.T) {
	m := NewModel(Config{})

	got := m.CenterTabs()
	want := []Tab{TabOverview, TabWorktrees, TabLazygit, TabDiff}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected center tabs\nwant=%v\ngot=%v", want, got)
	}
}

func TestView_RendersCenterTabListIncludingWorktrees(t *testing.T) {
	m := NewModel(Config{})

	view := m.View()
	if !strings.Contains(view, "tabs: overview | worktrees | lazygit | diff") {
		t.Fatalf("expected center tab list to include worktrees, got:\n%s", view)
	}
}
