package main

import (
	"context"
	"errors"
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

func TestRun_WorkspaceNotFoundError_BubblesFromRuntime(t *testing.T) {
	previousCompose := composeRuntimeModelForLaunch
	t.Cleanup(func() {
		composeRuntimeModelForLaunch = previousCompose
	})

	composeRuntimeModelForLaunch = func(_ context.Context, opts LaunchOptions) (app.Model, func() error, error) {
		if opts.Mode != LaunchWorkspace {
			t.Fatalf("expected workspace mode opts, got %#v", opts)
		}
		if opts.WorkspaceName != "does-not-exist" {
			t.Fatalf("expected workspace %q, got %q", "does-not-exist", opts.WorkspaceName)
		}
		return app.Model{}, nil, errors.New("workspace not found: does-not-exist")
	}

	err := run([]string{"-w", "does-not-exist"})
	if err == nil {
		t.Fatal("expected run error for unknown workspace")
	}
	if !strings.Contains(err.Error(), "workspace not found: does-not-exist") {
		t.Fatalf("expected runtime error to bubble up, got %v", err)
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
