//go:build windows

package main

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
	"golang.org/x/sys/windows"
)

const (
	gwlExStyle     = ^uintptr(19) // GWL_EXSTYLE (-20) as two's-complement uintptr
	wsExToolWindow = uintptr(0x00000080)
	wsExAppWindow  = uintptr(0x00040000)

	swpNoSize       = uintptr(0x0001)
	swpNoZOrder     = uintptr(0x0004)
	swpFrameChanged = uintptr(0x0020)
	swpShowWindow   = uintptr(0x0040)

	monitorDefaultToNearest = uintptr(0x00000002)
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetWindowLongPtrW        = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW        = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procMonitorFromWindow        = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW          = user32.NewProc("GetMonitorInfoW")
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

type monitorInfo struct {
	Size     uint32
	Monitor  winRect
	WorkArea winRect
	Flags    uint32
}

// focusedWorkArea returns the work area (screen minus taskbar) of the
// monitor containing the foreground window.
func focusedWorkArea() (winRect, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	hmon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if hmon == 0 {
		return winRect{}, fmt.Errorf("MonitorFromWindow failed")
	}
	var mi monitorInfo
	mi.Size = uint32(unsafe.Sizeof(mi))
	ret, _, err := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		return winRect{}, fmt.Errorf("GetMonitorInfo failed: %w", err)
	}
	return mi.WorkArea, nil
}

func findWindowByPID(pid int, timeout time.Duration) (uintptr, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var found uintptr
		cb := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
			var procID uint32
			procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&procID)))
			if int(procID) != pid {
				return 1
			}
			vis, _, _ := procIsWindowVisible.Call(hwnd)
			if vis == 0 {
				return 1
			}
			found = hwnd
			return 0
		})
		procEnumWindows.Call(cb, 0)
		if found != 0 {
			return found, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 0, fmt.Errorf("timeout: gvim window not found for PID %d", pid)
}

// setFloatingStyle makes the window a tool window so the shell treats it as
// a floating utility window (no taskbar button).
func setFloatingStyle(hwnd uintptr) {
	exStyle, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	exStyle = (exStyle | wsExToolWindow) &^ wsExAppWindow
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, exStyle)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		swpNoSize|swpNoZOrder|swpFrameChanged|swpShowWindow)
}

func moveWindow(hwnd uintptr, x, y int) {
	procSetWindowPos.Call(hwnd, 0,
		uintptr(int32(x)), uintptr(int32(y)),
		0, 0,
		swpNoSize|swpNoZOrder|swpShowWindow)
}

func windowSize(hwnd uintptr) (int, int) {
	var r winRect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return int(r.Right - r.Left), int(r.Bottom - r.Top)
}

// glazeWMRunning reports whether GlazeWM's IPC server is reachable. The app
// no longer sends any set-floating command over IPC: floating is delegated
// to a GlazeWM window rule matching the "vilatile" window title. This
// avoids the multi-instance race where an untargeted set-floating acted on
// GlazeWM's focused window (the wrong one), which could hang GVim. We only
// probe presence so we know whether to float the window ourselves via Win32
// (GlazeWM absent).
func glazeWMRunning(port int) bool {
	url := fmt.Sprintf("ws://localhost:%d", port)
	debugf("glazeWM: probing %s", url)
	wsConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		debugf("glazeWM: not running: %v", err)
		return false
	}
	wsConn.Close()
	debugf("glazeWM: running")
	return true
}

// waitForStableSize polls GetWindowRect until the size is unchanged for
// five consecutive 50 ms intervals, then returns the stable dimensions.
func waitForStableSize(hwnd uintptr, timeout time.Duration) (int, int) {
	deadline := time.Now().Add(timeout)
	var lastW, lastH, stable int
	for time.Now().Before(deadline) {
		w, h := windowSize(hwnd)
		if w == lastW && h == lastH && w > 0 && h > 0 {
			stable++
			if stable >= 5 {
				debugf("waitForStableSize: stable at %dx%d", w, h)
				return w, h
			}
		} else {
			stable = 0
			lastW, lastH = w, h
		}
		time.Sleep(50 * time.Millisecond)
	}
	debugf("waitForStableSize: timeout, last=%dx%d", lastW, lastH)
	return lastW, lastH
}

func placeWindow(pid int, cfg *Config) error {
	workArea, err := focusedWorkArea()
	if err != nil {
		return fmt.Errorf("get monitor: %w", err)
	}
	debugf("placeWindow: pid=%d workArea=%+v", pid, workArea)

	// Floating is delegated to a GlazeWM window rule (matching the
	// "vilatile" title) when GlazeWM is running; the app sends no IPC.
	glazeActive := glazeWMRunning(cfg.GlazeWMPort)
	debugf("placeWindow: glazeActive=%v", glazeActive)

	hwnd, err := findWindowByPID(pid, 5*time.Second)
	if err != nil {
		return err
	}
	debugf("placeWindow: hwnd=0x%x", hwnd)

	// When GlazeWM is not running, float the window ourselves via Win32;
	// otherwise its window rule floats it and init.vim's timer sets the size.
	if !glazeActive {
		setFloatingStyle(hwnd)
	}

	place := func(w, h int) {
		x := resolvePosition(cfg.Position.X,
			int(workArea.Left), int(workArea.Right), w)
		y := resolvePosition(cfg.Position.Y,
			int(workArea.Top), int(workArea.Bottom), h)
		debugf("placeWindow: place size=%dx%d pos=(%d,%d)", w, h, x, y)
		moveWindow(hwnd, x, y)
	}

	// Wait for gvim to finish resizing (init.vim's timer sets columns/lines).
	winW, winH := waitForStableSize(hwnd, 3*time.Second)
	place(winW, winH)

	// The size can change again after the first placement (floating placement
	// override, then gvim re-asserting its size), which invalidates a
	// right/bottom-anchored position. Watch briefly and re-place on change.
	lastW, lastH := winW, winH
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		w, h := windowSize(hwnd)
		if w != lastW || h != lastH {
			lastW, lastH = waitForStableSize(hwnd, 2*time.Second)
			place(lastW, lastH)
		}
	}
	debugf("placeWindow: done")
	return nil
}
