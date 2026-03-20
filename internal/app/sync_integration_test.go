package app

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	syncengine "github.com/allenneverland/gh-workspace/internal/sync"
)

func TestSync_Init_StartupRestore_TriggersImmediateRefreshForSelectedRepo(t *testing.T) {
	m := seededModelWithRepos()
	engine := newFakeSyncEngine()
	m.SyncEngine = engine

	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("expected init command when sync engine is configured")
	}

	msg := initCmd()
	if _, ok := msg.(MsgSyncStartup); !ok {
		t.Fatalf("expected init message %T, got %T", MsgSyncStartup{}, msg)
	}

	updated, followupCmd := m.Update(msg)
	got := updated.(Model)
	if len(engine.setSelectionCalls) != 1 {
		t.Fatalf("expected one startup selection set, got %d", len(engine.setSelectionCalls))
	}
	call := engine.setSelectionCalls[0]
	if call.workspaceID != "ws-1" || call.repoID != "repo-1" {
		t.Fatalf("expected startup selection ws-1/repo-1, got %s/%s", call.workspaceID, call.repoID)
	}
	if engine.refreshNowCalls != 0 {
		t.Fatalf("expected startup refresh to be async, got %d inline calls", engine.refreshNowCalls)
	}
	if followupCmd == nil {
		t.Fatal("expected follow-up command after startup sync")
	}
	if engine.startCalls != 1 {
		t.Fatalf("expected polling start call count 1, got %d", engine.startCalls)
	}

	_ = drainSyncCommands(t, got, followupCmd)
	if len(engine.refreshFetchCalls) != 1 {
		t.Fatalf("expected one startup refresh fetch, got %d", len(engine.refreshFetchCalls))
	}
	if engine.refreshFetchCalls[0].repoID != "repo-1" {
		t.Fatalf("expected startup refresh repo %q, got %q", "repo-1", engine.refreshFetchCalls[0].repoID)
	}
}

func TestSync_SelectRepo_TriggersImmediateRefreshForNewSelection(t *testing.T) {
	m := seededModelWithRepos()
	engine := newFakeSyncEngine()
	m.SyncEngine = engine

	updated, cmd := m.Update(MsgSelectRepo{RepoID: "repo-2"})
	got := updated.(Model)

	if got.State.CurrentRepoID() != "repo-2" {
		t.Fatalf("expected selected repo %q, got %q", "repo-2", got.State.CurrentRepoID())
	}
	if len(engine.setSelectionCalls) != 1 {
		t.Fatalf("expected one selection update, got %d", len(engine.setSelectionCalls))
	}
	setCall := engine.setSelectionCalls[0]
	if setCall.workspaceID != "ws-1" || setCall.repoID != "repo-2" {
		t.Fatalf("expected selection update ws-1/repo-2, got %s/%s", setCall.workspaceID, setCall.repoID)
	}
	if engine.refreshNowCalls != 0 {
		t.Fatalf("expected selection refresh to be async, got %d inline calls", engine.refreshNowCalls)
	}
	if cmd == nil {
		t.Fatal("expected immediate refresh command after repo selection")
	}

	_ = drainSyncCommands(t, got, cmd)
	if len(engine.refreshFetchCalls) != 1 {
		t.Fatalf("expected one selection refresh fetch, got %d", len(engine.refreshFetchCalls))
	}
	if engine.refreshFetchCalls[0].repoID != "repo-2" {
		t.Fatalf("expected selection refresh repo %q, got %q", "repo-2", engine.refreshFetchCalls[0].repoID)
	}
}

func TestSync_KeyManualRefresh_RefreshesSelectedRepoAsync(t *testing.T) {
	m := seededModelWithRepos()
	engine := newFakeSyncEngine()
	m.SyncEngine = engine

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected manual refresh key to emit a command")
	}

	msg := cmd()
	if _, ok := msg.(MsgRefreshSelectedRepo); !ok {
		t.Fatalf("expected command message %T, got %T", MsgRefreshSelectedRepo{}, msg)
	}

	afterMsg, refreshCmd := updated.(Model).Update(msg)
	afterModel := afterMsg.(Model)
	if len(engine.setSelectionCalls) != 1 {
		t.Fatalf("expected manual refresh to set current selection, got %d calls", len(engine.setSelectionCalls))
	}
	if engine.refreshNowCalls != 0 {
		t.Fatalf("expected manual refresh to run async, got %d inline calls", engine.refreshNowCalls)
	}
	if refreshCmd == nil {
		t.Fatal("expected async refresh command after manual refresh message")
	}

	_ = drainSyncCommands(t, afterModel, refreshCmd)
	if len(engine.refreshFetchCalls) != 1 {
		t.Fatalf("expected one manual refresh fetch, got %d", len(engine.refreshFetchCalls))
	}
	if engine.refreshFetchCalls[0].repoID != "repo-1" {
		t.Fatalf("expected manual refresh repo %q, got %q", "repo-1", engine.refreshFetchCalls[0].repoID)
	}
}

func TestSync_KeyToggleAutoPolling_TogglesEngineState(t *testing.T) {
	m := seededModelWithRepos()
	engine := newFakeSyncEngine()
	engine.autoPolling = true
	m.SyncEngine = engine

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatal("expected toggle polling key to emit a command")
	}
	msg := cmd()
	if _, ok := msg.(MsgToggleAutoPolling); !ok {
		t.Fatalf("expected command message %T, got %T", MsgToggleAutoPolling{}, msg)
	}

	afterFirst, firstFollowup := updated.(Model).Update(msg)
	first := afterFirst.(Model)
	if engine.autoPolling {
		t.Fatal("expected auto polling disabled after first toggle")
	}
	if firstFollowup != nil {
		t.Fatal("expected no ticker command when disabling auto polling")
	}

	updatedSecond, cmdSecond := first.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmdSecond == nil {
		t.Fatal("expected second toggle to emit command")
	}
	msgSecond := cmdSecond()
	afterSecond, secondFollowup := updatedSecond.(Model).Update(msgSecond)
	_ = afterSecond.(Model)
	if !engine.autoPolling {
		t.Fatal("expected auto polling enabled after second toggle")
	}
	if secondFollowup == nil {
		t.Fatal("expected ticker command when enabling auto polling")
	}
	if engine.startCalls != 1 {
		t.Fatalf("expected polling start call count 1, got %d", engine.startCalls)
	}
}

func TestSync_RemoveRepo_UpdatesSelectionAndNeverRefreshesDeletedRepo(t *testing.T) {
	m := seededModelWithRepos()
	engine := newFakeSyncEngine()
	m.SyncEngine = engine

	updated, removeCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := updated.(Model)
	if got.State.CurrentRepoID() != "repo-2" {
		t.Fatalf("expected selected repo %q after removal, got %q", "repo-2", got.State.CurrentRepoID())
	}
	if len(engine.setSelectionCalls) == 0 {
		t.Fatal("expected sync selection update after repo removal")
	}
	setCall := engine.setSelectionCalls[len(engine.setSelectionCalls)-1]
	if setCall.workspaceID != "ws-1" || setCall.repoID != "repo-2" {
		t.Fatalf("expected selection updated to ws-1/repo-2, got %s/%s", setCall.workspaceID, setCall.repoID)
	}
	if removeCmd == nil {
		t.Fatal("expected refresh command after removal selection change")
	}
	got = drainSyncCommands(t, got, removeCmd)
	if len(engine.refreshFetchCalls) != 1 {
		t.Fatalf("expected one refresh fetch after removal, got %d", len(engine.refreshFetchCalls))
	}
	if engine.refreshFetchCalls[0].repoID != "repo-2" {
		t.Fatalf("expected post-removal refresh repo %q, got %q", "repo-2", engine.refreshFetchCalls[0].repoID)
	}
	engine.clearRefreshHistory()

	afterKey, keyCmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if keyCmd == nil {
		t.Fatal("expected manual refresh command after removal")
	}
	msg := keyCmd()
	afterMsg, manualRefreshCmd := afterKey.(Model).Update(msg)
	if manualRefreshCmd == nil {
		t.Fatal("expected async refresh command from manual refresh message")
	}
	_ = drainSyncCommands(t, afterMsg.(Model), manualRefreshCmd)

	if len(engine.refreshFetchCalls) != 1 {
		t.Fatalf("expected one manual refresh fetch after removal, got %d", len(engine.refreshFetchCalls))
	}
	if engine.refreshFetchCalls[0].repoID != "repo-2" {
		t.Fatalf("expected manual refresh repo %q after removal, got %q", "repo-2", engine.refreshFetchCalls[0].repoID)
	}
	for _, refresh := range engine.refreshFetchCalls {
		if refresh.repoID == "repo-1" {
			t.Fatalf("refresh targeted deleted repo %q", refresh.repoID)
		}
	}
}

func TestSync_Tick_NoSelection_DoesNotFetch(t *testing.T) {
	m := NewModel(Config{
		InitialState: workspace.State{
			SelectedWorkspaceID: "ws-1",
			Workspaces: []workspace.Workspace{{
				ID:   "ws-1",
				Name: "alpha",
			}},
		},
	})
	engine := newFakeSyncEngine()
	m.SyncEngine = engine

	updated, cmd := m.Update(syncengine.MsgTick{})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected tick handling command")
	}
	if engine.onTickCalls != 0 {
		t.Fatalf("expected tick refresh to be async, got %d inline calls", engine.onTickCalls)
	}
	if engine.startCalls != 1 {
		t.Fatalf("expected tick to schedule next poll, got %d start calls", engine.startCalls)
	}

	_ = drainSyncCommands(t, got, cmd)
	if engine.onTickCalls != 1 {
		t.Fatalf("expected one async tick call, got %d", engine.onTickCalls)
	}
	if len(engine.tickFetchCalls) != 0 {
		t.Fatalf("expected no fetch on tick with empty selection, got %d", len(engine.tickFetchCalls))
	}
}

func drainSyncCommands(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	current := m
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}

		msg := next()
		if msg == nil {
			continue
		}

		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, []tea.Cmd(batch)...)
			continue
		}

		updated, followup := current.Update(msg)
		current = updated.(Model)
		if followup != nil {
			queue = append(queue, followup)
		}
	}
	return current
}

type fakeSyncEngine struct {
	setSelectionCalls   []selectionCall
	onSelectionCalls    []selectionCall
	refreshFetchCalls   []selectionCall
	tickFetchCalls      []selectionCall
	refreshNowCalls     int
	onTickCalls         int
	startCalls          int
	autoPolling         bool
	selectedWorkspaceID string
	selectedRepoID      string
}

type selectionCall struct {
	workspaceID string
	repoID      string
}

func newFakeSyncEngine() *fakeSyncEngine {
	return &fakeSyncEngine{autoPolling: true}
}

func (f *fakeSyncEngine) SetSelection(workspaceID, repoID string) {
	f.selectedWorkspaceID = strings.TrimSpace(workspaceID)
	f.selectedRepoID = strings.TrimSpace(repoID)
	f.setSelectionCalls = append(f.setSelectionCalls, selectionCall{workspaceID: f.selectedWorkspaceID, repoID: f.selectedRepoID})
}

func (f *fakeSyncEngine) RefreshNow(context.Context) (workspace.RepoStatus, error) {
	f.refreshNowCalls++
	if f.selectedWorkspaceID != "" && f.selectedRepoID != "" {
		f.refreshFetchCalls = append(f.refreshFetchCalls, selectionCall{workspaceID: f.selectedWorkspaceID, repoID: f.selectedRepoID})
	}
	return workspace.RepoStatus{}, nil
}

func (f *fakeSyncEngine) OnTick(context.Context) (workspace.RepoStatus, error) {
	f.onTickCalls++
	if !f.autoPolling {
		return workspace.RepoStatus{}, nil
	}
	if f.selectedWorkspaceID != "" && f.selectedRepoID != "" {
		f.tickFetchCalls = append(f.tickFetchCalls, selectionCall{workspaceID: f.selectedWorkspaceID, repoID: f.selectedRepoID})
	}
	return workspace.RepoStatus{}, nil
}

func (f *fakeSyncEngine) OnSelectionChanged(_ context.Context, workspaceID, repoID string) (workspace.RepoStatus, error) {
	f.onSelectionCalls = append(f.onSelectionCalls, selectionCall{workspaceID: workspaceID, repoID: repoID})
	f.SetSelection(workspaceID, repoID)
	return f.RefreshNow(context.Background())
}

func (f *fakeSyncEngine) Start(context.Context) tea.Cmd {
	if !f.autoPolling {
		return nil
	}
	f.startCalls++
	return func() tea.Msg { return nil }
}

func (f *fakeSyncEngine) SetAutoPolling(enabled bool) {
	f.autoPolling = enabled
}

func (f *fakeSyncEngine) AutoPollingEnabled() bool {
	return f.autoPolling
}

func (f *fakeSyncEngine) clearRefreshHistory() {
	f.refreshFetchCalls = nil
	f.tickFetchCalls = nil
	f.refreshNowCalls = 0
	f.onTickCalls = 0
}
