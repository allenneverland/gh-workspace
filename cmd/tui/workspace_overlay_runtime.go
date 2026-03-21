package main

import (
	"context"
	"path/filepath"

	repositoryadapter "github.com/allenneverland/gh-workspace/internal/adapters/repository"
	"github.com/allenneverland/gh-workspace/internal/app"
)

type runtimeWorkspaceOverlayScanner struct{}

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
