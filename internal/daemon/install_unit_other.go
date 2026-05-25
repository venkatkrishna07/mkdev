//go:build !darwin && !linux

package daemon

// UnitPath returns an error: lifecycle units aren't implemented on this OS.
func UnitPath() (string, error) { return "", ErrUnitUnsupported }

// InstallUnit returns ErrUnitUnsupported.
func InstallUnit(_ string) (string, error) { return "", ErrUnitUnsupported }

// UninstallUnit returns ErrUnitUnsupported.
func UninstallUnit() error { return ErrUnitUnsupported }

// EnableUnit returns ErrUnitUnsupported.
func EnableUnit() error { return ErrUnitUnsupported }

// DisableUnit returns ErrUnitUnsupported.
func DisableUnit() error { return ErrUnitUnsupported }

// QueryUnit returns ErrUnitUnsupported.
func QueryUnit() (UnitState, error) { return UnitState{}, ErrUnitUnsupported }
