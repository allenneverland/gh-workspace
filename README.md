# gh-workspace

Bubble Tea TUI for multi-workspace repo operations and selected-repo PR/CI/Release status tracking.

## Usage

1. Follow the local setup runbook: `docs/superpowers/runbooks/v1-local-setup.md`.
2. Confirm local prerequisites (`go`, `git`, `lazygit`, `delta`; `gh` is needed when GitHub-backed sync is enabled).
3. Start the TUI:

```bash
go run ./cmd/tui
```

4. Runtime behavior:
- status sync is GitHub-backed by default (uses your `gh auth` context and per-repo `releaseWorkflowRef`)
- state persists across runs in a local BoltDB file (default path: `${XDG_CONFIG_HOME:-~/.config}/gh-workspace/state.db`)
- test fallback mode (`WORKSPACE_TUI_TEST_MODE=1`) uses no-op sync and skips persistent runtime wiring

5. Runtime basics:
- `enter` attempts invalid-path recovery for the current repo
- `r` refreshes the selected repo status
- `p` toggles auto polling
