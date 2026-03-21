package app

import "strings"

type OverlayMode string

const (
	OverlayModeSwitch OverlayMode = "switch"
	OverlayModeCreate OverlayMode = "create"
)

type OverlayFocus string

const (
	OverlayFocusWorkspaceList   OverlayFocus = "workspace-list"
	OverlayFocusScanPathInput   OverlayFocus = "scan-path-input"
	OverlayFocusCandidateList   OverlayFocus = "candidate-list"
	OverlayFocusCreateNameInput OverlayFocus = "create-name-input"
	OverlayFocusStagedRepoList  OverlayFocus = "staged-repo-list"
)

type RepoCandidate struct {
	Name string
	Path string
}

type WorkspaceOverlayState struct {
	Active bool
	Mode   OverlayMode
	Focus  OverlayFocus

	ScanPathInput   string
	CreateNameInput string
	CandidateQuery  string

	Candidates  []RepoCandidate
	StagedRepos []RepoCandidate

	SelectedWorkspaceIndex int
	SelectedCandidateIndex int

	ScanInFlight bool
	ScanRevision int

	LastError string
}

func cloneWorkspaceOverlayState(src WorkspaceOverlayState) WorkspaceOverlayState {
	cloned := src
	cloned.Candidates = append([]RepoCandidate(nil), src.Candidates...)
	cloned.StagedRepos = append([]RepoCandidate(nil), src.StagedRepos...)
	return cloned
}

func openWorkspaceOverlay(current WorkspaceState, defaultScanPath string) WorkspaceOverlayState {
	overlay := resetWorkspaceOverlay(defaultScanPath)
	overlay.Active = true
	overlay.SelectedWorkspaceIndex = selectedWorkspaceIndex(current)
	return overlay
}

func resetWorkspaceOverlay(defaultScanPath string) WorkspaceOverlayState {
	return WorkspaceOverlayState{
		Mode:                   OverlayModeSwitch,
		Focus:                  OverlayFocusWorkspaceList,
		ScanPathInput:          defaultScanPath,
		SelectedWorkspaceIndex: -1,
		SelectedCandidateIndex: -1,
	}
}

func (s WorkspaceOverlayState) enterCreateMode() WorkspaceOverlayState {
	next := s
	next.Mode = OverlayModeCreate
	next.Focus = OverlayFocusCreateNameInput
	next.CreateNameInput = ""
	next.CandidateQuery = ""
	next.Candidates = nil
	next.StagedRepos = nil
	next.SelectedCandidateIndex = -1
	next.ScanInFlight = false
	next.ScanRevision = 0
	next.LastError = ""
	return next
}

func selectedWorkspaceIndex(current WorkspaceState) int {
	if len(current.Snapshot.Workspaces) == 0 {
		return -1
	}

	idx := findWorkspaceIndex(current.Snapshot.Workspaces, current.Snapshot.SelectedWorkspaceID)
	if idx >= 0 {
		return idx
	}
	return 0
}

func workspaceOverlayDefaultScanPath(current WorkspaceState) string {
	repo, ok := current.CurrentRepo()
	if !ok {
		return ""
	}
	return strings.TrimSpace(repo.Path)
}
