package daemon

import (
	"errors"
	"os"
)

// UnitState reports whether the OS user-service for the daemon is
// installed/loaded. Returned by Status() in lifecycle commands.
type UnitState struct {
	Installed bool   // unit file present on disk
	Loaded    bool   // OS supervisor has loaded the unit (launchctl/systemctl)
	Path      string // absolute path of the unit file (when known)
	Note      string // platform-specific advisory note (e.g. "linux user units require lingering")
}

// ErrUnitUnsupported is returned by Install/Uninstall/Enable/Disable when the
// current OS has no implementation (e.g. Windows in this release).
var ErrUnitUnsupported = errors.New("daemon: lifecycle units not supported on this platform")

// fileExists is a small helper used by the platform implementations.
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
