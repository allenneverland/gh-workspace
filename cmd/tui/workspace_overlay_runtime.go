package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	repositoryadapter "github.com/allenneverland/gh-workspace/internal/adapters/repository"
	"github.com/allenneverland/gh-workspace/internal/app"
	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	storepkg "github.com/allenneverland/gh-workspace/internal/store"
)

type runtimeWorkspaceOverlayScanner struct{}

func init() {
	app.DefaultWorkspaceOverlayDraftCommitterFactory = func(store storepkg.Store) app.WorkspaceOverlayDraftCommitter {
		if store == nil {
			return nil
		}
		return runtimeWorkspaceOverlayDraftCommitter{stateStore: store}
	}
}

func (runtimeWorkspaceOverlayScanner) ScanRepoCandidates(ctx context.Context, rootPath string) ([]app.RepoCandidate, error) {
	repoRoots, err := repositoryadapter.DiscoverRepoRoots(ctx, rootPath)
	if err != nil {
		return nil, err
	}

	candidates := make([]app.RepoCandidate, 0, len(repoRoots))
	for _, repoRoot := range repoRoots {
		candidates = append(candidates, app.RepoCandidate{
			Name: filepath.Base(repoRoot),
			Path: repoRoot,
		})
	}
	return candidates, nil
}

type runtimeWorkspaceOverlayDraftCommitter struct {
	stateStore workspace.StateStore
}

func (c runtimeWorkspaceOverlayDraftCommitter) CommitWorkspaceOverlayDraft(ctx context.Context, draft app.WorkspaceOverlayDraft) (workspace.State, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.stateStore == nil {
		return workspace.State{}, errors.New("state store not configured")
	}

	svc := workspace.NewService(c.stateStore)
	name := strings.TrimSpace(draft.Name)
	if _, found, err := svc.FindWorkspaceByName(name, false); err != nil {
		return workspace.State{}, err
	} else if found {
		return workspace.State{}, errors.New("workspace already exists")
	}

	created, err := svc.CreateWorkspace(name)
	if err != nil {
		return workspace.State{}, err
	}

	firstRepoID := ""
	for _, candidate := range draft.StagedRepos {
		repoName := strings.TrimSpace(candidate.Name)
		repoPath := strings.TrimSpace(candidate.Path)
		if repoName == "" {
			repoName = repoNameFromPath(repoPath)
		}

		repo, err := svc.AddRepo(created.ID, workspace.RepoInput{
			Name: repoName,
			Path: repoPath,
		})
		if err != nil {
			return workspace.State{}, err
		}
		if firstRepoID == "" {
			firstRepoID = repo.ID
		}
	}

	if firstRepoID != "" {
		if err := svc.SelectRepo(created.ID, firstRepoID); err != nil {
			return workspace.State{}, err
		}
	} else {
		if err := selectWorkspace(ctx, c.stateStore, created.ID); err != nil {
			return workspace.State{}, err
		}
	}

	state, err := c.stateStore.Load(ctx)
	if err != nil {
		return workspace.State{}, err
	}
	return state, nil
}
