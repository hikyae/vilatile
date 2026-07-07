//go:build linux

package main

// gvimFallbackPaths returns locations to try when gvim is not on PATH.
// On Linux the package manager installs gvim on PATH, so there are none.
func gvimFallbackPaths() []string { return nil }
