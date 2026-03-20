package main

import (
	"context"
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

	model, closeFn, err := composeRuntimeModel(context.Background())
	if err != nil {
		t.Fatalf("composeRuntimeModel() error = %v", err)
	}
	if closeFn == nil {
		t.Fatal("expected close function for persistent runtime model")
	}

	updated, _ := model.Update(app.MsgSelectRepo{RepoID: "repo-2"})
	if got := updated.(app.Model).State.CurrentRepoID(); got != "repo-2" {
		t.Fatalf("expected selected repo %q before restart, got %q", "repo-2", got)
	}
	if err := closeFn(); err != nil {
		t.Fatalf("closeFn() error = %v", err)
	}

	restarted, restartedClose, err := composeRuntimeModel(context.Background())
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

func TestComposeRuntimeModel_TestMode_UsesNoopFallback(t *testing.T) {
	t.Setenv(envTestMode, "1")
	t.Setenv(envStatePath, filepath.Join(t.TempDir(), "state.db"))

	model, closeFn, err := composeRuntimeModel(context.Background())
	if err != nil {
		t.Fatalf("composeRuntimeModel(test mode) error = %v", err)
	}
	if closeFn != nil {
		t.Fatal("expected no close function in test-mode fallback runtime")
	}
	if model.StateStore != nil {
		t.Fatal("expected test-mode runtime to skip persistent state store wiring")
	}
}
