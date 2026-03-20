package lazygit

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

func TestLazygitSessionManager_StartSession_ReusesByRepoPath(t *testing.T) {
	var starts int
	manager := newSessionManagerWithStarter(func(repoPath string) (*sessionProcess, error) {
		starts++
		return &sessionProcess{
			pty: newFakePTY(),
		}, nil
	})

	first, err := manager.StartSession("/tmp/api")
	if err != nil {
		t.Fatalf("expected first start to succeed, got error: %v", err)
	}
	second, err := manager.StartSession("/tmp/api")
	if err != nil {
		t.Fatalf("expected second start on same repo to succeed, got error: %v", err)
	}
	third, err := manager.StartSession("/tmp/web")
	if err != nil {
		t.Fatalf("expected start on different repo to succeed, got error: %v", err)
	}

	if starts != 2 {
		t.Fatalf("expected starter to be called for unique repos only, got %d calls", starts)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same session id for same repo, got %q and %q", first.ID, second.ID)
	}
	if third.ID == first.ID {
		t.Fatalf("expected different session id for different repo, got same id %q", third.ID)
	}
}

func TestLazygitSessionManager_WriteInput_ForwardsBytesToPTY(t *testing.T) {
	pty := newFakePTY()
	manager := newSessionManagerWithStarter(func(repoPath string) (*sessionProcess, error) {
		if repoPath != "/tmp/api" {
			t.Fatalf("expected repo path %q, got %q", "/tmp/api", repoPath)
		}
		return &sessionProcess{pty: pty}, nil
	})

	session, err := manager.StartSession("/tmp/api")
	if err != nil {
		t.Fatalf("expected start to succeed, got error: %v", err)
	}

	if err := manager.WriteInput(session.ID, []byte("jj")); err != nil {
		t.Fatalf("expected write to succeed, got error: %v", err)
	}
	if got := pty.lastWrite(); got != "jj" {
		t.Fatalf("expected forwarded write %q, got %q", "jj", got)
	}
}

func TestLazygitSessionManager_ReaderEmitsFrames(t *testing.T) {
	pty := newFakePTY()
	manager := newSessionManagerWithStarter(func(_ string) (*sessionProcess, error) {
		return &sessionProcess{pty: pty}, nil
	})

	session, err := manager.StartSession("/tmp/api")
	if err != nil {
		t.Fatalf("expected start to succeed, got error: %v", err)
	}

	pty.emit("frame-one")

	select {
	case frame := <-manager.Frames():
		if frame.SessionID != session.ID {
			t.Fatalf("expected frame session id %q, got %q", session.ID, frame.SessionID)
		}
		if got := string(frame.Data); got != "frame-one" {
			t.Fatalf("expected frame payload %q, got %q", "frame-one", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for lazygit frame")
	}
}

func TestLazygitSessionManager_StartSession_PropagatesStarterError(t *testing.T) {
	manager := newSessionManagerWithStarter(func(string) (*sessionProcess, error) {
		return nil, errors.New("start failed")
	})

	_, err := manager.StartSession("/tmp/api")
	if err == nil {
		t.Fatal("expected start error")
	}
}

type fakePTY struct {
	readQueue chan []byte
	closed    chan struct{}

	mu     sync.Mutex
	writes [][]byte
}

func newFakePTY() *fakePTY {
	return &fakePTY{
		readQueue: make(chan []byte, 16),
		closed:    make(chan struct{}),
	}
}

func (f *fakePTY) emit(data string) {
	f.readQueue <- []byte(data)
}

func (f *fakePTY) Read(p []byte) (int, error) {
	select {
	case data := <-f.readQueue:
		n := copy(p, data)
		return n, nil
	case <-f.closed:
		return 0, io.EOF
	}
}

func (f *fakePTY) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	buf := append([]byte(nil), p...)
	f.writes = append(f.writes, buf)
	return len(p), nil
}

func (f *fakePTY) Close() error {
	select {
	case <-f.closed:
	default:
		close(f.closed)
	}
	return nil
}

func (f *fakePTY) lastWrite() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.writes) == 0 {
		return ""
	}
	return string(f.writes[len(f.writes)-1])
}
