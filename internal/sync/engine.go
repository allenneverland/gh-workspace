package sync

import (
	"context"
	"strings"
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
	if e == nil || e.fetcher == nil {
		return workspace.RepoStatus{}, nil
	}

	workspaceID := strings.TrimSpace(e.selectedWorkspaceID)
	repoID := strings.TrimSpace(e.selectedRepoID)
	if workspaceID == "" || repoID == "" {
		return workspace.RepoStatus{}, nil
	}

	return e.fetcher.FetchSelectedRepoStatus(ctx, workspaceID, repoID)
}

func (e *Engine) OnTick(ctx context.Context) (workspace.RepoStatus, error) {
	if e == nil || !e.autoPolling {
		return workspace.RepoStatus{}, nil
	}
	return e.RefreshNow(ctx)
}

func (e *Engine) OnSelectionChanged(ctx context.Context, workspaceID, repoID string) (workspace.RepoStatus, error) {
	if e == nil {
		return workspace.RepoStatus{}, nil
	}

	workspaceID = strings.TrimSpace(workspaceID)
	repoID = strings.TrimSpace(repoID)
	changed := e.selectedWorkspaceID != workspaceID || e.selectedRepoID != repoID
	e.selectedWorkspaceID = workspaceID
	e.selectedRepoID = repoID

	if !changed {
		return workspace.RepoStatus{}, nil
	}

	return e.RefreshNow(ctx)
}

func (e *Engine) Start(context.Context) tea.Cmd {
	if e == nil || !e.autoPolling || e.interval <= 0 {
		return nil
	}
	return e.tickFactory(e.interval)
}

func (e *Engine) SetAutoPolling(enabled bool) {
	if e == nil {
		return
	}
	e.autoPolling = enabled
}

func (e *Engine) AutoPollingEnabled() bool {
	if e == nil {
		return false
	}
	return e.autoPolling
}

func defaultTickFactory(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return MsgTick{}
	})
}

type MsgTick struct{}
