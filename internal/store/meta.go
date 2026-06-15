package store

import (
	"fmt"

	bolt "go.etcd.io/bbolt"
)

var bucketMeta = []byte("meta")

func (s *Store) GetMeta(key string) (string, error) {
	var out string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		if b == nil {
			return nil
		}
		v := b.Get([]byte(key))
		if v == nil {
			return nil
		}
		out = string(v)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("store: get meta %q: %w", key, err)
	}
	return out, nil
}

func (s *Store) PutMeta(key, value string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketMeta)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), []byte(value))
	})
}

func (s *Store) DeleteMeta(key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
}
