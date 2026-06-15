//go:build darwin || windows

package bar

import "fyne.io/systray"

func setAppTooltip(s string) { systray.SetTooltip(s) }
