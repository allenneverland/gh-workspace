package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestUpdate_SelectRepo_ChangesCurrentRepo(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(MsgSelectRepo{RepoID: "repo-2"})
	got := updated.(Model)
	if got.State.CurrentRepoID() != "repo-2" {
		t.Fatalf("expected selected repo %q, got %q", "repo-2", got.State.CurrentRepoID())
	}
}

func TestUpdate_SelectWorkspace_ChangesCurrentWorkspace(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(MsgSelectWorkspace{WorkspaceID: "ws-2"})
	got := updated.(Model)
	if got.State.CurrentWorkspaceID() != "ws-2" {
		t.Fatalf("expected selected workspace %q, got %q", "ws-2", got.State.CurrentWorkspaceID())
	}
	if got.State.CurrentRepoID() != "repo-3" {
		t.Fatalf("expected selected repo %q after workspace switch, got %q", "repo-3", got.State.CurrentRepoID())
	}
}

func TestUpdate_KeyRemoveRepo_RemovesSelectedRepo(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := updated.(Model)

	workspace, ok := got.State.CurrentWorkspace()
	if !ok {
		t.Fatal("expected current workspace to exist")
	}
	if len(workspace.Repos) != 1 {
		t.Fatalf("expected one repo after remove, got %d", len(workspace.Repos))
	}
	if workspace.Repos[0].ID != "repo-2" {
		t.Fatalf("expected remaining repo %q, got %q", "repo-2", workspace.Repos[0].ID)
	}
	if workspace.SelectedRepoID != "repo-2" {
		t.Fatalf("expected selected repo %q after remove, got %q", "repo-2", workspace.SelectedRepoID)
	}
}

func TestUpdate_KeyAddRepo_TriggersRequestMessagePath(t *testing.T) {
	m := seededModelWithRepos()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected add-repo key to return a command")
	}

	msg := cmd()
	if _, ok := msg.(MsgRequestAddRepo); !ok {
		t.Fatalf("expected command message %T, got %T", MsgRequestAddRepo{}, msg)
	}

	afterMsg, _ := updated.(Model).Update(msg)
	got := afterMsg.(Model)
	if !got.AddRepoRequested {
		t.Fatal("expected AddRepoRequested to be true")
	}
	if got.StatusMessage == "" {
		t.Fatal("expected status message to be set")
	}
}

func TestUpdate_KeyEnter_InvalidRepoPathExists_MarksRepoHealthy(t *testing.T) {
	existingPath := t.TempDir()
	m := NewModel(Config{
		InitialState: singleRepoState(existingPath, workspace.RepoInvalid),
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	repo, ok := got.State.CurrentRepo()
	if !ok {
		t.Fatal("expected selected repo")
	}
	if repo.Health != workspace.RepoHealthy {
		t.Fatalf("expected repo health %q, got %q", workspace.RepoHealthy, repo.Health)
	}
	if !strings.Contains(got.StatusMessage, "recovered") {
		t.Fatalf("expected recovery status message, got %q", got.StatusMessage)
	}
}

func TestUpdate_KeyEnter_InvalidRepoPathMissing_StaysInvalid(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "does-not-exist")
	m := NewModel(Config{
		InitialState: singleRepoState(missingPath, workspace.RepoInvalid),
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	repo, ok := got.State.CurrentRepo()
	if !ok {
		t.Fatal("expected selected repo")
	}
	if repo.Health != workspace.RepoInvalid {
		t.Fatalf("expected repo health %q, got %q", workspace.RepoInvalid, repo.Health)
	}
	if !strings.Contains(got.StatusMessage, "invalid") {
		t.Fatalf("expected invalid status message, got %q", got.StatusMessage)
	}
}

func TestUpdate_DoesNotMutateSourceModel_OnSelectRepo(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(MsgSelectRepo{RepoID: "repo-2"})
	got := updated.(Model)
	if got.State.CurrentRepoID() != "repo-2" {
		t.Fatalf("expected updated model repo %q, got %q", "repo-2", got.State.CurrentRepoID())
	}
	if m.State.CurrentRepoID() != "repo-1" {
		t.Fatalf("expected source model repo to remain %q, got %q", "repo-1", m.State.CurrentRepoID())
	}
}

func TestUpdate_DoesNotMutateSourceModel_OnRemoveRepo(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := updated.(Model)
	updatedWorkspace, ok := got.State.CurrentWorkspace()
	if !ok {
		t.Fatal("expected updated workspace")
	}
	if len(updatedWorkspace.Repos) != 1 {
		t.Fatalf("expected updated model repo count %d, got %d", 1, len(updatedWorkspace.Repos))
	}

	sourceWorkspace, ok := m.State.CurrentWorkspace()
	if !ok {
		t.Fatal("expected source workspace")
	}
	if len(sourceWorkspace.Repos) != 2 {
		t.Fatalf("expected source model repo count %d, got %d", 2, len(sourceWorkspace.Repos))
	}
	if sourceWorkspace.SelectedRepoID != "repo-1" {
		t.Fatalf("expected source selected repo %q, got %q", "repo-1", sourceWorkspace.SelectedRepoID)
	}
}

func TestUpdate_SelectRepoBinding_DrivesInvalidRecovery(t *testing.T) {
	existingPath := t.TempDir()
	m := NewModel(Config{
		InitialState: singleRepoState(existingPath, workspace.RepoInvalid),
	})
	m.Keys.SelectRepo = key.NewBinding(key.WithKeys("s"))

	enterUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	enterModel := enterUpdated.(Model)
	enterRepo, ok := enterModel.State.CurrentRepo()
	if !ok {
		t.Fatal("expected repo after enter")
	}
	if enterRepo.Health != workspace.RepoInvalid {
		t.Fatalf("expected enter to not recover with remapped key binding, got %q", enterRepo.Health)
	}

	bindingUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	bindingModel := bindingUpdated.(Model)
	bindingRepo, ok := bindingModel.State.CurrentRepo()
	if !ok {
		t.Fatal("expected repo after binding key")
	}
	if bindingRepo.Health != workspace.RepoHealthy {
		t.Fatalf("expected bound select key to recover repo to %q, got %q", workspace.RepoHealthy, bindingRepo.Health)
	}
}

func TestUpdate_FolderMode_WorkspaceKeysAreDisabled(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()
	m.UIMode = ModeFolder
	m.State.Snapshot.SelectedWorkspaceID = workspace.LocalWorkspaceID

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if cmd != nil {
		t.Fatal("expected no command for disabled workspace navigation key in folder mode")
	}
	got := updated.(Model)
	if got.State.CurrentWorkspaceID() != workspace.LocalWorkspaceID {
		t.Fatalf("expected selected workspace to remain %q, got %q", workspace.LocalWorkspaceID, got.State.CurrentWorkspaceID())
	}
}

func TestUpdate_FolderMode_KeyAddRepo_ActivatesRepoPathInput(t *testing.T) {
	m := seededFolderModeModelWithLocalRepo()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil {
		t.Fatal("expected no command when entering folder-mode repo path input")
	}
	got := updated.(Model)
	if !got.RepoPathInputActive {
		t.Fatal("expected repo path input to become active in folder mode")
	}
}

func TestUpdate_FolderMode_RepoPathInput_EnterSubmitsPath(t *testing.T) {
	m := seededFolderModeModelWithLocalRepo()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	active := step.(Model)
	step, _ = active.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/new-repo")})
	typed := step.(Model)

	updated, cmd := typed.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter in repo path input mode to emit submit command")
	}

	msg := cmd()
	submit, ok := msg.(MsgSubmitRepoPath)
	if !ok {
		t.Fatalf("expected submit message type %T, got %T", MsgSubmitRepoPath{}, msg)
	}
	if submit.Path != "/tmp/new-repo" {
		t.Fatalf("expected submitted path %q, got %q", "/tmp/new-repo", submit.Path)
	}

	got := updated.(Model)
	if got.RepoPathInputActive {
		t.Fatal("expected repo path input to close after submit")
	}
}

func TestUpdate_FolderMode_RepoPathInput_EscCancelsWithoutMutation(t *testing.T) {
	m := seededFolderModeModelWithLocalRepo()
	originalRepoID := m.State.CurrentRepoID()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	active := step.(Model)
	updated, cmd := active.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("expected esc cancel path input to emit no command")
	}

	got := updated.(Model)
	if got.RepoPathInputActive {
		t.Fatal("expected esc to close repo path input")
	}
	if got.State.CurrentRepoID() != originalRepoID {
		t.Fatalf("expected cancel to preserve selected repo %q, got %q", originalRepoID, got.State.CurrentRepoID())
	}
}

func TestUpdate_FolderMode_SubmitRepoPath_InvalidClearsSelection(t *testing.T) {
	submitter := &fakeRepoPathSubmitter{
		result: RepoPathSubmissionResult{
			State:         localWorkspaceStateWithoutRepo(),
			StatusMessage: "current folder is not a git repo",
		},
	}
	m := NewModel(Config{
		InitialUIMode:     ModeFolder,
		InitialState:      localWorkspaceStateWithRepo("/tmp/old"),
		RepoPathSubmitter: submitter,
	})

	updated, _ := m.Update(MsgSubmitRepoPath{Path: "/tmp/not-a-repo"})
	got := updated.(Model)
	if submitter.calls != 1 || submitter.lastPath != "/tmp/not-a-repo" {
		t.Fatalf("expected submitter to be called once with path %q, got calls=%d path=%q", "/tmp/not-a-repo", submitter.calls, submitter.lastPath)
	}
	if got.State.CurrentRepoID() != "" {
		t.Fatalf("expected local repo selection cleared, got %q", got.State.CurrentRepoID())
	}
	if got.StatusMessage != "current folder is not a git repo" {
		t.Fatalf("expected status %q, got %q", "current folder is not a git repo", got.StatusMessage)
	}
}

func TestUpdate_FolderMode_SubmitRepoPath_ValidReplacesSelection(t *testing.T) {
	replaced := workspace.State{
		SelectedWorkspaceID: workspace.LocalWorkspaceID,
		Workspaces: []workspace.Workspace{
			{
				ID:             workspace.LocalWorkspaceID,
				Name:           workspace.LocalWorkspaceName,
				SelectedRepoID: "repo-new",
				Repos: []workspace.Repo{
					{ID: "repo-new", Name: "new-repo", Path: "/tmp/new-repo", Health: workspace.RepoHealthy},
				},
			},
		},
	}
	submitter := &fakeRepoPathSubmitter{
		result: RepoPathSubmissionResult{
			State:         replaced,
			StatusMessage: "added repo: new-repo",
		},
	}
	m := NewModel(Config{
		InitialUIMode:     ModeFolder,
		InitialState:      localWorkspaceStateWithRepo("/tmp/old"),
		RepoPathSubmitter: submitter,
	})

	updated, _ := m.Update(MsgSubmitRepoPath{Path: "/tmp/new-repo"})
	got := updated.(Model)
	repo, ok := got.State.CurrentRepo()
	if !ok {
		t.Fatal("expected selected repo after valid submit")
	}
	if repo.Path != "/tmp/new-repo" {
		t.Fatalf("expected selected repo path %q, got %q", "/tmp/new-repo", repo.Path)
	}
	if got.StatusMessage != "added repo: new-repo" {
		t.Fatalf("expected status %q, got %q", "added repo: new-repo", got.StatusMessage)
	}
}

func TestUpdate_WorkspaceMode_NavigationSkipsSystemWorkspace(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()
	m.State.Snapshot.Workspaces = append(
		m.State.Snapshot.Workspaces,
		workspace.Workspace{
			ID:             "ws-team-b",
			Name:           "team-b",
			SelectedRepoID: "repo-b",
			Repos: []workspace.Repo{
				{ID: "repo-b", Name: "web", Path: "/tmp/web", Health: workspace.RepoHealthy},
			},
		},
	)
	m.State.Snapshot.SelectedWorkspaceID = "ws-team-a"

	prevUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	prev := prevUpdated.(Model)
	if prev.State.CurrentWorkspaceID() != "ws-team-b" {
		t.Fatalf("expected previous workspace %q, got %q", "ws-team-b", prev.State.CurrentWorkspaceID())
	}
	if prev.State.CurrentWorkspaceID() == workspace.LocalWorkspaceID {
		t.Fatalf("workspace mode navigation should skip %q", workspace.LocalWorkspaceID)
	}

	nextUpdated, _ := prev.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	next := nextUpdated.(Model)
	if next.State.CurrentWorkspaceID() != "ws-team-a" {
		t.Fatalf("expected next workspace %q, got %q", "ws-team-a", next.State.CurrentWorkspaceID())
	}
	if next.State.CurrentWorkspaceID() == workspace.LocalWorkspaceID {
		t.Fatalf("workspace mode navigation should skip %q", workspace.LocalWorkspaceID)
	}
}

func TestUpdate_WorkspaceMode_SelectSystemWorkspaceAutoSelectsFirstUser(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	updated, _ := m.Update(MsgSelectWorkspace{WorkspaceID: workspace.LocalWorkspaceID})
	got := updated.(Model)
	if got.State.CurrentWorkspaceID() != "ws-team-a" {
		t.Fatalf("expected system selection to normalize to first user workspace %q, got %q", "ws-team-a", got.State.CurrentWorkspaceID())
	}
}

func TestUpdate_KeyQuit_OutsideLazygit_RequestsAppQuit(t *testing.T) {
	m := seededModelWithRepos()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_ = updated.(Model)
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected quit message type %T, got %T", tea.QuitMsg{}, cmd())
	}
}

func TestUpdate_KeyTab_CyclesCenterTabsForwardAndBackward(t *testing.T) {
	m := seededModelWithRepos()

	forwardUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	forward := forwardUpdated.(Model)
	if forward.ActiveTab != TabWorktrees {
		t.Fatalf("expected active tab %q after tab, got %q", TabWorktrees, forward.ActiveTab)
	}

	backwardUpdated, _ := forward.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	backward := backwardUpdated.(Model)
	if backward.ActiveTab != TabOverview {
		t.Fatalf("expected active tab %q after shift+tab, got %q", TabOverview, backward.ActiveTab)
	}
}

func TestUpdate_KeyNumber_SelectsCenterTabDirectly(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	got := updated.(Model)
	if got.ActiveTab != TabWorktrees {
		t.Fatalf("expected active tab %q after key 2, got %q", TabWorktrees, got.ActiveTab)
	}
}

func TestUpdate_WindowSizeMsg_AdjustsPaneWidths(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	got := updated.(Model)

	if got.WindowWidth != 120 || got.WindowHeight != 32 {
		t.Fatalf("expected window size 120x32, got %dx%d", got.WindowWidth, got.WindowHeight)
	}
	if got.LeftPaneWidth+got.CenterPaneWidth+got.RightPaneWidth != 120 {
		t.Fatalf(
			"expected pane widths to sum to %d, got left=%d center=%d right=%d",
			120,
			got.LeftPaneWidth,
			got.CenterPaneWidth,
			got.RightPaneWidth,
		)
	}
	if got.CenterPaneWidth < 1 {
		t.Fatalf("expected positive center width, got %d", got.CenterPaneWidth)
	}
}

func TestNewModel_WorkspaceMode_WithOnlySystemWorkspace_LeavesSelectionEmpty(t *testing.T) {
	m := NewModel(Config{
		InitialUIMode: ModeWorkspace,
		InitialState: workspace.State{
			SelectedWorkspaceID: workspace.LocalWorkspaceID,
			Workspaces: []workspace.Workspace{
				{
					ID:   workspace.LocalWorkspaceID,
					Name: workspace.LocalWorkspaceName,
				},
			},
		},
	})

	if got := m.State.CurrentWorkspaceID(); got != "" {
		t.Fatalf("expected empty selected workspace when only system workspace exists in workspace mode, got %q", got)
	}
}

func seededModelWithRepos() Model {
	return NewModel(Config{
		InitialState: workspace.State{
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
				{
					ID:             "ws-2",
					Name:           "beta",
					SelectedRepoID: "repo-3",
					Repos: []workspace.Repo{
						{ID: "repo-3", Name: "ops", Path: "/tmp/ops", Health: workspace.RepoHealthy},
					},
				},
			},
		},
	})
}

func singleRepoState(path string, health workspace.RepoHealth) workspace.State {
	return workspace.State{
		SelectedWorkspaceID: "ws-1",
		Workspaces: []workspace.Workspace{
			{
				ID:             "ws-1",
				Name:           "alpha",
				SelectedRepoID: "repo-1",
				Repos: []workspace.Repo{
					{ID: "repo-1", Name: "api", Path: path, Health: health},
				},
			},
		},
	}
}

func seededFolderModeModelWithLocalRepo() Model {
	return NewModel(Config{
		InitialUIMode: ModeFolder,
		InitialState:  localWorkspaceStateWithRepo("/tmp/current"),
	})
}

func localWorkspaceStateWithRepo(path string) workspace.State {
	return workspace.State{
		SelectedWorkspaceID: workspace.LocalWorkspaceID,
		Workspaces: []workspace.Workspace{
			{
				ID:             workspace.LocalWorkspaceID,
				Name:           workspace.LocalWorkspaceName,
				SelectedRepoID: "repo-local",
				Repos: []workspace.Repo{
					{ID: "repo-local", Name: "local", Path: path, Health: workspace.RepoHealthy},
				},
			},
		},
	}
}

func localWorkspaceStateWithoutRepo() workspace.State {
	return workspace.State{
		SelectedWorkspaceID: workspace.LocalWorkspaceID,
		Workspaces: []workspace.Workspace{
			{
				ID:   workspace.LocalWorkspaceID,
				Name: workspace.LocalWorkspaceName,
			},
		},
	}
}

type fakeRepoPathSubmitter struct {
	result   RepoPathSubmissionResult
	err      error
	lastPath string
	calls    int
}

func (f *fakeRepoPathSubmitter) SubmitRepoPath(_ context.Context, path string) (RepoPathSubmissionResult, error) {
	f.calls++
	f.lastPath = path
	if f.err != nil {
		return RepoPathSubmissionResult{}, f.err
	}
	return f.result, nil
}
