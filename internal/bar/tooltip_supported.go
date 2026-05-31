//go:build darwin || windows

package bar

import "github.com/getlantern/systray"

func setAppTooltip(s string) { systray.SetTooltip(s) }
