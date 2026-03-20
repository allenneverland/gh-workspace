package app

type MsgSelectWorkspace struct {
	WorkspaceID string
}

type MsgSelectRepo struct {
	RepoID string
}

type MsgSetActiveTab struct {
	Tab Tab
}

type MsgRequestAddRepo struct{}

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
