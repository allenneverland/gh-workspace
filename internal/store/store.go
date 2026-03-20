package store

import (
	"context"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

type Store interface {
	Load(context.Context) (workspace.State, error)
	Save(context.Context, workspace.State) error
}
