package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	diffadapter "github.com/allenneverland/gh-workspace/internal/adapters/diff"
	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	syncengine "github.com/allenneverland/gh-workspace/internal/sync"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MsgSelectWorkspace:
		changed := false
		if m.State.SelectWorkspace(msg.WorkspaceID) {
			changed = true
		}
		if m.UIMode == ModeWorkspace && normalizeWorkspaceModeState(&m.State) {
			changed = true
		}
		if changed {
			m = persistAndPublishState(m)
		}
		next, syncCmd := syncOnSelectionChanged(m)
		if m.ActiveTab == TabLazygit {
			next = ensureLazygitSession(next)
			next = resizeLazygitViewport(next)
			afterLazygit, lazygitCmd := scheduleLazygitFrameWait(next)
			return afterLazygit, tea.Batch(syncCmd, lazygitCmd)
		}
		if m.ActiveTab == TabDiff {
			afterDiff, diffCmd := requestDiffRender(next)
			return afterDiff, tea.Batch(syncCmd, diffCmd)
		}
		return next, syncCmd
	case MsgSelectRepo:
		if m.State.SelectRepo(msg.RepoID) {
			m = persistAndPublishState(m)
		}
		next, syncCmd := syncOnSelectionChanged(m)
		if m.ActiveTab == TabLazygit {
			next = ensureLazygitSession(next)
			next = resizeLazygitViewport(next)
			afterLazygit, lazygitCmd := scheduleLazygitFrameWait(next)
			return afterLazygit, tea.Batch(syncCmd, lazygitCmd)
		}
		if m.ActiveTab == TabDiff {
			afterDiff, diffCmd := requestDiffRender(next)
			return afterDiff, tea.Batch(syncCmd, diffCmd)
		}
		return next, syncCmd
	case MsgSyncStartup:
		syncSetSelection(m)
		m = publishSyncState(m)
		next, syncCmd := requestSyncRefreshNow(m)
		return next, tea.Batch(syncCmd, scheduleSyncPolling(next))
	case MsgRefreshSelectedRepo:
		syncSetSelection(m)
		return requestSyncRefreshNow(m)
	case MsgToggleAutoPolling:
		return toggleAutoPolling(m)
	case MsgSyncRefreshCompleted:
		return handleSyncRefreshCompleted(m, msg)
	case MsgOverlayScanScheduled:
		return handleOverlayScanScheduled(m, msg)
	case MsgOverlayScanCompleted:
		return handleOverlayScanCompleted(m, msg)
	case MsgSetActiveTab:
		m.ActiveTab = msg.Tab
		if m.ActiveTab == TabLazygit {
			m = ensureLazygitSession(m)
			m = resizeLazygitViewport(m)
			return scheduleLazygitFrameWait(m)
		}
		if m.ActiveTab == TabDiff {
			return requestDiffRender(m)
		}
	case MsgRefreshDiff:
		if m.ActiveTab == TabDiff {
			return requestDiffRender(m)
		}
	case MsgRequestAddRepo:
		if m.ActiveTab == TabDiff {
			return m, nil
		}
		if m.UIMode == ModeFolder {
			m.RepoPathInput = newRepoPathInput()
			m.RepoPathInputActive = true
			return m, nil
		}
		m.AddRepoRequested = true
		m.StatusMessage = "add repo requested"
	case MsgSubmitRepoPath:
		if m.UIMode != ModeFolder {
			return m, nil
		}
		return submitFolderRepoPath(m, msg.Path)
	case MsgCreateWorktree:
		if m.ActiveTab == TabDiff {
			return m, nil
		}
		m = createWorktree(m, msg)
	case MsgSwitchWorktree:
		if m.ActiveTab == TabDiff {
			return m, nil
		}
		var changed bool
		m, changed = switchWorktree(m, msg)
		if changed {
			m = persistAndPublishState(m)
		}
	case MsgDiffRendered:
		return handleDiffRendered(m, msg)
	case MsgLazygitFrame:
		m.lazygitFrameListenerInFlight = false
		if msg.SessionID == m.LazygitSessionID {
			m.LazygitCenterFrameText = string(msg.Data)
		}
		return scheduleLazygitFrameWait(m)
	case MsgLazygitFrameClosed:
		m.lazygitFrameListenerInFlight = false
	case syncengine.MsgTick:
		syncSetSelection(m)
		next, syncCmd := requestSyncTick(m)
		return next, tea.Batch(syncCmd, scheduleSyncPolling(next))
	case tea.KeyMsg:
		if m.RepoPathInputActive {
			return handleRepoPathInputKey(m, msg)
		}
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if key.Matches(msg, m.Keys.Quit) {
			return m, tea.Quit
		}
		if m.Overlay.Active {
			var cmd tea.Cmd
			var handled bool
			m, cmd, handled = updateWorkspaceOverlayKey(m, msg)
			if handled {
				return m, cmd
			}
		}
		if tab, ok := tabFromKey(m, msg); ok {
			return updateModel(m, MsgSetActiveTab{Tab: tab})
		}
		if lazygitOwnsKeys(m) {
			return forwardLazygitInput(m, msg), nil
		}
		if diffTabBlocksMutatingKeys(m, msg) {
			return m, nil
		}
		var handled bool
		switch {
		case key.Matches(msg, m.Keys.WorkspaceOverlay):
			var cmd tea.Cmd
			m, cmd, handled = updateWorkspaceOverlayKey(m, msg)
			if handled {
				return m, cmd
			}
		case key.Matches(msg, m.Keys.AddRepo):
			if m.UIMode == ModeFolder {
				m.RepoPathInput = newRepoPathInput()
				m.RepoPathInputActive = true
				return m, nil
			}
			return m, func() tea.Msg { return MsgRequestAddRepo{} }
		case key.Matches(msg, m.Keys.RemoveRepo):
			if removed, ok := m.State.RemoveCurrentRepo(); ok {
				m.StatusMessage = "removed repo: " + removed.Name
				m = persistAndPublishState(m)
				return syncOnSelectionChanged(m)
			} else {
				m.StatusMessage = "no selected repo to remove"
			}
		case key.Matches(msg, m.Keys.SelectRepo):
			var changed bool
			m, changed = attemptRepoRecovery(m)
			if changed {
				m = persistAndPublishState(m)
			}
		case key.Matches(msg, m.Keys.ManualRefresh):
			return m, func() tea.Msg { return MsgRefreshSelectedRepo{} }
		case key.Matches(msg, m.Keys.TogglePolling):
			return m, func() tea.Msg { return MsgToggleAutoPolling{} }
		case key.Matches(msg, m.Keys.NextWorkspace):
			if m.UIMode == ModeFolder {
				return m, nil
			}
			if m.UIMode == ModeWorkspace {
				var changed bool
				m, changed = selectAdjacentWorkspaceModeWorkspace(m, true)
				if changed {
					m = persistAndPublishState(m)
				}
				return syncOnSelectionChanged(m)
			}
			if m.State.SelectNextWorkspace() {
				m = persistAndPublishState(m)
			}
			return syncOnSelectionChanged(m)
		case key.Matches(msg, m.Keys.PrevWorkspace):
			if m.UIMode == ModeFolder {
				return m, nil
			}
			if m.UIMode == ModeWorkspace {
				var changed bool
				m, changed = selectAdjacentWorkspaceModeWorkspace(m, false)
				if changed {
					m = persistAndPublishState(m)
				}
				return syncOnSelectionChanged(m)
			}
			if m.State.SelectPrevWorkspace() {
				m = persistAndPublishState(m)
			}
			return syncOnSelectionChanged(m)
		}
	case tea.WindowSizeMsg:
		m = applyWindowSize(m, msg.Width, msg.Height)
		m = resizeLazygitViewport(m)
		return m, nil
	}

	return m, nil
}

func updateWorkspaceOverlayKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	defaultScanPath := workspaceOverlayDefaultScanPath(m.State)

	if m.Overlay.Active {
		switch {
		case msg.Type == tea.KeyEsc:
			m.Overlay = resetWorkspaceOverlay(defaultScanPath)
			return m, nil, true
		case msg.Type == tea.KeyTab:
			m.Overlay = cycleWorkspaceOverlayFocus(m.Overlay, true)
			return m, nil, true
		case msg.Type == tea.KeyShiftTab:
			m.Overlay = cycleWorkspaceOverlayFocus(m.Overlay, false)
			return m, nil, true
		}

		if next, cmd, handled := updateWorkspaceOverlayTextInput(m, msg); handled {
			return next, cmd, true
		}

		switch {
		case key.Matches(msg, m.Keys.WorkspaceOverlay):
			m.Overlay = resetWorkspaceOverlay(defaultScanPath)
			return m, nil, true
		case key.Matches(msg, m.Keys.OverlayCreate):
			if m.Overlay.Mode == OverlayModeSwitch {
				m.Overlay = m.Overlay.enterCreateMode()
			}
			return m, nil, true
		case key.Matches(msg, m.Keys.OverlaySave):
			return m, nil, true
		case msg.Type == tea.KeyEnter && m.Overlay.Mode == OverlayModeCreate && m.Overlay.Focus == OverlayFocusCandidateList:
			m = overlayAddSelectedCandidate(m)
			return m, nil, true
		default:
			return m, nil, false
		}
	}

	if key.Matches(msg, m.Keys.WorkspaceOverlay) {
		m.Overlay = openWorkspaceOverlay(m.State, defaultScanPath)
		return m, nil, true
	}

	return m, nil, false
}

func updateWorkspaceOverlayTextInput(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	if !m.Overlay.Active || m.Overlay.Mode != OverlayModeCreate {
		return m, nil, false
	}

	switch m.Overlay.Focus {
	case OverlayFocusScanPathInput:
		next, cmd, handled := updateOverlayScanPathInput(m, msg)
		if handled {
			return next, cmd, true
		}
	case OverlayFocusCreateNameInput:
		next, handled := applyOverlayTextInput(m.Overlay.CreateNameInput, msg)
		if handled {
			m.Overlay.CreateNameInput = next
			m.Overlay.LastError = ""
			return m, nil, true
		}
	case OverlayFocusCandidateList:
		next, handled := applyOverlayTextInput(m.Overlay.CandidateQuery, msg)
		if handled {
			m.Overlay.CandidateQuery = next
			m.Overlay.SelectedCandidateIndex = candidateSelectionIndex(m.Overlay.Candidates, m.Overlay.CandidateQuery, 0)
			m.Overlay.LastError = ""
			return m, nil, true
		}
	}

	return m, nil, false
}

func updateOverlayScanPathInput(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	next, changed := applyOverlayTextInput(m.Overlay.ScanPathInput, msg)
	if !changed {
		return m, nil, false
	}

	m.Overlay.ScanPathInput = next
	m.Overlay.ScanRevision++
	m.Overlay.ScanInFlight = true
	m.Overlay.LastError = ""
	revision := m.Overlay.ScanRevision

	return m, func() tea.Msg {
		return MsgOverlayScanScheduled{Revision: revision}
	}, true
}

func applyOverlayTextInput(current string, msg tea.KeyMsg) (string, bool) {
	switch msg.Type {
	case tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return current, false
		}
		return current + string(msg.Runes), true
	case tea.KeySpace:
		return current + " ", true
	case tea.KeyBackspace, tea.KeyDelete:
		if current == "" {
			return current, false
		}
		runes := []rune(current)
		if len(runes) == 0 {
			return current, false
		}
		return string(runes[:len(runes)-1]), true
	default:
		return current, false
	}
}

func updateOverlayScanScheduled(m Model, msg MsgOverlayScanScheduled) (Model, bool) {
	if !m.Overlay.Active || msg.Revision != m.Overlay.ScanRevision {
		return m, false
	}

	m.Overlay.ScanInFlight = true
	m.Overlay.LastError = ""
	return m, true
}

func handleOverlayScanScheduled(m Model, msg MsgOverlayScanScheduled) (Model, tea.Cmd) {
	updated, matched := updateOverlayScanScheduled(m, msg)
	if !matched {
		return m, nil
	}
	return updated, nil
}

func handleOverlayScanCompleted(m Model, msg MsgOverlayScanCompleted) (Model, tea.Cmd) {
	if !m.Overlay.Active || msg.Revision != m.Overlay.ScanRevision {
		return m, nil
	}

	m.Overlay.ScanInFlight = false
	if msg.Err != nil {
		m.Overlay.LastError = "scan failed: " + msg.Err.Error()
		return m, nil
	}

	m.Overlay.Candidates = append([]RepoCandidate(nil), msg.Candidates...)
	m.Overlay.SelectedCandidateIndex = candidateSelectionIndex(m.Overlay.Candidates, m.Overlay.CandidateQuery, 0)
	m.Overlay.LastError = ""
	return m, nil
}

func overlayAddSelectedCandidate(m Model) Model {
	candidate, ok := selectedOverlayCandidate(m)
	if !ok {
		return m
	}

	for _, staged := range m.Overlay.StagedRepos {
		if normalizedOverlayRepoPath(staged.Path) == normalizedOverlayRepoPath(candidate.Path) {
			m.Overlay.LastError = "already added"
			return m
		}
	}

	m.Overlay.StagedRepos = append(m.Overlay.StagedRepos, candidate)
	m.Overlay.LastError = ""
	return m
}

func normalizedOverlayRepoPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func selectedOverlayCandidate(m Model) (RepoCandidate, bool) {
	candidates := FilterCandidates(m.Overlay.Candidates, m.Overlay.CandidateQuery)
	idx := candidateSelectionIndex(candidates, "", m.Overlay.SelectedCandidateIndex)
	if idx < 0 || idx >= len(candidates) {
		return RepoCandidate{}, false
	}
	return candidates[idx], true
}

func candidateSelectionIndex(candidates []RepoCandidate, query string, current int) int {
	visible := FilterCandidates(candidates, query)
	if len(visible) == 0 {
		return -1
	}
	if current < 0 || current >= len(visible) {
		return 0
	}
	return current
}

func cycleWorkspaceOverlayFocus(overlay WorkspaceOverlayState, forward bool) WorkspaceOverlayState {
	if overlay.Mode != OverlayModeCreate {
		overlay.Focus = OverlayFocusWorkspaceList
		return overlay
	}

	order := []OverlayFocus{
		OverlayFocusCreateNameInput,
		OverlayFocusScanPathInput,
		OverlayFocusCandidateList,
		OverlayFocusStagedRepoList,
	}
	current := 0
	for i, focus := range order {
		if overlay.Focus == focus {
			current = i
			break
		}
	}

	if forward {
		current = (current + 1) % len(order)
	} else {
		current--
		if current < 0 {
			current = len(order) - 1
		}
	}

	overlay.Focus = order[current]
	return overlay
}

func selectAdjacentWorkspaceModeWorkspace(m Model, forward bool) (Model, bool) {
	workspaceIDs := userWorkspaceIDs(m.State.Snapshot.Workspaces)
	if len(workspaceIDs) == 0 {
		if m.State.Snapshot.SelectedWorkspaceID == "" {
			return m, false
		}
		m.State.Snapshot.SelectedWorkspaceID = ""
		return m, true
	}

	currentIdx := -1
	for i, id := range workspaceIDs {
		if id == m.State.Snapshot.SelectedWorkspaceID {
			currentIdx = i
			break
		}
	}
	if currentIdx < 0 {
		m.State.Snapshot.SelectedWorkspaceID = workspaceIDs[0]
		m.State.ensureSelection()
		return m, true
	}

	nextIdx := currentIdx
	if forward {
		nextIdx = (currentIdx + 1) % len(workspaceIDs)
	} else {
		nextIdx = currentIdx - 1
		if nextIdx < 0 {
			nextIdx = len(workspaceIDs) - 1
		}
	}

	nextWorkspaceID := workspaceIDs[nextIdx]
	if nextWorkspaceID == m.State.Snapshot.SelectedWorkspaceID {
		return m, false
	}

	m.State.Snapshot.SelectedWorkspaceID = nextWorkspaceID
	m.State.ensureSelection()
	return m, true
}

func handleRepoPathInputKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	submitted, canceled, _ := m.RepoPathInput.Update(msg)
	switch {
	case canceled:
		m.RepoPathInputActive = false
		m.RepoPathInput = newRepoPathInput()
		return m, nil
	case submitted:
		path := m.RepoPathInput.Value()
		m.RepoPathInputActive = false
		m.RepoPathInput = newRepoPathInput()
		return m, func() tea.Msg {
			return MsgSubmitRepoPath{Path: path}
		}
	default:
		return m, nil
	}
}

func submitFolderRepoPath(m Model, path string) (Model, tea.Cmd) {
	if m.RepoPathSubmitter == nil {
		m.StatusMessage = "repo path submitter unavailable"
		return m, nil
	}

	result, err := m.RepoPathSubmitter.SubmitRepoPath(context.Background(), path)
	if err != nil {
		m.StatusMessage = "failed to submit repo path: " + err.Error()
		return m, nil
	}

	m.State = NewWorkspaceState(result.State)
	statusMessage := strings.TrimSpace(result.StatusMessage)
	if statusMessage == "" {
		if repo, ok := m.State.CurrentRepo(); ok && strings.TrimSpace(repo.Name) != "" {
			statusMessage = "added repo: " + repo.Name
		} else {
			statusMessage = "current folder is not a git repo"
		}
	}
	m.StatusMessage = statusMessage

	m = publishSyncState(m)
	return syncOnSelectionChanged(m)
}

func ensureLazygitSession(m Model) Model {
	repo, ok := m.State.CurrentRepo()
	if !ok {
		m = clearLazygitSessionState(m)
		m.StatusMessage = "請先選擇 repo"
		return m
	}
	if m.LazygitSessionManager == nil {
		m = clearLazygitSessionState(m)
		m.StatusMessage = "lazygit session manager unavailable"
		return m
	}

	handle, err := m.LazygitSessionManager.StartSession(repo.Path)
	if err != nil {
		m = clearLazygitSessionState(m)
		m.StatusMessage = "failed to start lazygit session: " + err.Error()
		return m
	}

	if m.LazygitSessionID != handle.ID {
		m.LazygitCenterFrameText = ""
	}
	m.LazygitSessionID = handle.ID
	return m
}

func clearLazygitSessionState(m Model) Model {
	m.LazygitSessionID = ""
	m.LazygitCenterFrameText = ""
	return m
}

func scheduleLazygitFrameWait(m Model) (Model, tea.Cmd) {
	if m.ActiveTab != TabLazygit || m.LazygitSessionManager == nil || m.LazygitSessionID == "" {
		return m, nil
	}
	if m.lazygitFrameListenerInFlight {
		return m, nil
	}

	cmd := waitForLazygitFrame(m.LazygitSessionManager)
	if cmd == nil {
		return m, nil
	}
	m.lazygitFrameListenerInFlight = true
	return m, cmd
}

func lazygitOwnsKeys(m Model) bool {
	return m.ActiveTab == TabLazygit && m.LazygitSessionManager != nil && m.LazygitSessionID != ""
}

func diffTabBlocksMutatingKeys(m Model, msg tea.KeyMsg) bool {
	if m.ActiveTab != TabDiff {
		return false
	}
	return key.Matches(msg, m.Keys.AddRepo) ||
		key.Matches(msg, m.Keys.RemoveRepo) ||
		key.Matches(msg, m.Keys.SelectRepo) ||
		key.Matches(msg, m.Keys.NextWorkspace) ||
		key.Matches(msg, m.Keys.PrevWorkspace)
}

func syncOnSelectionChanged(m Model) (Model, tea.Cmd) {
	syncSetSelection(m)
	return requestSyncRefreshNow(m)
}

func syncSetSelection(m Model) {
	if m.SyncEngine == nil {
		return
	}
	m.SyncEngine.SetSelection(m.State.CurrentWorkspaceID(), m.State.CurrentRepoID())
}

func syncRefreshNowCmd(m Model) tea.Cmd {
	if m.SyncEngine == nil {
		return nil
	}
	workspaceID := m.State.CurrentWorkspaceID()
	repoID := m.State.CurrentRepoID()

	return func() tea.Msg {
		status, err := m.SyncEngine.RefreshNow(context.Background())
		return MsgSyncRefreshCompleted{
			WorkspaceID: workspaceID,
			RepoID:      repoID,
			Status:      status,
			Err:         err,
		}
	}
}

func syncOnTickCmd(m Model) tea.Cmd {
	if m.SyncEngine == nil {
		return nil
	}
	workspaceID := m.State.CurrentWorkspaceID()
	repoID := m.State.CurrentRepoID()

	return func() tea.Msg {
		status, err := m.SyncEngine.OnTick(context.Background())
		return MsgSyncRefreshCompleted{
			WorkspaceID: workspaceID,
			RepoID:      repoID,
			Status:      status,
			Err:         err,
		}
	}
}

func requestSyncRefreshNow(m Model) (Model, tea.Cmd) {
	if m.SyncEngine == nil {
		return m, nil
	}
	if m.syncCommandInFlight {
		m.syncRefreshPending = true
		return m, nil
	}
	cmd := syncRefreshNowCmd(m)
	if cmd == nil {
		return m, nil
	}
	m.syncCommandInFlight = true
	return m, cmd
}

func requestSyncTick(m Model) (Model, tea.Cmd) {
	if m.SyncEngine == nil {
		return m, nil
	}
	if m.syncCommandInFlight {
		m.syncTickPending = true
		return m, nil
	}
	cmd := syncOnTickCmd(m)
	if cmd == nil {
		return m, nil
	}
	m.syncCommandInFlight = true
	return m, cmd
}

func handleSyncRefreshCompleted(m Model, msg MsgSyncRefreshCompleted) (Model, tea.Cmd) {
	m.syncCommandInFlight = false
	workspaceID := strings.TrimSpace(msg.WorkspaceID)
	repoID := strings.TrimSpace(msg.RepoID)
	if workspaceID != "" && repoID != "" {
		snapshot := m.repoStatusSnapshotByKey(workspaceID, repoID)
		if msg.Err != nil {
			snapshot.IsStale = true
			snapshot.LatestError = msg.Err.Error()
		} else {
			if msg.Status.PR != "" {
				snapshot.PR = msg.Status.PR
			}
			if msg.Status.CI != "" {
				snapshot.CI = msg.Status.CI
			}
			if msg.Status.Release != "" {
				snapshot.Release = msg.Status.Release
			}
			snapshot.ReleaseRun = msg.Status.ReleaseRun
			snapshot.LastSyncedAt = time.Now().UTC()
			snapshot.IsStale = false
			snapshot.LatestError = ""
		}
		m.setRepoStatusSnapshot(workspaceID, repoID, snapshot)
		m = persistAndPublishState(m)
	}
	if msg.Err != nil {
		m.StatusMessage = "failed to refresh selected repo: " + msg.Err.Error()
	}

	if m.syncRefreshPending {
		m.syncRefreshPending = false
		return requestSyncRefreshNow(m)
	}
	if m.syncTickPending {
		m.syncTickPending = false
		return requestSyncTick(m)
	}
	return m, nil
}

func scheduleSyncPolling(m Model) tea.Cmd {
	if m.SyncEngine == nil || !m.SyncEngine.AutoPollingEnabled() {
		return nil
	}
	return m.SyncEngine.Start(context.Background())
}

func toggleAutoPolling(m Model) (Model, tea.Cmd) {
	if m.SyncEngine == nil {
		return m, nil
	}

	enabled := !m.SyncEngine.AutoPollingEnabled()
	m.SyncEngine.SetAutoPolling(enabled)
	if !enabled {
		return m, nil
	}
	return m, scheduleSyncPolling(m)
}

func requestDiffRender(m Model) (Model, tea.Cmd) {
	if m.ActiveTab != TabDiff {
		return m, nil
	}

	repo, ok := m.State.CurrentRepo()
	if !ok || strings.TrimSpace(repo.Path) == "" {
		m.DiffLoading = false
		m.DiffOutput = ""
		m.DiffStatus = ""
		m.diffRenderInFlight = false
		m.diffRenderPending = false
		return m, nil
	}
	if m.DiffRenderer == nil {
		m.DiffLoading = false
		m.DiffOutput = ""
		m.DiffStatus = "diff renderer unavailable"
		m.diffRenderInFlight = false
		m.diffRenderPending = false
		return m, nil
	}
	if m.diffRenderInFlight {
		m.diffRenderPending = true
		return m, nil
	}

	m.diffRenderRequestID++
	requestID := m.diffRenderRequestID
	repoPath := repo.Path
	m.DiffLoading = true
	m.DiffStatus = ""
	m.diffRenderInFlight = true

	return m, func() tea.Msg {
		out, err := m.DiffRenderer.Render(context.Background(), repoPath)
		return MsgDiffRendered{
			RequestID: requestID,
			Output:    out,
			Err:       err,
		}
	}
}

func applyDiffRenderResult(m Model, msg MsgDiffRendered) Model {
	if msg.RequestID != m.diffRenderRequestID {
		return m
	}

	m.DiffLoading = false
	m.diffRenderInFlight = false
	if msg.Err != nil {
		m.DiffOutput = ""
		if errors.Is(msg.Err, diffadapter.ErrDeltaNotFound) {
			m.DiffStatus = "delta not found; install delta to use Diff tab"
			return m
		}
		m.DiffStatus = "failed to render diff: " + msg.Err.Error()
		return m
	}

	m.DiffOutput = msg.Output
	if strings.TrimSpace(msg.Output) == "" {
		m.DiffStatus = "(no changes)"
		return m
	}

	m.DiffStatus = ""
	return m
}

func handleDiffRendered(m Model, msg MsgDiffRendered) (Model, tea.Cmd) {
	m = applyDiffRenderResult(m, msg)
	if msg.RequestID != m.diffRenderRequestID {
		return m, nil
	}
	if m.diffRenderPending && m.ActiveTab == TabDiff {
		m.diffRenderPending = false
		return requestDiffRender(m)
	}
	m.diffRenderPending = false
	return m, nil
}

func waitForLazygitFrame(manager LazygitSessionManager) tea.Cmd {
	if manager == nil {
		return nil
	}
	frames := manager.Frames()
	if frames == nil {
		return nil
	}

	return func() tea.Msg {
		frame, ok := <-frames
		if !ok {
			return MsgLazygitFrameClosed{}
		}
		return MsgLazygitFrame{
			SessionID: frame.SessionID,
			Data:      frame.Data,
		}
	}
}

func resizeLazygitViewport(m Model) Model {
	if m.ActiveTab != TabLazygit || m.LazygitSessionManager == nil || m.LazygitSessionID == "" {
		return m
	}
	cols, rows, ok := lazygitViewportSize(m)
	if !ok {
		return m
	}
	if err := m.LazygitSessionManager.ResizeSession(m.LazygitSessionID, cols, rows); err != nil {
		m.StatusMessage = "failed to resize lazygit viewport: " + err.Error()
	}
	return m
}

func lazygitViewportSize(m Model) (cols, rows int, ok bool) {
	if m.CenterPaneWidth <= 0 || m.WindowHeight <= 0 {
		return 0, 0, false
	}

	textCols := m.CenterPaneWidth - 4 // pane border + horizontal padding
	if textCols < 1 {
		textCols = 1
	}
	textRows := m.WindowHeight - 2 // pane border
	if textRows < 1 {
		textRows = 1
	}
	contentRows := textRows - 3 // "Center", "tabs", "active tab"
	if contentRows < 1 {
		contentRows = 1
	}
	return textCols, contentRows, true
}

func forwardLazygitInput(m Model, msg tea.KeyMsg) Model {
	if m.ActiveTab != TabLazygit {
		return m
	}
	if m.LazygitSessionManager == nil || m.LazygitSessionID == "" {
		return m
	}

	input, ok := keyMsgToBytes(msg)
	if !ok {
		return m
	}

	if err := m.LazygitSessionManager.WriteInput(m.LazygitSessionID, input); err != nil {
		m.StatusMessage = "failed to write lazygit input: " + err.Error()
	}
	return m
}

func keyMsgToBytes(msg tea.KeyMsg) ([]byte, bool) {
	switch msg.Type {
	case tea.KeyRunes:
		return []byte(string(msg.Runes)), true
	case tea.KeyEnter:
		return []byte{'\r'}, true
	case tea.KeyBackspace:
		return []byte{0x7f}, true
	case tea.KeySpace:
		return []byte{' '}, true
	case tea.KeyTab:
		return []byte{'\t'}, true
	case tea.KeyUp:
		return []byte("\x1b[A"), true
	case tea.KeyDown:
		return []byte("\x1b[B"), true
	case tea.KeyRight:
		return []byte("\x1b[C"), true
	case tea.KeyLeft:
		return []byte("\x1b[D"), true
	default:
		return nil, false
	}
}

func createWorktree(m Model, msg MsgCreateWorktree) Model {
	repo, ok := m.State.CurrentRepo()
	if !ok {
		m.StatusMessage = "no selected repo for worktree create"
		return m
	}
	if m.WorktreeAdapter == nil {
		m.StatusMessage = "worktree adapter unavailable"
		return m
	}

	if err := m.WorktreeAdapter.Create(context.Background(), repo.Path, msg.Branch, msg.Path); err != nil {
		m.StatusMessage = "failed to create worktree: " + err.Error()
		return m
	}

	worktrees, err := m.WorktreeAdapter.List(context.Background(), repo.Path)
	if err != nil {
		m.StatusMessage = "failed to refresh worktrees after create: " + err.Error()
		return m
	}
	m.Worktrees = worktrees

	m.StatusMessage = "created worktree: " + msg.Path
	return m
}

func switchWorktree(m Model, msg MsgSwitchWorktree) (Model, bool) {
	repo, ok := m.State.CurrentRepo()
	if !ok {
		m.StatusMessage = "no selected repo for worktree switch"
		return m, false
	}
	if m.WorktreeAdapter == nil {
		m.StatusMessage = "worktree adapter unavailable"
		return m, false
	}

	worktrees, err := m.WorktreeAdapter.List(context.Background(), repo.Path)
	if err != nil {
		m.StatusMessage = "failed to list worktrees: " + err.Error()
		return m, false
	}
	m.Worktrees = worktrees

	selected, exists := findWorktreeByPath(worktrees, msg.WorktreePath)
	if !exists {
		m.StatusMessage = "worktree not found in list: " + msg.WorktreePath
		return m, false
	}

	if err := m.WorktreeAdapter.ValidateSwitchTarget(context.Background(), msg.WorktreePath); err != nil {
		m.StatusMessage = "failed to validate worktree target: " + err.Error()
		return m, false
	}

	if !m.State.SetRepoSelectedWorktree(m.State.CurrentWorkspaceID(), repo.ID, selected.ID, selected.Path) {
		m.StatusMessage = "failed to persist selected worktree"
		return m, false
	}
	m.StatusMessage = "switched worktree: " + selected.Path
	return m, true
}

func findWorktreeByPath(worktrees []WorktreeItem, path string) (WorktreeItem, bool) {
	for _, wt := range worktrees {
		if wt.Path == path {
			return wt, true
		}
	}
	return WorktreeItem{}, false
}

func attemptRepoRecovery(m Model) (Model, bool) {
	repo, ok := m.State.CurrentRepo()
	if !ok {
		m.StatusMessage = "no selected repo to recover"
		return m, false
	}

	if repo.Health != workspace.RepoInvalid {
		m.StatusMessage = "selected repo is already healthy"
		return m, false
	}

	if pathExists(repo.Path) {
		m.State.SetCurrentRepoHealth(workspace.RepoHealthy)
		m.StatusMessage = "repo path recovered: " + repo.Path
		return m, true
	}

	m.StatusMessage = "repo path still invalid: " + repo.Path
	return m, false
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func tabFromKey(m Model, msg tea.KeyMsg) (Tab, bool) {
	switch {
	case key.Matches(msg, m.Keys.NextTab):
		return nextTab(m.ActiveTab, m.CenterTabs(), 1), true
	case key.Matches(msg, m.Keys.PrevTab):
		return nextTab(m.ActiveTab, m.CenterTabs(), -1), true
	case key.Matches(msg, m.Keys.TabOverview):
		return TabOverview, true
	case key.Matches(msg, m.Keys.TabWorktrees):
		return TabWorktrees, true
	case key.Matches(msg, m.Keys.TabLazygit):
		return TabLazygit, true
	case key.Matches(msg, m.Keys.TabDiff):
		return TabDiff, true
	default:
		return "", false
	}
}

func nextTab(current Tab, tabs []Tab, delta int) Tab {
	if len(tabs) == 0 {
		return current
	}

	currentIndex := 0
	found := false
	for i := range tabs {
		if tabs[i] == current {
			currentIndex = i
			found = true
			break
		}
	}
	if !found {
		return tabs[0]
	}

	next := (currentIndex + delta) % len(tabs)
	if next < 0 {
		next += len(tabs)
	}
	return tabs[next]
}

func applyWindowSize(m Model, width, height int) Model {
	if width <= 0 || height <= 0 {
		return m
	}

	left, center, right := calculatePaneWidths(width)
	m.WindowWidth = width
	m.WindowHeight = height
	m.LeftPaneWidth = left
	m.CenterPaneWidth = center
	m.RightPaneWidth = right
	return m
}

func calculatePaneWidths(totalWidth int) (left, center, right int) {
	const (
		minLeft   = 20
		minCenter = 30
		minRight  = 24
	)

	if totalWidth <= 0 {
		return 30, 80, 40
	}

	if totalWidth < minLeft+minCenter+minRight {
		left = maxInt(minLeft, totalWidth/4)
		right = maxInt(minRight, totalWidth/4)
		center = totalWidth - left - right
		if center < 1 {
			center = 1
		}
		return left, center, right
	}

	left = maxInt(minLeft, totalWidth*24/100)
	right = maxInt(minRight, totalWidth*26/100)
	center = totalWidth - left - right

	if center < minCenter {
		deficit := minCenter - center
		shrinkRight := minInt(deficit, right-minRight)
		right -= shrinkRight
		deficit -= shrinkRight
		shrinkLeft := minInt(deficit, left-minLeft)
		left -= shrinkLeft
		center = totalWidth - left - right
	}

	return left, maxInt(1, center), right
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
