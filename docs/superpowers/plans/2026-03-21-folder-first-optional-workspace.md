# Folder-First Optional Workspace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement folder-first startup (`gh-workspace` / `-f`) with optional named workspace mode (`-w`) while preserving current workspace data model and selected-repo status features.

**Architecture:** Keep one persistent state model (`workspace.State`) and add a system-reserved local workspace (`__local_internal__`) for Folder Mode. Add a thin CLI launch-option parser plus runtime bootstrap that resolves folder/workspace intent before the Bubble Tea model starts. In app rendering/update logic, add explicit UI mode gates so Folder Mode hides workspace controls and enforces single-repo local behavior.

**Tech Stack:** Go 1.24+, Bubble Tea/Bubbles/Lip Gloss, bbolt, existing workspace domain service, Go `testing`.

---

## Scope Check

This spec is one coherent subsystem (startup + state bootstrap + mode-aware UI behavior). No additional project split needed.

## Execution Rules

- Use `@superpowers/test-driven-development` for every task step pair (red -> green -> refactor).
- Use `@superpowers/verification-before-completion` before every “task complete” and final completion claim.
- Prefer `@superpowers/subagent-driven-development` during execution (or `@superpowers/executing-plans` if running inline).

## Planned File Structure

- Create: `cmd/tui/launch_options.go` (CLI option parsing and exit-code-safe launch intent)
- Create: `cmd/tui/launch_options_test.go` (CLI parse and validation tests)
- Modify: `cmd/tui/main.go` (wire parser, pass launch options into runtime)
- Modify: `cmd/tui/main_test.go` (run-level CLI error text and exit behavior tests)
- Modify: `cmd/tui/runtime.go` (apply launch intent before model creation)
- Modify: `cmd/tui/runtime_test.go` (bootstrap behavior tests for folder/workspace startup)
- Create: `internal/adapters/repository/resolver.go` (resolve git repo root from any path)
- Create: `internal/adapters/repository/resolver_test.go` (repo-root and non-git behavior tests)
- Create: `internal/domain/workspace/local_workspace.go` (reserved local workspace constants + helpers)
- Modify: `internal/domain/workspace/service.go` (ensure local workspace, replace/clear local repo, workspace-name lookup rules)
- Modify: `internal/domain/workspace/service_test.go` (local workspace and collision rules)
- Modify: `internal/app/model.go` (add UI mode state and config)
- Modify: `internal/app/messages.go` (add add-repo-path submission message)
- Modify: `internal/app/update.go` (mode-aware key behavior, folder-mode add-path flow)
- Modify: `internal/app/view.go` (hide workspaces in Folder Mode, empty-state messaging)
- Create: `internal/app/add_repo_input.go` (folder-mode add-repo text-input state machine)
- Modify: `internal/app/keymap.go` (mode-aware workspace navigation behavior unchanged in keymap, gated in update)
- Modify: `internal/app/update_workspace_test.go` (folder/workspace mode key behavior tests)
- Modify: `internal/app/view_leftpane_test.go` (folder mode rendering expectations)
- Modify: `README.md` (new CLI contract and mode behavior)
- Modify: `docs/superpowers/runbooks/v1-local-setup.md` (new invocation examples)

## Task 1: Add Launch Option Parser and CLI Contract

**Files:**
- Create: `cmd/tui/launch_options.go`
- Test: `cmd/tui/launch_options_test.go`
- Modify: `cmd/tui/main.go`

- [ ] **Step 1: Write failing tests for `gh-workspace`, `-f`, `-w`, and invalid flag combos**

```go
func TestParseLaunchOptions_DefaultUsesPWD(t *testing.T) {
	opts, err := ParseLaunchOptions([]string{}, "/tmp/current")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if opts.Mode != LaunchFolder || opts.Path != "/tmp/current" { t.Fatalf("unexpected opts: %#v", opts) }
}
```

Add explicit empty-value tests:

```go
func TestParseLaunchOptions_RejectsEmptyFolderFlagValue(t *testing.T) {
\t_, err := ParseLaunchOptions([]string{\"-f\", \"\"}, \"/tmp/current\")
\tif err == nil || !strings.Contains(err.Error(), \"-f cannot be empty\") {
\t\tt.Fatalf(\"unexpected err: %v\", err)
\t}
}
```

Mirror for `-w \"\"`.

- [ ] **Step 2: Run parser tests and verify failure**

Run: `go test ./cmd/tui -run "TestParseLaunchOptions" -v`
Expected: FAIL with `undefined: ParseLaunchOptions`.

- [ ] **Step 3: Implement parser with strict validation and reserved-name protection**

```go
type LaunchMode string
const (
	LaunchFolder LaunchMode = "folder"
	LaunchWorkspace LaunchMode = "workspace"
)

type LaunchOptions struct {
	Mode LaunchMode
	Path string
	WorkspaceName string
}
```

Rules:
- no args => folder + cwd
- `-f <path>` => folder
- `-w <name>` => workspace
- `-f` + `-w` => error
- `-f \"\"` => error
- `-w \"\"` => error
- `-w __local_internal__` => error

- [ ] **Step 4: Extract `run(args []string) error` in `main.go` and route through parser**

```go
func run(args []string) error {
	opts, err := ParseLaunchOptions(args, mustGetwd())
	if err != nil { return err }
	model, closeFn, err := composeRuntimeModel(context.Background(), opts)
	// existing tea startup
}
```

- [ ] **Step 5: Add run-level tests for exact CLI error text and non-zero exit cases**

```go
func TestRun_InvalidFlagCombo_ReturnsUsageError(t *testing.T) {
	err := runWithIO([]string{\"-f\", \"/tmp/repo\", \"-w\", \"team-a\"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), \"cannot use -f with -w\") {
		t.Fatalf(\"unexpected error: %v\", err)
	}
}
```

Also add tests for:
- `-w does-not-exist` prints/returns `workspace not found: does-not-exist`
- reserved workspace name returns explicit error
- invalid argument combinations print usage text and return non-zero

- [ ] **Step 6: Re-run parser and package tests**

Run: `go test ./cmd/tui -run "TestParseLaunchOptions" -v && go test ./cmd/tui -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/tui/launch_options.go cmd/tui/launch_options_test.go cmd/tui/main.go cmd/tui/main_test.go
git commit -m "feat: add folder/workspace launch option parsing"
```

## Task 2: Implement Git Repo Root Resolver Adapter

**Files:**
- Create: `internal/adapters/repository/resolver.go`
- Create: `internal/adapters/repository/resolver_test.go`

- [ ] **Step 1: Write failing tests for subdir -> repo root, non-git, and missing path**

```go
func TestResolver_ResolveRepoRoot_SubdirReturnsTopLevel(t *testing.T) {
	root := initTempGitRepo(t)
	subdir := filepath.Join(root, "nested")
	require.NoError(t, os.MkdirAll(subdir, 0o755))
	resolved, ok, err := ResolveRepoRoot(context.Background(), subdir)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, root, resolved)
}
```

- [ ] **Step 2: Run resolver tests and confirm red**

Run: `go test ./internal/adapters/repository -v`
Expected: FAIL (missing resolver implementation).

- [ ] **Step 3: Implement resolver using `git -C <path> rev-parse --show-toplevel`**

```go
func ResolveRepoRoot(ctx context.Context, path string) (string, bool, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil { return "", false, nil }
	cmd := exec.CommandContext(ctx, "git", "-C", abs, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil { return "", false, nil }
	return strings.TrimSpace(string(out)), true, nil
}
```

- [ ] **Step 4: Re-run adapter tests**

Run: `go test ./internal/adapters/repository -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/repository/resolver.go internal/adapters/repository/resolver_test.go
git commit -m "feat: add git repo root resolver adapter"
```

## Task 3: Extend Workspace Service for System Local Workspace

**Files:**
- Create: `internal/domain/workspace/local_workspace.go`
- Modify: `internal/domain/workspace/service.go`
- Modify: `internal/domain/workspace/service_test.go`

- [ ] **Step 1: Write failing service tests for local workspace creation, single-repo replace, and clear behavior**

```go
func TestService_EnsureLocalWorkspace_FirstRunCreatesSystemWorkspace(t *testing.T) {
	svc := NewService(newInMemoryStore())
	ws, err := svc.EnsureLocalWorkspace()
	require.NoError(t, err)
	require.Equal(t, LocalWorkspaceID, ws.ID)
	require.Equal(t, LocalWorkspaceName, ws.Name)
}
```

- [ ] **Step 2: Run service tests and verify failure**

Run: `go test ./internal/domain/workspace -run "LocalWorkspace|EnsureLocal" -v`
Expected: FAIL (undefined methods/constants).

- [ ] **Step 3: Add constants and service operations**

```go
const (
	LocalWorkspaceID = "__local_internal__"
	LocalWorkspaceName = "__local_internal__"
)

func (s *Service) EnsureLocalWorkspace() (Workspace, error)
func (s *Service) ReplaceLocalRepo(input RepoInput) (Repo, error)
func (s *Service) ClearLocalRepos() error
func (s *Service) FindWorkspaceByName(name string, includeSystem bool) (Workspace, bool, error)
```

Behavior:
- local workspace exists exactly once
- `ReplaceLocalRepo` wipes existing local repos then inserts one repo
- `ClearLocalRepos` leaves workspace but empties repos and selected repo
- name lookup for `-w` excludes system workspace

- [ ] **Step 4: Add ID/name collision migration helper for legacy `__local_internal__`**

```go
func (s *Service) EnsureLocalWorkspaceIntegrity() error
```

At startup:
- if user workspace collides by ID or name, rename legacy workspace with deterministic uniqueness:
  - first try `<original>-legacy-1`
  - increment suffix until name+ID are both unique
  - persist migration before local workspace creation/select.

- [ ] **Step 5: Re-run full domain tests**

Run: `go test ./internal/domain/workspace -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/workspace/local_workspace.go internal/domain/workspace/service.go internal/domain/workspace/service_test.go
git commit -m "feat: add system local workspace service operations"
```

## Task 4: Apply Launch Intent in Runtime Bootstrap

**Files:**
- Modify: `cmd/tui/runtime.go`
- Modify: `cmd/tui/runtime_test.go`

- [ ] **Step 1: Write failing runtime tests for `open-folder` and `open-workspace` flows**

```go
func TestComposeRuntimeModel_LaunchFolder_NonGitClearsLocalRepo(t *testing.T) {
	opts := LaunchOptions{Mode: LaunchFolder, Path: t.TempDir()}
	model, closeFn, err := composeRuntimeModel(context.Background(), opts)
	require.NoError(t, err)
	defer closeFn()
	repoID := model.State.CurrentRepoID()
	require.Equal(t, "", repoID)
	require.Contains(t, model.StatusMessage, "not a git repo")
}
```

Add mode assertions:

```go
func TestComposeRuntimeModel_LaunchWorkspace_SetsWorkspaceMode(t *testing.T) {
\topts := LaunchOptions{Mode: LaunchWorkspace, WorkspaceName: \"team-a\"}
\tmodel, closeFn, err := composeRuntimeModel(context.Background(), opts)
\trequire.NoError(t, err)
\tdefer closeFn()
\trequire.Equal(t, app.ModeWorkspace, model.UIMode)
}
```

And for folder launch:
- `gh-workspace` / `-f` => `model.UIMode == app.ModeFolder`.

- [ ] **Step 2: Run runtime tests and confirm failure**

Run: `go test ./cmd/tui -run "ComposeRuntimeModel_Launch" -v`
Expected: FAIL (signature/behavior mismatch).

- [ ] **Step 3: Change runtime compose to accept launch options and call workspace service + repo resolver**

```go
func composeRuntimeModel(ctx context.Context, opts LaunchOptions) (app.Model, func() error, error)
```

Folder mode:
- ensure local workspace integrity
- resolve path to repo root (if git)
- git => `ReplaceLocalRepo` + select local workspace/repo
- non-git => `ClearLocalRepos` + select local workspace
- always set `app.Config.InitialUIMode = app.ModeFolder`

Workspace mode:
- find workspace by name excluding system
- not found => return `ErrWorkspaceNotFound`
- found => select workspace
- set `app.Config.InitialUIMode = app.ModeWorkspace`

- [ ] **Step 4: Set deterministic status messages for non-git folder empty state**

```go
const StatusCurrentFolderNotGit = "current folder is not a git repo"
```

- [ ] **Step 5: Re-run runtime tests and package tests**

Run: `go test ./cmd/tui -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/tui/runtime.go cmd/tui/runtime_test.go
git commit -m "feat: apply folder/workspace launch intent at runtime bootstrap"
```

## Task 5: Add Explicit App UI Mode and Folder-Mode Workspace Gating

**Files:**
- Modify: `internal/app/model.go`
- Modify: `internal/app/view.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_workspace_test.go`
- Modify: `internal/app/view_leftpane_test.go`

- [ ] **Step 1: Write failing tests for Folder Mode left-pane visibility and disabled workspace keys**

```go
func TestView_FolderMode_HidesWorkspacesSection(t *testing.T) {
	m := seededModelWithRepos()
	m.UIMode = ModeFolder
	out := m.View()
	if strings.Contains(out, "Workspaces") { t.Fatalf("workspaces should be hidden in folder mode") }
}
```

Add test coverage for Workspace Mode filtering:

```go
func TestView_WorkspaceMode_HidesSystemWorkspaceEntry(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()
	m.UIMode = ModeWorkspace
	out := m.View()
	if strings.Contains(out, "__local_internal__") {
		t.Fatalf("system workspace must not be rendered in workspace list")
	}
}
```

Add empty-state guidance assertion:

```go
func TestView_FolderMode_EmptyState_ShowsGuidance(t *testing.T) {
\tm := NewModel(Config{InitialUIMode: ModeFolder})
\tout := m.View()
\tif !strings.Contains(out, \"current folder is not a git repo\") { t.Fatalf(\"missing non-git hint\") }
\tif !strings.Contains(out, \"a\") { t.Fatalf(\"missing add-repo hint\") }
\tif !strings.Contains(out, \"-w <name>\") { t.Fatalf(\"missing workspace hint\") }
}
```

- [ ] **Step 2: Run targeted app tests and verify failure**

Run: `go test ./internal/app -run "FolderMode|HidesWorkspaces|WorkspaceKeys" -v`
Expected: FAIL (missing mode fields/logic).

- [ ] **Step 3: Add `ModeFolder` / `ModeWorkspace` to app model config and state**

```go
type UIMode string
const (
	ModeWorkspace UIMode = "workspace"
	ModeFolder UIMode = "folder"
)
```

- [ ] **Step 4: Gate rendering and workspace key handlers by mode**

```go
if m.UIMode == ModeFolder {
	// hide workspace section
}

if m.UIMode == ModeFolder && (key.Matches(msg, m.Keys.NextWorkspace) || key.Matches(msg, m.Keys.PrevWorkspace)) {
	return m, nil
}
```

Workspace Mode behavior update:
- workspace list rendering must filter system workspace
- `[` `]` navigation must iterate only non-system workspaces
- if selected workspace is system while in Workspace Mode, auto-select first non-system workspace (or remain empty if none)

- [ ] **Step 5: Re-run app tests**

Run: `go test ./internal/app -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/model.go internal/app/view.go internal/app/update.go internal/app/update_workspace_test.go internal/app/view_leftpane_test.go
git commit -m "feat: add folder mode UI gating for workspace controls"
```

## Task 6: Implement Folder-Mode `a` Path Submission and Single-Repo Enforcement

**Files:**
- Modify: `internal/app/messages.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/view.go`
- Create: `internal/app/add_repo_input.go`
- Modify: `internal/app/update_workspace_test.go`
- Modify: `cmd/tui/runtime.go` (wire path handler dependency)

- [ ] **Step 1: Write failing tests for `MsgSubmitRepoPath` in Folder Mode (valid git path replaces, invalid clears)**

```go
func TestUpdate_FolderMode_SubmitRepoPath_InvalidClearsSelection(t *testing.T) {
	m := seededFolderModeModelWithRepo()
	updated, _ := m.Update(MsgSubmitRepoPath{Path: "/tmp/not-a-repo"})
	got := updated.(Model)
	if got.State.CurrentRepoID() != "" { t.Fatalf("expected local repo to be cleared") }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/app -run "SubmitRepoPath|FolderMode" -v`
Expected: FAIL (message missing).

- [ ] **Step 3: Add submit message and resolver-backed handler in update path**

```go
type MsgSubmitRepoPath struct { Path string }
```

Update behavior in Folder Mode:
- resolve repo root
- valid git => replace local workspace repo with resolved root
- invalid/non-readable => clear local repo
- set status (`added repo: ...` or `current folder is not a git repo`)

- [ ] **Step 4: Implement concrete in-TUI path input flow for `a` in Folder Mode**

```go
case key.Matches(msg, m.Keys.AddRepo):
	m.RepoPathInput = newRepoPathInput()
	m.RepoPathInputActive = true
```

Required behavior:
- while `RepoPathInputActive`, route rune/backspace/left/right keys to input model
- `enter` submits `MsgSubmitRepoPath{Path: input.Value()}` then closes input
- `esc` cancels input without state mutation
- left pane renders prompt line (e.g., `repo path> ...`) in Folder Mode

- [ ] **Step 5: Re-run app tests and targeted runtime tests**

Run: `go test ./internal/app -v && go test ./cmd/tui -run "ComposeRuntimeModel_LaunchFolder" -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/messages.go internal/app/model.go internal/app/update.go internal/app/view.go internal/app/add_repo_input.go internal/app/update_workspace_test.go cmd/tui/runtime.go
git commit -m "feat: enforce folder mode single-repo behavior via submit-path flow"
```

## Task 7: Documentation and Final Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/runbooks/v1-local-setup.md`

- [ ] **Step 1: Update usage docs to new CLI contract and mode semantics**

Required doc updates:
- `gh-workspace` uses current folder
- `-f` opens folder path
- `-w` opens existing workspace only
- folder non-git behavior and status message
- folder mode hides workspace pane and disables `[` `]`

- [ ] **Step 2: Add explicit acceptance command examples**

```bash
gh-workspace
gh-workspace -f ../repo
gh-workspace -w team-a
```

- [ ] **Step 3: Run full verification suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 4: Manual smoke check in terminal**

Run:
```bash
WORKSPACE_TUI_STATE_PATH=/tmp/gh-workspace-smoke.db go run ./cmd/tui -f /tmp/not-a-repo
```
Expected: TUI starts, empty repo state, status includes `current folder is not a git repo`.

Run:
```bash
WORKSPACE_TUI_STATE_PATH=/tmp/gh-workspace-smoke.db go run ./cmd/tui -w does-not-exist
```
Expected: process exits non-zero with `workspace not found`.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/superpowers/runbooks/v1-local-setup.md
git commit -m "docs: document folder-first optional workspace behavior"
```

## Final Completion Gate

- [ ] Run `go test ./...` and confirm all packages pass.
- [ ] Verify no task introduced workspace-file import/export behavior.
- [ ] Verify Folder Mode always holds at most one repo in `__local_internal__`.
- [ ] Verify `-w __local_internal__` is rejected.
- [ ] Verify spec alignment with: `/Users/allenneverland/Repositories/gh-workspace/docs/superpowers/specs/2026-03-21-folder-first-optional-workspace-design.md`.
