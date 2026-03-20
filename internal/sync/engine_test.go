package sync

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestSyncEngine_AutoPoll_OnlySelectedRepo(t *testing.T) {
	fetcher := &fakeStatusFetcher{}
	engine := NewEngine(fetcher, WithInterval(time.Minute))

	if _, err := engine.OnSelectionChanged(context.Background(), "workspace-1", "repo-2"); err != nil {
		t.Fatalf("OnSelectionChanged() error = %v", err)
	}
	fetcher.reset()

	if _, err := engine.OnTick(context.Background()); err != nil {
		t.Fatalf("OnTick() error = %v", err)
	}

	calls := fetcher.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("expected one polling call, got %d", len(calls))
	}
	call := calls[0]
	if call.workspaceID != "workspace-1" || call.repoID != "repo-2" {
		t.Fatalf("expected call to workspace-1/repo-2, got %s/%s", call.workspaceID, call.repoID)
	}
}

func TestSyncEngine_OnRepoSwitch_TriggersImmediateRefresh(t *testing.T) {
	fetcher := &fakeStatusFetcher{}
	engine := NewEngine(fetcher, WithInterval(time.Minute))

	if _, err := engine.OnSelectionChanged(context.Background(), "workspace-1", "repo-2"); err != nil {
		t.Fatalf("OnSelectionChanged() initial error = %v", err)
	}
	fetcher.reset()

	if _, err := engine.OnSelectionChanged(context.Background(), "workspace-1", "repo-3"); err != nil {
		t.Fatalf("OnSelectionChanged() switch error = %v", err)
	}

	calls := fetcher.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("expected one refresh call after switch, got %d", len(calls))
	}
	call := calls[0]
	if call.workspaceID != "workspace-1" || call.repoID != "repo-3" {
		t.Fatalf("expected immediate refresh for workspace-1/repo-3, got %s/%s", call.workspaceID, call.repoID)
	}
}

func TestSyncEngine_OnTick_NoSelection_DoesNotFetch(t *testing.T) {
	fetcher := &fakeStatusFetcher{}
	engine := NewEngine(fetcher, WithInterval(time.Minute))

	if _, err := engine.OnTick(context.Background()); err != nil {
		t.Fatalf("OnTick() error = %v", err)
	}

	calls := fetcher.snapshotCalls()
	if len(calls) != 0 {
		t.Fatalf("expected no fetch calls without selection, got %d", len(calls))
	}
}

func TestSyncEngine_ConcurrentSelectionAndRefresh_UsesConsistentSelectionSnapshot(t *testing.T) {
	fetcher := &fakeStatusFetcher{}
	engine := NewEngine(fetcher, WithInterval(time.Millisecond))

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		worker := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 120; j++ {
				repoID := fmt.Sprintf("repo-%d", (worker+j)%5)
				engine.SetSelection("workspace-1", repoID)
				if _, err := engine.RefreshNow(context.Background()); err != nil {
					t.Errorf("RefreshNow() error = %v", err)
					return
				}
				if _, err := engine.OnTick(context.Background()); err != nil {
					t.Errorf("OnTick() error = %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	calls := fetcher.snapshotCalls()
	if len(calls) == 0 {
		t.Fatal("expected fetch calls during concurrent refresh")
	}
	for i, call := range calls {
		if call.workspaceID != "workspace-1" {
			t.Fatalf("call[%d] workspace mismatch: got %q", i, call.workspaceID)
		}
		if call.repoID == "" {
			t.Fatalf("call[%d] has empty repo ID", i)
		}
	}
}

type fakeStatusFetcher struct {
	mu    sync.Mutex
	calls []fetchCall
}

type fetchCall struct {
	workspaceID string
	repoID      string
}

func (f *fakeStatusFetcher) FetchSelectedRepoStatus(_ context.Context, workspaceID, repoID string) (workspace.RepoStatus, error) {
	f.mu.Lock()
	f.calls = append(f.calls, fetchCall{workspaceID: workspaceID, repoID: repoID})
	f.mu.Unlock()
	return workspace.RepoStatus{
		PR:      workspace.StatusSuccess,
		CI:      workspace.StatusSuccess,
		Release: workspace.StatusSuccess,
	}, nil
}

func (f *fakeStatusFetcher) reset() {
	f.mu.Lock()
	f.calls = nil
	f.mu.Unlock()
}

func (f *fakeStatusFetcher) snapshotCalls() []fetchCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := make([]fetchCall, len(f.calls))
	copy(copied, f.calls)
	return copied
}
