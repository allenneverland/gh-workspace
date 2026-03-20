package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
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

	_, followupCmd := m.Update(msg)
	if len(engine.selectionCalls) != 1 {
		t.Fatalf("expected one startup selection refresh, got %d", len(engine.selectionCalls))
	}
	call := engine.selectionCalls[0]
	if call.workspaceID != "ws-1" || call.repoID != "repo-1" {
		t.Fatalf("expected startup selection ws-1/repo-1, got %s/%s", call.workspaceID, call.repoID)
	}
	if followupCmd == nil {
		t.Fatal("expected follow-up polling command after startup sync")
	}
	if engine.startCalls != 1 {
		t.Fatalf("expected polling start call count 1, got %d", engine.startCalls)
	}
}

func TestSync_SelectRepo_TriggersImmediateRefreshForNewSelection(t *testing.T) {
	m := seededModelWithRepos()
	engine := newFakeSyncEngine()
	m.SyncEngine = engine

	updated, _ := m.Update(MsgSelectRepo{RepoID: "repo-2"})
	got := updated.(Model)

	if got.State.CurrentRepoID() != "repo-2" {
		t.Fatalf("expected selected repo %q, got %q", "repo-2", got.State.CurrentRepoID())
	}
	if len(engine.selectionCalls) != 1 {
		t.Fatalf("expected one selection-change refresh, got %d", len(engine.selectionCalls))
	}
	call := engine.selectionCalls[0]
	if call.workspaceID != "ws-1" || call.repoID != "repo-2" {
		t.Fatalf("expected selection refresh ws-1/repo-2, got %s/%s", call.workspaceID, call.repoID)
	}
}

func TestSync_KeyManualRefresh_RefreshesSelectedRepo(t *testing.T) {
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

	afterMsg, _ := updated.(Model).Update(msg)
	_ = afterMsg.(Model)
	if engine.refreshNowCalls != 1 {
		t.Fatalf("expected refresh call count 1, got %d", engine.refreshNowCalls)
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

type fakeSyncEngine struct {
	selectionCalls  []selectionCall
	refreshNowCalls int
	onTickCalls     int
	startCalls      int
	autoPolling     bool
}

type selectionCall struct {
	workspaceID string
	repoID      string
}

func newFakeSyncEngine() *fakeSyncEngine {
	return &fakeSyncEngine{autoPolling: true}
}

func (f *fakeSyncEngine) RefreshNow(context.Context) (workspace.RepoStatus, error) {
	f.refreshNowCalls++
	return workspace.RepoStatus{}, nil
}

func (f *fakeSyncEngine) OnTick(context.Context) (workspace.RepoStatus, error) {
	f.onTickCalls++
	return workspace.RepoStatus{}, nil
}

func (f *fakeSyncEngine) OnSelectionChanged(_ context.Context, workspaceID, repoID string) (workspace.RepoStatus, error) {
	f.selectionCalls = append(f.selectionCalls, selectionCall{workspaceID: workspaceID, repoID: repoID})
	return workspace.RepoStatus{}, nil
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
