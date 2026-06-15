// Package clipboard writes text to the user's system clipboard.
package clipboard

// Copy writes s to the system clipboard.
func Copy(s string) error { return copyText(s) }
