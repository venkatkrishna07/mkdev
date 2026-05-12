// Package msg holds Bubble Tea messages shared between the root TUI and its
// subpackages. Keeping these in a leaf package avoids an import cycle between
// internal/tui and internal/tui/tabs.
package msg

import (
	"github.com/venkatkrishna07/mkdev/internal/store"
)

// RoutesRefreshed is dispatched when a fresh snapshot of the routes table
// has been pulled from the bbolt store.
type RoutesRefreshed struct {
	Routes []store.Route
	Err    error
}

// ProxyState carries the latest proxy liveness signal.
type ProxyState struct {
	Up   bool
	Addr string
	Err  error
}
