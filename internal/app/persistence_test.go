package app

import (
	"context"
	"testing"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestPersistence_RestartRestorePath_SaveThenLoad(t *testing.T) {
	store := &fakeStateStore{}
	m := seededModelWithRepos()
	m.StateStore = store
	m.State.Snapshot.Workspaces[0].Repos[1].ReleaseWorkflowRef = ".github/workflows/release.yml"

	updated, _ := m.Update(MsgSelectRepo{RepoID: "repo-2"})
	current := updated.(Model)
	if store.saveCalls == 0 {
		t.Fatal("expected state mutation to trigger persistence save")
	}

	syncUpdate, _ := current.Update(MsgSyncRefreshCompleted{
		WorkspaceID: "ws-1",
		RepoID:      "repo-2",
		Status: workspace.RepoStatus{
			PR:      workspace.StatusSuccess,
			CI:      workspace.StatusFailure,
			Release: workspace.StatusInProgress,
		},
	})
	afterSync := syncUpdate.(Model)
	if store.saveCalls < 2 {
		t.Fatalf("expected snapshot update to trigger another save, got %d saves", store.saveCalls)
	}

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	restored := NewModel(Config{InitialState: loaded})
	if restored.State.CurrentRepoID() != "repo-2" {
		t.Fatalf("expected selected repo %q after restart restore, got %q", "repo-2", restored.State.CurrentRepoID())
	}
	snapshot := restored.repoStatusSnapshotForCurrentRepo()
	if snapshot.PR != workspace.StatusSuccess || snapshot.CI != workspace.StatusFailure || snapshot.Release != workspace.StatusInProgress {
		t.Fatalf(
			"expected restored snapshot {pr=%q ci=%q release=%q}, got {pr=%q ci=%q release=%q}",
			workspace.StatusSuccess,
			workspace.StatusFailure,
			workspace.StatusInProgress,
			snapshot.PR,
			snapshot.CI,
			snapshot.Release,
		)
	}
	if snapshot.LastSyncedAt.IsZero() {
		t.Fatal("expected restored snapshot to retain LastSyncedAt")
	}

	if got := afterSync.State.CurrentRepoID(); got != "repo-2" {
		t.Fatalf("expected current repo to remain repo-2 after sync update, got %q", got)
	}
}

type fakeStateStore struct {
	state     workspace.State
	saveCalls int
}

func (f *fakeStateStore) Load(context.Context) (workspace.State, error) {
	return cloneWorkspaceState(f.state), nil
}

func (f *fakeStateStore) Save(_ context.Context, st workspace.State) error {
	f.saveCalls++
	f.state = cloneWorkspaceState(st)
	return nil
}
