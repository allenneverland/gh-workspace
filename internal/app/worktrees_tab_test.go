package app

import (
	"context"
	"strings"
	"testing"

	worktreeadapter "github.com/allenneverland/gh-workspace/internal/adapters/worktree"
)

func TestUpdate_MsgCreateWorktree_UsesAdapterCreate(t *testing.T) {
	m := seededModelWithRepos()
	adapter := &fakeWorktreeAdapter{}
	m.WorktreeAdapter = adapter

	updated, _ := m.Update(MsgCreateWorktree{
		Branch: "feature/a",
		Path:   "../api-feature-a",
	})
	got := updated.(Model)

	if len(adapter.createCalls) != 1 {
		t.Fatalf("expected create to be called once, got %d", len(adapter.createCalls))
	}
	call := adapter.createCalls[0]
	if call.repoPath != "/tmp/api" {
		t.Fatalf("expected create repo path %q, got %q", "/tmp/api", call.repoPath)
	}
	if call.branch != "feature/a" {
		t.Fatalf("expected create branch %q, got %q", "feature/a", call.branch)
	}
	if call.targetPath != "../api-feature-a" {
		t.Fatalf("expected create target path %q, got %q", "../api-feature-a", call.targetPath)
	}
	if got.StatusMessage == "" {
		t.Fatal("expected status message after creating worktree")
	}
}

func TestUpdate_MsgSwitchWorktree_ExistingTarget_ValidatesAndPersistsSelection(t *testing.T) {
	m := seededModelWithRepos()
	adapter := &fakeWorktreeAdapter{
		listEntries: []worktreeadapter.Entry{
			{ID: "wt-main", Path: "/tmp/api"},
			{ID: "wt-feature-a", Path: "../api-feature-a"},
		},
	}
	m.WorktreeAdapter = adapter

	updated, _ := m.Update(MsgSwitchWorktree{WorktreePath: "../api-feature-a"})
	got := updated.(Model)

	if len(adapter.listCalls) != 1 {
		t.Fatalf("expected list to be called once, got %d", len(adapter.listCalls))
	}
	if len(adapter.validateCalls) != 1 {
		t.Fatalf("expected validate to be called once, got %d", len(adapter.validateCalls))
	}
	if len(adapter.createCalls) != 0 {
		t.Fatalf("expected create not to be called on switch, got %d", len(adapter.createCalls))
	}
	if got.SelectedWorktreeID != "wt-feature-a" {
		t.Fatalf("expected selected worktree id %q, got %q", "wt-feature-a", got.SelectedWorktreeID)
	}
	if got.SelectedWorktreePath != "../api-feature-a" {
		t.Fatalf("expected selected worktree path %q, got %q", "../api-feature-a", got.SelectedWorktreePath)
	}
	if !strings.Contains(got.StatusMessage, "switched worktree") {
		t.Fatalf("expected switch status message, got %q", got.StatusMessage)
	}
}

func TestUpdate_MsgSwitchWorktree_NonListedTarget_DoesNotValidateOrPersist(t *testing.T) {
	m := seededModelWithRepos()
	adapter := &fakeWorktreeAdapter{
		listEntries: []worktreeadapter.Entry{
			{ID: "wt-main", Path: "/tmp/api"},
		},
	}
	m.WorktreeAdapter = adapter

	updated, _ := m.Update(MsgSwitchWorktree{WorktreePath: "../api-feature-a"})
	got := updated.(Model)

	if len(adapter.listCalls) != 1 {
		t.Fatalf("expected list to be called once, got %d", len(adapter.listCalls))
	}
	if len(adapter.validateCalls) != 0 {
		t.Fatalf("expected validate not to be called for non-listed target, got %d", len(adapter.validateCalls))
	}
	if got.SelectedWorktreeID != "" || got.SelectedWorktreePath != "" {
		t.Fatalf("expected no selected worktree persisted, got id=%q path=%q", got.SelectedWorktreeID, got.SelectedWorktreePath)
	}
	if !strings.Contains(got.StatusMessage, "not found") {
		t.Fatalf("expected not-found status, got %q", got.StatusMessage)
	}
}

func TestView_RendersWorktreesSectionAndSelection(t *testing.T) {
	m := seededModelWithRepos()
	m.Worktrees = []worktreeadapter.Entry{
		{ID: "wt-main", Path: "/tmp/api", Branch: "main"},
		{ID: "wt-feature-a", Path: "../api-feature-a", Branch: "feature/a"},
	}
	m.SelectedWorktreeID = "wt-feature-a"
	m.SelectedWorktreePath = "../api-feature-a"

	got := m.View()

	assertContains(t, got, "Worktrees")
	assertContains(t, got, "wt-feature-a")
	assertContains(t, got, "selected worktree: ../api-feature-a")
	assertContains(t, got, "worktree actions: create/switch")
}

type fakeWorktreeAdapter struct {
	createCalls   []createCall
	listCalls     []string
	validateCalls []string

	listEntries []worktreeadapter.Entry
	createErr   error
	listErr     error
	validateErr error
}

type createCall struct {
	repoPath   string
	branch     string
	targetPath string
}

func (f *fakeWorktreeAdapter) Create(_ context.Context, repoPath, branch, targetPath string) error {
	f.createCalls = append(f.createCalls, createCall{
		repoPath:   repoPath,
		branch:     branch,
		targetPath: targetPath,
	})
	return f.createErr
}

func (f *fakeWorktreeAdapter) List(_ context.Context, repoPath string) ([]worktreeadapter.Entry, error) {
	f.listCalls = append(f.listCalls, repoPath)
	return append([]worktreeadapter.Entry(nil), f.listEntries...), f.listErr
}

func (f *fakeWorktreeAdapter) ValidateSwitchTarget(_ context.Context, worktreePath string) error {
	f.validateCalls = append(f.validateCalls, worktreePath)
	return f.validateErr
}
