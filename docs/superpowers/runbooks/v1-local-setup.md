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

The current default runtime in this branch uses a no-op status fetcher at startup. This means:

- right-pane PR/CI/Release cards do not perform live GitHub fetches by default
- status cards stay on local/default values (`neutral` / `unconfigured`) unless a real GitHub adapter is wired in

## GitHub Authentication Requirement (When GitHub Adapter Is Enabled)

For builds/configurations that wire the GitHub status adapter, the app reuses your local GitHub CLI authentication context.

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
- In the current default no-op runtime, this is configuration/preflight state only; live Release workflow status requires a GitHub-backed status adapter.

## Run Locally

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
