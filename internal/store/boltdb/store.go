package boltdb

import (
	"context"
	"encoding/json"
	"time"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
	storepkg "github.com/allenneverland/gh-workspace/internal/store"
	bolt "go.etcd.io/bbolt"
)

var _ storepkg.Store = (*Store)(nil)

type Store struct {
	db *bolt.DB
}

func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Load(ctx context.Context) (workspace.State, error) {
	var state workspace.State

	if err := s.db.View(func(tx *bolt.Tx) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		bucket := tx.Bucket(bucketState)
		if bucket == nil {
			return errStateBucketMissing
		}

		raw := bucket.Get(keyCurrent)
		if len(raw) == 0 {
			return nil
		}
		return json.Unmarshal(raw, &state)
	}); err != nil {
		return workspace.State{}, err
	}

	return state, nil
}

func (s *Store) Save(ctx context.Context, state workspace.State) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		bucket := tx.Bucket(bucketState)
		if bucket == nil {
			return errStateBucketMissing
		}

		raw, err := json.Marshal(state)
		if err != nil {
			return err
		}
		return bucket.Put(keyCurrent, raw)
	})
}
