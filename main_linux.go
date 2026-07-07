//go:build linux

package main

import (
	"fmt"
	"os"
)

func reportError(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}
