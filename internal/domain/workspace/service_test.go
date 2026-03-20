package workspace

import (
	"context"
	"errors"
	"testing"
)

func TestService_CreateWorkspaceAndSelectRepo_PersistsSelectedRepo(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	ws, err := svc.CreateWorkspace("default")
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	if ws.ID == "" {
		t.Fatal("CreateWorkspace() returned empty workspace ID")
	}

	repo, err := svc.AddRepo(ws.ID, RepoInput{
		Name:               "api",
		Path:               "/tmp/api",
		DefaultBranch:      "main",
		ReleaseWorkflowRef: ".github/workflows/release.yml",
	})
	if err != nil {
		t.Fatalf("AddRepo() error = %v", err)
	}
	if repo.ID == "" {
		t.Fatal("AddRepo() returned empty repo ID")
	}

	if err := svc.SelectRepo(ws.ID, repo.ID); err != nil {
		t.Fatalf("SelectRepo() error = %v", err)
	}

	reloaded := NewService(mem)
	state, err := reloaded.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	gotWorkspace, ok := findWorkspace(state, ws.ID)
	if !ok {
		t.Fatalf("workspace %q not found after reload", ws.ID)
	}
	if gotWorkspace.SelectedRepoID != repo.ID {
		t.Fatalf("selected repo mismatch: want %q, got %q", repo.ID, gotWorkspace.SelectedRepoID)
	}
	if len(gotWorkspace.Repos) != 1 {
		t.Fatalf("expected exactly one repo, got %d", len(gotWorkspace.Repos))
	}
	gotRepo := gotWorkspace.Repos[0]
	if gotRepo.Name != "api" {
		t.Fatalf("repo name mismatch: want %q, got %q", "api", gotRepo.Name)
	}
	if gotRepo.Path != "/tmp/api" {
		t.Fatalf("repo path mismatch: want %q, got %q", "/tmp/api", gotRepo.Path)
	}
	if gotRepo.DefaultBranch != "main" {
		t.Fatalf("default branch mismatch: want %q, got %q", "main", gotRepo.DefaultBranch)
	}
	if gotRepo.ReleaseWorkflowRef != ".github/workflows/release.yml" {
		t.Fatalf("release workflow ref mismatch: want %q, got %q", ".github/workflows/release.yml", gotRepo.ReleaseWorkflowRef)
	}
}

func TestService_CreateWorkspace_SupportsMultipleWorkspaces(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	first, err := svc.CreateWorkspace("first")
	if err != nil {
		t.Fatalf("CreateWorkspace(first) error = %v", err)
	}
	second, err := svc.CreateWorkspace("second")
	if err != nil {
		t.Fatalf("CreateWorkspace(second) error = %v", err)
	}

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(state.Workspaces) != 2 {
		t.Fatalf("expected two workspaces, got %d", len(state.Workspaces))
	}
	if first.ID == "" || second.ID == "" {
		t.Fatalf("expected non-empty workspace IDs, got %q and %q", first.ID, second.ID)
	}
	if first.ID == second.ID {
		t.Fatalf("expected unique workspace IDs, both were %q", first.ID)
	}
}

func TestService_DeleteWorkspace_RemovesWorkspaceAndUpdatesSelection(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	first, err := svc.CreateWorkspace("first")
	if err != nil {
		t.Fatalf("CreateWorkspace(first) error = %v", err)
	}
	second, err := svc.CreateWorkspace("second")
	if err != nil {
		t.Fatalf("CreateWorkspace(second) error = %v", err)
	}

	if err := svc.DeleteWorkspace(first.ID); err != nil {
		t.Fatalf("DeleteWorkspace(first) error = %v", err)
	}

	reloaded := NewService(mem)
	state, err := reloaded.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(state.Workspaces) != 1 {
		t.Fatalf("expected one workspace after deletion, got %d", len(state.Workspaces))
	}
	if state.Workspaces[0].ID != second.ID {
		t.Fatalf("expected remaining workspace %q, got %q", second.ID, state.Workspaces[0].ID)
	}
	if state.SelectedWorkspaceID != second.ID {
		t.Fatalf("expected selected workspace %q after deletion, got %q", second.ID, state.SelectedWorkspaceID)
	}
}

func TestService_DeleteWorkspace_ReturnsErrorWhenWorkspaceMissing(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	err := svc.DeleteWorkspace("missing")
	if err == nil {
		t.Fatal("DeleteWorkspace(missing) error = nil, want non-nil")
	}
	if !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("DeleteWorkspace(missing) error mismatch: want errors.Is(..., ErrWorkspaceNotFound), got %v", err)
	}
}

func TestService_MarkRepoInvalid_PersistsRepoHealth(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	ws, err := svc.CreateWorkspace("default")
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	repo, err := svc.AddRepo(ws.ID, RepoInput{Name: "api", Path: "/tmp/api"})
	if err != nil {
		t.Fatalf("AddRepo() error = %v", err)
	}

	if err := svc.MarkRepoInvalid(ws.ID, repo.ID); err != nil {
		t.Fatalf("MarkRepoInvalid() error = %v", err)
	}

	state, err := NewService(mem).LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	gotWorkspace, ok := findWorkspace(state, ws.ID)
	if !ok {
		t.Fatalf("workspace %q not found", ws.ID)
	}
	if len(gotWorkspace.Repos) != 1 {
		t.Fatalf("expected one repo, got %d", len(gotWorkspace.Repos))
	}
	if gotWorkspace.Repos[0].Health != RepoInvalid {
		t.Fatalf("expected repo health %q, got %q", RepoInvalid, gotWorkspace.Repos[0].Health)
	}
}

func TestService_UpdateRepoPath_UpdatesPathAndSetsHealthy(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	ws, err := svc.CreateWorkspace("default")
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}

	repo, err := svc.AddRepo(ws.ID, RepoInput{Name: "api", Path: "/tmp/api"})
	if err != nil {
		t.Fatalf("AddRepo() error = %v", err)
	}
	if err := svc.MarkRepoInvalid(ws.ID, repo.ID); err != nil {
		t.Fatalf("MarkRepoInvalid() error = %v", err)
	}

	if err := svc.UpdateRepoPath(ws.ID, repo.ID, "/tmp/api-new"); err != nil {
		t.Fatalf("UpdateRepoPath() error = %v", err)
	}

	state, err := NewService(mem).LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	gotWorkspace, ok := findWorkspace(state, ws.ID)
	if !ok {
		t.Fatalf("workspace %q not found", ws.ID)
	}
	if len(gotWorkspace.Repos) != 1 {
		t.Fatalf("expected one repo, got %d", len(gotWorkspace.Repos))
	}
	if gotWorkspace.Repos[0].Path != "/tmp/api-new" {
		t.Fatalf("expected path %q, got %q", "/tmp/api-new", gotWorkspace.Repos[0].Path)
	}
	if gotWorkspace.Repos[0].Health != RepoHealthy {
		t.Fatalf("expected repo health %q, got %q", RepoHealthy, gotWorkspace.Repos[0].Health)
	}
}

func TestService_UpdateRepoPath_RejectsEmptyPath(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	ws, err := svc.CreateWorkspace("default")
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	repo, err := svc.AddRepo(ws.ID, RepoInput{Name: "api", Path: "/tmp/api"})
	if err != nil {
		t.Fatalf("AddRepo() error = %v", err)
	}

	err = svc.UpdateRepoPath(ws.ID, repo.ID, " ")
	if err == nil {
		t.Fatal("UpdateRepoPath() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrRepoPathMissing) {
		t.Fatalf("UpdateRepoPath() error mismatch: want errors.Is(..., ErrRepoPathMissing), got %v", err)
	}
}

func TestService_RemoveRepo_RemovesRepoAndUpdatesSelection(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	ws, err := svc.CreateWorkspace("default")
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	first, err := svc.AddRepo(ws.ID, RepoInput{Name: "api", Path: "/tmp/api"})
	if err != nil {
		t.Fatalf("AddRepo(first) error = %v", err)
	}
	second, err := svc.AddRepo(ws.ID, RepoInput{Name: "web", Path: "/tmp/web"})
	if err != nil {
		t.Fatalf("AddRepo(second) error = %v", err)
	}
	if err := svc.SelectRepo(ws.ID, second.ID); err != nil {
		t.Fatalf("SelectRepo() error = %v", err)
	}

	if err := svc.RemoveRepo(ws.ID, second.ID); err != nil {
		t.Fatalf("RemoveRepo() error = %v", err)
	}

	state, err := NewService(mem).LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	gotWorkspace, ok := findWorkspace(state, ws.ID)
	if !ok {
		t.Fatalf("workspace %q not found", ws.ID)
	}
	if len(gotWorkspace.Repos) != 1 {
		t.Fatalf("expected one repo, got %d", len(gotWorkspace.Repos))
	}
	if gotWorkspace.Repos[0].ID != first.ID {
		t.Fatalf("expected remaining repo %q, got %q", first.ID, gotWorkspace.Repos[0].ID)
	}
	if gotWorkspace.SelectedRepoID != first.ID {
		t.Fatalf("expected selected repo %q, got %q", first.ID, gotWorkspace.SelectedRepoID)
	}
}

type memoryStore struct {
	state State
}

func (m *memoryStore) Load(context.Context) (State, error) {
	return cloneState(m.state), nil
}

func (m *memoryStore) Save(_ context.Context, st State) error {
	m.state = cloneState(st)
	return nil
}

func cloneState(st State) State {
	cloned := st
	cloned.Workspaces = make([]Workspace, len(st.Workspaces))
	for i := range st.Workspaces {
		ws := st.Workspaces[i]
		ws.Repos = append([]Repo(nil), ws.Repos...)
		cloned.Workspaces[i] = ws
	}
	return cloned
}

func findWorkspace(st State, id string) (Workspace, bool) {
	for _, ws := range st.Workspaces {
		if ws.ID == id {
			return ws, true
		}
	}
	return Workspace{}, false
}
