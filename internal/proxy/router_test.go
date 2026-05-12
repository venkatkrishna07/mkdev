package proxy_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/proxy"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

func TestRouterLookup(t *testing.T) {
	r := proxy.NewRouter()
	r.Set([]store.Route{
		{Domain: "foo.local", Target: "localhost:3000", Enabled: true},
		{Domain: "bar.local", Target: "localhost:4000", Enabled: true},
		{Domain: "off.local", Target: "localhost:5000", Enabled: false},
	})
	target, ok := r.Lookup("foo.local")
	require.True(t, ok)
	require.Equal(t, "localhost:3000", target)

	_, ok = r.Lookup("nope.local")
	require.False(t, ok)

	_, ok = r.Lookup("off.local")
	require.False(t, ok, "disabled routes must not resolve")
}

func TestRouterLookupProxy(t *testing.T) {
	r := proxy.NewRouter()
	r.Set([]store.Route{{Domain: "foo.local", Target: "localhost:3000", Enabled: true}})
	rp, ok := r.LookupProxy("foo.local")
	require.True(t, ok)
	require.NotNil(t, rp)

	_, ok = r.LookupProxy("missing.local")
	require.False(t, ok)
}

func TestRouterLookupCaseInsensitive(t *testing.T) {
	r := proxy.NewRouter()
	r.Set([]store.Route{{Domain: "Foo.Local", Target: "localhost:3000", Enabled: true}})
	target, ok := r.Lookup("FOO.local")
	require.True(t, ok)
	require.Equal(t, "localhost:3000", target)
}

func TestRouterHotReload(t *testing.T) {
	r := proxy.NewRouter()
	r.Set([]store.Route{{Domain: "foo.local", Target: "old", Enabled: true}})
	r.Set([]store.Route{{Domain: "foo.local", Target: "new", Enabled: true}})
	target, ok := r.Lookup("foo.local")
	require.True(t, ok)
	require.Equal(t, "new", target)
}
