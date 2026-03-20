package smoke

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/app"
	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	syncengine "github.com/allenneverland/gh-workspace/internal/sync"
)

func TestSmoke_BootstrapAndPreflight(t *testing.T) {
	t.Setenv("WORKSPACE_TUI_TEST_MODE", "1")
	if err := RunSmoke(); err != nil {
		t.Fatalf("RunSmoke() error = %v", err)
	}
}

func TestSmoke_SyncEngine_RepoSwitchTriggersImmediateRefresh(t *testing.T) {
	fetcher := &fakeStatusFetcher{}
	engine := syncengine.NewEngine(fetcher, syncengine.WithInterval(time.Minute))

	if _, err := engine.OnSelectionChanged(context.Background(), "ws-1", "repo-1"); err != nil {
		t.Fatalf("OnSelectionChanged(initial) error = %v", err)
	}
	fetcher.reset()

	if _, err := engine.OnSelectionChanged(context.Background(), "ws-1", "repo-2"); err != nil {
		t.Fatalf("OnSelectionChanged(switch) error = %v", err)
	}

	calls := fetcher.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("expected one immediate refresh call after repo switch, got %d", len(calls))
	}
	if calls[0].workspaceID != "ws-1" || calls[0].repoID != "repo-2" {
		t.Fatalf("expected immediate refresh for ws-1/repo-2, got %s/%s", calls[0].workspaceID, calls[0].repoID)
	}
}

func RunSmoke() error {
	if strings.TrimSpace(os.Getenv("WORKSPACE_TUI_TEST_MODE")) != "1" {
		return errors.New("WORKSPACE_TUI_TEST_MODE must be set to 1 for deterministic smoke runs")
	}

	m := app.NewModel(app.Config{InitialState: seededSmokeState()})
	engine := newFakeSyncEngine()
	m.SyncEngine = engine

	initCmd := m.Init()
	if initCmd == nil {
		return errors.New("expected init command with configured sync engine")
	}
	initMsg := initCmd()
	if _, ok := initMsg.(app.MsgSyncStartup); !ok {
		return fmt.Errorf("expected init message %T, got %T", app.MsgSyncStartup{}, initMsg)
	}

	updated, startupCmd := m.Update(initMsg)
	if startupCmd == nil {
		return errors.New("expected startup sync command")
	}
	current := updated.(app.Model)

	if err := assertLastSelection(engine, "startup selection", "ws-1", "repo-1"); err != nil {
		return err
	}
	current = drainCommands(current, startupCmd)
	if err := assertLastRefresh(engine, "startup refresh", "ws-1", "repo-1"); err != nil {
		return err
	}
	if engine.startCalls != 1 {
		return fmt.Errorf("expected one polling start call after startup, got %d", engine.startCalls)
	}

	updated, switchCmd := current.Update(app.MsgSelectRepo{RepoID: "repo-2"})
	if switchCmd == nil {
		return errors.New("expected immediate refresh command on repo switch")
	}
	current = updated.(app.Model)
	if current.State.CurrentRepoID() != "repo-2" {
		return fmt.Errorf("expected selected repo %q after switch, got %q", "repo-2", current.State.CurrentRepoID())
	}
	if err := assertLastSelection(engine, "repo-switch selection", "ws-1", "repo-2"); err != nil {
		return err
	}
	current = drainCommands(current, switchCmd)
	if err := assertLastRefresh(engine, "repo-switch refresh", "ws-1", "repo-2"); err != nil {
		return err
	}

	updated, keyCmd := current.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if keyCmd == nil {
		return errors.New("expected refresh key to emit command")
	}
	refreshMsg := keyCmd()
	if _, ok := refreshMsg.(app.MsgRefreshSelectedRepo); !ok {
		return fmt.Errorf("expected refresh key message %T, got %T", app.MsgRefreshSelectedRepo{}, refreshMsg)
	}
	updated, refreshCmd := updated.(app.Model).Update(refreshMsg)
	if refreshCmd == nil {
		return errors.New("expected refresh command after MsgRefreshSelectedRepo")
	}
	current = drainCommands(updated.(app.Model), refreshCmd)
	if err := assertLastRefresh(engine, "manual-refresh", "ws-1", "repo-2"); err != nil {
		return err
	}

	updated, toggleCmd := current.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if toggleCmd == nil {
		return errors.New("expected polling toggle key to emit command")
	}
	toggleMsg := toggleCmd()
	if _, ok := toggleMsg.(app.MsgToggleAutoPolling); !ok {
		return fmt.Errorf("expected polling toggle message %T, got %T", app.MsgToggleAutoPolling{}, toggleMsg)
	}
	updated, disableFollowup := updated.(app.Model).Update(toggleMsg)
	current = updated.(app.Model)
	if engine.AutoPollingEnabled() {
		return errors.New("expected polling disabled after first toggle")
	}
	if disableFollowup != nil {
		return errors.New("expected no follow-up command when disabling polling")
	}

	updated, toggleCmd = current.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if toggleCmd == nil {
		return errors.New("expected second polling toggle key to emit command")
	}
	toggleMsg = toggleCmd()
	updated, enableFollowup := updated.(app.Model).Update(toggleMsg)
	current = updated.(app.Model)
	if !engine.AutoPollingEnabled() {
		return errors.New("expected polling enabled after second toggle")
	}
	if enableFollowup == nil {
		return errors.New("expected follow-up command when enabling polling")
	}
	_ = drainCommands(current, enableFollowup)
	if engine.startCalls != 2 {
		return fmt.Errorf("expected polling start call count 2 (startup + enable), got %d", engine.startCalls)
	}

	return nil
}

func seededSmokeState() workspace.State {
	return workspace.State{
		SelectedWorkspaceID: "ws-1",
		Workspaces: []workspace.Workspace{
			{
				ID:             "ws-1",
				Name:           "default",
				SelectedRepoID: "repo-1",
				Repos: []workspace.Repo{
					{ID: "repo-1", Name: "acme/api", Path: "/tmp/api", ReleaseWorkflowRef: ".github/workflows/release.yml", Health: workspace.RepoHealthy},
					{ID: "repo-2", Name: "acme/web", Path: "/tmp/web", ReleaseWorkflowRef: ".github/workflows/release.yml", Health: workspace.RepoHealthy},
				},
			},
		},
	}
}

func drainCommands(m app.Model, cmd tea.Cmd) app.Model {
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
		current = updated.(app.Model)
		if followup != nil {
			queue = append(queue, followup)
		}
	}
	return current
}

func assertLastSelection(engine *fakeSyncEngine, context string, workspaceID, repoID string) error {
	if len(engine.setSelectionCalls) == 0 {
		return fmt.Errorf("%s: expected at least one SetSelection call", context)
	}
	last := engine.setSelectionCalls[len(engine.setSelectionCalls)-1]
	if last.workspaceID != workspaceID || last.repoID != repoID {
		return fmt.Errorf("%s: expected SetSelection %s/%s, got %s/%s", context, workspaceID, repoID, last.workspaceID, last.repoID)
	}
	return nil
}

func assertLastRefresh(engine *fakeSyncEngine, context string, workspaceID, repoID string) error {
	if len(engine.refreshCalls) == 0 {
		return fmt.Errorf("%s: expected at least one RefreshNow call", context)
	}
	last := engine.refreshCalls[len(engine.refreshCalls)-1]
	if last.workspaceID != workspaceID || last.repoID != repoID {
		return fmt.Errorf("%s: expected RefreshNow %s/%s, got %s/%s", context, workspaceID, repoID, last.workspaceID, last.repoID)
	}
	return nil
}

type fakeSyncEngine struct {
	autoPolling       bool
	startCalls        int
	selected          selectionCall
	setSelectionCalls []selectionCall
	refreshCalls      []selectionCall
}

func newFakeSyncEngine() *fakeSyncEngine {
	return &fakeSyncEngine{autoPolling: true}
}

func (f *fakeSyncEngine) SetSelection(workspaceID, repoID string) {
	f.selected = selectionCall{
		workspaceID: strings.TrimSpace(workspaceID),
		repoID:      strings.TrimSpace(repoID),
	}
	f.setSelectionCalls = append(f.setSelectionCalls, f.selected)
}

func (f *fakeSyncEngine) RefreshNow(context.Context) (workspace.RepoStatus, error) {
	if f.selected.workspaceID != "" && f.selected.repoID != "" {
		f.refreshCalls = append(f.refreshCalls, f.selected)
	}
	return workspace.RepoStatus{
		PR:      workspace.StatusNeutral,
		CI:      workspace.StatusNeutral,
		Release: workspace.StatusNeutral,
	}, nil
}

func (f *fakeSyncEngine) OnTick(context.Context) (workspace.RepoStatus, error) {
	return workspace.RepoStatus{}, nil
}

func (f *fakeSyncEngine) OnSelectionChanged(context.Context, string, string) (workspace.RepoStatus, error) {
	return workspace.RepoStatus{}, nil
}

func (f *fakeSyncEngine) Start(context.Context) tea.Cmd {
	f.startCalls++
	return func() tea.Msg {
		return nil
	}
}

func (f *fakeSyncEngine) SetAutoPolling(enabled bool) {
	f.autoPolling = enabled
}

func (f *fakeSyncEngine) AutoPollingEnabled() bool {
	return f.autoPolling
}

type fakeStatusFetcher struct {
	calls []selectionCall
}

func (f *fakeStatusFetcher) FetchSelectedRepoStatus(_ context.Context, workspaceID, repoID string) (workspace.RepoStatus, error) {
	f.calls = append(f.calls, selectionCall{
		workspaceID: workspaceID,
		repoID:      repoID,
	})
	return workspace.RepoStatus{
		PR:      workspace.StatusSuccess,
		CI:      workspace.StatusSuccess,
		Release: workspace.StatusSuccess,
	}, nil
}

func (f *fakeStatusFetcher) reset() {
	f.calls = nil
}

func (f *fakeStatusFetcher) snapshotCalls() []selectionCall {
	copied := make([]selectionCall, len(f.calls))
	copy(copied, f.calls)
	return copied
}

type selectionCall struct {
	workspaceID string
	repoID      string
}
