package boltdb

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestStore_LoadReturnsEmptyStateWhenNotYetSaved(t *testing.T) {
	store := openTestStore(t)
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.SelectedWorkspaceID != "" {
		t.Fatalf("expected empty selected workspace ID, got %q", got.SelectedWorkspaceID)
	}
	if len(got.Workspaces) != 0 {
		t.Fatalf("expected zero workspaces, got %d", len(got.Workspaces))
	}
}

func TestStore_SaveThenLoad_PersistsWorkspaceStateAcrossReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "workspace-state.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	want := workspace.State{
		SelectedWorkspaceID: "ws-1",
		Workspaces: []workspace.Workspace{
			{
				ID:             "ws-1",
				Name:           "default",
				SelectedRepoID: "repo-1",
				Repos: []workspace.Repo{
					{
						ID:                 "repo-1",
						Name:               "api",
						Path:               "/tmp/api",
						DefaultBranch:      "main",
						ReleaseWorkflowRef: ".github/workflows/release.yml",
					},
				},
			},
		},
	}

	if err := store.Save(context.Background(), want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() after Save() error = %v", err)
	}

	reopened, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() after close error = %v", err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("Close() reopen error = %v", err)
		}
	}()

	got, err := reopened.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() after reopen error = %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("state mismatch after reopen:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "workspace-state.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return store
}
