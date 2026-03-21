package lazygit

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

const (
	defaultPTYRows = 40
	defaultPTYCols = 120
)

var ptyStartWithSize = pty.StartWithSize
var ptySetSize = pty.Setsize

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
	pendingByRepo  map[string]*pendingStart
	frames         chan Frame
}

type session struct {
	handle SessionHandle
	proc   *sessionProcess
	vt     vt10x.Terminal
	cols   int
	rows   int
	mu     sync.Mutex
}

type pendingStart struct {
	done   chan struct{}
	handle SessionHandle
	err    error
}

type sessionProcess struct {
	pty    io.ReadWriteCloser
	wait   func() error
	resize func(cols, rows int) error
}

func NewSessionManager() *SessionManager {
	return newSessionManagerWithStarter(startLazygitProcess)
}

func newSessionManagerWithStarter(start func(repoPath string) (*sessionProcess, error)) *SessionManager {
	return &SessionManager{
		start:          start,
		sessionsByRepo: make(map[string]*session),
		sessionsByID:   make(map[string]*session),
		pendingByRepo:  make(map[string]*pendingStart),
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
	if pending, ok := m.pendingByRepo[repoPath]; ok {
		m.mu.Unlock()
		<-pending.done
		return pending.handle, pending.err
	}
	pending := &pendingStart{done: make(chan struct{})}
	m.pendingByRepo[repoPath] = pending
	m.mu.Unlock()

	proc, err := m.start(repoPath)

	m.mu.Lock()
	delete(m.pendingByRepo, repoPath)
	if err != nil {
		pending.err = err
		close(pending.done)
		m.mu.Unlock()
		return SessionHandle{}, err
	}
	m.nextID++
	handle := SessionHandle{ID: fmt.Sprintf("lazygit-%d", m.nextID), RepoPath: repoPath}
	s := &session{
		handle: handle,
		proc:   proc,
		vt:     vt10x.New(vt10x.WithSize(defaultPTYCols, defaultPTYRows)),
		cols:   defaultPTYCols,
		rows:   defaultPTYRows,
	}
	m.sessionsByRepo[repoPath] = s
	m.sessionsByID[handle.ID] = s
	pending.handle = handle
	close(pending.done)
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

func (m *SessionManager) ResizeSession(sessionID string, cols, rows int) error {
	if sessionID == "" {
		return errors.New("session id is empty")
	}
	if cols <= 0 || rows <= 0 {
		return nil
	}

	m.mu.Lock()
	s, ok := m.sessionsByID[sessionID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.proc.resize != nil {
		if err := s.proc.resize(cols, rows); err != nil {
			return fmt.Errorf("resize session: %w", err)
		}
	}
	s.cols = cols
	s.rows = rows
	s.vt.Resize(cols, rows)
	frame := Frame{
		SessionID: s.handle.ID,
		Data:      []byte(renderTerminalSnapshot(s.vt, cols, rows)),
	}
	m.enqueueFrame(frame)
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
			s.mu.Lock()
			_, _ = s.vt.Write(buf[:n])
			snapshot := renderTerminalSnapshot(s.vt, s.cols, s.rows)
			s.mu.Unlock()

			frame := Frame{
				SessionID: s.handle.ID,
				Data:      []byte(snapshot),
			}
			m.enqueueFrame(frame)
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

func (m *SessionManager) enqueueFrame(frame Frame) {
	for {
		select {
		case m.frames <- frame:
			return
		default:
		}

		select {
		case <-m.frames:
		default:
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

	ptmx, err := ptyStartWithSize(cmd, &pty.Winsize{
		Rows: defaultPTYRows,
		Cols: defaultPTYCols,
	})
	if err != nil {
		return nil, fmt.Errorf("start lazygit pty: %w", err)
	}

	return &sessionProcess{
		pty:  ptmx,
		wait: cmd.Wait,
		resize: func(cols, rows int) error {
			if cols <= 0 || rows <= 0 {
				return nil
			}
			if err := ptySetSize(ptmx, &pty.Winsize{
				Rows: uint16(rows),
				Cols: uint16(cols),
			}); err != nil {
				return fmt.Errorf("set pty size: %w", err)
			}
			return nil
		},
	}, nil
}

const (
	glyphAttrReverse = 1 << iota
	glyphAttrUnderline
	glyphAttrBold
	_
	glyphAttrItalic
	glyphAttrBlink
)

type cellStyle struct {
	fg        vt10x.Color
	bg        vt10x.Color
	bold      bool
	underline bool
	italic    bool
	blink     bool
	reverse   bool
}

func renderTerminalSnapshot(view vt10x.View, cols, rows int) string {
	if view == nil || cols <= 0 || rows <= 0 {
		return ""
	}

	defaultStyle := cellStyle{
		fg: vt10x.DefaultFG,
		bg: vt10x.DefaultBG,
	}
	var out strings.Builder
	estimatedLine := cols*2 + 12
	out.Grow(rows * estimatedLine)

	for y := 0; y < rows; y++ {
		current := defaultStyle
		for x := 0; x < cols; x++ {
			cell := view.Cell(x, y)
			target := cellStyleFromGlyph(cell)
			if target != current {
				out.WriteString(sgrFromStyle(target))
				current = target
			}
			ch := cell.Char
			if ch == 0 {
				ch = ' '
			}
			out.WriteRune(ch)
		}
		if current != defaultStyle {
			out.WriteString("\x1b[0m")
		}
		if y < rows-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func cellStyleFromGlyph(cell vt10x.Glyph) cellStyle {
	mode := int(cell.Mode)
	return cellStyle{
		fg:        cell.FG,
		bg:        cell.BG,
		bold:      mode&glyphAttrBold != 0,
		underline: mode&glyphAttrUnderline != 0,
		italic:    mode&glyphAttrItalic != 0,
		blink:     mode&glyphAttrBlink != 0,
		reverse:   mode&glyphAttrReverse != 0,
	}
}

func sgrFromStyle(style cellStyle) string {
	codes := []string{"0"}
	if style.bold {
		codes = append(codes, "1")
	}
	if style.italic {
		codes = append(codes, "3")
	}
	if style.underline {
		codes = append(codes, "4")
	}
	if style.blink {
		codes = append(codes, "5")
	}
	if style.reverse {
		codes = append(codes, "7")
	}
	codes = append(codes, colorCodes(style.fg, true)...)
	codes = append(codes, colorCodes(style.bg, false)...)
	return "\x1b[" + strings.Join(codes, ";") + "m"
}

func colorCodes(color vt10x.Color, foreground bool) []string {
	defaultCode := "39"
	if !foreground {
		defaultCode = "49"
	}

	if color == vt10x.DefaultFG || color == vt10x.DefaultBG || color == vt10x.DefaultCursor {
		return []string{defaultCode}
	}
	value := int(color)
	if value < 0 {
		return []string{defaultCode}
	}
	if value < 8 {
		base := 30
		if !foreground {
			base = 40
		}
		return []string{strconv.Itoa(base + value)}
	}
	if value < 16 {
		base := 90
		if !foreground {
			base = 100
		}
		return []string{strconv.Itoa(base + (value - 8))}
	}
	if value < 256 {
		if foreground {
			return []string{"38", "5", strconv.Itoa(value)}
		}
		return []string{"48", "5", strconv.Itoa(value)}
	}
	return []string{defaultCode}
}
