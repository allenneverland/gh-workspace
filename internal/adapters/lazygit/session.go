package lazygit

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

type SessionHandle struct {
	ID       string
	RepoPath string
}

type Frame struct {
	SessionID string
	Data      []byte
}

type SessionManager struct {
	mu sync.Mutex

	nextID int
	start  func(repoPath string) (*sessionProcess, error)

	sessionsByRepo map[string]*session
	sessionsByID   map[string]*session
	frames         chan Frame
}

type session struct {
	handle SessionHandle
	proc   *sessionProcess
}

type sessionProcess struct {
	pty  io.ReadWriteCloser
	wait func() error
}

func NewSessionManager() *SessionManager {
	return newSessionManagerWithStarter(startLazygitProcess)
}

func newSessionManagerWithStarter(start func(repoPath string) (*sessionProcess, error)) *SessionManager {
	return &SessionManager{
		start:          start,
		sessionsByRepo: make(map[string]*session),
		sessionsByID:   make(map[string]*session),
		frames:         make(chan Frame, 128),
	}
}

func (m *SessionManager) StartSession(repoPath string) (SessionHandle, error) {
	if repoPath == "" {
		return SessionHandle{}, errors.New("repo path is empty")
	}

	m.mu.Lock()
	if existing, ok := m.sessionsByRepo[repoPath]; ok {
		handle := existing.handle
		m.mu.Unlock()
		return handle, nil
	}
	m.mu.Unlock()

	proc, err := m.start(repoPath)
	if err != nil {
		return SessionHandle{}, err
	}

	m.mu.Lock()
	m.nextID++
	handle := SessionHandle{
		ID:       fmt.Sprintf("lazygit-%d", m.nextID),
		RepoPath: repoPath,
	}
	s := &session{
		handle: handle,
		proc:   proc,
	}
	m.sessionsByRepo[repoPath] = s
	m.sessionsByID[handle.ID] = s
	m.mu.Unlock()

	go m.readLoop(s)
	if proc.wait != nil {
		go m.waitLoop(s)
	}

	return handle, nil
}

func (m *SessionManager) WriteInput(sessionID string, input []byte) error {
	if sessionID == "" {
		return errors.New("session id is empty")
	}
	if len(input) == 0 {
		return nil
	}

	m.mu.Lock()
	s, ok := m.sessionsByID[sessionID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if _, err := s.proc.pty.Write(input); err != nil {
		return fmt.Errorf("write input: %w", err)
	}
	return nil
}

func (m *SessionManager) Frames() <-chan Frame {
	return m.frames
}

func (m *SessionManager) readLoop(s *session) {
	buf := make([]byte, 4096)
	for {
		n, err := s.proc.pty.Read(buf)
		if n > 0 {
			frame := Frame{
				SessionID: s.handle.ID,
				Data:      append([]byte(nil), buf[:n]...),
			}
			select {
			case m.frames <- frame:
			default:
			}
		}

		if err != nil {
			if !errors.Is(err, io.EOF) {
				// Non-EOF read errors still terminate this session loop.
			}
			_ = s.proc.pty.Close()
			m.removeSession(s)
			return
		}
	}
}

func (m *SessionManager) waitLoop(s *session) {
	_ = s.proc.wait()
	_ = s.proc.pty.Close()
	m.removeSession(s)
}

func (m *SessionManager) removeSession(s *session) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.sessionsByID[s.handle.ID]
	if ok && current == s {
		delete(m.sessionsByID, s.handle.ID)
	}
	current, ok = m.sessionsByRepo[s.handle.RepoPath]
	if ok && current == s {
		delete(m.sessionsByRepo, s.handle.RepoPath)
	}
}

func startLazygitProcess(repoPath string) (*sessionProcess, error) {
	cmd := exec.Command("lazygit")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start lazygit pty: %w", err)
	}

	return &sessionProcess{
		pty:  ptmx,
		wait: cmd.Wait,
	}, nil
}
