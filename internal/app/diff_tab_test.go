package app

import (
	"context"
	"errors"
	"testing"

	diffadapter "github.com/allenneverland/gh-workspace/internal/adapters/diff"
)

func TestView_DiffTab_RendersDiffFromSelectedRepo(t *testing.T) {
	m := seededModelWithRepos()
	m.ActiveTab = Tab("diff")

	var calledRepoPath string
	restore := renderRepoDiff
	renderRepoDiff = func(_ context.Context, repoPath string) (string, error) {
		calledRepoPath = repoPath
		return "@@ -1 +1 @@\n+hello\n", nil
	}
	defer func() {
		renderRepoDiff = restore
	}()

	got := m.View()
	assertContains(t, got, "@@ -1 +1 @@")
	assertContains(t, got, "+hello")
	if calledRepoPath != "/tmp/api" {
		t.Fatalf("expected diff renderer to receive repo path %q, got %q", "/tmp/api", calledRepoPath)
	}
}

func TestView_DiffTab_DeltaMissing_ShowsInstallHint(t *testing.T) {
	m := seededModelWithRepos()
	m.ActiveTab = Tab("diff")

	restore := renderRepoDiff
	renderRepoDiff = func(context.Context, string) (string, error) {
		return "", diffadapter.ErrDeltaNotFound
	}
	defer func() {
		renderRepoDiff = restore
	}()

	got := m.View()
	assertContains(t, got, "delta not found; install delta to use Diff tab")
}

func TestView_DiffTab_RenderFailure_ShowsError(t *testing.T) {
	m := seededModelWithRepos()
	m.ActiveTab = Tab("diff")

	restore := renderRepoDiff
	renderRepoDiff = func(context.Context, string) (string, error) {
		return "", errors.New("boom")
	}
	defer func() {
		renderRepoDiff = restore
	}()

	got := m.View()
	assertContains(t, got, "failed to render diff: boom")
}
