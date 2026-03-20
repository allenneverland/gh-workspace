package sync

import (
	"context"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

type NoopSelectedRepoStatusFetcher struct{}

func (NoopSelectedRepoStatusFetcher) FetchSelectedRepoStatus(context.Context, string, string) (workspace.RepoStatus, error) {
	return workspace.RepoStatus{}, nil
}
