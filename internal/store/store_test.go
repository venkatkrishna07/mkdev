package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

func openTest(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestRoutesPutGetDelete(t *testing.T) {
	s := openTest(t)

	r := store.Route{
		Domain:  "foo.local",
		Target:  "localhost:3000",
		TLD:     ".local",
		Enabled: true,
		Source:  "ad-hoc",
		AddedAt: time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, s.PutRoute(r))

	got, err := s.GetRoute("foo.local")
	require.NoError(t, err)
	require.Equal(t, r, got)

	all, err := s.ListRoutes()
	require.NoError(t, err)
	require.Len(t, all, 1)

	require.NoError(t, s.DeleteRoute("foo.local"))
	_, err = s.GetRoute("foo.local")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestGetRouteMissing(t *testing.T) {
	s := openTest(t)
	_, err := s.GetRoute("nope.local")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestListRoutesEmpty(t *testing.T) {
	s := openTest(t)
	all, err := s.ListRoutes()
	require.NoError(t, err)
	require.Empty(t, all)
}
