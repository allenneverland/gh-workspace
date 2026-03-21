package app

import "github.com/allenneverland/gh-workspace/internal/domain/workspace"

type MsgSelectWorkspace struct {
	WorkspaceID string
}

type MsgSelectRepo struct {
	RepoID string
}

type MsgSetActiveTab struct {
	Tab Tab
}

type MsgRefreshDiff struct{}

type MsgRequestAddRepo struct{}

type MsgOpenWorkspaceOverlay struct{}

type MsgCloseWorkspaceOverlay struct{}

type MsgEnterWorkspaceOverlayCreate struct{}

type MsgOverlayScanScheduled struct {
	Revision int
}

type MsgOverlayScanCompleted struct {
	Revision   int
	Candidates []RepoCandidate
	Err        error
}

type MsgSubmitRepoPath struct {
	Path string
}

type MsgCreateWorktree struct {
	Branch string
	Path   string
}

type MsgSwitchWorktree struct {
	WorktreePath string
}

type MsgLazygitFrame struct {
	SessionID string
	Data      []byte
}

type MsgLazygitFrameClosed struct{}

type MsgDiffRendered struct {
	RequestID int
	Output    string
	Err       error
}

type MsgSyncStartup struct{}

type MsgRefreshSelectedRepo struct{}

type MsgToggleAutoPolling struct{}

type MsgSyncRefreshCompleted struct {
	WorkspaceID string
	RepoID      string
	Status      workspace.RepoStatus
	Err         error
}
