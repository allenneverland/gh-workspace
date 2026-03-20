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

Initialize state with at least one workspace and repo:

```bash
go run ./cmd/tui init \
  --workspace default \
  --repo-path /absolute/path/to/your/repo \
  --repo-name your-repo \
  --default-branch main \
  --release-workflow-ref .github/workflows/release.yml
```

Then start the app:

```bash
go run ./cmd/tui
```

## Runtime Keymap (v1)

- `a`: add repo path
- `enter`: attempt invalid-path recovery for the current repo
- `x`: remove selected repo
- `]`: next workspace
- `[`: previous workspace
- `r`: refresh selected repo now
- `p`: toggle auto polling on/off
