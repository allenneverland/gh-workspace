package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	diffadapter "github.com/allenneverland/gh-workspace/internal/adapters/diff"
	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestUpdate_MsgSetActiveTab_Diff_FetchesAsyncAndViewUsesCachedOutput(t *testing.T) {
	m := seededModelWithRepos()
	renderer := &fakeDiffRenderer{
		outputs: []string{"@@ -1 +1 @@\n+hello\n"},
	}
	m.DiffRenderer = renderer

	updated, cmd := m.Update(MsgSetActiveTab{Tab: TabDiff})
	got := updated.(Model)

	if got.ActiveTab != TabDiff {
		t.Fatalf("expected active tab %q, got %q", TabDiff, got.ActiveTab)
	}
	if !got.DiffLoading {
		t.Fatal("expected diff loading state after entering diff tab")
	}
	if cmd == nil {
		t.Fatal("expected async diff command when entering diff tab")
	}
	if len(renderer.calls) != 0 {
		t.Fatalf("expected renderer not called until command executes, got %d calls", len(renderer.calls))
	}

	loadingView := got.View()
	assertContains(t, loadingView, "Loading diff...")
	if len(renderer.calls) != 0 {
		t.Fatalf("expected view to not execute renderer, got %d calls", len(renderer.calls))
	}

	msg := cmd()
	if len(renderer.calls) != 1 {
		t.Fatalf("expected renderer call after command execution, got %d calls", len(renderer.calls))
	}

	afterDiff, _ := got.Update(msg)
	final := afterDiff.(Model)
	if final.DiffLoading {
		t.Fatal("expected diff loading to be false after result message")
	}
	if final.DiffOutput != "@@ -1 +1 @@\n+hello\n" {
		t.Fatalf("expected cached diff output, got %q", final.DiffOutput)
	}

	view := final.View()
	assertContains(t, view, "@@ -1 +1 @@")
	assertContains(t, view, "+hello")
	if len(renderer.calls) != 1 {
		t.Fatalf("expected cached view rendering to avoid re-running renderer, got %d calls", len(renderer.calls))
	}
}

func TestUpdate_DiffTab_DeltaMissing_ShowsInstallHint(t *testing.T) {
	m := seededModelWithRepos()
	renderer := &fakeDiffRenderer{
		errs: []error{diffadapter.ErrDeltaNotFound},
	}
	m.DiffRenderer = renderer

	entered, cmd := m.Update(MsgSetActiveTab{Tab: TabDiff})
	if cmd == nil {
		t.Fatal("expected diff fetch command")
	}
	msg := cmd()
	updated, _ := entered.(Model).Update(msg)
	got := updated.(Model)

	assertContains(t, got.View(), "delta not found; install delta to use Diff tab")
}

func TestUpdate_DiffTab_RefreshMessage_TriggersRefetch(t *testing.T) {
	m := seededModelWithRepos()
	renderer := &fakeDiffRenderer{
		outputs: []string{
			"@@ first @@",
			"@@ second @@",
		},
	}
	m.DiffRenderer = renderer

	entered, cmd := m.Update(MsgSetActiveTab{Tab: TabDiff})
	if cmd == nil {
		t.Fatal("expected first diff fetch command")
	}
	firstResult := cmd()
	afterFirst, _ := entered.(Model).Update(firstResult)
	first := afterFirst.(Model)
	if first.DiffOutput != "@@ first @@" {
		t.Fatalf("expected first diff output, got %q", first.DiffOutput)
	}

	refreshed, refreshCmd := first.Update(MsgRefreshDiff{})
	refreshing := refreshed.(Model)
	if refreshCmd == nil {
		t.Fatal("expected refresh command in diff tab")
	}
	if !refreshing.DiffLoading {
		t.Fatal("expected loading state while refresh is in flight")
	}
	secondResult := refreshCmd()
	afterSecond, _ := refreshing.Update(secondResult)
	final := afterSecond.(Model)

	if final.DiffOutput != "@@ second @@" {
		t.Fatalf("expected refreshed diff output, got %q", final.DiffOutput)
	}
	if len(renderer.calls) != 2 {
		t.Fatalf("expected two renderer calls after refresh, got %d", len(renderer.calls))
	}
}

func TestUpdate_DiffTab_MutatingKeysAndActions_DoNothing(t *testing.T) {
	m := NewModel(Config{
		InitialState: workspace.State{
			SelectedWorkspaceID: "ws-1",
			Workspaces: []workspace.Workspace{
				{
					ID:             "ws-1",
					Name:           "alpha",
					SelectedRepoID: "repo-1",
					Repos: []workspace.Repo{
						{ID: "repo-1", Name: "api", Path: "/tmp/api", Health: workspace.RepoInvalid},
					},
				},
				{
					ID:             "ws-2",
					Name:           "beta",
					SelectedRepoID: "repo-2",
					Repos: []workspace.Repo{
						{ID: "repo-2", Name: "web", Path: "/tmp/web", Health: workspace.RepoHealthy},
					},
				},
			},
		},
	})
	m.ActiveTab = TabDiff
	m.WorktreeAdapter = &fakeWorktreeAdapter{
		listItems: []WorktreeItem{
			{ID: "wt-main", Path: "/tmp/api"},
		},
	}

	initialState := cloneWorkspaceState(m.State.Snapshot)
	initialStatus := m.StatusMessage
	initialAddRepoRequested := m.AddRepoRequested

	messages := []tea.Msg{
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}},
		tea.KeyMsg{Type: tea.KeyEnter},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}},
		MsgRequestAddRepo{},
		MsgCreateWorktree{Branch: "feature/a", Path: "../api-feature-a"},
		MsgSwitchWorktree{WorktreePath: "../api-feature-a"},
	}

	current := m
	for _, msg := range messages {
		updated, cmd := current.Update(msg)
		if cmd != nil {
			t.Fatalf("expected no command for mutating message %T while diff tab is active", msg)
		}
		current = updated.(Model)
	}

	if !reflect.DeepEqual(current.State.Snapshot, initialState) {
		t.Fatalf("expected workspace snapshot unchanged in diff tab\nwant=%#v\ngot=%#v", initialState, current.State.Snapshot)
	}
	if current.StatusMessage != initialStatus {
		t.Fatalf("expected status message unchanged, got %q", current.StatusMessage)
	}
	if current.AddRepoRequested != initialAddRepoRequested {
		t.Fatalf("expected AddRepoRequested unchanged, got %v", current.AddRepoRequested)
	}
}

type fakeDiffRenderer struct {
	calls   []string
	outputs []string
	errs    []error
}

func (f *fakeDiffRenderer) Render(_ context.Context, repoPath string) (string, error) {
	f.calls = append(f.calls, repoPath)
	index := len(f.calls) - 1

	var output string
	if index < len(f.outputs) {
		output = f.outputs[index]
	}

	var err error
	if index < len(f.errs) {
		err = f.errs[index]
	}
	return output, err
}

func TestUpdate_DiffTab_RenderFailure_ShowsError(t *testing.T) {
	m := seededModelWithRepos()
	renderer := &fakeDiffRenderer{
		errs: []error{errors.New("boom")},
	}
	m.DiffRenderer = renderer

	entered, cmd := m.Update(MsgSetActiveTab{Tab: TabDiff})
	if cmd == nil {
		t.Fatal("expected diff fetch command")
	}
	result := cmd()
	updated, _ := entered.(Model).Update(result)
	got := updated.(Model)

	assertContains(t, got.View(), "failed to render diff: boom")
}
