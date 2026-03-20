# v1 Local Setup Runbook

This runbook covers local prerequisites and basic runtime controls for `gh-workspace` v1.

## Prerequisites

Install the required binaries and confirm they are available on your `PATH`:

- `gh`
- `lazygit`
- `delta`
- `git`

Quick check:

```bash
gh --version
lazygit --version
delta --version
git --version
```

## GitHub Authentication Requirement

The app reuses your local GitHub CLI authentication context. You must authenticate before PR/CI/Release sync can work.

```bash
gh auth login
gh auth status --hostname github.com
```

If authentication is missing, the right pane status sync will fail until `gh auth` is configured.

## Repo `releaseWorkflowRef` Setup

Release tracking depends on each repo’s `releaseWorkflowRef` value.

- Use a workflow file path (for example: `.github/workflows/release.yml`) or workflow ID.
- This value is stored per repo in workspace state.
- If `releaseWorkflowRef` is empty, Release status is shown as `unconfigured`.

## Run Locally

```bash
go run ./cmd/tui
```

## Runtime Keymap (v1)

- `a`: add repo path
- `enter`: select repo / recover invalid path
- `x`: remove selected repo
- `]`: next workspace
- `[`: previous workspace
- `r`: refresh selected repo now
- `p`: toggle auto polling on/off
