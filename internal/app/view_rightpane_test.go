package app

import (
	"strings"
	"testing"
	"time"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestRightPane_RendersUnconfiguredRelease(t *testing.T) {
	m := seededModelWithRepos()

	out := m.View()

	assertContains(t, out, "release: unconfigured")
}

func TestRightPane_UnconfiguredReleasePrecedence_OverridesStoredSnapshot(t *testing.T) {
	tests := []workspace.Status{
		workspace.StatusSuccess,
		workspace.StatusFailure,
	}

	for _, storedRelease := range tests {
		t.Run(string(storedRelease), func(t *testing.T) {
			m := seededModelWithRepos()
			m.State.Snapshot.Workspaces[0].Repos[0].ReleaseWorkflowRef = ""
			m.State.Snapshot.RepoStatusSnapshots = map[string]workspace.RepoStatusSnapshot{
				repoStatusSnapshotKey("ws-1", "repo-1"): {
					Release: storedRelease,
				},
			}

			out := m.View()

			assertContains(t, out, "release: unconfigured")
			if strings.Contains(out, "release: "+string(storedRelease)) {
				t.Fatalf("expected unconfigured precedence over stored release %q, got:\n%s", storedRelease, out)
			}
		})
	}
}

func TestRightPane_RendersStatusAndLastSyncedAt(t *testing.T) {
	m := seededModelWithRepos()
	m.State.Snapshot.Workspaces[0].Repos[0].ReleaseWorkflowRef = ".github/workflows/release.yml"
	m.State.Snapshot.RepoStatusSnapshots = map[string]workspace.RepoStatusSnapshot{
		repoStatusSnapshotKey("ws-1", "repo-1"): {
			PR:           workspace.StatusInProgress,
			CI:           workspace.StatusFailure,
			Release:      workspace.StatusSuccess,
			LastSyncedAt: time.Date(2026, time.March, 20, 8, 30, 0, 0, time.UTC),
		},
	}

	out := m.View()

	assertContains(t, out, "pr: in_progress")
	assertContains(t, out, "ci: failure")
	assertContains(t, out, "release: success")
	assertContains(t, out, "lastSyncedAt: 2026-03-20T08:30:00Z")
}

func TestRightPane_RendersStaleBadgeAndLatestError(t *testing.T) {
	m := seededModelWithRepos()
	m.State.Snapshot.Workspaces[0].Repos[0].ReleaseWorkflowRef = ".github/workflows/release.yml"
	m.State.Snapshot.RepoStatusSnapshots = map[string]workspace.RepoStatusSnapshot{
		repoStatusSnapshotKey("ws-1", "repo-1"): {
			PR:          workspace.StatusSuccess,
			CI:          workspace.StatusSuccess,
			Release:     workspace.StatusFailure,
			IsStale:     true,
			LatestError: "gh api rate limited",
		},
	}

	out := m.View()

	assertContains(t, out, "[stale]")
	assertContains(t, out, "error: gh api rate limited")
}

func TestRightPane_RendersOnlySelectedRepoStatus(t *testing.T) {
	m := seededModelWithRepos()
	m.State.Snapshot.Workspaces[0].Repos[0].ReleaseWorkflowRef = ".github/workflows/release.yml"
	m.State.Snapshot.Workspaces[0].Repos[1].ReleaseWorkflowRef = ".github/workflows/release.yml"
	m.State.Snapshot.RepoStatusSnapshots = map[string]workspace.RepoStatusSnapshot{
		repoStatusSnapshotKey("ws-1", "repo-1"): {
			PR:      workspace.StatusFailure,
			CI:      workspace.StatusFailure,
			Release: workspace.StatusFailure,
		},
		repoStatusSnapshotKey("ws-1", "repo-2"): {
			PR:      workspace.StatusSuccess,
			CI:      workspace.StatusSuccess,
			Release: workspace.StatusSuccess,
		},
	}
	if !m.State.SelectRepo("repo-2") {
		t.Fatal("expected repo selection to succeed")
	}

	out := m.View()

	assertContains(t, out, "repo: web")
	assertContains(t, out, "pr: success")
	assertContains(t, out, "ci: success")
	assertContains(t, out, "release: success")
	if strings.Contains(out, "repo: api") {
		t.Fatalf("expected right pane to scope rendering to selected repo, got:\n%s", out)
	}
	if strings.Contains(out, "pr: failure") {
		t.Fatalf("expected selected repo status only, got:\n%s", out)
	}
}
