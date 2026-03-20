package workspace

import (
	"fmt"
	"strings"
)

const (
	LocalWorkspaceID   = "__local_internal__"
	LocalWorkspaceName = "__local_internal__"
)

func isCanonicalLocalWorkspace(ws Workspace) bool {
	return ws.ID == LocalWorkspaceID && ws.Name == LocalWorkspaceName
}

func collidesWithLocalWorkspace(ws Workspace) bool {
	return ws.ID == LocalWorkspaceID || ws.Name == LocalWorkspaceName
}

func localWorkspaceLegacyBase(ws Workspace) string {
	name := strings.TrimSpace(ws.Name)
	id := strings.TrimSpace(ws.ID)
	switch {
	case name != "" && name != LocalWorkspaceName:
		return name
	case id != "" && id != LocalWorkspaceID:
		return id
	case name != "":
		return name
	case id != "":
		return id
	default:
		return "workspace"
	}
}

func localWorkspaceLegacyName(base string, suffix int) string {
	return fmt.Sprintf("%s-legacy-%d", base, suffix)
}
