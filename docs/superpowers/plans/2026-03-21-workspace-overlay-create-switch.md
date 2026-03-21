# Workspace Overlay Create/Switch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `w`-triggered Workspace Overlay that supports switching existing workspaces and draft-based creation of a new workspace with fzf-like repo picking.

**Architecture:** Keep `workspace.State` as the only persisted model and add an app-level overlay sub-state for ephemeral create drafts. Implement scanning/commit as runtime-injected interfaces so app update logic stays testable with fakes. Split overlay rendering/state helpers into focused files to avoid inflating `update.go`/`view.go`.

**Tech Stack:** Go 1.24+, Bubble Tea/Bubbles/Lip Gloss, existing workspace service + BoltDB store, Go `testing`.

---

## Scope Check

This is one coherent subsystem (overlay UX + runtime wiring for scan/save). No split into separate plans is needed.

## Execution Rules

- Run this plan in a dedicated git worktree (`@superpowers/using-git-worktrees`) before code changes.
- Use `@superpowers/test-driven-development` for each behavior change (red -> green -> refactor).
- Use `@superpowers/verification-before-completion` before claiming any task is done.
- Keep commits small and behavior-focused (one task or sub-task per commit).

## Planned File Structure

- Create: `internal/app/workspace_overlay.go` (overlay state, modes, focus, draft reset/open helpers)
- Create: `internal/app/workspace_overlay_update_test.go` (overlay key flow and state-machine tests)
- Create: `internal/app/workspace_overlay_scan_test.go` (scan debounce/revision and candidate-add tests)
- Create: `internal/app/view_overlay_test.go` (overlay rendering contract tests)
- Create: `internal/app/view_overlay.go` (overlay renderer)
- Create: `internal/app/fuzzy_match.go` (fzf-like lightweight scoring/filter helper)
- Create: `internal/adapters/repository/discover.go` (recursive git repo discovery from a root path)
- Create: `internal/adapters/repository/discover_test.go` (repo discovery behavior tests)
- Create: `cmd/tui/workspace_overlay_runtime.go` (runtime scanner + draft committer implementations)
- Create: `cmd/tui/workspace_overlay_runtime_test.go` (runtime scanner/committer tests)
- Modify: `internal/app/model.go` (overlay config/dependencies and model fields)
- Modify: `internal/app/messages.go` (overlay scan/save async messages)
- Modify: `internal/app/keymap.go` (add `w`, `c`, `s` bindings)
- Modify: `internal/app/update.go` (overlay-first key handling, scan/save orchestration)
- Modify: `internal/app/view.go` (overlay mount hook and status integration)
- Modify: `cmd/tui/runtime.go` (inject runtime scanner/committer + default scan path from cwd)
- Modify: `README.md` (document `w` overlay and create flow)
- Modify: `docs/superpowers/runbooks/v1-local-setup.md` (operator-level overlay usage notes)

## Task 1: Add Overlay State Machine Skeleton (`w`/`c`/`esc`)

**Files:**
- Create: `internal/app/workspace_overlay.go`
- Create: `internal/app/workspace_overlay_update_test.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/messages.go`
- Modify: `internal/app/keymap.go`
- Modify: `internal/app/update.go`

- [ ] **Step 1: Write failing tests for open/close/create transitions**

```go
func TestOverlay_KeyW_OpensSwitchMode(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model)
	if !got.Overlay.Active || got.Overlay.Mode != OverlayModeSwitch {
		t.Fatalf("expected active switch overlay, got %#v", got.Overlay)
	}
}
```

Also add:
- `esc` closes overlay
- `c` from switch mode enters create mode
- `esc` in create mode discards draft fields

- [ ] **Step 2: Run targeted app tests to verify red**

Run: `go test ./internal/app -run "TestOverlay_" -v`  
Expected: FAIL with undefined overlay fields/types/bindings.

- [ ] **Step 3: Add overlay state types and constructor/reset helpers**

```go
type OverlayMode string
const (
	OverlayModeSwitch OverlayMode = "switch"
	OverlayModeCreate OverlayMode = "create"
)

type WorkspaceOverlayState struct {
	Active bool
	Mode   OverlayMode
	// focus + input + draft fields
}
```

Include helpers:
- `openWorkspaceOverlay(current WorkspaceState, defaultScanPath string)`
- `resetWorkspaceOverlay(defaultScanPath string)`
- `enterCreateMode()`

- [ ] **Step 4: Add key bindings and wire overlay-first key routing**

Add bindings in `DefaultKeyMap()`:
- `WorkspaceOverlay` => `w`
- `OverlayCreate` => `c`
- `OverlaySave` => `s`

In `update.go`, when overlay is active, process overlay keys before repo-path/diff/lazygit handlers.

- [ ] **Step 5: Re-run targeted tests and ensure green**

Run: `go test ./internal/app -run "TestOverlay_" -v`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/workspace_overlay.go internal/app/workspace_overlay_update_test.go internal/app/model.go internal/app/messages.go internal/app/keymap.go internal/app/update.go
git commit -m "feat: add workspace overlay state machine skeleton"
```

## Task 2: Implement Create-Mode Candidate Flow (Scan Path + fzf-like Add)

**Files:**
- Create: `internal/app/fuzzy_match.go`
- Create: `internal/app/workspace_overlay_scan_test.go`
- Modify: `internal/app/workspace_overlay.go`
- Modify: `internal/app/messages.go`
- Modify: `internal/app/update.go`

- [ ] **Step 1: Write failing tests for create-mode candidate behaviors**

Add tests for:
- path input change schedules scan with revision
- stale scan result is ignored
- `enter` on candidate adds repo to staged list
- duplicate staged path is rejected with `already added`

```go
func TestOverlay_Create_EnterCandidate_AddsToStaged(t *testing.T) {
	m := seededCreateOverlayModelWithCandidates()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if len(got.Overlay.StagedRepos) != 1 {
		t.Fatalf("expected staged repo append, got %d", len(got.Overlay.StagedRepos))
	}
}
```

- [ ] **Step 2: Run focused tests and confirm failures**

Run: `go test ./internal/app -run "TestOverlay_Create|TestOverlay_Scan" -v`  
Expected: FAIL with missing scan/candidate logic.

- [ ] **Step 3: Add scan-related messages and revision gating**

Introduce messages:
- `MsgOverlayScanScheduled{Revision int}`
- `MsgOverlayScanCompleted{Revision int, Candidates []RepoCandidate, Err error}`

Behavior:
- path changes increment revision
- apply result only when revision equals current overlay revision

- [ ] **Step 4: Implement candidate filtering and staged-add behavior**

`fuzzy_match.go` should provide deterministic, testable matching:

```go
func FilterCandidates(candidates []RepoCandidate, query string) []RepoCandidate
```

Rules:
- empty query returns original ordering
- case-insensitive subsequence match
- tie-break by shorter display name then lexical order

- [ ] **Step 5: Re-run focused overlay tests**

Run: `go test ./internal/app -run "TestOverlay_Create|TestOverlay_Scan" -v`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/fuzzy_match.go internal/app/workspace_overlay_scan_test.go internal/app/workspace_overlay.go internal/app/messages.go internal/app/update.go
git commit -m "feat: implement overlay create candidate staging flow"
```

## Task 3: Add Runtime Repo Discovery Scanner

**Files:**
- Create: `internal/adapters/repository/discover.go`
- Create: `internal/adapters/repository/discover_test.go`
- Create: `cmd/tui/workspace_overlay_runtime.go`
- Create: `cmd/tui/workspace_overlay_runtime_test.go`
- Modify: `cmd/tui/runtime.go`
- Modify: `internal/app/model.go`

- [ ] **Step 1: Write failing repository discovery tests**

Cases:
- root containing two git repos discovers both
- non-existent root returns clear error
- non-git folders are ignored
- duplicates (nested paths in same repo) dedup to repo root

```go
func TestDiscoverRepoRoots_DedupsSameRepoRoot(t *testing.T) {
	roots, err := DiscoverRepoRoots(context.Background(), fixtureRoot)
	if err != nil { t.Fatal(err) }
	if len(roots) != 1 { t.Fatalf("want 1 unique root, got %d", len(roots)) }
}
```

- [ ] **Step 2: Run adapter tests and verify red**

Run: `go test ./internal/adapters/repository -run "TestDiscoverRepoRoots" -v`  
Expected: FAIL (missing discoverer implementation).

- [ ] **Step 3: Implement recursive discovery**

Implementation constraints:
- `filepath.WalkDir` from `rootPath`
- treat folder as repo when `.git` directory/file exists
- resolve canonical repo root once, then skip descending inside discovered repo root
- bound output (e.g. max 500) to cap cost
- honor context cancellation

- [ ] **Step 4: Wire runtime scanner injection**

In `cmd/tui/workspace_overlay_runtime.go`:
- implement `runtimeWorkspaceOverlayScanner` adapter for app interface
- map discovered path -> `RepoCandidate{Name, Path}`

In `composeRuntimeModel`:
- set `WorkspaceOverlayScanner`
- set overlay default scan path from process cwd

- [ ] **Step 5: Run runtime+adapter tests**

Run: `go test ./internal/adapters/repository ./cmd/tui -run "DiscoverRepoRoots|WorkspaceOverlayScanner" -v`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/repository/discover.go internal/adapters/repository/discover_test.go cmd/tui/workspace_overlay_runtime.go cmd/tui/workspace_overlay_runtime_test.go cmd/tui/runtime.go internal/app/model.go
git commit -m "feat: add runtime workspace overlay repo scanner"
```

## Task 4: Implement `s` Save Flow (Create + Add Repos + Switch)

**Files:**
- Modify: `cmd/tui/workspace_overlay_runtime.go`
- Modify: `cmd/tui/workspace_overlay_runtime_test.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/messages.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/workspace_overlay_update_test.go`

- [ ] **Step 1: Write failing save-flow tests in app layer**

Add tests:
- save success closes overlay and switches to workspace mode
- save failure keeps overlay open and preserves draft
- duplicate name returns `workspace already exists`
- empty staged repos is allowed

```go
func TestOverlay_Save_EmptyStagedRepos_StillCreatesWorkspace(t *testing.T) {
	m := seededCreateOverlayModelWithDraft("team-x", nil)
	m.WorkspaceDraftCommitter = &fakeDraftCommitter{state: createdEmptyWorkspaceState()}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	_ = cmd // assert async commit message path
	_ = updated
}
```

- [ ] **Step 2: Run app tests and confirm red**

Run: `go test ./internal/app -run "TestOverlay_Save" -v`  
Expected: FAIL with missing save message handling/committer usage.

- [ ] **Step 3: Implement runtime draft committer**

In `runtimeWorkspaceDraftCommitter`:
- check trimmed name exists -> error if duplicate
- `CreateWorkspace(name)`
- add all staged repos (may be zero)
- if repos exist: `SelectRepo(newWS.ID, firstRepoID)`
- if repos empty: set selected workspace only
- reload state and return

- [ ] **Step 4: Handle save async lifecycle in app**

Add messages:
- `MsgOverlaySaveCompleted{State workspace.State, Err error}`

Update flow:
- on `s`, dispatch save command with current draft snapshot
- success: update model state, `UIMode=ModeWorkspace`, close overlay
- failure: keep overlay open + set error status

- [ ] **Step 5: Re-run save-flow tests and runtime tests**

Run: `go test ./internal/app -run "TestOverlay_Save" -v && go test ./cmd/tui -run "WorkspaceOverlayDraftCommitter" -v`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/tui/workspace_overlay_runtime.go cmd/tui/workspace_overlay_runtime_test.go internal/app/model.go internal/app/messages.go internal/app/update.go internal/app/workspace_overlay_update_test.go
git commit -m "feat: add workspace overlay save and switch workflow"
```

## Task 5: Render Overlay UI Contract (Switch vs Create)

**Files:**
- Create: `internal/app/view_overlay.go`
- Create: `internal/app/view_overlay_test.go`
- Modify: `internal/app/view.go`
- Modify: `internal/app/view_leftpane_test.go`

- [ ] **Step 1: Write failing view tests for mode-specific content**

Must assert:
- switch overlay shows workspace list only (no scan input / no candidate list)
- create overlay shows scan path input + candidate list + staged list + `s` hint
- status/error line is visible for duplicate/add/save failures

```go
func TestView_OverlaySwitchMode_HidesScanPane(t *testing.T) {
	m := seededOverlaySwitchModel()
	got := m.View()
	assertContains(t, got, "Workspace Overlay")
	assertNotContains(t, got, "scan path>")
}
```

- [ ] **Step 2: Run view tests and verify red**

Run: `go test ./internal/app -run "TestView_Overlay" -v`  
Expected: FAIL (overlay renderer not mounted).

- [ ] **Step 3: Implement overlay renderer and mount logic**

Approach:
- `view.go` keeps normal 3-pane render
- if overlay active, append centered overlay box (lipgloss) above base view
- `view_overlay.go` builds switch/create mode content blocks

- [ ] **Step 4: Re-run view tests**

Run: `go test ./internal/app -run "TestView_Overlay|TestView_LeftPane" -v`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/view_overlay.go internal/app/view_overlay_test.go internal/app/view.go internal/app/view_leftpane_test.go
git commit -m "feat: render workspace overlay for switch and create modes"
```

## Task 6: Docs + Regression Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/runbooks/v1-local-setup.md`

- [ ] **Step 1: Update README usage with overlay keys**

Add concise section:
- `w`: open workspace overlay
- switch mode behavior
- `c` / `s` create flow behavior
- `esc` draft discard semantics

- [ ] **Step 2: Update runbook with operator flow**

Document:
- create empty workspace is allowed
- duplicate name behavior
- duplicate staged repo behavior

- [ ] **Step 3: Run targeted package tests**

Run:
`go test ./internal/app ./internal/adapters/repository ./cmd/tui -v`

Expected:
- overlay tests pass
- existing folder/workspace launch tests still pass

- [ ] **Step 4: Run full regression suite**

Run: `go test ./...`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/superpowers/runbooks/v1-local-setup.md
git commit -m "docs: describe workspace overlay create and switch flow"
```

## Final Verification Checklist

- [ ] Overlay opens in both folder/workspace modes via `w`
- [ ] Switch mode has no scan input/fzf content
- [ ] Create mode has scan path input + candidate list + staged repos
- [ ] `s` commits and switches; empty staged repos still valid
- [ ] `esc` discards draft completely
- [ ] Duplicate workspace name blocked
- [ ] Duplicate staged repo blocked with `already added`
- [ ] `go test ./...` passes before merge
