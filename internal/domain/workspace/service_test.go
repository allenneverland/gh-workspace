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

func TestService_EnsureLocalWorkspace_FirstRunCreatesSystemWorkspace(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	ws, err := svc.EnsureLocalWorkspace()
	if err != nil {
		t.Fatalf("EnsureLocalWorkspace() error = %v", err)
	}
	if ws.ID != LocalWorkspaceID {
		t.Fatalf("expected local workspace ID %q, got %q", LocalWorkspaceID, ws.ID)
	}
	if ws.Name != LocalWorkspaceName {
		t.Fatalf("expected local workspace name %q, got %q", LocalWorkspaceName, ws.Name)
	}
	if len(ws.Repos) != 0 {
		t.Fatalf("expected no repos on first run, got %d", len(ws.Repos))
	}

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if len(state.Workspaces) != 1 {
		t.Fatalf("expected exactly one workspace in state, got %d", len(state.Workspaces))
	}
	if state.Workspaces[0].ID != LocalWorkspaceID {
		t.Fatalf("expected persisted workspace ID %q, got %q", LocalWorkspaceID, state.Workspaces[0].ID)
	}
}

func TestService_ReplaceLocalRepo_EnsuresSingleRepoInLocalWorkspace(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	first, err := svc.ReplaceLocalRepo(RepoInput{
		Name: "first",
		Path: "/tmp/first",
	})
	if err != nil {
		t.Fatalf("ReplaceLocalRepo(first) error = %v", err)
	}
	second, err := svc.ReplaceLocalRepo(RepoInput{
		Name:          "second",
		Path:          "/tmp/second",
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("ReplaceLocalRepo(second) error = %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("expected a newly created repo ID on replace, both were %q", first.ID)
	}

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	ws, ok := findWorkspace(state, LocalWorkspaceID)
	if !ok {
		t.Fatalf("local workspace %q not found", LocalWorkspaceID)
	}
	if len(ws.Repos) != 1 {
		t.Fatalf("expected exactly one local repo, got %d", len(ws.Repos))
	}
	if ws.Repos[0].ID != second.ID {
		t.Fatalf("expected remaining repo ID %q, got %q", second.ID, ws.Repos[0].ID)
	}
	if ws.Repos[0].Name != "second" {
		t.Fatalf("expected remaining repo name %q, got %q", "second", ws.Repos[0].Name)
	}
	if ws.Repos[0].Path != "/tmp/second" {
		t.Fatalf("expected remaining repo path %q, got %q", "/tmp/second", ws.Repos[0].Path)
	}
}

func TestService_ClearLocalRepos_LeavesWorkspaceButNoRepos(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	repo, err := svc.ReplaceLocalRepo(RepoInput{
		Name: "first",
		Path: "/tmp/first",
	})
	if err != nil {
		t.Fatalf("ReplaceLocalRepo() error = %v", err)
	}
	if repo.ID == "" {
		t.Fatal("ReplaceLocalRepo() returned empty repo ID")
	}

	if err := svc.ClearLocalRepos(); err != nil {
		t.Fatalf("ClearLocalRepos() error = %v", err)
	}

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	ws, ok := findWorkspace(state, LocalWorkspaceID)
	if !ok {
		t.Fatalf("local workspace %q not found", LocalWorkspaceID)
	}
	if len(ws.Repos) != 0 {
		t.Fatalf("expected local repos to be cleared, got %d", len(ws.Repos))
	}
	if ws.SelectedRepoID != "" {
		t.Fatalf("expected local selected repo to be cleared, got %q", ws.SelectedRepoID)
	}
}

func TestService_FindWorkspaceByName_ExcludesSystemByDefault(t *testing.T) {
	mem := &memoryStore{}
	svc := NewService(mem)

	if _, err := svc.EnsureLocalWorkspace(); err != nil {
		t.Fatalf("EnsureLocalWorkspace() error = %v", err)
	}
	userWorkspace, err := svc.CreateWorkspace("team-a")
	if err != nil {
		t.Fatalf("CreateWorkspace(team-a) error = %v", err)
	}

	_, found, err := svc.FindWorkspaceByName(LocalWorkspaceName, false)
	if err != nil {
		t.Fatalf("FindWorkspaceByName(includeSystem=false) error = %v", err)
	}
	if found {
		t.Fatal("expected system workspace lookup to be excluded when includeSystem=false")
	}

	ws, found, err := svc.FindWorkspaceByName(LocalWorkspaceName, true)
	if err != nil {
		t.Fatalf("FindWorkspaceByName(includeSystem=true) error = %v", err)
	}
	if !found {
		t.Fatal("expected system workspace lookup to succeed when includeSystem=true")
	}
	if ws.ID != LocalWorkspaceID {
		t.Fatalf("expected system workspace ID %q, got %q", LocalWorkspaceID, ws.ID)
	}

	ws, found, err = svc.FindWorkspaceByName("team-a", false)
	if err != nil {
		t.Fatalf("FindWorkspaceByName(user, includeSystem=false) error = %v", err)
	}
	if !found {
		t.Fatal("expected user workspace lookup to succeed")
	}
	if ws.ID != userWorkspace.ID {
		t.Fatalf("expected workspace ID %q, got %q", userWorkspace.ID, ws.ID)
	}
}

func TestService_EnsureLocalWorkspaceIntegrity_RenamesCollisionsDeterministically(t *testing.T) {
	mem := &memoryStore{
		state: State{
			SelectedWorkspaceID: LocalWorkspaceID,
			Workspaces: []Workspace{
				{
					ID:   LocalWorkspaceID,
					Name: "my-workspace",
				},
				{
					ID:   "ws-001",
					Name: LocalWorkspaceName,
				},
				{
					ID:   "my-workspace-legacy-1",
					Name: "my-workspace-legacy-1",
				},
				{
					ID:   "ws-001-legacy-1",
					Name: "ws-001-legacy-1",
				},
			},
		},
	}
	svc := NewService(mem)

	if err := svc.EnsureLocalWorkspaceIntegrity(); err != nil {
		t.Fatalf("EnsureLocalWorkspaceIntegrity() error = %v", err)
	}

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(state.Workspaces) != 4 {
		t.Fatalf("expected workspace count to remain 4, got %d", len(state.Workspaces))
	}

	first := state.Workspaces[0]
	if first.ID != "my-workspace-legacy-2" {
		t.Fatalf("expected first workspace ID %q, got %q", "my-workspace-legacy-2", first.ID)
	}
	if first.Name != "my-workspace-legacy-2" {
		t.Fatalf("expected first workspace name %q, got %q", "my-workspace-legacy-2", first.Name)
	}

	second := state.Workspaces[1]
	if second.ID != "ws-001-legacy-2" {
		t.Fatalf("expected second workspace ID %q, got %q", "ws-001-legacy-2", second.ID)
	}
	if second.Name != "ws-001-legacy-2" {
		t.Fatalf("expected second workspace name %q, got %q", "ws-001-legacy-2", second.Name)
	}

	if state.SelectedWorkspaceID != "my-workspace-legacy-2" {
		t.Fatalf("expected selected workspace ID %q, got %q", "my-workspace-legacy-2", state.SelectedWorkspaceID)
	}
}

func TestService_EnsureLocalWorkspace_ReusesExistingSystemWorkspace(t *testing.T) {
	mem := &memoryStore{
		state: State{
			Workspaces: []Workspace{
				{
					ID:   LocalWorkspaceID,
					Name: LocalWorkspaceName,
					Repos: []Repo{
						{
							ID:   "repo-1",
							Name: "local",
							Path: "/tmp/local",
						},
					},
				},
			},
		},
	}
	svc := NewService(mem)

	ws, err := svc.EnsureLocalWorkspace()
	if err != nil {
		t.Fatalf("EnsureLocalWorkspace() error = %v", err)
	}
	if len(ws.Repos) != 1 {
		t.Fatalf("expected existing local repo to be preserved, got %d", len(ws.Repos))
	}

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if len(state.Workspaces) != 1 {
		t.Fatalf("expected local workspace to remain single, got %d", len(state.Workspaces))
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
