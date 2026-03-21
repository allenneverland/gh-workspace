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
