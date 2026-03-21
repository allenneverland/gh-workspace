package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestOverlay_Create_TextInputWDoesNotToggleOverlay(t *testing.T) {
	tests := []struct {
		name        string
		focus       OverlayFocus
		setup       func(*Model)
		wantInput   func(WorkspaceOverlayState) string
		wantInputFn func(*WorkspaceOverlayState)
	}{
		{
			name:  "create-name-input",
			focus: OverlayFocusCreateNameInput,
			setup: func(m *Model) {
				m.Overlay.CreateNameInput = "team"
			},
			wantInputFn: func(overlay *WorkspaceOverlayState) { overlay.CreateNameInput += "w" },
		},
		{
			name:  "scan-path-input",
			focus: OverlayFocusScanPathInput,
			setup: func(m *Model) {
				m.Overlay.ScanPathInput = "/tmp/projects"
			},
			wantInputFn: func(overlay *WorkspaceOverlayState) { overlay.ScanPathInput += "w" },
		},
		{
			name:  "candidate-query",
			focus: OverlayFocusCandidateList,
			setup: func(m *Model) {
				m.Overlay.CandidateQuery = "api"
			},
			wantInputFn: func(overlay *WorkspaceOverlayState) { overlay.CandidateQuery += "w" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := seededCreateOverlayModelForScanTests()
			m.Overlay.Focus = tt.focus
			tt.setup(&m)

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
			got := updated.(Model)

			if !got.Overlay.Active {
				t.Fatalf("expected overlay to remain active, got %#v", got.Overlay)
			}

			want := m.Overlay
			tt.wantInputFn(&want)
			if got.Overlay.CreateNameInput != want.CreateNameInput ||
				got.Overlay.ScanPathInput != want.ScanPathInput ||
				got.Overlay.CandidateQuery != want.CandidateQuery {
				t.Fatalf("expected typed w to update focused input, got %#v want %#v", got.Overlay, want)
			}
		})
	}
}

func TestOverlay_Create_ScanPathInputChangeSchedulesScanWithRevision(t *testing.T) {
	m := seededCreateOverlayModelForScanTests()
	m.Overlay.Focus = OverlayFocusScanPathInput
	m.Overlay.ScanPathInput = "/tmp/projects"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := updated.(Model)

	if got.Overlay.ScanPathInput != "/tmp/projectsx" {
		t.Fatalf("expected scan path input to update, got %#v", got.Overlay)
	}
	if got.Overlay.ScanRevision != 1 {
		t.Fatalf("expected scan revision to increment to 1, got %d", got.Overlay.ScanRevision)
	}
	if cmd == nil {
		t.Fatal("expected scan path change to schedule a scan")
	}

	msg := cmd()
	scheduled, ok := msg.(MsgOverlayScanScheduled)
	if !ok {
		t.Fatalf("expected scheduled message %T, got %T", MsgOverlayScanScheduled{}, msg)
	}
	if scheduled.Revision != 1 {
		t.Fatalf("expected scheduled revision 1, got %d", scheduled.Revision)
	}
}

func TestOverlay_ScanCompleted_StaleResultIgnored(t *testing.T) {
	m := seededCreateOverlayModelForScanTests()
	m.Overlay.Candidates = []RepoCandidate{{Name: "kept", Path: "/tmp/kept"}}
	m.Overlay.ScanRevision = 2
	m.Overlay.ScanInFlight = true

	updated, _ := m.Update(MsgOverlayScanCompleted{
		Revision:   1,
		Candidates: []RepoCandidate{{Name: "stale", Path: "/tmp/stale"}},
	})
	got := updated.(Model)

	if got.Overlay.ScanRevision != 2 {
		t.Fatalf("expected scan revision to stay at 2, got %d", got.Overlay.ScanRevision)
	}
	if !got.Overlay.ScanInFlight {
		t.Fatal("expected stale scan result to keep scan in flight state unchanged")
	}
	if len(got.Overlay.Candidates) != 1 || got.Overlay.Candidates[0].Path != "/tmp/kept" {
		t.Fatalf("expected stale scan result to be ignored, got %#v", got.Overlay.Candidates)
	}
}

func TestOverlay_Create_EnterCandidate_AddsToStaged(t *testing.T) {
	m := seededCreateOverlayModelForScanTests()
	m.Overlay.Focus = OverlayFocusCandidateList
	m.Overlay.CandidateQuery = "api"
	m.Overlay.Candidates = []RepoCandidate{
		{Name: "web", Path: "/tmp/web"},
		{Name: "api", Path: "/tmp/api"},
	}
	m.Overlay.SelectedCandidateIndex = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected enter on candidate to stay local to overlay state")
	}
	if len(got.Overlay.StagedRepos) != 1 {
		t.Fatalf("expected staged repo append, got %#v", got.Overlay)
	}
	if got.Overlay.StagedRepos[0].Path != "/tmp/api" {
		t.Fatalf("expected staged repo path %q, got %q", "/tmp/api", got.Overlay.StagedRepos[0].Path)
	}
	if got.Overlay.LastError != "" {
		t.Fatalf("expected no overlay error, got %q", got.Overlay.LastError)
	}
}

func TestOverlay_Create_EnterCandidate_DuplicateRejectedWithAlreadyAdded(t *testing.T) {
	m := seededCreateOverlayModelForScanTests()
	m.Overlay.Focus = OverlayFocusCandidateList
	m.Overlay.CandidateQuery = "api"
	m.Overlay.Candidates = []RepoCandidate{
		{Name: "web", Path: "/tmp/web"},
		{Name: "api", Path: "/tmp/api"},
	}
	m.Overlay.StagedRepos = []RepoCandidate{{Name: "api", Path: "/tmp/api"}}
	m.Overlay.SelectedCandidateIndex = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected duplicate candidate add to stay local to overlay state")
	}
	if len(got.Overlay.StagedRepos) != 1 {
		t.Fatalf("expected staged repos to remain unchanged, got %#v", got.Overlay)
	}
	if got.Overlay.LastError != "already added" {
		t.Fatalf("expected duplicate add error %q, got %q", "already added", got.Overlay.LastError)
	}
}

func TestOverlay_Create_EnterCandidate_DuplicateRejectedAfterPathNormalization(t *testing.T) {
	m := seededCreateOverlayModelForScanTests()
	m.Overlay.Focus = OverlayFocusCandidateList
	m.Overlay.CandidateQuery = "repo"
	m.Overlay.Candidates = []RepoCandidate{
		{Name: "repo", Path: "/tmp/repo"},
	}
	m.Overlay.StagedRepos = []RepoCandidate{{Name: "repo", Path: "/tmp/repo/"}}
	m.Overlay.SelectedCandidateIndex = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if got.Overlay.LastError != "already added" {
		t.Fatalf("expected normalized duplicate add error %q, got %q", "already added", got.Overlay.LastError)
	}
	if len(got.Overlay.StagedRepos) != 1 {
		t.Fatalf("expected staged repos to remain unchanged, got %#v", got.Overlay)
	}
}

func TestFilterCandidates_EmptyQueryPreservesOriginalOrder(t *testing.T) {
	candidates := []RepoCandidate{
		{Name: "beta", Path: "/tmp/beta"},
		{Name: "alpha", Path: "/tmp/alpha"},
		{Name: "zeta", Path: "/tmp/zeta"},
	}

	got := FilterCandidates(candidates, "")

	if len(got) != len(candidates) {
		t.Fatalf("expected %d candidates, got %d", len(candidates), len(got))
	}
	for i := range candidates {
		if got[i] != candidates[i] {
			t.Fatalf("expected original ordering, got %#v", got)
		}
	}
}

func TestFilterCandidates_CaseInsensitiveSubsequenceMatches(t *testing.T) {
	candidates := []RepoCandidate{
		{Name: "BuildDeploy", Path: "/tmp/build"},
		{Name: "bXdp", Path: "/tmp/bxdp"},
		{Name: "zzz", Path: "/tmp/zzz"},
	}

	got := FilterCandidates(candidates, "BDP")

	want := []RepoCandidate{
		{Name: "bXdp", Path: "/tmp/bxdp"},
		{Name: "BuildDeploy", Path: "/tmp/build"},
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d candidates, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected filtered order %#v, got %#v", want, got)
		}
	}
}

func TestFilterCandidates_SubsequenceMatchSortsByDisplayNameLengthThenLexical(t *testing.T) {
	candidates := []RepoCandidate{
		{Name: "bca", Path: "/tmp/bca"},
		{Name: "aa", Path: "/tmp/aa"},
		{Name: "aca", Path: "/tmp/aca"},
		{Name: "zzz", Path: "/tmp/zzz"},
	}

	got := FilterCandidates(candidates, "a")

	want := []RepoCandidate{
		{Name: "aa", Path: "/tmp/aa"},
		{Name: "aca", Path: "/tmp/aca"},
		{Name: "bca", Path: "/tmp/bca"},
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d candidates, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected filtered order %#v, got %#v", want, got)
		}
	}
}

func seededCreateOverlayModelForScanTests() Model {
	m := seededModelWithSystemAndUserWorkspaces()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	opened := step.(Model)
	step, _ = opened.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	create := step.(Model)
	create.Overlay.Focus = OverlayFocusCreateNameInput
	return create
}
