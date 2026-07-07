//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procMessageBoxW = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")

// reportError shows a message box: the binary is built with -H windowsgui,
// so there is no console to print to.
func reportError(msg string) {
	title, _ := windows.UTF16PtrFromString("vilatile")
	text, _ := windows.UTF16PtrFromString(msg)
	procMessageBoxW.Call(0,
		uintptr(unsafe.Pointer(text)),
		uintptr(unsafe.Pointer(title)),
		0x10) // MB_ICONERROR
}
