package main

import (
	"strconv"
	"strings"
)

// Each platform implements window management in window_GOOS.go:
//
//	placeWindow(pid int, cfg *Config) error
//		Float the gvim window owned by pid and move it to the configured
//		position on the focused monitor. Called right after gvim starts.
//
// Windows uses the Win32 API (and detects GlazeWM, which floats the window
// via a window rule instead); Linux uses swaymsg or Hyprland's hyprctl.

// resolvePosition converts one axis of a position spec to a screen
// coordinate. spec is "left"/"top" (start), "right"/"bottom" (end),
// "center", or a pixel offset from the start; [lo, hi) is the monitor
// range on that axis and size is the window extent.
func resolvePosition(spec string, lo, hi, size int) int {
	switch strings.ToLower(spec) {
	case "left", "top":
		return lo
	case "right", "bottom":
		return hi - size
	case "center":
		return lo + (hi-lo-size)/2
	default:
		n, err := strconv.Atoi(spec)
		if err != nil {
			return lo
		}
		return lo + n
	}
}
