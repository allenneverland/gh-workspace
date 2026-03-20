package sync

import (
	"context"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

const defaultInterval = 30 * time.Second

type SelectedRepoStatusFetcher interface {
	FetchSelectedRepoStatus(ctx context.Context, workspaceID, repoID string) (workspace.RepoStatus, error)
}

type Option func(*Engine)

type Engine struct {
	mu sync.RWMutex

	fetcher SelectedRepoStatusFetcher

	interval    time.Duration
	autoPolling bool

	selectedWorkspaceID string
	selectedRepoID      string

	tickFactory func(interval time.Duration) tea.Cmd
}

func NewEngine(fetcher SelectedRepoStatusFetcher, opts ...Option) *Engine {
	e := &Engine{
		fetcher:     fetcher,
		interval:    defaultInterval,
		autoPolling: true,
		tickFactory: defaultTickFactory,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	if e.tickFactory == nil {
		e.tickFactory = defaultTickFactory
	}
	return e
}

func WithInterval(interval time.Duration) Option {
	return func(e *Engine) {
		if interval > 0 {
			e.interval = interval
		}
	}
}

func WithAutoPolling(enabled bool) Option {
	return func(e *Engine) {
		e.autoPolling = enabled
	}
}

func WithTickFactory(factory func(interval time.Duration) tea.Cmd) Option {
	return func(e *Engine) {
		e.tickFactory = factory
	}
}

func (e *Engine) RefreshNow(ctx context.Context) (workspace.RepoStatus, error) {
	if e == nil {
		return workspace.RepoStatus{}, nil
	}
	fetcher, workspaceID, repoID := e.snapshotSelection()
	return fetchSelectedRepoStatus(ctx, fetcher, workspaceID, repoID)
}

func (e *Engine) SetSelection(workspaceID, repoID string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.selectedWorkspaceID = strings.TrimSpace(workspaceID)
	e.selectedRepoID = strings.TrimSpace(repoID)
}

func (e *Engine) OnTick(ctx context.Context) (workspace.RepoStatus, error) {
	if e == nil {
		return workspace.RepoStatus{}, nil
	}
	fetcher, workspaceID, repoID, enabled := e.snapshotPollingSelection()
	if !enabled {
		return workspace.RepoStatus{}, nil
	}
	return fetchSelectedRepoStatus(ctx, fetcher, workspaceID, repoID)
}

func (e *Engine) OnSelectionChanged(ctx context.Context, workspaceID, repoID string) (workspace.RepoStatus, error) {
	if e == nil {
		return workspace.RepoStatus{}, nil
	}

	workspaceID = strings.TrimSpace(workspaceID)
	repoID = strings.TrimSpace(repoID)
	e.mu.Lock()
	changed := e.selectedWorkspaceID != workspaceID || e.selectedRepoID != repoID
	e.selectedWorkspaceID = workspaceID
	e.selectedRepoID = repoID
	fetcher := e.fetcher
	e.mu.Unlock()

	if !changed {
		return workspace.RepoStatus{}, nil
	}

	return fetchSelectedRepoStatus(ctx, fetcher, workspaceID, repoID)
}

func (e *Engine) Start(context.Context) tea.Cmd {
	if e == nil {
		return nil
	}

	e.mu.RLock()
	enabled := e.autoPolling
	interval := e.interval
	tickFactory := e.tickFactory
	e.mu.RUnlock()

	if !enabled || interval <= 0 || tickFactory == nil {
		return nil
	}
	return tickFactory(interval)
}

func (e *Engine) SetAutoPolling(enabled bool) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.autoPolling = enabled
}

func (e *Engine) AutoPollingEnabled() bool {
	if e == nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.autoPolling
}

func defaultTickFactory(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return MsgTick{}
	})
}

type MsgTick struct{}

func (e *Engine) snapshotSelection() (SelectedRepoStatusFetcher, string, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.fetcher, e.selectedWorkspaceID, e.selectedRepoID
}

func (e *Engine) snapshotPollingSelection() (SelectedRepoStatusFetcher, string, string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.fetcher, e.selectedWorkspaceID, e.selectedRepoID, e.autoPolling
}

func fetchSelectedRepoStatus(
	ctx context.Context,
	fetcher SelectedRepoStatusFetcher,
	workspaceID,
	repoID string,
) (workspace.RepoStatus, error) {
	if fetcher == nil {
		return workspace.RepoStatus{}, nil
	}

	workspaceID = strings.TrimSpace(workspaceID)
	repoID = strings.TrimSpace(repoID)
	if workspaceID == "" || repoID == "" {
		return workspace.RepoStatus{}, nil
	}

	return fetcher.FetchSelectedRepoStatus(ctx, workspaceID, repoID)
}
