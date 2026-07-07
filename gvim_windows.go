//go:build windows

package main

import (
	"os"
	"path/filepath"
)

// gvimFallbackPaths returns locations to try when gvim is not on PATH:
// the standard Vim installer directories, Chocolatey, and Scoop.
func gvimFallbackPaths() []string {
	var paths []string
	for _, base := range []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("LOCALAPPDATA"),
	} {
		if base == "" {
			continue
		}
		vimDir := filepath.Join(base, "Vim")
		entries, err := os.ReadDir(vimDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				paths = append(paths, filepath.Join(vimDir, e.Name(), "gvim.exe"))
			}
		}
	}
	paths = append(paths,
		filepath.Join(os.Getenv("ChocolateyInstall"), "bin", "gvim.exe"),
		filepath.Join(os.Getenv("USERPROFILE"), "scoop", "apps", "vim", "current", "gvim.exe"),
	)
	return paths
}
