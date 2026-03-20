package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	"github.com/allenneverland/gh-workspace/internal/store/boltdb"
)

func runInitCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	workspaceName := fs.String("workspace", "default", "workspace name")
	repoPath := fs.String("repo-path", "", "local repository path (required)")
	repoName := fs.String("repo-name", "", "display name (default: directory name)")
	defaultBranch := fs.String("default-branch", "main", "default branch name")
	releaseWorkflowRef := fs.String("release-workflow-ref", "", "workflow file path or workflow id")

	if err := fs.Parse(args); err != nil {
		return err
	}

	path := strings.TrimSpace(*repoPath)
	if path == "" {
		return errors.New("--repo-path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("repo path not accessible: %w", err)
	}

	name := strings.TrimSpace(*repoName)
	if name == "" {
		name = filepath.Base(absPath)
	}

	statePath, err := resolveStateStorePath()
	if err != nil {
		return fmt.Errorf("resolve state store path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return fmt.Errorf("create state store directory: %w", err)
	}

	stateStore, err := boltdb.Open(statePath)
	if err != nil {
		return fmt.Errorf("open state store: %w", err)
	}
	defer func() { _ = stateStore.Close() }()

	service := workspace.NewService(stateStore)
	state, err := service.LoadState()
	if err != nil {
		return fmt.Errorf("load existing state: %w", err)
	}

	wsID := findWorkspaceIDByName(state.Workspaces, strings.TrimSpace(*workspaceName))
	if wsID == "" {
		ws, err := service.CreateWorkspace(strings.TrimSpace(*workspaceName))
		if err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}
		wsID = ws.ID
	}

	existingRepoID := findRepoIDByPath(state.Workspaces, wsID, absPath)
	if existingRepoID == "" {
		repo, err := service.AddRepo(wsID, workspace.RepoInput{
			Name:               name,
			Path:               absPath,
			DefaultBranch:      strings.TrimSpace(*defaultBranch),
			ReleaseWorkflowRef: strings.TrimSpace(*releaseWorkflowRef),
		})
		if err != nil {
			return fmt.Errorf("add repo: %w", err)
		}
		existingRepoID = repo.ID
	}

	if err := service.SelectRepo(wsID, existingRepoID); err != nil {
		return fmt.Errorf("select repo: %w", err)
	}

	fmt.Printf("Initialized workspace state at %s\n", statePath)
	fmt.Printf("Workspace: %s\n", strings.TrimSpace(*workspaceName))
	fmt.Printf("Repo: %s (%s)\n", name, absPath)
	return nil
}

func findWorkspaceIDByName(workspaces []workspace.Workspace, name string) string {
	for _, ws := range workspaces {
		if ws.Name == name {
			return ws.ID
		}
	}
	return ""
}

func findRepoIDByPath(workspaces []workspace.Workspace, workspaceID, repoPath string) string {
	for _, ws := range workspaces {
		if ws.ID != workspaceID {
			continue
		}
		for _, repo := range ws.Repos {
			if repo.Path == repoPath {
				return repo.ID
			}
		}
	}
	return ""
}
