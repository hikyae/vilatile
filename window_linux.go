//go:build linux

package main

import (
	"encoding/json"
	"fmt"
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

// swaymsgJSON runs `swaymsg -t <msgType>` and decodes the JSON reply.
func swaymsgJSON(msgType string, out any) error {
	data, err := exec.Command("swaymsg", "-t", msgType).Output()
	if err != nil {
		return fmt.Errorf("swaymsg -t %s: %w", msgType, err)
	}
	return json.Unmarshal(data, out)
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
