package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var debugFile *os.File

// openDebugLog enables debugf output (-debug flag), appending to
// vilatile_debug.log in the temp directory.
func openDebugLog() {
	f, err := os.OpenFile(filepath.Join(os.TempDir(), "vilatile_debug.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	debugFile = f
	debugf("=== vilatile start ===")
}

// debugf writes a timestamped line to the debug log; no-op unless -debug.
func debugf(format string, args ...any) {
	if debugFile == nil {
		return
	}
	fmt.Fprintf(debugFile, time.Now().Format("15:04:05.000")+" "+format+"\n", args...)
}
