# v1 Local Setup Runbook

This runbook covers local prerequisites and basic runtime controls for `gh-workspace` v1.

## Prerequisites

Install the required binaries and confirm they are available on your `PATH`:

- `go`
- `gh`
- `lazygit`
- `delta`
- `git`

Quick check:

```bash
go version
gh --version
lazygit --version
delta --version
git --version
```

## Current Runtime Scope (Important)

The default runtime is GitHub-backed and persists state on disk:

- selected-repo PR/CI/Release refresh uses `gh` CLI via your existing `gh auth` session
- workspace selection and sync snapshots are persisted in BoltDB
- default store path: `${XDG_CONFIG_HOME:-~/.config}/gh-workspace/state.db`
- optional override path: `WORKSPACE_TUI_STATE_PATH`
- test fallback mode (`WORKSPACE_TUI_TEST_MODE=1`) uses no-op sync and skips persistent runtime wiring

## GitHub Authentication Requirement

The app reuses your local GitHub CLI authentication context.

```bash
gh auth login
gh auth status --hostname github.com
```

If authentication is missing in that mode, remote status sync will fail.

## Repo `releaseWorkflowRef` Setup

Release tracking depends on each repo’s `releaseWorkflowRef` value.

- Use a workflow file path (for example: `.github/workflows/release.yml`) or workflow ID.
- This value is stored per repo in workspace state.
- If `releaseWorkflowRef` is empty, Release status is shown as `unconfigured`.
- In test fallback mode (`WORKSPACE_TUI_TEST_MODE=1`), live GitHub sync is intentionally disabled.

## Run Locally

You can launch directly in folder mode (workspace optional):

```bash
gh-workspace
gh-workspace -f ../repo
```

Or open an existing named workspace:

```bash
gh-workspace -w team-a
```

Behavior notes:

- `gh-workspace` uses current directory as folder mode root.
- `gh-workspace -f <path>` uses the provided folder path.
- `gh-workspace -w <name>` opens only existing workspaces (missing name => `workspace not found: <name>`).
- `-w __local_internal__` is rejected (reserved system workspace name).
- Folder mode non-git paths clear local repo selection and show `current folder is not a git repo`.

## Runtime Keymap (v1)

- `a`: add repo path
- `enter`: attempt invalid-path recovery for the current repo
- `x`: remove selected repo
- `w`: open workspace overlay (switch/create)
- `]`: next workspace (workspace mode only)
- `[`: previous workspace (workspace mode only)
- `r`: refresh selected repo now
- `p`: toggle auto polling on/off

Folder mode specifics:

- workspace list is hidden in the left pane
- `[` and `]` are disabled
- `a` opens repo-path input (`enter` submit, `esc` cancel)

## Workspace Overlay Flow (v1)

- `w` opens switch mode; this view shows only workspace list and no create inputs.
- `c` enters create mode.
- Create mode has:
  - `name>` workspace name input
  - `scan path>` input with immediate scan updates
  - candidate list (`enter` stages selected repo)
  - staged repo list
- `s` in create mode saves, closes the overlay, and switches to the new workspace.
- `esc` discards all draft state and closes the overlay.
- Creating a workspace with no staged repos is valid.
- Duplicate workspace name save returns `workspace already exists`.
- Staging a duplicate repo path returns `already added`.
