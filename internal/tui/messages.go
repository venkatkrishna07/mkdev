package tui

import (
	"github.com/venkatkrishna07/mkdev/internal/tui/msg"
)

// RoutesRefreshed is dispatched when a fresh snapshot of the routes table
// has been pulled from the bbolt store. Aliased here so the root package
// continues to expose it while the canonical type lives in tui/msg to break
// the import cycle with tui/tabs.
type RoutesRefreshed = msg.RoutesRefreshed

// ProxyState carries the latest proxy liveness signal. See RoutesRefreshed
// for the rationale of the alias.
type ProxyState = msg.ProxyState
