//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

type swayRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type swayWorkspace struct {
	Focused bool   `json:"focused"`
	Output  string `json:"output"`
}

type swayOutput struct {
	Name   string   `json:"name"`
	Rect   swayRect `json:"rect"`
	Active bool     `json:"active"`
}

type swayNode struct {
	PID           int        `json:"pid"`
	Rect          swayRect   `json:"rect"`
	DecoRect      swayRect   `json:"deco_rect"`
	Nodes         []swayNode `json:"nodes"`
	FloatingNodes []swayNode `json:"floating_nodes"`
}

type hyprlandMonitor struct {
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Scale   float64 `json:"scale"`
	Focused bool    `json:"focused"`
}

type hyprlandClient struct {
	Address string `json:"address"`
	PID     int    `json:"pid"`
	Mapped  bool   `json:"mapped"`
	Hidden  bool   `json:"hidden"`
	Size    []int  `json:"size"`
}

// swaymsgJSON runs `swaymsg -t <msgType>` and decodes the JSON reply.
func swaymsgJSON(msgType string, out any) error {
	data, err := exec.Command("swaymsg", "-t", msgType).Output()
	if err != nil {
		return fmt.Errorf("swaymsg -t %s: %w", msgType, err)
	}
	return json.Unmarshal(data, out)
}

// hyprctlJSON runs `hyprctl -j <command>` and decodes the JSON reply.
func hyprctlJSON(command string, out any) error {
	data, err := exec.Command("hyprctl", "-j", command).Output()
	if err != nil {
		return fmt.Errorf("hyprctl -j %s: %w", command, err)
	}
	return json.Unmarshal(data, out)
}

// focusedHyprlandOutput returns the focused monitor bounds in Hyprland's
// logical coordinates, which are the coordinates used by its dispatchers.
func focusedHyprlandOutput() (hyprlandMonitor, error) {
	var monitors []hyprlandMonitor
	if err := hyprctlJSON("monitors", &monitors); err != nil {
		return hyprlandMonitor{}, err
	}
	for _, monitor := range monitors {
		if monitor.Focused {
			return monitor, nil
		}
	}
	if len(monitors) > 0 {
		return monitors[0], nil
	}
	return hyprlandMonitor{}, fmt.Errorf("no active monitor found")
}

func hyprlandWindowByPID(pid int) (*hyprlandClient, error) {
	var clients []hyprlandClient
	if err := hyprctlJSON("clients", &clients); err != nil {
		return nil, err
	}
	for i := range clients {
		if clients[i].PID == pid && clients[i].Mapped && !clients[i].Hidden {
			return &clients[i], nil
		}
	}
	return nil, nil
}

func waitForHyprlandWindow(pid int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		client, err := hyprlandWindowByPID(pid)
		if err == nil && client != nil {
			return client.Address, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", fmt.Errorf("timeout: gvim window not found for PID %d", pid)
}

func waitForStableHyprlandWindow(pid int, timeout time.Duration) (hyprlandClient, error) {
	deadline := time.Now().Add(timeout)
	var lastW, lastH, stable int
	for time.Now().Before(deadline) {
		winW, winH := 0, 0
		client, err := hyprlandWindowByPID(pid)
		if err == nil && client != nil && len(client.Size) == 2 {
			winW, winH = client.Size[0], client.Size[1]
		}
		if winW > 0 && winH > 0 && winW == lastW && winH == lastH {
			stable++
			if stable >= 3 {
				return *client, nil
			}
		} else {
			stable = 0
			lastW, lastH = winW, winH
		}
		time.Sleep(50 * time.Millisecond)
	}
	return hyprlandClient{}, fmt.Errorf("timeout: window size not stable for PID %d", pid)
}

func placeHyprlandWindow(pid int, cfg *Config) error {
	monitor, err := focusedHyprlandOutput()
	if err != nil {
		return fmt.Errorf("get output rect: %w", err)
	}
	scale := monitor.Scale
	if scale <= 0 {
		scale = 1
	}
	monitorWidth := int(float64(monitor.Width) / scale)
	monitorHeight := int(float64(monitor.Height) / scale)
	debugf("placeWindow: pid=%d hyprland output=(%d,%d %dx%d scale=%g)",
		pid, monitor.X, monitor.Y, monitorWidth, monitorHeight, scale)

	address, err := waitForHyprlandWindow(pid, 5*time.Second)
	if err != nil {
		return err
	}
	if out, err := exec.Command("hyprctl", "dispatch", "setfloating", "address:"+address).CombinedOutput(); err != nil {
		return fmt.Errorf("hyprctl setfloating: %w: %s", err, out)
	}

	client, err := waitForStableHyprlandWindow(pid, 5*time.Second)
	if err != nil {
		return fmt.Errorf("get stable window size: %w", err)
	}
	winW, winH := client.Size[0], client.Size[1]
	x := resolvePosition(cfg.Position.X, monitor.X, monitor.X+monitorWidth, winW)
	y := resolvePosition(cfg.Position.Y, monitor.Y, monitor.Y+monitorHeight, winH)
	debugf("placeWindow: size=%dx%d pos=(%d,%d)", winW, winH, x, y)

	moveArgs := fmt.Sprintf("exact %d %d,address:%s", x, y, address)
	if out, err := exec.Command("hyprctl", "dispatch", "movewindowpixel", moveArgs).CombinedOutput(); err != nil {
		return fmt.Errorf("hyprctl move: %w: %s", err, out)
	}
	debugf("placeWindow: done")
	return nil
}

// focusedOutputRect returns the rectangle of the output (monitor) holding
// the focused workspace, falling back to any active output.
func focusedOutputRect() (swayRect, error) {
	var workspaces []swayWorkspace
	if err := swaymsgJSON("get_workspaces", &workspaces); err != nil {
		return swayRect{}, err
	}
	outputName := ""
	for _, ws := range workspaces {
		if ws.Focused {
			outputName = ws.Output
			break
		}
	}

	var outputs []swayOutput
	if err := swaymsgJSON("get_outputs", &outputs); err != nil {
		return swayRect{}, err
	}
	for _, o := range outputs {
		if o.Name == outputName {
			return o.Rect, nil
		}
	}
	for _, o := range outputs {
		if o.Active {
			return o.Rect, nil
		}
	}
	return swayRect{}, fmt.Errorf("no active output found")
}

func findNodeByPID(node swayNode, pid int) *swayNode {
	if node.PID == pid {
		return &node
	}
	for _, child := range node.Nodes {
		if found := findNodeByPID(child, pid); found != nil {
			return found
		}
	}
	for _, child := range node.FloatingNodes {
		if found := findNodeByPID(child, pid); found != nil {
			return found
		}
	}
	return nil
}

func waitForSwayWindow(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var root swayNode
		if swaymsgJSON("get_tree", &root) == nil {
			if findNodeByPID(root, pid) != nil {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout: gvim window not found for PID %d", pid)
}

func waitForStableSwayWindow(pid int, timeout time.Duration) (int, int, error) {
	deadline := time.Now().Add(timeout)
	var lastW, lastH, stable int
	for time.Now().Before(deadline) {
		winW, winH := 0, 0
		var root swayNode
		if err := swaymsgJSON("get_tree", &root); err == nil {
			node := findNodeByPID(root, pid)
			if node != nil && node.Rect.Width > 0 && node.Rect.Height > 0 {
				winW = node.Rect.Width
				winH = node.Rect.Height + node.DecoRect.Height
			}
		}
		if winW > 0 && winH > 0 && winW == lastW && winH == lastH {
			stable++
			if stable >= 3 {
				return winW, winH, nil
			}
		} else {
			stable = 0
			lastW, lastH = winW, winH
		}
		time.Sleep(50 * time.Millisecond)
	}
	return 0, 0, fmt.Errorf("timeout: window size not stable for PID %d", pid)
}

func placeWindow(pid int, cfg *Config) error {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		return placeHyprlandWindow(pid, cfg)
	}

	outputRect, err := focusedOutputRect()
	if err != nil {
		return fmt.Errorf("get output rect: %w", err)
	}
	debugf("placeWindow: pid=%d outputRect=%+v", pid, outputRect)

	if err := waitForSwayWindow(pid, 5*time.Second); err != nil {
		return err
	}

	if out, err := exec.Command("swaymsg",
		fmt.Sprintf("[pid=%d] floating enable", pid),
	).CombinedOutput(); err != nil {
		return fmt.Errorf("swaymsg floating enable: %w: %s", err, out)
	}

	winW, winH, err := waitForStableSwayWindow(pid, 5*time.Second)
	if err != nil {
		return fmt.Errorf("get stable window size: %w", err)
	}

	x := resolvePosition(cfg.Position.X, outputRect.X, outputRect.X+outputRect.Width, winW)
	y := resolvePosition(cfg.Position.Y, outputRect.Y, outputRect.Y+outputRect.Height, winH)
	debugf("placeWindow: size=%dx%d pos=(%d,%d)", winW, winH, x, y)

	moveCmd := fmt.Sprintf("[pid=%d] move position %s %s",
		pid, strconv.Itoa(x), strconv.Itoa(y))
	if out, err := exec.Command("swaymsg", moveCmd).CombinedOutput(); err != nil {
		return fmt.Errorf("swaymsg move: %w: %s", err, out)
	}
	debugf("placeWindow: done")

	return nil
}
