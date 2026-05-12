package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var bucketRoutes = []byte("routes")

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("store: not found")

// Store wraps a bbolt database.
type Store struct {
	db *bolt.DB
}

// Open opens (or creates) the database at path with 0600 perms.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("store: mkdir: %w", err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketRoutes)
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: init buckets: %w", err)
	}
	return &Store{db: db}, nil
}

// Close flushes and closes the database.
func (s *Store) Close() error { return s.db.Close() }
