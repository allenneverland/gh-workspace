# gh-workspace

Bubble Tea TUI for multi-workspace repo operations and selected-repo PR/CI/Release status tracking.

## Usage

1. Follow the local setup runbook: `docs/superpowers/runbooks/v1-local-setup.md`.
2. Confirm local prerequisites (`go`, `git`, `lazygit`, `delta`; `gh` is needed when GitHub-backed sync is enabled).
3. Launch the TUI in one of these modes:

```bash
gh-workspace
gh-workspace -f ../repo
gh-workspace -w team-a
```

4. Launch behavior:
- `gh-workspace`: folder mode using current directory
- `gh-workspace -f <path>`: folder mode using the given path
- `gh-workspace -w <name>`: workspace mode for an existing named workspace
- `-w __local_internal__` is rejected (reserved system workspace name)
- `-w <name>` exits with `workspace not found: <name>` when missing

5. Folder mode:
- uses a system local workspace (`__local_internal__`) that holds at most one repo
- git path: replaces the local repo with resolved repo root
- non-git path: clears local repo and shows `current folder is not a git repo`
- left pane hides workspace list; `[` and `]` are disabled
- press `a` to open repo-path input, `enter` to submit, `esc` to cancel

6. Runtime behavior:
- status sync is GitHub-backed by default (uses your `gh auth` context and per-repo `releaseWorkflowRef`)
- state persists across runs in a local BoltDB file (default path: `${XDG_CONFIG_HOME:-~/.config}/gh-workspace/state.db`)
- test fallback mode (`WORKSPACE_TUI_TEST_MODE=1`) uses no-op sync and skips persistent runtime wiring

7. Runtime basics:
- `enter` attempts invalid-path recovery for the current repo
- `r` refreshes the selected repo status
- `p` toggles auto polling
