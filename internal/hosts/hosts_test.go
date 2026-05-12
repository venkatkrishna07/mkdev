package hosts_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
)

func TestParseSkipsCommentsAndBlanks(t *testing.T) {
	in := `# header
127.0.0.1   foo.local
   # indented comment
::1         bar.local
`
	got := hosts.Parse(strings.NewReader(in))
	require.Equal(t, []hosts.Entry{
		{IP: "127.0.0.1", Host: "foo.local"},
		{IP: "::1", Host: "bar.local"},
	}, got)
}

func TestAddIdempotent(t *testing.T) {
	existing := "127.0.0.1 foo.local\n"
	out, changed := hosts.AddEntry(existing, "127.0.0.1", "foo.local", "managed by mkdev")
	require.False(t, changed)
	require.Equal(t, existing, out)
}

func TestAddAppendsWhenMissing(t *testing.T) {
	out, changed := hosts.AddEntry("127.0.0.1 foo.local\n", "127.0.0.1", "bar.local", "managed by mkdev")
	require.True(t, changed)
	require.Contains(t, out, "127.0.0.1\tbar.local\t# managed by mkdev")
}

func TestRemoveDropsLine(t *testing.T) {
	in := "127.0.0.1 foo.local\n127.0.0.1\tbar.local\t# managed by mkdev\n"
	out, changed := hosts.RemoveEntry(in, "bar.local")
	require.True(t, changed)
	require.Equal(t, "127.0.0.1 foo.local\n", out)
}

func TestRemoveNoopWhenAbsent(t *testing.T) {
	in := "127.0.0.1 foo.local\n"
	out, changed := hosts.RemoveEntry(in, "missing.local")
	require.False(t, changed)
	require.Equal(t, in, out)
}

func TestValidHostnameStrict(t *testing.T) {
	require.True(t, hosts.ValidHostname("foo.local"))
	require.True(t, hosts.ValidHostname("api.foo.local"))
	require.False(t, hosts.ValidHostname(""))
	require.False(t, hosts.ValidHostname("-bad.local"))
	require.False(t, hosts.ValidHostname("bad..local"))
	require.False(t, hosts.ValidHostname("evil; rm -rf /"))
}
