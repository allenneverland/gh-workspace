package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/allenneverland/gh-workspace/internal/adapters/github"
	repositoryadapter "github.com/allenneverland/gh-workspace/internal/adapters/repository"
	"github.com/allenneverland/gh-workspace/internal/app"
	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	"github.com/allenneverland/gh-workspace/internal/store/boltdb"
	syncengine "github.com/allenneverland/gh-workspace/internal/sync"
)

const (
	envTestMode               = "WORKSPACE_TUI_TEST_MODE"
	envStatePath              = "WORKSPACE_TUI_STATE_PATH"
	StatusCurrentFolderNotGit = "current folder is not a git repo"
)

var ErrWorkspaceNotFound = errors.New("workspace not found")

func composeRuntimeModel(ctx context.Context, opts LaunchOptions) (app.Model, func() error, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	initialMode := uiModeForLaunchMode(opts.Mode)
	if isTestMode() {
		model := app.NewModel(app.Config{
			InitialUIMode: initialMode,
			SyncEngine:    syncengine.NewEngine(syncengine.NoopSelectedRepoStatusFetcher{}),
		})
		return model, nil, nil
	}

	statePath, err := resolveStateStorePath()
	if err != nil {
		return app.Model{}, nil, err
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return app.Model{}, nil, err
	}

	stateStore, err := boltdb.Open(statePath)
	if err != nil {
		return app.Model{}, nil, err
	}

	statusMessage, err := applyLaunchIntent(ctx, stateStore, opts)
	if err != nil {
		_ = stateStore.Close()
		return app.Model{}, nil, err
	}

	loadedState, err := stateStore.Load(ctx)
	if err != nil {
		_ = stateStore.Close()
		return app.Model{}, nil, err
	}

	ghAdapter := github.NewAdapter(nil)
	selectedRepoFetcher := syncengine.NewStateBackedFetcher(ghAdapter)
	selectedRepoFetcher.SetState(loadedState)
	engine := syncengine.NewEngine(selectedRepoFetcher)

	model := app.NewModel(app.Config{
		InitialState:       loadedState,
		InitialUIMode:      initialMode,
		SyncEngine:         engine,
		StateStore:         stateStore,
		SyncStatePublisher: selectedRepoFetcher,
	})
	if strings.TrimSpace(statusMessage) != "" {
		model.StatusMessage = statusMessage
	}

	return model, stateStore.Close, nil
}

func applyLaunchIntent(ctx context.Context, stateStore workspace.StateStore, opts LaunchOptions) (string, error) {
	svc := workspace.NewService(stateStore)
	switch opts.Mode {
	case LaunchFolder:
		return applyFolderLaunchIntent(ctx, svc, opts.Path)
	case LaunchWorkspace:
		return applyWorkspaceLaunchIntent(ctx, stateStore, svc, opts.WorkspaceName)
	default:
		return "", fmt.Errorf("unsupported launch mode %q", opts.Mode)
	}
}

func applyFolderLaunchIntent(ctx context.Context, svc *workspace.Service, path string) (string, error) {
	if err := svc.EnsureLocalWorkspaceIntegrity(); err != nil {
		return "", fmt.Errorf("ensure local workspace integrity: %w", err)
	}
	if _, err := svc.EnsureLocalWorkspace(); err != nil {
		return "", fmt.Errorf("ensure local workspace: %w", err)
	}

	repoRoot, ok, err := repositoryadapter.ResolveRepoRoot(ctx, path)
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	if !ok {
		if err := svc.ClearLocalRepos(); err != nil {
			return "", fmt.Errorf("clear local repos: %w", err)
		}
		return StatusCurrentFolderNotGit, nil
	}

	if _, err := svc.ReplaceLocalRepo(workspace.RepoInput{
		Name: repoNameFromPath(repoRoot),
		Path: repoRoot,
	}); err != nil {
		return "", fmt.Errorf("replace local repo: %w", err)
	}
	return "", nil
}

func applyWorkspaceLaunchIntent(ctx context.Context, stateStore workspace.StateStore, svc *workspace.Service, workspaceName string) (string, error) {
	ws, found, err := svc.FindWorkspaceByName(workspaceName, false)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%w: %s", ErrWorkspaceNotFound, workspaceName)
	}

	if err := selectWorkspace(ctx, stateStore, ws.ID); err != nil {
		return "", fmt.Errorf("select workspace: %w", err)
	}
	return "", nil
}

func selectWorkspace(ctx context.Context, stateStore workspace.StateStore, workspaceID string) error {
	state, err := stateStore.Load(ctx)
	if err != nil {
		return err
	}

	idx := -1
	for i := range state.Workspaces {
		if state.Workspaces[i].ID == workspaceID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("%w: %s", workspace.ErrWorkspaceNotFound, workspaceID)
	}

	state.SelectedWorkspaceID = workspaceID
	return stateStore.Save(ctx, state)
}

func repoNameFromPath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	name := filepath.Base(cleaned)
	switch name {
	case "", ".", string(filepath.Separator):
		return cleaned
	default:
		return name
	}
}

func uiModeForLaunchMode(mode LaunchMode) app.UIMode {
	if mode == LaunchFolder {
		return app.ModeFolder
	}
	return app.ModeWorkspace
}

func isTestMode() bool {
	return strings.TrimSpace(os.Getenv(envTestMode)) == "1"
}

func resolveStateStorePath() (string, error) {
	if override := strings.TrimSpace(os.Getenv(envStatePath)); override != "" {
		return override, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "gh-workspace", "state.db"), nil
}
