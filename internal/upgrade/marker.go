package upgrade

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/venkatkrishna07/mkdev/internal/store"
)

const (
	metaKeyVersion = "upgrade.version"
	metaKeyPending = "upgrade.pending"
)

func dbPath(home string) string { return filepath.Join(home, "state.db") }

func withStore(home string, fn func(*store.Store) error) error {
	s, err := store.Open(dbPath(home))
	if err != nil {
		if errors.Is(err, store.ErrLocked) {
			return err
		}
		return err
	}
	defer func() { _ = s.Close() }()
	return fn(s)
}

func ReadMarker(home string) string {
	var out string
	_ = withStore(home, func(s *store.Store) error {
		v, err := s.GetMeta(metaKeyVersion)
		if err == nil {
			out = v
		}
		return nil
	})
	return out
}

func WriteMarker(home, v string) error {
	return withStore(home, func(s *store.Store) error {
		return s.PutMeta(metaKeyVersion, v)
	})
}

func WritePending(home string, res Result) error {
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return withStore(home, func(s *store.Store) error {
		return s.PutMeta(metaKeyPending, string(b))
	})
}

func ReadPending(home string) (Result, bool) {
	var res Result
	ok := false
	_ = withStore(home, func(s *store.Store) error {
		v, err := s.GetMeta(metaKeyPending)
		if err != nil || v == "" {
			return nil
		}
		if err := json.Unmarshal([]byte(v), &res); err == nil {
			ok = true
		}
		return nil
	})
	return res, ok
}

func ClearPending(home string) error {
	return withStore(home, func(s *store.Store) error {
		return s.DeleteMeta(metaKeyPending)
	})
}
