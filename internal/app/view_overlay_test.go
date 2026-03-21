package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestView_OverlaySwitchMode_HidesCreatePanels(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model).View()

	assertContains(t, got, "Workspace Overlay")
	assertContains(t, got, "mode: switch")
	assertContains(t, got, "team-a")
	assertNotContains(t, got, "scan path>")
	assertNotContains(t, got, "candidates:")
	assertNotContains(t, got, "staged repos:")
}

func TestView_OverlayCreateMode_ShowsScanCandidatesAndStaged(t *testing.T) {
	m := seededCreateOverlayModelForScanTests()
	m.Overlay.CreateNameInput = "team-x"
	m.Overlay.ScanPathInput = "/tmp/projects"
	m.Overlay.CandidateQuery = "api"
	m.Overlay.Candidates = []RepoCandidate{
		{Name: "api", Path: "/tmp/api"},
		{Name: "web", Path: "/tmp/web"},
	}
	m.Overlay.StagedRepos = []RepoCandidate{{Name: "api", Path: "/tmp/api"}}
	m.Overlay.SelectedCandidateIndex = 0

	got := m.View()

	assertContains(t, got, "Workspace Overlay")
	assertContains(t, got, "mode: create")
	assertContains(t, got, "name> team-x")
	assertContains(t, got, "scan path> /tmp/projects")
	assertContains(t, got, "query> api")
	assertContains(t, got, "candidates:")
	assertContains(t, got, "staged repos:")
	assertContains(t, got, "s: save and switch")
}

func TestView_OverlayCreateMode_ShowsOverlayErrorStatus(t *testing.T) {
	m := seededCreateOverlayModelForScanTests()
	m.Overlay.LastError = "already added"

	got := m.View()

	assertContains(t, got, "status: already added")
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if contains := strings.Contains(got, want); contains {
		t.Fatalf("expected output to not contain %q, got:\n%s", want, got)
	}
}
