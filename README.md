# gh-workspace

Bubble Tea TUI for multi-workspace repo operations and selected-repo PR/CI/Release status tracking.

## Usage

1. Follow the local setup runbook: `docs/superpowers/runbooks/v1-local-setup.md`.
2. Confirm local prerequisites (`go`, `git`, `lazygit`, `delta`; `gh` is needed when GitHub-backed sync is enabled).
3. Start the TUI:

```bash
go run ./cmd/tui
```

4. Current runtime behavior in this branch:
- status sync defaults to a no-op fetcher, so right-pane PR/CI/Release does not query GitHub by default
- for GitHub-backed status sync, configure `gh auth` and repo `releaseWorkflowRef` as described in the runbook

5. Runtime basics:
- `enter` attempts invalid-path recovery for the current repo
- `r` refreshes the selected repo status
- `p` toggles auto polling
