package boltdb

import (
	"errors"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketState = []byte("state")
	keyCurrent  = []byte("current")

	errStoreNotInitialized = errors.New("boltdb: store not initialized")
	errStateBucketMissing  = errors.New("boltdb: state bucket missing")
)

func (s *Store) migrate() error {
	if s == nil || s.db == nil {
		return errStoreNotInitialized
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketState)
		return err
	})
}
