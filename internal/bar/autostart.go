package bar

// AutostartEnabled reports whether the bar is configured to launch on login.
func AutostartEnabled() bool { return autostartEnabled() }

// InstallAutostart registers the bar to launch on user login.
func InstallAutostart() error { return installAutostart() }

// UninstallAutostart removes the login-launch registration.
func UninstallAutostart() error { return uninstallAutostart() }
