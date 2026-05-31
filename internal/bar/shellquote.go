//go:build darwin

package bar

import "strings"

// shellJoin joins argv into a single POSIX shell command line, single-quoting
// each arg so paths with spaces or shell metacharacters are safe.
func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(parts, " ")
}
