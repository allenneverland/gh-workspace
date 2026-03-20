package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/allenneverland/gh-workspace/internal/adapters/github"
	"github.com/allenneverland/gh-workspace/internal/app"
	"github.com/allenneverland/gh-workspace/internal/store/boltdb"
	syncengine "github.com/allenneverland/gh-workspace/internal/sync"
)

const (
	envTestMode  = "WORKSPACE_TUI_TEST_MODE"
	envStatePath = "WORKSPACE_TUI_STATE_PATH"
)

func composeRuntimeModel(ctx context.Context) (app.Model, func() error, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if isTestMode() {
		model := app.NewModel(app.Config{
			SyncEngine: syncengine.NewEngine(syncengine.NoopSelectedRepoStatusFetcher{}),
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
		SyncEngine:         engine,
		StateStore:         stateStore,
		SyncStatePublisher: selectedRepoFetcher,
	})

	return model, stateStore.Close, nil
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
