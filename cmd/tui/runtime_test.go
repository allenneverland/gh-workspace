package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/allenneverland/gh-workspace/internal/app"
	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	"github.com/allenneverland/gh-workspace/internal/store/boltdb"
)

func TestComposeRuntimeModel_RestartRestorePath_SaveThenLoad(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	t.Setenv(envTestMode, "0")
	t.Setenv(envStatePath, statePath)

	seedStore, err := boltdb.Open(statePath)
	if err != nil {
		t.Fatalf("Open(seed store) error = %v", err)
	}
	seed := workspace.State{
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
	if err := seedStore.Save(context.Background(), seed); err != nil {
		t.Fatalf("Save(seed) error = %v", err)
	}
	if err := seedStore.Close(); err != nil {
		t.Fatalf("Close(seed store) error = %v", err)
	}

	opts := LaunchOptions{Mode: LaunchWorkspace, WorkspaceName: "alpha"}
	model, closeFn, err := composeRuntimeModel(context.Background(), opts)
	if err != nil {
		t.Fatalf("composeRuntimeModel() error = %v", err)
	}
	if closeFn == nil {
		t.Fatal("expected close function for persistent runtime model")
	}

	if !model.State.SelectRepo("repo-2") {
		t.Fatal("expected repo selection update to succeed before restart")
	}
	if err := model.StateStore.Save(context.Background(), model.State.Snapshot); err != nil {
		t.Fatalf("Save(updated state) error = %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatalf("closeFn() error = %v", err)
	}

	restarted, restartedClose, err := composeRuntimeModel(context.Background(), opts)
	if err != nil {
		t.Fatalf("composeRuntimeModel(restart) error = %v", err)
	}
	if restartedClose == nil {
		t.Fatal("expected close function on restart runtime model")
	}
	defer func() { _ = restartedClose() }()

	if got := restarted.State.CurrentRepoID(); got != "repo-2" {
		t.Fatalf("expected selected repo %q after restart restore, got %q", "repo-2", got)
	}
}

func TestComposeRuntimeModel_LaunchFolder_NonGitClearsLocalRepo(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	t.Setenv(envTestMode, "0")
	t.Setenv(envStatePath, statePath)

	seedState := workspace.State{
		SelectedWorkspaceID: workspace.LocalWorkspaceID,
		Workspaces: []workspace.Workspace{
			{
				ID:             workspace.LocalWorkspaceID,
				Name:           workspace.LocalWorkspaceName,
				SelectedRepoID: "repo-1",
				Repos: []workspace.Repo{
					{ID: "repo-1", Name: "legacy", Path: "/tmp/legacy", Health: workspace.RepoHealthy},
				},
			},
		},
	}
	seedRuntimeState(t, statePath, seedState)

	opts := LaunchOptions{Mode: LaunchFolder, Path: t.TempDir()}
	model, closeFn, err := composeRuntimeModel(context.Background(), opts)
	if err != nil {
		t.Fatalf("composeRuntimeModel(folder non-git) error = %v", err)
	}
	if closeFn == nil {
		t.Fatal("expected close function for persistent runtime model")
	}
	defer func() { _ = closeFn() }()

	if got := model.State.CurrentWorkspaceID(); got != workspace.LocalWorkspaceID {
		t.Fatalf("expected selected workspace %q, got %q", workspace.LocalWorkspaceID, got)
	}
	if got := model.UIMode; got != app.ModeFolder {
		t.Fatalf("expected UI mode %q, got %q", app.ModeFolder, got)
	}
	if got := model.State.CurrentRepoID(); got != "" {
		t.Fatalf("expected local repo to be cleared, got selected repo %q", got)
	}
	if got := model.StatusMessage; got != StatusCurrentFolderNotGit {
		t.Fatalf("expected status %q, got %q", StatusCurrentFolderNotGit, got)
	}
}

func TestComposeRuntimeModel_LaunchFolder_GitPathReplacesLocalRepo(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	t.Setenv(envTestMode, "0")
	t.Setenv(envStatePath, statePath)
	seedRuntimeState(t, statePath, workspace.State{})

	repoRoot := initTempGitRepo(t)
	nestedPath := filepath.Join(repoRoot, "nested")
	if err := os.MkdirAll(nestedPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(nested) error = %v", err)
	}

	opts := LaunchOptions{Mode: LaunchFolder, Path: nestedPath}
	model, closeFn, err := composeRuntimeModel(context.Background(), opts)
	if err != nil {
		t.Fatalf("composeRuntimeModel(folder git) error = %v", err)
	}
	if closeFn == nil {
		t.Fatal("expected close function for persistent runtime model")
	}
	defer func() { _ = closeFn() }()

	if got := model.State.CurrentWorkspaceID(); got != workspace.LocalWorkspaceID {
		t.Fatalf("expected selected workspace %q, got %q", workspace.LocalWorkspaceID, got)
	}
	if got := model.UIMode; got != app.ModeFolder {
		t.Fatalf("expected UI mode %q, got %q", app.ModeFolder, got)
	}
	repo, ok := model.State.CurrentRepo()
	if !ok {
		t.Fatal("expected a selected local repo after git folder launch")
	}
	if canonicalPath(t, repo.Path) != canonicalPath(t, repoRoot) {
		t.Fatalf("expected selected repo path equivalent to %q, got %q", repoRoot, repo.Path)
	}
	if repo.Name != filepath.Base(repoRoot) {
		t.Fatalf("expected selected repo name %q, got %q", filepath.Base(repoRoot), repo.Name)
	}
}

func TestComposeRuntimeModel_WiresRepoPathSubmitter(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	t.Setenv(envTestMode, "0")
	t.Setenv(envStatePath, statePath)
	seedRuntimeState(t, statePath, workspace.State{})

	opts := LaunchOptions{Mode: LaunchFolder, Path: t.TempDir()}
	model, closeFn, err := composeRuntimeModel(context.Background(), opts)
	if err != nil {
		t.Fatalf("composeRuntimeModel(folder) error = %v", err)
	}
	if closeFn == nil {
		t.Fatal("expected close function for persistent runtime model")
	}
	defer func() { _ = closeFn() }()

	if model.RepoPathSubmitter == nil {
		t.Fatal("expected repo path submitter to be wired in runtime model")
	}

	repoRoot := initTempGitRepo(t)
	result, err := model.RepoPathSubmitter.SubmitRepoPath(context.Background(), repoRoot)
	if err != nil {
		t.Fatalf("SubmitRepoPath() error = %v", err)
	}
	if got := currentRepoPathFromState(result.State); canonicalPath(t, got) != canonicalPath(t, repoRoot) {
		t.Fatalf("expected submitter state to select repo path %q, got %q", repoRoot, got)
	}
	if result.StatusMessage == "" {
		t.Fatal("expected submitter to return non-empty status message")
	}
}

func TestComposeRuntimeModel_LaunchWorkspace_SelectsNamedWorkspace(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	t.Setenv(envTestMode, "0")
	t.Setenv(envStatePath, statePath)

	seedState := workspace.State{
		SelectedWorkspaceID: workspace.LocalWorkspaceID,
		Workspaces: []workspace.Workspace{
			{ID: workspace.LocalWorkspaceID, Name: workspace.LocalWorkspaceName},
			{
				ID:             "ws-team-a",
				Name:           "team-a",
				SelectedRepoID: "repo-a",
				Repos: []workspace.Repo{
					{ID: "repo-a", Name: "api", Path: "/tmp/api", Health: workspace.RepoHealthy},
				},
			},
		},
	}
	seedRuntimeState(t, statePath, seedState)

	opts := LaunchOptions{Mode: LaunchWorkspace, WorkspaceName: "team-a"}
	model, closeFn, err := composeRuntimeModel(context.Background(), opts)
	if err != nil {
		t.Fatalf("composeRuntimeModel(workspace) error = %v", err)
	}
	if closeFn == nil {
		t.Fatal("expected close function for persistent runtime model")
	}
	defer func() { _ = closeFn() }()

	if got := model.State.CurrentWorkspaceID(); got != "ws-team-a" {
		t.Fatalf("expected selected workspace %q, got %q", "ws-team-a", got)
	}
	if got := model.UIMode; got != app.ModeWorkspace {
		t.Fatalf("expected UI mode %q, got %q", app.ModeWorkspace, got)
	}
}

func TestComposeRuntimeModel_LaunchWorkspace_NotFound(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	t.Setenv(envTestMode, "0")
	t.Setenv(envStatePath, statePath)
	seedRuntimeState(t, statePath, workspace.State{})

	opts := LaunchOptions{Mode: LaunchWorkspace, WorkspaceName: "does-not-exist"}
	_, closeFn, err := composeRuntimeModel(context.Background(), opts)
	if closeFn != nil {
		t.Fatal("expected nil close function when compose fails")
	}
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}
	if !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("expected ErrWorkspaceNotFound, got %v", err)
	}
}

func TestComposeRuntimeModel_TestMode_UsesNoopFallback(t *testing.T) {
	t.Setenv(envTestMode, "1")
	t.Setenv(envStatePath, filepath.Join(t.TempDir(), "state.db"))

	model, closeFn, err := composeRuntimeModel(context.Background(), LaunchOptions{
		Mode: LaunchFolder,
		Path: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("composeRuntimeModel(test mode) error = %v", err)
	}
	if closeFn != nil {
		t.Fatal("expected no close function in test-mode fallback runtime")
	}
	if model.StateStore != nil {
		t.Fatal("expected test-mode runtime to skip persistent state store wiring")
	}
	if got := model.UIMode; got != app.ModeFolder {
		t.Fatalf("expected UI mode %q in test mode, got %q", app.ModeFolder, got)
	}
}

func seedRuntimeState(t *testing.T, statePath string, state workspace.State) {
	t.Helper()

	store, err := boltdb.Open(statePath)
	if err != nil {
		t.Fatalf("Open(seed store) error = %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Save(context.Background(), state); err != nil {
		t.Fatalf("Save(seed) error = %v", err)
	}
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	cmd := exec.Command("git", "-C", root, "init")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, string(out))
	}
	return root
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("filepath.Abs(%q) error = %v", path, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(resolved)
}

func currentRepoPathFromState(state workspace.State) string {
	workspaceID := state.SelectedWorkspaceID
	if workspaceID == "" {
		return ""
	}
	for _, ws := range state.Workspaces {
		if ws.ID != workspaceID {
			continue
		}
		if ws.SelectedRepoID == "" {
			return ""
		}
		for _, repo := range ws.Repos {
			if repo.ID == ws.SelectedRepoID {
				return repo.Path
			}
		}
	}
	return ""
}
