# Workspace GitOps Release TUI v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Bubble Tea TUI that supports multi-workspace repo management, worktree flows, embedded lazygit, delta-based diff view, and selected-repo PR/CI/Release tracking via `gh auth`.

**Architecture:** Use a modular core (`AppShell`, `StateStore`, `SyncEngine`) with adapters for worktree, lazygit PTY embedding, delta diff rendering, and GitHub (`gh` CLI). Keep UI state and domain state isolated behind interfaces so polling, rendering, and shell integrations can evolve independently. Scope v1 to selected-repo synchronization and explicit non-goals from the approved spec.

**Tech Stack:** Go 1.24+, Bubble Tea, Bubbles, Lip Gloss, `creack/pty`, `go.etcd.io/bbolt`, `gh` CLI, `lazygit`, `delta`, Go `testing`.

---

## Scope Check

Spec is broad but still one coherent product slice (single TUI with one state model). This plan keeps all subsystems in one implementation stream while hard-limiting v1 boundaries:

- PR creation path: only via embedded lazygit/custom `gh` command
- Polling scope: selected repo only
- Diff scope: read-only `git diff | delta`
- Publish scope: configured GitHub Release workflow only

## Planned File Structure

- Create: `go.mod` (module and dependencies)
- Create: `cmd/tui/main.go` (app entrypoint)
- Create: `internal/app/model.go` (top-level app model and bootstrap)
- Create: `internal/app/update.go` (event routing and key handling)
- Create: `internal/app/view.go` (3-pane rendering and tab rendering)
- Create: `internal/app/keymap.go` (centralized key bindings)
- Create: `internal/app/messages.go` (Tea messages/commands contracts)
- Create: `internal/domain/workspace/types.go` (workspace/repo/status types)
- Create: `internal/domain/workspace/service.go` (workspace/repo selection logic)
- Create: `internal/store/store.go` (store interface)
- Create: `internal/store/boltdb/store.go` (single-file persistent store)
- Create: `internal/store/boltdb/migrations.go` (bucket setup/versioning)
- Create: `internal/adapters/worktree/adapter.go` (git worktree operations)
- Create: `internal/adapters/worktree/adapter_test.go` (command mapping tests)
- Create: `internal/adapters/lazygit/session.go` (PTY-backed lazygit sessions)
- Create: `internal/adapters/lazygit/session_test.go` (session lifecycle tests)
- Create: `internal/adapters/diff/delta.go` (delta diff adapter)
- Create: `internal/adapters/diff/delta_test.go` (delta behavior tests)
- Create: `internal/adapters/github/ghcli.go` (`gh api` calls and parsing)
- Create: `internal/adapters/github/ghcli_test.go` (API parsing and mapping tests)
- Create: `internal/sync/engine.go` (manual refresh + polling scheduler)
- Create: `internal/sync/engine_test.go` (selected-repo polling tests)
- Create: `internal/testutil/fakeexec/fakeexec.go` (shared fake command runner)
- Create: `internal/testutil/faketime/faketime.go` (ticker/time control)
- Create: `test/smoke/smoke_test.go` (non-interactive smoke flow)
- Create: `docs/superpowers/runbooks/v1-local-setup.md` (runtime prerequisites and keymap)

## Task 1: Bootstrap Go Module and Minimal Bubble Tea Shell

**Files:**
- Create: `go.mod`
- Create: `cmd/tui/main.go`
- Create: `internal/app/model.go`
- Test: `internal/app/model_test.go`

- [ ] **Step 1: Write failing test for initial app state**

```go
func TestNewModel_InitialLayout(t *testing.T) {
	m := NewModel(Config{})
	if m.ActiveTab != TabOverview {
		t.Fatalf("expected default tab %q, got %q", TabOverview, m.ActiveTab)
	}
	if m.LeftPaneWidth <= 0 || m.CenterPaneWidth <= 0 || m.RightPaneWidth <= 0 {
		t.Fatalf("expected pane widths to be initialized")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app -run TestNewModel_InitialLayout -v`  
Expected: FAIL with `undefined: NewModel` (or missing fields).

- [ ] **Step 3: Write minimal model and entrypoint**

```go
type Model struct {
	ActiveTab        Tab
	LeftPaneWidth    int
	CenterPaneWidth  int
	RightPaneWidth   int
}

func NewModel(_ Config) Model {
	return Model{
		ActiveTab:       TabOverview,
		LeftPaneWidth:   30,
		CenterPaneWidth: 80,
		RightPaneWidth:  40,
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app -run TestNewModel_InitialLayout -v`  
Expected: PASS.

- [ ] **Step 5: Verify module compiles**

Run: `go test ./...`  
Expected: PASS (or only known TODO skips).

- [ ] **Step 6: Commit**

```bash
git add go.mod cmd/tui/main.go internal/app/model.go internal/app/model_test.go
git commit -m "chore: bootstrap bubble tea shell and initial model"
```

## Task 2: Implement Workspace Domain and Persistent Store (bbolt)

**Files:**
- Create: `internal/domain/workspace/types.go`
- Create: `internal/domain/workspace/service.go`
- Create: `internal/store/store.go`
- Create: `internal/store/boltdb/store.go`
- Create: `internal/store/boltdb/migrations.go`
- Test: `internal/store/boltdb/store_test.go`
- Test: `internal/domain/workspace/service_test.go`

- [ ] **Step 1: Write failing tests for workspace CRUD and selected repo persistence**

```go
func TestService_CreateWorkspaceAndSelectRepo(t *testing.T) {
	svc := NewService(newInMemoryStore())
	ws, err := svc.CreateWorkspace("default")
	require.NoError(t, err)
	_, err = svc.AddRepo(ws.ID, RepoInput{Name: "api", Path: "/tmp/api"})
	require.NoError(t, err)
	require.NoError(t, svc.SelectRepo(ws.ID, "api"))
	state, err := svc.LoadState()
	require.NoError(t, err)
	require.Equal(t, "api", state.Workspaces[0].SelectedRepoID)
}
```

- [ ] **Step 2: Run tests and confirm failure**

Run: `go test ./internal/domain/workspace ./internal/store/boltdb -v`  
Expected: FAIL due to missing types/interfaces.

- [ ] **Step 3: Add domain types and store interface**

```go
type Repo struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Path               string `json:"path"`
	DefaultBranch      string `json:"default_branch"`
	ReleaseWorkflowRef string `json:"release_workflow_ref"`
}

type Store interface {
	Load(context.Context) (State, error)
	Save(context.Context, State) error
}
```

- [ ] **Step 4: Implement bbolt store with single `state` payload bucket**

```go
func (s *Store) Save(ctx context.Context, st workspace.State) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketState)
		raw, err := json.Marshal(st)
		if err != nil { return err }
		return b.Put([]byte("current"), raw)
	})
}
```

- [ ] **Step 5: Re-run tests and full suite**

Run: `go test ./internal/domain/workspace ./internal/store/boltdb -v && go test ./...`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/workspace internal/store
git commit -m "feat: add workspace domain service and boltdb persistence"
```

## Task 3: Left Pane UX for Multi-Workspace and Manual Repo Add

**Files:**
- Modify: `internal/app/model.go`
- Create: `internal/app/keymap.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/view.go`
- Modify: `internal/domain/workspace/service.go`
- Test: `internal/app/update_workspace_test.go`
- Test: `internal/app/view_leftpane_test.go`
- Test: `internal/domain/workspace/service_test.go`

- [ ] **Step 1: Write failing update test for workspace switch and repo select events**

```go
func TestUpdate_SelectRepo_ChangesCurrentRepo(t *testing.T) {
	m := seededModelWithRepos()
	updated, _ := m.Update(MsgSelectRepo{RepoID: "repo-2"})
	got := updated.(Model)
	require.Equal(t, "repo-2", got.State.CurrentRepoID())
}
```

- [ ] **Step 2: Run targeted tests to capture failure**

Run: `go test ./internal/app -run "TestUpdate_SelectRepo_ChangesCurrentRepo|TestView_LeftPaneRendersWorkspaceAndRepo" -v`  
Expected: FAIL with missing messages/rendering.

- [ ] **Step 3: Implement keymap + update handlers**

```go
type KeyMap struct {
	NextWorkspace key.Binding
	PrevWorkspace key.Binding
	AddRepo       key.Binding
	SelectRepo    key.Binding
}
```

- [ ] **Step 4: Implement left pane render including manual add hint**

```go
func (m Model) renderLeftPane() string {
	// show workspaces, repos, selected marker, and "a: add repo path"
}
```

- [ ] **Step 5: Add invalid repo path handling (mark invalid + fix/remove actions)**

```go
type RepoHealth string

const (
	RepoHealthy RepoHealth = "healthy"
	RepoInvalid RepoHealth = "invalid"
)
```

Add service operations:

- `MarkRepoInvalid(workspaceID, repoID)`
- `UpdateRepoPath(workspaceID, repoID, newPath)`
- `RemoveRepo(workspaceID, repoID)`

Render invalid badge in left pane and actionable hint keys.

- [ ] **Step 6: Re-run app/domain tests**

Run: `go test ./internal/app -v`  
Run: `go test ./internal/domain/workspace -v`  
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/model.go internal/app/keymap.go internal/app/update.go internal/app/view.go internal/app/*_test.go internal/domain/workspace/service.go internal/domain/workspace/service_test.go
git commit -m "feat: add workspace/repo left pane flow with invalid-path recovery"
```

## Task 4: Worktree Adapter and Worktrees Tab

**Files:**
- Create: `internal/adapters/worktree/adapter.go`
- Test: `internal/adapters/worktree/adapter_test.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/view.go`
- Test: `internal/app/worktrees_tab_test.go`

- [ ] **Step 1: Write failing adapter test for command mapping**

```go
func TestAdapter_CreateWorktree_UsesGitWorktreeAdd(t *testing.T) {
	runner := &fakeexec.Runner{}
	a := NewAdapter(runner)
	_ = a.Create(context.Background(), "/repo", "feature/a", "../repo-feature-a")
	require.Equal(t, []string{"git","-C","/repo","worktree","add","../repo-feature-a","feature/a"}, runner.LastArgs())
}

func TestAdapter_ListAndSwitchWorktree(t *testing.T) {
	runner := &fakeexec.Runner{}
	a := NewAdapter(runner)
	_, _ = a.List(context.Background(), "/repo")
	_ = a.ValidateSwitchTarget(context.Background(), "../repo-feature-a")
	require.Equal(t, []string{"git","-C","/repo","worktree","list","--porcelain"}, runner.Commands[0])
	require.Equal(t, []string{"git","-C","../repo-feature-a","rev-parse","--is-inside-work-tree"}, runner.Commands[1])
}
```

- [ ] **Step 2: Run adapter test and confirm failure**

Run: `go test ./internal/adapters/worktree -v`  
Expected: FAIL due to missing adapter.

- [ ] **Step 3: Implement worktree adapter (create/list + switch target validation) with injected command runner**

```go
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}
```

- [ ] **Step 4: Add Worktrees tab actions (create/list/switch + selectedWorktree persistence)**

```go
case MsgCreateWorktree:
	cmd := m.worktreeAdapter.Create(ctx, repo.Path, msg.Branch, msg.Path)
case MsgSwitchWorktree:
	cmd := m.worktreeAdapter.ValidateSwitchTarget(ctx, msg.WorktreePath)
	m.state.SetSelectedWorktree(repo.ID, msg.WorktreePath)
	_ = m.store.Save(ctx, m.state)
```

Switch semantics for v1:

- only switch to an existing worktree discovered from `worktree list`
- no `git worktree add` call when switching
- switching updates app active context (`selectedWorktreeId/path`)

- [ ] **Step 5: Run tests for adapter + app tab**

Run: `go test ./internal/adapters/worktree ./internal/app -run Worktree -v`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/worktree internal/app/update.go internal/app/view.go internal/app/worktrees_tab_test.go
git commit -m "feat: add worktree adapter and worktrees tab actions"
```

## Task 5: Embed Lazygit in Center Tab (No Popup)

**Files:**
- Create: `internal/adapters/lazygit/session.go`
- Test: `internal/adapters/lazygit/session_test.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/view.go`
- Modify: `internal/app/messages.go`
- Test: `internal/app/lazygit_tab_test.go`

- [x] **Step 1: Write failing tests for no-repo and spawn behavior**

```go
func TestLazygitTab_NoRepo_ShowsPrompt(t *testing.T) {
	m := modelWithoutSelectedRepo()
	m.ActiveTab = TabLazygit
	view := m.View()
	require.Contains(t, view, "請先選擇 repo")
}

func TestLazygitTab_WithRepo_StartsSession(t *testing.T) {
	adapter := newFakeLazygitAdapter()
	m := modelWithRepoAndAdapter(adapter)
	_, _ = m.Update(MsgSwitchTab{Tab: TabLazygit})
	require.Equal(t, 1, adapter.StartCalls)
}

func TestLazygitTab_ForwardsInputToPTY(t *testing.T) {
	adapter := newFakeLazygitAdapter()
	m := modelWithRepoAndAdapter(adapter)
	m.ActiveTab = TabLazygit
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	require.Equal(t, "j", adapter.LastInput)
}
```

- [x] **Step 2: Run tests and verify failure**

Run: `go test ./internal/app ./internal/adapters/lazygit -run Lazygit -v`  
Expected: FAIL with missing session manager.

- [x] **Step 3: Implement PTY-backed session manager with I/O bridge**

```go
func (m *Manager) Start(ctx context.Context, repoPath string) (SessionID, error) {
	cmd := exec.CommandContext(ctx, "lazygit", "-p", repoPath)
	ptmx, err := pty.Start(cmd)
	// retain session handle by repoPath and start goroutine to read VT output frames
}

func (m *Manager) WriteInput(id SessionID, b []byte) error {
	_, err := m.sessions[id].PTY.Write(b)
	return err
}
```

- [x] **Step 4: Wire tab switch + PTY output rendering + key forwarding**

Implement:

- `MsgLazygitFrame` emitted by session reader goroutine
- Center pane lazygit renderer consuming latest frame buffer
- Key forwarding to PTY only when active tab is `TabLazygit`

- [x] **Step 5: Run tests**

Run: `go test ./internal/adapters/lazygit ./internal/app -run Lazygit -v`  
Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add internal/adapters/lazygit internal/app/update.go internal/app/view.go internal/app/lazygit_tab_test.go
git commit -m "feat: embed lazygit tab with pty session lifecycle"
```

## Task 6: Add Read-Only Diff Tab Backed by Delta

**Files:**
- Create: `internal/adapters/diff/delta.go`
- Test: `internal/adapters/diff/delta_test.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/view.go`
- Test: `internal/app/diff_tab_test.go`

- [ ] **Step 1: Write failing adapter tests for `git diff | delta` behavior**

```go
func TestDeltaRenderer_RenderRepoDiff(t *testing.T) {
	r := NewRenderer(fakeexec.New())
	out, err := r.Render(context.Background(), "/repo")
	require.NoError(t, err)
	require.Contains(t, out, "@@")
}
```

- [ ] **Step 2: Run tests and confirm failure**

Run: `go test ./internal/adapters/diff -v`  
Expected: FAIL due to missing renderer.

- [ ] **Step 3: Implement read-only renderer with explicit command chain**

```go
gitDiff := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", "--no-ext-diff")
delta := exec.CommandContext(ctx, "delta", "--paging=never")
```

- [ ] **Step 4: Add Diff tab display + error message when delta missing**

Show: `"delta not found; install delta to use Diff tab"` when executable lookup fails.

- [ ] **Step 5: Run diff tests and app tests**

Run: `go test ./internal/adapters/diff ./internal/app -run Diff -v`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/diff internal/app/update.go internal/app/view.go internal/app/diff_tab_test.go
git commit -m "feat: add read-only diff tab rendered by delta"
```

## Task 7: Implement GitHub Adapter via `gh` CLI (PR/CI/Release)

**Files:**
- Create: `internal/adapters/github/ghcli.go`
- Test: `internal/adapters/github/ghcli_test.go`
- Modify: `internal/domain/workspace/types.go`
- Test: `internal/domain/workspace/types_test.go`

- [ ] **Step 1: Write failing tests for status mapping and release workflow selection**

```go
func TestAdapter_ReleaseStatus_UsesConfiguredWorkflowRef(t *testing.T) {
	gh := newFakeGH().
		WithWorkflowRuns("release.yml", []Run{{Conclusion: "success"}})
	a := NewAdapter(gh)
	st, err := a.FetchRepoStatus(ctx, Repo{
		Name: "svc",
		DefaultBranch: "main",
		ReleaseWorkflowRef: "release.yml",
	})
	require.NoError(t, err)
	require.Equal(t, StatusSuccess, st.Release)
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/adapters/github -v`  
Expected: FAIL due to missing adapter/types.

- [ ] **Step 3: Implement `gh auth` preflight and API calls**

```go
func (a *Adapter) CheckAuth(ctx context.Context) error {
	_, err := a.exec.Run(ctx, "gh", "auth", "status", "--hostname", "github.com")
	return err
}
```

- [ ] **Step 4: Implement status mapping rules**

Map workflow conclusions:

- `success` -> `StatusSuccess`
- `failure` -> `StatusFailure`
- `cancelled|skipped` -> `StatusNeutral`
- running jobs -> `StatusInProgress`
- no `ReleaseWorkflowRef` -> `StatusUnconfigured`

- [ ] **Step 5: Run tests**

Run: `go test ./internal/adapters/github ./internal/domain/workspace -v`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/github internal/domain/workspace
git commit -m "feat: add gh-based pr ci release status adapter"
```

## Task 8: Build Sync Engine (Manual Refresh + Selected-Repo Polling)

**Files:**
- Create: `internal/sync/engine.go`
- Test: `internal/sync/engine_test.go`
- Modify: `internal/app/messages.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/model.go`
- Test: `internal/app/sync_integration_test.go`

- [ ] **Step 1: Write failing tests for polling scope + startup + repo-switch lifecycle**

```go
func TestEngine_AutoPoll_OnlySelectedRepo(t *testing.T) {
	engine := NewEngine(fakeFetcher, WithInterval(time.Minute))
	engine.SetSelection("workspace-1", "repo-2")
	engine.Tick()
	require.Equal(t, []string{"repo-2"}, fakeFetcher.RequestedRepoIDs())
}

func TestEngine_OnRepoSwitch_TriggersImmediateRefresh(t *testing.T) {
	engine := NewEngine(fakeFetcher, WithInterval(time.Minute))
	engine.SetSelection("workspace-1", "repo-2")
	_ = engine.OnSelectionChanged(context.Background(), "workspace-1", "repo-3")
	require.Equal(t, "repo-3", fakeFetcher.LastRequestedRepoID())
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/sync -v`  
Expected: FAIL due to missing engine.

- [ ] **Step 3: Implement scheduler + startup restore trigger + manual trigger**

```go
func (e *Engine) RefreshNow(ctx context.Context) (workspace.RepoStatus, error) { ... }
func (e *Engine) OnTick(ctx context.Context) (workspace.RepoStatus, error) { ... }
func (e *Engine) OnSelectionChanged(ctx context.Context, wsID, repoID string) (workspace.RepoStatus, error) { ... }
func (e *Engine) Start(ctx context.Context) tea.Cmd { ... }
```

- [ ] **Step 4: Wire key bindings and lifecycle hooks**

- `r` -> manual refresh for selected repo
- `p` -> toggle auto polling on/off
- app startup -> restore last workspace/repo, then trigger immediate refresh
- repo switch -> call `OnSelectionChanged` for immediate refresh

- [ ] **Step 5: Run tests**

Run: `go test ./internal/sync ./internal/app -run Sync -v`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sync internal/app/messages.go internal/app/update.go internal/app/sync_integration_test.go
git commit -m "feat: add selected-repo sync engine with manual and auto refresh"
```

## Task 9: Right Pane Status Rendering + Stale/Error/Unconfigured States

**Files:**
- Modify: `internal/app/view.go`
- Modify: `internal/app/model.go`
- Test: `internal/app/view_rightpane_test.go`

- [ ] **Step 1: Write failing view tests for all right-pane states**

```go
func TestRightPane_RendersUnconfiguredRelease(t *testing.T) {
	m := modelWithStatus(StatusSnapshot{Release: StatusUnconfigured})
	out := m.View()
	require.Contains(t, out, "release: unconfigured")
}
```

- [ ] **Step 2: Run test and confirm failure**

Run: `go test ./internal/app -run RightPane -v`  
Expected: FAIL due to missing rendering branches.

- [ ] **Step 3: Implement right-pane status cards**

Render fields:

- PR status
- CI status
- Release status
- `lastSyncedAt`
- stale badge
- latest error text (if any)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app -run RightPane -v`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/view.go internal/app/model.go internal/app/view_rightpane_test.go
git commit -m "feat: render right pane status cards with stale and error states"
```

## Task 10: Verification, Smoke Test, and Runbook

**Files:**
- Create: `test/smoke/smoke_test.go`
- Create: `docs/superpowers/runbooks/v1-local-setup.md`
- Modify: `README.md` (if exists; otherwise create minimal usage section)

- [ ] **Step 1: Write smoke test for startup restore + initial sync + repo-switch immediate sync**

```go
func TestSmoke_BootstrapAndPreflight(t *testing.T) {
	t.Setenv("WORKSPACE_TUI_TEST_MODE", "1")
	err := RunSmoke() // restore state, run initial sync, switch repo, verify immediate sync
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run smoke test and confirm behavior**

Run: `go test ./test/smoke -v`  
Expected: PASS in test mode (without requiring live GitHub).

- [ ] **Step 3: Write runbook with prerequisites and keymap**

Include:

- required binaries: `gh`, `lazygit`, `delta`, `git`
- `gh auth login` requirement
- repo `releaseWorkflowRef` setup guidance
- refresh key `r`, polling toggle key `p`

- [ ] **Step 4: Run full verification before completion**

Run: `go test ./...`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add test/smoke docs/superpowers/runbooks/v1-local-setup.md README.md
git commit -m "test: add smoke coverage and v1 runtime runbook"
```

## Requirement Traceability

- Multi-workspace + manual repo add: Task 2, Task 3
- Worktree create/list/switch + selected worktree persistence: Task 4
- Center tab embedded lazygit (operable I/O, no popup): Task 5
- Diff tab read-only via delta: Task 6
- `gh auth` + PR/CI/Release mapping + `releaseWorkflowRef`: Task 7
- Manual refresh + auto polling (selected repo only): Task 8
- Startup restore + immediate sync on repo switch: Task 8, Task 10
- Right pane selected-repo PR/CI/Release with stale/error/unconfigured: Task 9

## Execution Notes

- Use `@superpowers/test-driven-development` behavior in every task: fail test -> minimal code -> pass.
- Use `@superpowers/verification-before-completion` before any merge/PR claims.
- Keep commits task-scoped; do not batch unrelated changes.
- Do not expand beyond v1 non-goals during execution.
