package app

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestOverlay_KeyW_OpensSwitchMode_WorkspaceMode(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model)
	if !got.Overlay.Active || got.Overlay.Mode != OverlayModeSwitch {
		t.Fatalf("expected active switch overlay, got %#v", got.Overlay)
	}
}

func TestOverlay_KeyW_OpensSwitchMode_FolderMode(t *testing.T) {
	m := seededFolderModeModelWithLocalRepo()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model)
	if !got.Overlay.Active || got.Overlay.Mode != OverlayModeSwitch {
		t.Fatalf("expected active switch overlay, got %#v", got.Overlay)
	}
}

func TestOverlay_KeyEsc_ClosesOverlay(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	opened := step.(Model)
	updated, _ := opened.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.Overlay.Active {
		t.Fatalf("expected esc to close overlay, got %#v", got.Overlay)
	}
}

func TestOverlay_KeyC_FromSwitchMode_EntersCreateMode(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	opened := step.(Model)
	updated, _ := opened.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := updated.(Model)
	if !got.Overlay.Active || got.Overlay.Mode != OverlayModeCreate {
		t.Fatalf("expected active create overlay, got %#v", got.Overlay)
	}
}

func TestOverlay_KeyEsc_FromCreateMode_DiscardsDraftFields(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	opened := step.(Model)
	step, _ = opened.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	create := step.(Model)
	wantScanPath := workspaceOverlayDefaultScanPath(create.State)
	create.Overlay.CreateNameInput = "team-x"
	create.Overlay.ScanPathInput = "/tmp/projects"
	create.Overlay.Candidates = []RepoCandidate{{Name: "api", Path: "/tmp/api"}}
	create.Overlay.StagedRepos = []RepoCandidate{{Name: "web", Path: "/tmp/web"}}

	updated, _ := create.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.Overlay.Active {
		t.Fatalf("expected esc to close overlay, got %#v", got.Overlay)
	}
	if got.Overlay.CreateNameInput != "" {
		t.Fatalf("expected create name draft to be cleared, got %q", got.Overlay.CreateNameInput)
	}
	if got.Overlay.ScanPathInput != wantScanPath {
		t.Fatalf("expected scan path draft to reset to %q, got %q", wantScanPath, got.Overlay.ScanPathInput)
	}
	if len(got.Overlay.Candidates) != 0 {
		t.Fatalf("expected candidate draft to be cleared, got %#v", got.Overlay.Candidates)
	}
	if len(got.Overlay.StagedRepos) != 0 {
		t.Fatalf("expected staged repo draft to be cleared, got %#v", got.Overlay.StagedRepos)
	}
}

func TestOverlay_RepoPathInputActive_KeyW_TypesInputWithoutOpeningOverlay(t *testing.T) {
	m := seededFolderModeModelWithLocalRepo()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	active := step.(Model)
	updated, cmd := active.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command while typing in repo path input")
	}
	if got.RepoPathInput.Value() != "w" {
		t.Fatalf("expected repo path input to capture %q, got %q", "w", got.RepoPathInput.Value())
	}
	if got.Overlay.Active {
		t.Fatalf("expected overlay to remain closed, got %#v", got.Overlay)
	}
}

func TestOverlay_RepoPathInputActive_KeyQ_TypesInputWithoutQuitting(t *testing.T) {
	m := seededFolderModeModelWithLocalRepo()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	active := step.(Model)
	updated, cmd := active.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command while typing in repo path input")
	}
	if got.RepoPathInput.Value() != "q" {
		t.Fatalf("expected repo path input to capture %q, got %q", "q", got.RepoPathInput.Value())
	}
	if got.Overlay.Active {
		t.Fatalf("expected overlay to remain closed, got %#v", got.Overlay)
	}
}

func TestOverlay_LazygitActive_KeyW_ForwardsToPTYInsteadOfOpeningOverlay(t *testing.T) {
	m := seededModelWithRepos()
	manager := newFakeLazygitSessionManager()
	manager.sessionsByRepo["/tmp/api"] = LazygitSessionHandle{
		ID:       "session-api",
		RepoPath: "/tmp/api",
	}
	m.LazygitSessionManager = manager

	enteredTab, _ := m.Update(MsgSetActiveTab{Tab: TabLazygit})
	lazygitModel := enteredTab.(Model)

	updated, cmd := lazygitModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command for lazygit-owned key")
	}
	if got.Overlay.Active {
		t.Fatalf("expected overlay to remain closed while lazygit owns keys, got %#v", got.Overlay)
	}
	if len(manager.writeCalls) != 1 {
		t.Fatalf("expected one PTY write for lazygit-owned key, got %d", len(manager.writeCalls))
	}
	if payload := string(manager.writeCalls[0].data); payload != "w" {
		t.Fatalf("expected forwarded payload %q, got %q", "w", payload)
	}
}

func TestOverlay_Save_SuccessClosesOverlayAndSwitchesWorkspaceMode(t *testing.T) {
	committer := &fakeWorkspaceOverlayDraftCommitter{
		state: workspace.State{
			SelectedWorkspaceID: "ws-team-b",
			Workspaces: []workspace.Workspace{
				{
					ID:             "ws-team-b",
					Name:           "team-b",
					SelectedRepoID: "repo-web",
					Repos: []workspace.Repo{
						{ID: "repo-web", Name: "web", Path: "/tmp/web", Health: workspace.RepoHealthy},
					},
				},
			},
		},
	}
	m := seededCreateOverlayModelForSaveTests(committer)
	m.UIMode = ModeFolder
	m.Overlay.Focus = OverlayFocusStagedRepoList
	m.Overlay.CreateNameInput = "team-b"
	m.Overlay.StagedRepos = []RepoCandidate{
		{Name: "web", Path: "/tmp/web"},
		{Name: "api", Path: "/tmp/api"},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected save key to dispatch a save command")
	}

	msg := cmd()
	completed, ok := msg.(MsgOverlaySaveCompleted)
	if !ok {
		t.Fatalf("expected save completion message %T, got %T", MsgOverlaySaveCompleted{}, msg)
	}
	if completed.Revision != 1 {
		t.Fatalf("expected save revision %d, got %d", 1, completed.Revision)
	}
	if completed.Err != nil {
		t.Fatalf("expected nil save error, got %v", completed.Err)
	}

	gotBeforeCompletion := updated.(Model)
	if !gotBeforeCompletion.Overlay.Active {
		t.Fatal("expected overlay to remain open until save completion")
	}

	completedUpdate, _ := gotBeforeCompletion.Update(completed)
	got := completedUpdate.(Model)
	if got.UIMode != ModeWorkspace {
		t.Fatalf("expected UI mode %q after save, got %q", ModeWorkspace, got.UIMode)
	}
	if got.Overlay.Active {
		t.Fatalf("expected overlay to close after successful save, got %#v", got.Overlay)
	}
	if got.State.CurrentWorkspaceID() != "ws-team-b" {
		t.Fatalf("expected selected workspace %q, got %q", "ws-team-b", got.State.CurrentWorkspaceID())
	}
	if got.State.CurrentRepoID() != "repo-web" {
		t.Fatalf("expected selected repo %q, got %q", "repo-web", got.State.CurrentRepoID())
	}
	if committer.calls != 1 {
		t.Fatalf("expected one save call, got %d", committer.calls)
	}
	if committer.lastDraft.Name != "team-b" {
		t.Fatalf("expected draft name %q, got %q", "team-b", committer.lastDraft.Name)
	}
	if len(committer.lastDraft.StagedRepos) != 2 {
		t.Fatalf("expected staged repos to be forwarded, got %#v", committer.lastDraft.StagedRepos)
	}
}

func TestOverlay_Save_FailureKeepsOverlayOpenAndPreservesDraft(t *testing.T) {
	committer := &fakeWorkspaceOverlayDraftCommitter{err: errors.New("save sentinel")}
	m := seededCreateOverlayModelForSaveTests(committer)
	m.UIMode = ModeFolder
	m.Overlay.Focus = OverlayFocusStagedRepoList
	m.Overlay.CreateNameInput = "team-c"
	m.Overlay.ScanPathInput = "/tmp/projects"
	m.Overlay.CandidateQuery = "api"
	m.Overlay.Candidates = []RepoCandidate{{Name: "api", Path: "/tmp/api"}}
	m.Overlay.StagedRepos = []RepoCandidate{{Name: "api", Path: "/tmp/api"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected save key to dispatch a save command")
	}

	msg := cmd()
	completed, ok := msg.(MsgOverlaySaveCompleted)
	if !ok {
		t.Fatalf("expected save completion message %T, got %T", MsgOverlaySaveCompleted{}, msg)
	}
	if !errors.Is(completed.Err, committer.err) {
		t.Fatalf("expected completion error %v, got %v", committer.err, completed.Err)
	}

	completedUpdate, _ := updated.(Model).Update(completed)
	got := completedUpdate.(Model)
	if got.UIMode != ModeFolder {
		t.Fatalf("expected UI mode to remain %q, got %q", ModeFolder, got.UIMode)
	}
	if !got.Overlay.Active || got.Overlay.Mode != OverlayModeCreate {
		t.Fatalf("expected create overlay to remain open, got %#v", got.Overlay)
	}
	if got.Overlay.CreateNameInput != "team-c" {
		t.Fatalf("expected create name draft to be preserved, got %q", got.Overlay.CreateNameInput)
	}
	if got.Overlay.ScanPathInput != "/tmp/projects" {
		t.Fatalf("expected scan path draft to be preserved, got %q", got.Overlay.ScanPathInput)
	}
	if got.Overlay.CandidateQuery != "api" {
		t.Fatalf("expected candidate query to be preserved, got %q", got.Overlay.CandidateQuery)
	}
	if len(got.Overlay.Candidates) != 1 || got.Overlay.Candidates[0].Path != "/tmp/api" {
		t.Fatalf("expected candidates to be preserved, got %#v", got.Overlay.Candidates)
	}
	if len(got.Overlay.StagedRepos) != 1 || got.Overlay.StagedRepos[0].Path != "/tmp/api" {
		t.Fatalf("expected staged repos to be preserved, got %#v", got.Overlay.StagedRepos)
	}
	if got.StatusMessage != "save sentinel" {
		t.Fatalf("expected error status %q, got %q", "save sentinel", got.StatusMessage)
	}
}

func TestOverlay_Save_DuplicateWorkspaceNameReturnsWorkspaceAlreadyExists(t *testing.T) {
	committer := &fakeWorkspaceOverlayDraftCommitter{err: errors.New("workspace already exists")}
	m := seededCreateOverlayModelForSaveTests(committer)
	m.Overlay.Focus = OverlayFocusStagedRepoList
	m.Overlay.CreateNameInput = "team-a"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected save key to dispatch a save command")
	}

	msg := cmd()
	completed, ok := msg.(MsgOverlaySaveCompleted)
	if !ok {
		t.Fatalf("expected save completion message %T, got %T", MsgOverlaySaveCompleted{}, msg)
	}

	completedUpdate, _ := updated.(Model).Update(completed)
	got := completedUpdate.(Model)
	if got.StatusMessage != "workspace already exists" {
		t.Fatalf("expected duplicate name status %q, got %q", "workspace already exists", got.StatusMessage)
	}
	if !got.Overlay.Active {
		t.Fatalf("expected overlay to stay open on duplicate name, got %#v", got.Overlay)
	}
}

func TestOverlay_Save_EmptyStagedReposIsAllowed(t *testing.T) {
	committer := &fakeWorkspaceOverlayDraftCommitter{
		state: workspace.State{
			SelectedWorkspaceID: "ws-empty",
			Workspaces: []workspace.Workspace{
				{
					ID:   "ws-empty",
					Name: "empty",
				},
			},
		},
	}
	m := seededCreateOverlayModelForSaveTests(committer)
	m.UIMode = ModeFolder
	m.Overlay.Focus = OverlayFocusStagedRepoList
	m.Overlay.CreateNameInput = "empty"
	m.Overlay.StagedRepos = nil

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected save with no staged repos to dispatch a save command")
	}

	msg := cmd()
	completed, ok := msg.(MsgOverlaySaveCompleted)
	if !ok {
		t.Fatalf("expected save completion message %T, got %T", MsgOverlaySaveCompleted{}, msg)
	}
	if completed.Err != nil {
		t.Fatalf("expected nil save error, got %v", completed.Err)
	}

	completedUpdate, _ := updated.(Model).Update(completed)
	got := completedUpdate.(Model)
	if got.UIMode != ModeWorkspace {
		t.Fatalf("expected UI mode %q, got %q", ModeWorkspace, got.UIMode)
	}
	if got.State.CurrentWorkspaceID() != "ws-empty" {
		t.Fatalf("expected selected workspace %q, got %q", "ws-empty", got.State.CurrentWorkspaceID())
	}
	if got.State.CurrentRepoID() != "" {
		t.Fatalf("expected no selected repo for empty workspace, got %q", got.State.CurrentRepoID())
	}
	if len(committer.lastDraft.StagedRepos) != 0 {
		t.Fatalf("expected empty staged repo draft, got %#v", committer.lastDraft.StagedRepos)
	}
}

func TestOverlay_Save_RepeatedKeyPressesWhileInFlightDoNotDispatchDuplicateSaveCommands(t *testing.T) {
	committer := &fakeWorkspaceOverlayDraftCommitter{}
	m := seededCreateOverlayModelForSaveTests(committer)
	m.Overlay.Focus = OverlayFocusStagedRepoList
	m.Overlay.CreateNameInput = "team-b"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected first save key press to dispatch a save command")
	}

	inFlight := updated.(Model)
	if !inFlight.Overlay.SaveInFlight {
		t.Fatalf("expected save to be marked in flight, got %#v", inFlight.Overlay)
	}
	if inFlight.Overlay.SaveRevision != 1 {
		t.Fatalf("expected save revision %d, got %d", 1, inFlight.Overlay.SaveRevision)
	}

	updated, secondCmd := inFlight.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if secondCmd != nil {
		t.Fatal("expected repeated save key while in flight to not dispatch a second save command")
	}

	stillInFlight := updated.(Model)
	if !stillInFlight.Overlay.SaveInFlight {
		t.Fatalf("expected save to remain in flight, got %#v", stillInFlight.Overlay)
	}
	if stillInFlight.Overlay.SaveRevision != 1 {
		t.Fatalf("expected save revision to remain %d, got %d", 1, stillInFlight.Overlay.SaveRevision)
	}
}

func TestOverlay_Save_StaleCompletionMessageIgnored(t *testing.T) {
	committer := &fakeWorkspaceOverlayDraftCommitter{}
	m := seededCreateOverlayModelForSaveTests(committer)
	m.UIMode = ModeFolder
	m.Overlay.Focus = OverlayFocusStagedRepoList
	m.Overlay.CreateNameInput = "team-b"
	m.Overlay.StagedRepos = []RepoCandidate{{Name: "web", Path: "/tmp/web"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected save key to dispatch a save command")
	}

	inFlight := updated.(Model)
	ignoredState := workspace.State{
		SelectedWorkspaceID: "ws-stale",
		Workspaces:          []workspace.Workspace{{ID: "ws-stale", Name: "stale"}},
	}
	updated, _ = inFlight.Update(MsgOverlaySaveCompleted{
		Revision: inFlight.Overlay.SaveRevision - 1,
		State:    ignoredState,
	})
	got := updated.(Model)
	if got.UIMode != ModeFolder {
		t.Fatalf("expected UI mode to remain %q, got %q", ModeFolder, got.UIMode)
	}
	if got.State.CurrentWorkspaceID() == "ws-stale" {
		t.Fatalf("expected stale completion state to be ignored, got %#v", got.State.Snapshot)
	}
	if !got.Overlay.Active || !got.Overlay.SaveInFlight {
		t.Fatalf("expected overlay save to remain in flight after stale completion, got %#v", got.Overlay)
	}
}

func TestOverlay_Save_CompletionFromPriorSessionIgnoredAfterEscAndResave(t *testing.T) {
	committer := &fakeWorkspaceOverlayDraftCommitter{
		state: workspace.State{
			SelectedWorkspaceID: "ws-team-b",
			Workspaces: []workspace.Workspace{
				{ID: "ws-team-b", Name: "team-b"},
			},
		},
	}
	m := seededCreateOverlayModelForSaveTests(committer)
	m.UIMode = ModeFolder
	m.Overlay.Focus = OverlayFocusStagedRepoList
	m.Overlay.CreateNameInput = "team-first"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected first save key to dispatch a save command")
	}

	firstInFlight := updated.(Model)
	if firstInFlight.Overlay.SaveRevision != 1 {
		t.Fatalf("expected first save revision %d, got %d", 1, firstInFlight.Overlay.SaveRevision)
	}

	updated, _ = firstInFlight.Update(tea.KeyMsg{Type: tea.KeyEsc})
	closed := updated.(Model)
	if closed.Overlay.Active {
		t.Fatalf("expected esc to close overlay while first save in flight, got %#v", closed.Overlay)
	}

	updated, _ = closed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	reopened := updated.(Model)
	updated, _ = reopened.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	secondCreate := updated.(Model)
	secondCreate.Overlay.Focus = OverlayFocusStagedRepoList
	secondCreate.Overlay.CreateNameInput = "team-second"

	updated, secondCmd := secondCreate.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if secondCmd == nil {
		t.Fatal("expected second save key to dispatch a save command")
	}

	secondInFlight := updated.(Model)
	if secondInFlight.Overlay.SaveRevision != 2 {
		t.Fatalf("expected second save revision %d, got %d", 2, secondInFlight.Overlay.SaveRevision)
	}

	priorCompletion := MsgOverlaySaveCompleted{
		Revision: 1,
		State: workspace.State{
			SelectedWorkspaceID: "ws-old",
			Workspaces:          []workspace.Workspace{{ID: "ws-old", Name: "old"}},
		},
	}
	updated, _ = secondInFlight.Update(priorCompletion)
	got := updated.(Model)
	if got.State.CurrentWorkspaceID() == "ws-old" {
		t.Fatalf("expected prior-session save completion to be ignored, got %#v", got.State.Snapshot)
	}
	if !got.Overlay.Active || !got.Overlay.SaveInFlight {
		t.Fatalf("expected second save to remain in flight after prior completion, got %#v", got.Overlay)
	}
	if got.UIMode != ModeFolder {
		t.Fatalf("expected UI mode to remain %q while second save in flight, got %q", ModeFolder, got.UIMode)
	}
}

func seededCreateOverlayModelForSaveTests(committer WorkspaceOverlayDraftCommitter) Model {
	m := NewModel(Config{
		InitialUIMode:                  ModeWorkspace,
		InitialState:                   localWorkspaceStateWithRepo("/tmp/current"),
		WorkspaceOverlayDraftCommitter: committer,
	})
	m.Overlay.Active = true
	m.Overlay.Mode = OverlayModeCreate
	m.Overlay.Focus = OverlayFocusCreateNameInput
	return m
}

type fakeWorkspaceOverlayDraftCommitter struct {
	state     workspace.State
	err       error
	lastDraft WorkspaceOverlayDraft
	calls     int
}

func (f *fakeWorkspaceOverlayDraftCommitter) CommitWorkspaceOverlayDraft(_ context.Context, draft WorkspaceOverlayDraft) (workspace.State, error) {
	f.calls++
	f.lastDraft = WorkspaceOverlayDraft{
		Name:        draft.Name,
		StagedRepos: append([]RepoCandidate(nil), draft.StagedRepos...),
	}
	if f.err != nil {
		return workspace.State{}, f.err
	}
	return cloneWorkspaceState(f.state), nil
}
