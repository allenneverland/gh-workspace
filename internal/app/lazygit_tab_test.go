package app

import (
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

	sessionsByRepo map[string]LazygitSessionHandle
	frames         chan LazygitFrame
}

type lazygitWriteCall struct {
	sessionID string
	data      []byte
}

func newFakeLazygitSessionManager() *fakeLazygitSessionManager {
	return &fakeLazygitSessionManager{
		sessionsByRepo: make(map[string]LazygitSessionHandle),
		frames:         make(chan LazygitFrame, 8),
	}
}

func (f *fakeLazygitSessionManager) StartSession(repoPath string) (LazygitSessionHandle, error) {
	f.startCalls = append(f.startCalls, repoPath)
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
	return f.frames
}
