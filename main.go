// Command vilatile opens a temporary floating GVim window for quick
// text editing. The buffer is scratch-only: writing it (:w) copies the
// contents to the system clipboard instead of saving a file. Text piped to
// stdin becomes the initial buffer contents.
//
// The code is organized as one file per concern, with per-OS variants in
// <concern>_GOOS.go files:
//
//	config.go   configuration file and defaults
//	gvim.go     locating and running gvim
//	window.go   floating and positioning the gvim window
//	debug.go    opt-in debug log (-debug)
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			reportError(fmt.Sprintf("panic: %v\n%s", r, debug.Stack()))
			os.Exit(1)
		}
	}()

	var (
		configPath string
		gvimPath   string
		columns    int
		lines      int
		posX       string
		posY       string
		debugLog   bool
	)

	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.StringVar(&gvimPath, "gvim-path", "", "path to gvim executable")
	flag.IntVar(&columns, "columns", 0, "window width in columns (overrides config)")
	flag.IntVar(&lines, "lines", 0, "window height in lines (overrides config)")
	flag.StringVar(&posX, "x", "", "x position: left/right/center/number (overrides config)")
	flag.StringVar(&posY, "y", "", "y position: top/bottom/center/number (overrides config)")
	flag.BoolVar(&debugLog, "debug", false, "write debug log to vilatile_debug.log in the temp directory")
	flag.Parse()

	if debugLog {
		openDebugLog()
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		reportError(fmt.Sprintf("error loading config: %v", err))
		os.Exit(1)
	}

	// Flags override config file values.
	if gvimPath != "" {
		cfg.GvimPath = gvimPath
	}
	if columns != 0 {
		cfg.Columns = columns
	}
	if lines != 0 {
		cfg.Lines = lines
	}
	if posX != "" {
		cfg.Position.X = posX
	}
	if posY != "" {
		cfg.Position.Y = posY
	}

	if err := runGvim(cfg, readStdin()); err != nil {
		reportError(fmt.Sprintf("error: %v", err))
		os.Exit(1)
	}
}

// readStdin returns text piped to stdin, or "" when stdin is a terminal.
func readStdin() string {
	stat, err := os.Stdin.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice != 0 {
		return ""
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return ""
	}
	return string(data)
}
