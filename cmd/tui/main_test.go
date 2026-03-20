package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/allenneverland/gh-workspace/internal/app"
)

func TestRun_InvalidFolderWorkspaceCombo_ReturnsUsageError(t *testing.T) {
	err := run([]string{"-f", "/tmp/repo", "-w", "team-a"})
	if err == nil {
		t.Fatal("expected run error for invalid -f/-w combo")
	}
	if !strings.Contains(err.Error(), "cannot use -f with -w") {
		t.Fatalf("expected useful combo error, got %v", err)
	}
	if !strings.Contains(err.Error(), "Usage: gh-workspace") {
		t.Fatalf("expected usage text in launch parse error, got %v", err)
	}
}

func TestRun_DefaultNoArgs_StillComposesRuntime(t *testing.T) {
	called := false
	var captured LaunchOptions

	previousCompose := composeRuntimeModelForLaunch
	t.Cleanup(func() {
		composeRuntimeModelForLaunch = previousCompose
	})
	composeRuntimeModelForLaunch = func(_ context.Context, opts LaunchOptions) (app.Model, func() error, error) {
		called = true
		captured = opts
		return app.Model{}, nil, errors.New("compose sentinel")
	}

	err := run(nil)
	if err == nil {
		t.Fatal("expected compose sentinel error from runtime composition")
	}
	if !called {
		t.Fatal("expected default no-arg launch to reach runtime composition")
	}
	if captured.Mode != LaunchFolder {
		t.Fatalf("expected folder mode launch options, got %#v", captured)
	}
	cwd, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatalf("Getwd() error = %v", wdErr)
	}
	if captured.Path != cwd {
		t.Fatalf("expected folder path %q, got %q", cwd, captured.Path)
	}
	if !strings.Contains(err.Error(), "compose sentinel") {
		t.Fatalf("expected composed runtime error to bubble up, got %v", err)
	}
}

func TestRun_WorkspaceIntent_ComposesRuntime(t *testing.T) {
	called := false
	var captured LaunchOptions

	previousCompose := composeRuntimeModelForLaunch
	t.Cleanup(func() {
		composeRuntimeModelForLaunch = previousCompose
	})
	composeRuntimeModelForLaunch = func(_ context.Context, opts LaunchOptions) (app.Model, func() error, error) {
		called = true
		captured = opts
		return app.Model{}, nil, errors.New("compose sentinel")
	}

	err := run([]string{"-w", "does-not-exist"})
	if err == nil {
		t.Fatal("expected runtime composition error to bubble up")
	}
	if !called {
		t.Fatal("expected -w launch to reach runtime composition")
	}
	if captured.Mode != LaunchWorkspace || captured.WorkspaceName != "does-not-exist" {
		t.Fatalf("unexpected launch options captured: %#v", captured)
	}
	if !strings.Contains(err.Error(), "compose sentinel") {
		t.Fatalf("expected composed runtime error to bubble up, got %v", err)
	}
}

func TestRun_ReservedWorkspaceNameRejectedEarly(t *testing.T) {
	called := false

	previousCompose := composeRuntimeModelForLaunch
	t.Cleanup(func() {
		composeRuntimeModelForLaunch = previousCompose
	})
	composeRuntimeModelForLaunch = func(_ context.Context, _ LaunchOptions) (app.Model, func() error, error) {
		called = true
		return app.Model{}, nil, nil
	}

	err := run([]string{"-w", "__local_internal__"})
	if err == nil {
		t.Fatal("expected run error for reserved workspace name")
	}
	if called {
		t.Fatal("expected parser rejection to happen before runtime composition")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved-name parse error, got %v", err)
	}
}

func TestRun_NonCWDFolderIntent_ComposesRuntime(t *testing.T) {
	called := false
	var captured LaunchOptions

	previousCompose := composeRuntimeModelForLaunch
	t.Cleanup(func() {
		composeRuntimeModelForLaunch = previousCompose
	})
	composeRuntimeModelForLaunch = func(_ context.Context, opts LaunchOptions) (app.Model, func() error, error) {
		called = true
		captured = opts
		return app.Model{}, nil, errors.New("compose sentinel")
	}

	nonCWDPath := t.TempDir()
	err := run([]string{"-f", nonCWDPath})
	if err == nil {
		t.Fatal("expected runtime composition error to bubble up")
	}
	if !called {
		t.Fatal("expected -f launch to reach runtime composition")
	}
	if captured.Mode != LaunchFolder || captured.Path != nonCWDPath {
		t.Fatalf("unexpected launch options captured: %#v", captured)
	}
	if !strings.Contains(err.Error(), "compose sentinel") {
		t.Fatalf("expected composed runtime error to bubble up, got %v", err)
	}
}

func TestRun_InvalidArgumentCombination_IncludesUsageText(t *testing.T) {
	err := run([]string{"-f", "/tmp/repo", "extra"})
	if err == nil {
		t.Fatal("expected run error for invalid argument combination")
	}
	if !strings.Contains(err.Error(), "unexpected arguments") {
		t.Fatalf("expected argument validation error, got %v", err)
	}
	if !strings.Contains(err.Error(), "Usage: gh-workspace") {
		t.Fatalf("expected usage text in invalid-args error, got %v", err)
	}
}
