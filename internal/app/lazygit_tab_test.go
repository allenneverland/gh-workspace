package app

import (
	"errors"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestView_LazygitTab_NoSelectedRepo_ShowsHint(t *testing.T) {
	m := NewModel(Config{
		InitialState: workspace.State{
			SelectedWorkspaceID: "ws-1",
			Workspaces: []workspace.Workspace{
				{
					ID:   "ws-1",
					Name: "alpha",
				},
			},
		},
	})
	m.ActiveTab = TabLazygit

	got := m.View()
	assertContains(t, got, "請先選擇 repo")
}

func TestUpdate_MsgSetActiveTab_Lazygit_StartsSessionForSelectedRepo(t *testing.T) {
	m := seededModelWithRepos()
	manager := newFakeLazygitSessionManager()
	manager.sessionsByRepo["/tmp/api"] = LazygitSessionHandle{
		ID:       "session-api",
		RepoPath: "/tmp/api",
	}
	m.LazygitSessionManager = manager

	updated, cmd := m.Update(MsgSetActiveTab{Tab: TabLazygit})
	got := updated.(Model)

	if got.ActiveTab != TabLazygit {
		t.Fatalf("expected active tab %q, got %q", TabLazygit, got.ActiveTab)
	}
	if len(manager.startCalls) != 1 {
		t.Fatalf("expected StartSession to be called once, got %d", len(manager.startCalls))
	}
	if manager.startCalls[0] != "/tmp/api" {
		t.Fatalf("expected StartSession repo path %q, got %q", "/tmp/api", manager.startCalls[0])
	}
	if got.LazygitSessionID != "session-api" {
		t.Fatalf("expected active lazygit session id %q, got %q", "session-api", got.LazygitSessionID)
	}
	if cmd == nil {
		t.Fatal("expected frame listener command after entering lazygit tab")
	}
}

func TestUpdate_KeyMsg_LazygitTab_ForwardsInputToSession(t *testing.T) {
	m := seededModelWithRepos()
	manager := newFakeLazygitSessionManager()
	manager.sessionsByRepo["/tmp/api"] = LazygitSessionHandle{
		ID:       "session-api",
		RepoPath: "/tmp/api",
	}
	m.LazygitSessionManager = manager

	overviewUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	overviewModel := overviewUpdated.(Model)
	if len(manager.writeCalls) != 0 {
		t.Fatalf("expected no input forwarding while not in lazygit tab, got %d writes", len(manager.writeCalls))
	}

	lazygitUpdated, _ := overviewModel.Update(MsgSetActiveTab{Tab: TabLazygit})
	lazygitModel := lazygitUpdated.(Model)

	afterKeyUpdated, _ := lazygitModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_ = afterKeyUpdated.(Model)
	if len(manager.writeCalls) != 1 {
		t.Fatalf("expected one forwarded write in lazygit tab, got %d", len(manager.writeCalls))
	}
	if manager.writeCalls[0].sessionID != "session-api" {
		t.Fatalf("expected forwarded session id %q, got %q", "session-api", manager.writeCalls[0].sessionID)
	}
	if got := string(manager.writeCalls[0].data); got != "j" {
		t.Fatalf("expected forwarded payload %q, got %q", "j", got)
	}
}

func TestUpdate_KeyMsg_LazygitTab_WithSession_PrioritizesPTYOverAppBindings(t *testing.T) {
	m := seededModelWithRepos()
	manager := newFakeLazygitSessionManager()
	manager.sessionsByRepo["/tmp/api"] = LazygitSessionHandle{
		ID:       "session-api",
		RepoPath: "/tmp/api",
	}
	m.LazygitSessionManager = manager

	enteredTab, _ := m.Update(MsgSetActiveTab{Tab: TabLazygit})
	tabModel := enteredTab.(Model)

	afterKey, cmd := tabModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := afterKey.(Model)

	if got.AddRepoRequested {
		t.Fatal("expected lazygit tab key ownership to prevent add-repo action")
	}
	if len(manager.writeCalls) != 1 {
		t.Fatalf("expected key to forward to PTY, got %d writes", len(manager.writeCalls))
	}
	if payload := string(manager.writeCalls[0].data); payload != "a" {
		t.Fatalf("expected forwarded payload %q, got %q", "a", payload)
	}
	if cmd != nil {
		if _, ok := cmd().(MsgRequestAddRepo); ok {
			t.Fatal("expected no add-repo command while lazygit owns keys")
		}
	}
}

func TestUpdate_LazygitTab_DoesNotAccumulateFrameListeners(t *testing.T) {
	m := seededModelWithRepos()
	manager := newFakeLazygitSessionManager()
	manager.sessionsByRepo["/tmp/api"] = LazygitSessionHandle{
		ID:       "session-api",
		RepoPath: "/tmp/api",
	}
	manager.sessionsByRepo["/tmp/web"] = LazygitSessionHandle{
		ID:       "session-web",
		RepoPath: "/tmp/web",
	}
	m.LazygitSessionManager = manager

	enteredTab, firstCmd := m.Update(MsgSetActiveTab{Tab: TabLazygit})
	first := enteredTab.(Model)
	if firstCmd == nil {
		t.Fatal("expected first frame wait command")
	}
	if got := atomic.LoadInt32(&manager.framesCalls); got != 1 {
		t.Fatalf("expected one frame subscription, got %d", got)
	}

	switchedRepo, secondCmd := first.Update(MsgSelectRepo{RepoID: "repo-2"})
	second := switchedRepo.(Model)
	if secondCmd != nil {
		t.Fatal("expected no extra frame wait command while one is already in flight")
	}
	if got := atomic.LoadInt32(&manager.framesCalls); got != 1 {
		t.Fatalf("expected frame subscription count to remain 1, got %d", got)
	}
	if !second.lazygitFrameListenerInFlight {
		t.Fatal("expected listener in-flight flag to remain set before frame delivery")
	}
}

func TestUpdate_LazygitTab_StartFailure_ClearsSessionState(t *testing.T) {
	m := seededModelWithRepos()
	manager := newFakeLazygitSessionManager()
	manager.startErrByRepo["/tmp/api"] = errors.New("boom")
	m.LazygitSessionManager = manager
	m.ActiveTab = TabLazygit
	m.LazygitSessionID = "stale-session"
	m.LazygitCenterFrameText = "stale-frame"

	updated, _ := m.Update(MsgSetActiveTab{Tab: TabLazygit})
	got := updated.(Model)

	if got.LazygitSessionID != "" {
		t.Fatalf("expected session id cleared on start failure, got %q", got.LazygitSessionID)
	}
	if got.LazygitCenterFrameText != "" {
		t.Fatalf("expected frame text cleared on start failure, got %q", got.LazygitCenterFrameText)
	}

	afterKey, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	final := afterKey.(Model)
	if len(manager.writeCalls) != 0 {
		t.Fatalf("expected no PTY writes after failed start, got %d", len(manager.writeCalls))
	}
	if final.StatusMessage == "" {
		t.Fatal("expected status message after failed lazygit start")
	}
}

func TestUpdate_LazygitTab_InFlightListener_PersistsAcrossStartFailureWithoutDuplicateWaiters(t *testing.T) {
	m := seededModelWithRepos()
	manager := newFakeLazygitSessionManager()
	manager.sessionsByRepo["/tmp/api"] = LazygitSessionHandle{
		ID:       "session-api",
		RepoPath: "/tmp/api",
	}
	manager.startErrByRepo["/tmp/web"] = errors.New("boom")
	m.LazygitSessionManager = manager

	enteredTab, firstCmd := m.Update(MsgSetActiveTab{Tab: TabLazygit})
	first := enteredTab.(Model)
	if firstCmd == nil {
		t.Fatal("expected initial lazygit frame wait command")
	}
	if got := atomic.LoadInt32(&manager.framesCalls); got != 1 {
		t.Fatalf("expected one frame subscription, got %d", got)
	}

	afterFailure, failureCmd := first.Update(MsgSelectRepo{RepoID: "repo-2"})
	failed := afterFailure.(Model)
	if failureCmd != nil {
		t.Fatal("expected no new wait command on start failure with listener already in flight")
	}
	if failed.LazygitSessionID != "" {
		t.Fatalf("expected session id cleared after start failure, got %q", failed.LazygitSessionID)
	}
	if !failed.lazygitFrameListenerInFlight {
		t.Fatal("expected in-flight listener flag to remain true while original waiter is still blocked")
	}
	if got := atomic.LoadInt32(&manager.framesCalls); got != 1 {
		t.Fatalf("expected frame subscription count to remain 1 after failure, got %d", got)
	}

	afterRestart, restartCmd := failed.Update(MsgSelectRepo{RepoID: "repo-1"})
	restarted := afterRestart.(Model)
	if restartCmd != nil {
		t.Fatal("expected no duplicate wait command while previous waiter remains in flight")
	}
	if restarted.LazygitSessionID != "session-api" {
		t.Fatalf("expected session restart for repo-1, got %q", restarted.LazygitSessionID)
	}
	if got := atomic.LoadInt32(&manager.framesCalls); got != 1 {
		t.Fatalf("expected still one frame subscription after restart attempt, got %d", got)
	}
}

func TestUpdate_LazygitFrameMessage_RendersInView(t *testing.T) {
	m := seededModelWithRepos()
	manager := newFakeLazygitSessionManager()
	manager.sessionsByRepo["/tmp/api"] = LazygitSessionHandle{
		ID:       "session-api",
		RepoPath: "/tmp/api",
	}
	manager.frames <- LazygitFrame{
		SessionID: "session-api",
		Data:      []byte("frame-one"),
	}
	m.LazygitSessionManager = manager

	enteredTab, cmd := m.Update(MsgSetActiveTab{Tab: TabLazygit})
	tabModel := enteredTab.(Model)
	if cmd == nil {
		t.Fatal("expected frame wait command after switching to lazygit tab")
	}

	msg := cmd()
	frameMsg, ok := msg.(MsgLazygitFrame)
	if !ok {
		t.Fatalf("expected lazygit frame message, got %T", msg)
	}

	afterFrame, _ := tabModel.Update(frameMsg)
	frameModel := afterFrame.(Model)

	if frameModel.LazygitCenterFrameText != "frame-one" {
		t.Fatalf("expected frame text %q, got %q", "frame-one", frameModel.LazygitCenterFrameText)
	}
	assertContains(t, frameModel.View(), "frame-one")
}

type fakeLazygitSessionManager struct {
	startCalls []string
	writeCalls []lazygitWriteCall

	startErrByRepo map[string]error
	sessionsByRepo map[string]LazygitSessionHandle
	frames         chan LazygitFrame
	framesCalls    int32
}

type lazygitWriteCall struct {
	sessionID string
	data      []byte
}

func newFakeLazygitSessionManager() *fakeLazygitSessionManager {
	return &fakeLazygitSessionManager{
		startErrByRepo: make(map[string]error),
		sessionsByRepo: make(map[string]LazygitSessionHandle),
		frames:         make(chan LazygitFrame, 8),
	}
}

func (f *fakeLazygitSessionManager) StartSession(repoPath string) (LazygitSessionHandle, error) {
	f.startCalls = append(f.startCalls, repoPath)
	if err, ok := f.startErrByRepo[repoPath]; ok {
		return LazygitSessionHandle{}, err
	}
	session, ok := f.sessionsByRepo[repoPath]
	if ok {
		return session, nil
	}
	session = LazygitSessionHandle{
		ID:       "session-default",
		RepoPath: repoPath,
	}
	f.sessionsByRepo[repoPath] = session
	return session, nil
}

func (f *fakeLazygitSessionManager) WriteInput(sessionID string, input []byte) error {
	buf := append([]byte(nil), input...)
	f.writeCalls = append(f.writeCalls, lazygitWriteCall{
		sessionID: sessionID,
		data:      buf,
	})
	return nil
}

func (f *fakeLazygitSessionManager) Frames() <-chan LazygitFrame {
	atomic.AddInt32(&f.framesCalls, 1)
	return f.frames
}
