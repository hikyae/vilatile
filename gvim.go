package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
)

// init.vim configures the scratch buffer, minimal UI, and copy-to-clipboard
// on :w. It is written to a temp file and sourced with -S at startup.
//
//go:embed init.vim
var initVimScript []byte

// findGvim returns the gvim executable to run: the configured path if set,
// otherwise PATH, otherwise the platform's fallback locations
// (gvimFallbackPaths in gvim_GOOS.go).
func findGvim(cfgPath string) (string, error) {
	if cfgPath != "" {
		if _, err := os.Stat(cfgPath); err != nil {
			return "", fmt.Errorf("gvim not found at %s: %w", cfgPath, err)
		}
		return cfgPath, nil
	}
	if path, err := exec.LookPath("gvim"); err == nil {
		return path, nil
	}
	for _, path := range gvimFallbackPaths() {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("gvim not found: install Vim or set gvim_path in config.yaml")
}

func writeTempFile(pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// runGvim starts gvim, floats and positions its window, and waits for it to
// exit. Text in initialText (piped stdin) is loaded as the buffer contents.
func runGvim(cfg *Config, initialText string) error {
	gvimPath, err := findGvim(cfg.GvimPath)
	if err != nil {
		return err
	}

	scriptPath, err := writeTempFile("vilatile_init_*.vim", initVimScript)
	if err != nil {
		return fmt.Errorf("failed to write init.vim: %w", err)
	}
	defer os.Remove(scriptPath)

	// init.vim applies the size from a timer; only pass the values.
	sizeCmd := fmt.Sprintf("let g:vilatile_columns=%d | let g:vilatile_lines=%d",
		cfg.Columns, cfg.Lines)

	// -f: prevent gvim from forking on GUI start; otherwise the parent exits
	// immediately, Wait() returns, and the init script is removed before the
	// forked child sources it (E484). Also keeps Pid valid for window matching.
	args := []string{"-f", "-S", scriptPath, "-c", sizeCmd}

	if initialText != "" {
		textPath, err := writeTempFile("vilatile_stdin_*", []byte(initialText))
		if err != nil {
			return fmt.Errorf("failed to write stdin temp: %w", err)
		}
		defer os.Remove(textPath)
		args = append(args, textPath)
	}

	cmd := exec.Command(gvimPath, args...)
	cmd.Env = gvimEnvironment()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start gvim: %w", err)
	}

	if err := placeWindow(cmd.Process.Pid, cfg); err != nil {
		debugf("placeWindow: %v", err)
		fmt.Fprintf(os.Stderr, "warning: window placement: %v\n", err)
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return fmt.Errorf("gvim exited with code %d", exitErr.ExitCode())
		}
	}
	return nil
}
