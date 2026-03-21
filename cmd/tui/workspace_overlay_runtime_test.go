package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/app"
	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestRuntimeWorkspaceOverlayScanner_ScanMapsDiscoveredReposToCandidates(t *testing.T) {
	root := t.TempDir()
	repoA := initTempGitRepoAtPath(t, filepath.Join(root, "zeta"))
	repoB := initTempGitRepoAtPath(t, filepath.Join(root, "alpha"))

	scanner := runtimeWorkspaceOverlayScanner{}
	got, err := scanner.ScanRepoCandidates(context.Background(), root)
	if err != nil {
		t.Fatalf("ScanRepoCandidates() error = %v", err)
	}

	want := []app.RepoCandidate{
		{Name: filepath.Base(repoB), Path: canonicalPath(t, repoB)},
		{Name: filepath.Base(repoA), Path: canonicalPath(t, repoA)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ScanRepoCandidates() = %#v, want %#v", got, want)
	}
}

func TestComposeRuntimeModel_WiresWorkspaceOverlayScannerAndDefaultScanPath(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	t.Setenv(envTestMode, "0")
	t.Setenv(envStatePath, statePath)
	seedRuntimeState(t, statePath, localWorkspaceStateForRuntime())

	cwd := t.TempDir()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir(%q) error = %v", cwd, err)
	}
	defer func() {
		_ = os.Chdir(previousWD)
	}()

	model, closeFn, err := composeRuntimeModel(context.Background(), LaunchOptions{Mode: LaunchWorkspace, WorkspaceName: "team-a"})
	if err != nil {
		t.Fatalf("composeRuntimeModel() error = %v", err)
	}
	if closeFn == nil {
		t.Fatal("expected close function")
	}
	defer func() { _ = closeFn() }()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	opened := updated.(app.Model)
	if got := opened.Overlay.ScanPathInput; canonicalPath(t, got) != canonicalPath(t, cwd) {
		t.Fatalf("expected overlay scan path equivalent to %q, got %q", cwd, got)
	}

	updated, _ = opened.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	create := updated.(app.Model)
	create.Overlay.ScanPathInput = cwd
	create.Overlay.ScanRevision = 7
	create.Overlay.Active = true
	create.Overlay.Mode = app.OverlayModeCreate
	create.Overlay.Focus = app.OverlayFocusScanPathInput

	updated, scanCmd := create.Update(app.MsgOverlayScanScheduled{Revision: 7})
	scanning := updated.(app.Model)
	if !scanning.Overlay.ScanInFlight {
		t.Fatal("expected overlay scan to stay in flight while scanner command is scheduled")
	}
	if scanCmd == nil {
		t.Fatal("expected overlay scan scheduled to return a command")
	}

	msg := scanCmd()
	completed, ok := msg.(app.MsgOverlayScanCompleted)
	if !ok {
		t.Fatalf("expected %T, got %T", app.MsgOverlayScanCompleted{}, msg)
	}
	if completed.Revision != 7 {
		t.Fatalf("expected revision %d, got %d", 7, completed.Revision)
	}
	if completed.Err != nil {
		t.Fatalf("expected nil completion error, got %v", completed.Err)
	}
}

func TestComposeRuntimeModel_GetwdFailureFallsBackAndStillSucceeds(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	t.Setenv(envTestMode, "0")
	t.Setenv(envStatePath, statePath)
	seedRuntimeState(t, statePath, localWorkspaceStateForRuntime())

	previousGetwd := runtimeGetwd
	runtimeGetwd = func() (string, error) {
		return "", errors.New("getwd sentinel")
	}
	t.Cleanup(func() {
		runtimeGetwd = previousGetwd
	})

	model, closeFn, err := composeRuntimeModel(context.Background(), LaunchOptions{Mode: LaunchWorkspace, WorkspaceName: "team-a"})
	if err != nil {
		t.Fatalf("composeRuntimeModel() error = %v", err)
	}
	if closeFn == nil {
		t.Fatal("expected close function")
	}
	defer func() { _ = closeFn() }()

	if got := model.DefaultOverlayScanPath; got != "" {
		t.Fatalf("expected empty runtime default overlay scan path, got %q", got)
	}
}

func TestWorkspaceOverlayScanner_ModelUpdate_UsesScannerResult(t *testing.T) {
	wantCandidates := []app.RepoCandidate{{Name: "api", Path: "/tmp/api"}}
	m := app.NewModel(app.Config{
		InitialUIMode: app.ModeWorkspace,
		WorkspaceOverlayScanner: fakeWorkspaceOverlayScanner{
			candidates: wantCandidates,
		},
		DefaultOverlayScanPath: "/tmp/projects",
	})
	m.Overlay.Active = true
	m.Overlay.Mode = app.OverlayModeCreate
	m.Overlay.Focus = app.OverlayFocusScanPathInput
	m.Overlay.ScanPathInput = "/tmp/projects"
	m.Overlay.ScanRevision = 3

	updated, cmd := m.Update(app.MsgOverlayScanScheduled{Revision: 3})
	got := updated.(app.Model)
	if !got.Overlay.ScanInFlight {
		t.Fatal("expected scan in flight after scheduled message")
	}
	if cmd == nil {
		t.Fatal("expected scheduled scan command")
	}

	msg := cmd()
	completed, ok := msg.(app.MsgOverlayScanCompleted)
	if !ok {
		t.Fatalf("expected %T, got %T", app.MsgOverlayScanCompleted{}, msg)
	}
	if completed.Revision != 3 {
		t.Fatalf("expected revision %d, got %d", 3, completed.Revision)
	}
	if !reflect.DeepEqual(completed.Candidates, wantCandidates) {
		t.Fatalf("expected candidates %#v, got %#v", wantCandidates, completed.Candidates)
	}
}

func TestWorkspaceOverlayScanner_ModelUpdate_PropagatesScannerError(t *testing.T) {
	wantErr := errors.New("scan sentinel")
	m := app.NewModel(app.Config{
		WorkspaceOverlayScanner: fakeWorkspaceOverlayScanner{err: wantErr},
	})
	m.Overlay.Active = true
	m.Overlay.Mode = app.OverlayModeCreate
	m.Overlay.ScanRevision = 2
	m.Overlay.ScanPathInput = "/tmp/projects"

	_, cmd := m.Update(app.MsgOverlayScanScheduled{Revision: 2})
	if cmd == nil {
		t.Fatal("expected scanner command")
	}

	msg := cmd()
	completed, ok := msg.(app.MsgOverlayScanCompleted)
	if !ok {
		t.Fatalf("expected %T, got %T", app.MsgOverlayScanCompleted{}, msg)
	}
	if !errors.Is(completed.Err, wantErr) {
		t.Fatalf("expected completion error %v, got %v", wantErr, completed.Err)
	}
}

func TestWorkspaceOverlayDraftCommitter_CommitCreatesWorkspaceAddsReposAndSelectsFirstRepo(t *testing.T) {
	store := &runtimeMemoryStateStore{state: localWorkspaceStateForRuntime()}
	committer := runtimeWorkspaceOverlayDraftCommitter{stateStore: store}

	got, err := committer.CommitWorkspaceOverlayDraft(context.Background(), app.WorkspaceOverlayDraft{
		Name: " team-b ",
		StagedRepos: []app.RepoCandidate{
			{Name: "web", Path: "/tmp/web"},
			{Name: "api", Path: "/tmp/api"},
		},
	})
	if err != nil {
		t.Fatalf("CommitWorkspaceOverlayDraft() error = %v", err)
	}

	ws, ok := runtimeWorkspaceByName(got, "team-b")
	if !ok {
		t.Fatalf("expected workspace %q in %#v", "team-b", got.Workspaces)
	}
	if got.SelectedWorkspaceID != ws.ID {
		t.Fatalf("expected selected workspace %q, got %q", ws.ID, got.SelectedWorkspaceID)
	}
	if len(ws.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %#v", ws.Repos)
	}
	if ws.SelectedRepoID != ws.Repos[0].ID {
		t.Fatalf("expected selected repo %q, got %q", ws.Repos[0].ID, ws.SelectedRepoID)
	}
	if ws.Repos[0].Path != "/tmp/web" {
		t.Fatalf("expected first repo path %q, got %q", "/tmp/web", ws.Repos[0].Path)
	}
}

func TestWorkspaceOverlayDraftCommitter_DuplicateWorkspaceNameReturnsWorkspaceAlreadyExists(t *testing.T) {
	store := &runtimeMemoryStateStore{state: localWorkspaceStateForRuntime()}
	committer := runtimeWorkspaceOverlayDraftCommitter{stateStore: store}

	_, err := committer.CommitWorkspaceOverlayDraft(context.Background(), app.WorkspaceOverlayDraft{
		Name: " team-a ",
	})
	if err == nil {
		t.Fatal("expected duplicate workspace error")
	}
	if err.Error() != "workspace already exists" {
		t.Fatalf("expected duplicate error %q, got %q", "workspace already exists", err.Error())
	}

	if got := store.state.SelectedWorkspaceID; got != "ws-team-a" {
		t.Fatalf("expected selected workspace to remain %q, got %q", "ws-team-a", got)
	}
}

func TestWorkspaceOverlayDraftCommitter_EmptyStagedReposSelectsWorkspaceOnly(t *testing.T) {
	store := &runtimeMemoryStateStore{state: localWorkspaceStateForRuntime()}
	committer := runtimeWorkspaceOverlayDraftCommitter{stateStore: store}

	got, err := committer.CommitWorkspaceOverlayDraft(context.Background(), app.WorkspaceOverlayDraft{
		Name: "empty",
	})
	if err != nil {
		t.Fatalf("CommitWorkspaceOverlayDraft() error = %v", err)
	}

	ws, ok := runtimeWorkspaceByName(got, "empty")
	if !ok {
		t.Fatalf("expected workspace %q in %#v", "empty", got.Workspaces)
	}
	if got.SelectedWorkspaceID != ws.ID {
		t.Fatalf("expected selected workspace %q, got %q", ws.ID, got.SelectedWorkspaceID)
	}
	if ws.SelectedRepoID != "" {
		t.Fatalf("expected no selected repo, got %q", ws.SelectedRepoID)
	}
	if len(ws.Repos) != 0 {
		t.Fatalf("expected empty repo list, got %#v", ws.Repos)
	}
}

type fakeWorkspaceOverlayScanner struct {
	candidates []app.RepoCandidate
	err        error
}

func (f fakeWorkspaceOverlayScanner) ScanRepoCandidates(_ context.Context, rootPath string) ([]app.RepoCandidate, error) {
	_ = rootPath
	if f.err != nil {
		return nil, f.err
	}
	return append([]app.RepoCandidate(nil), f.candidates...), nil
}

func initTempGitRepoAtPath(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	cmd := exec.Command("git", "-C", path, "init")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init at %q failed: %v\n%s", path, err, string(out))
	}
	return path
}

func localWorkspaceStateForRuntime() workspace.State {
	return workspace.State{
		SelectedWorkspaceID: "ws-team-a",
		Workspaces: []workspace.Workspace{
			{
				ID:             "ws-team-a",
				Name:           "team-a",
				SelectedRepoID: "repo-a",
				Repos:          []workspace.Repo{{ID: "repo-a", Name: "api", Path: "/tmp/api"}},
			},
		},
	}
}

type runtimeMemoryStateStore struct {
	state workspace.State
}

func (s *runtimeMemoryStateStore) Load(context.Context) (workspace.State, error) {
	return cloneWorkspaceStateForRuntimeTests(s.state), nil
}

func (s *runtimeMemoryStateStore) Save(_ context.Context, state workspace.State) error {
	s.state = cloneWorkspaceStateForRuntimeTests(state)
	return nil
}

func cloneWorkspaceStateForRuntimeTests(state workspace.State) workspace.State {
	cloned := state
	cloned.Workspaces = make([]workspace.Workspace, len(state.Workspaces))
	for i := range state.Workspaces {
		ws := state.Workspaces[i]
		ws.Repos = append([]workspace.Repo(nil), ws.Repos...)
		cloned.Workspaces[i] = ws
	}
	if state.RepoStatusSnapshots != nil {
		cloned.RepoStatusSnapshots = make(map[string]workspace.RepoStatusSnapshot, len(state.RepoStatusSnapshots))
		for key, value := range state.RepoStatusSnapshots {
			cloned.RepoStatusSnapshots[key] = value
		}
	}
	return cloned
}

func runtimeWorkspaceByName(state workspace.State, name string) (workspace.Workspace, bool) {
	for _, ws := range state.Workspaces {
		if ws.Name == name {
			return ws, true
		}
	}
	return workspace.Workspace{}, false
}
