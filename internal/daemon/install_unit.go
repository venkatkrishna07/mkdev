package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	svc "github.com/kardianos/service"
)

const (
	serviceName    = "sh.mkdev.daemon"
	serviceDisplay = "mkdev daemon"
	serviceDesc    = "Local HTTPS reverse proxy + management API for mkdev"
)

type UnitState struct {
	Installed bool
	Loaded    bool
	Path      string
	Note      string
}

var ErrUnitUnsupported = errors.New("daemon: lifecycle units not supported in this environment")

func newService(exePath string) (svc.Service, error) {
	if exePath == "" {
		if p, err := os.Executable(); err == nil {
			exePath = p
		}
	}
	cfg := &svc.Config{
		Name:        serviceName,
		DisplayName: serviceDisplay,
		Description: serviceDesc,
		Arguments:   []string{"daemon", "serve"},
		Executable:  exePath,
		Option: svc.KeyValue{
			"UserService": true,
			"KeepAlive":   true,
			"RunAtLoad":   true,
		},
	}
	s, err := svc.New(noopProgram{}, cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnitUnsupported, err)
	}
	return s, nil
}

type noopProgram struct{}

func (noopProgram) Start(_ svc.Service) error { return nil }
func (noopProgram) Stop(_ svc.Service) error  { return nil }

func UnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "LaunchAgents", serviceName+".plist"), nil
	case "linux":
		return filepath.Join(home, ".config", "systemd", "user", serviceName+".service"), nil
	default:
		return "", nil
	}
}

func InstallUnit(exePath string) (string, error) {
	s, err := newService(exePath)
	if err != nil {
		return "", err
	}
	if err := s.Install(); err != nil {
		return "", fmt.Errorf("daemon: install: %w", err)
	}
	path, _ := UnitPath()
	return path, nil
}

func UninstallUnit() error {
	s, err := newService("")
	if err != nil {
		return err
	}
	if err := s.Uninstall(); err != nil && !isAbsentError(err) {
		return fmt.Errorf("daemon: uninstall: %w", err)
	}
	return nil
}

func EnableUnit() error {
	s, err := newService("")
	if err != nil {
		return err
	}
	if err := s.Start(); err != nil {
		return fmt.Errorf("daemon: enable: %w", err)
	}
	return nil
}

func DisableUnit() error {
	s, err := newService("")
	if err != nil {
		return err
	}
	if err := s.Stop(); err != nil && !isAbsentError(err) {
		return fmt.Errorf("daemon: disable: %w", err)
	}
	return nil
}

func QueryUnit() (UnitState, error) {
	st := UnitState{}
	st.Path, _ = UnitPath()
	s, err := newService("")
	if err != nil {
		return st, err
	}
	status, err := s.Status()
	if err != nil {
		if isAbsentError(err) {
			return st, nil
		}
		return st, fmt.Errorf("daemon: status: %w", err)
	}
	st.Installed = true
	st.Loaded = status == svc.StatusRunning
	if runtime.GOOS == "linux" {
		st.Note = "user services may need `loginctl enable-linger $USER` to run when not logged in"
	}
	return st, nil
}

func isAbsentError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, svc.ErrNotInstalled) {
		return true
	}

	msg := err.Error()
	return strings.Contains(msg, "not installed") || strings.Contains(msg, "does not exist")
}
