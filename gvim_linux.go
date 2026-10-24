//go:build linux

package main

import "os"

// gvimFallbackPaths returns locations to try when gvim is not on PATH.
// On Linux the package manager installs gvim on PATH, so there are none.
func gvimFallbackPaths() []string { return nil }

// gvimEnvironment makes GVim use XWayland under Hyprland when it is available.
// A native Wayland GVim window cannot resize itself; the compositor keeps the
// tiled dimensions even after the app changes its columns and lines.
func gvimEnvironment() []string {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" || os.Getenv("DISPLAY") == "" {
		return nil
	}
	debugf("gvim: using XWayland backend under Hyprland")
	return append(os.Environ(), "GDK_BACKEND=x11")
}
