# vilatile

Temporary floating GVim for quick text editing.
The buffer is scratch-only: writing it (`:w`) copies the contents to the
system clipboard instead of saving a file.

Works on Windows (optionally with [GlazeWM](https://github.com/glzr-io/glazewm))
and Linux ([Sway](https://swaywm.org/)).

## Features

- Opens GVim as a small floating window at a configurable position
  (e.g. bottom-right of the current monitor)
- `:w` copies the buffer to the clipboard (no file is written)
- Text piped to stdin is loaded as the initial buffer contents
- Minimal GVim UI (no menu, toolbar, scrollbars, statusline)
- Safe to run multiple instances concurrently

## Build

```sh
make windows   # vilatile.exe (cross-compiled)
make linux     # vilatile
```

Or plain Go builds:

```sh
GOOS=windows go build -trimpath -ldflags "-s -w -H windowsgui" -o vilatile.exe .
go build -o vilatile .
```

## Source layout

One file per concern, with per-OS variants in `<concern>_GOOS.go` files
selected by build tags:

| File | Role |
|------|------|
| `main.go` | CLI entry point: flags → config → run gvim |
| `config.go` | Config file (YAML) and defaults |
| `gvim.go` | Locating gvim, temp files, spawning and waiting |
| `window.go` | Position spec parsing; documents the per-OS contract |
| `window_windows.go` / `window_linux.go` | Floating/positioning via Win32+GlazeWM / swaymsg |
| `debug.go` | Opt-in debug log (`-debug`) |
| `init.vim` | Embedded GVim setup (scratch buffer, `:w` → clipboard) |

## Usage

```sh
vilatile [flags]
echo "initial text" | vilatile
```

Text piped to stdin becomes the initial buffer contents. Since `:w` copies
the buffer to the clipboard, this turns vilatile into a quick editing
stage for any pipeline — most usefully, an in-place clipboard editor:

```sh
wl-paste | vilatile                 # edit the clipboard contents, :w writes them back
git log --oneline -10 | vilatile    # tidy up command output, then copy it
cat ~/templates/reply.txt | vilatile  # fill in a template, then copy it
```

On Windows, the clipboard round-trip works with
`powershell -c Get-Clipboard | vilatile`.

| Flag | Description |
|------|-------------|
| `-config PATH` | Path to config file |
| `-gvim-path PATH` | Path to the gvim executable |
| `-columns N` / `-lines N` | Window size in characters |
| `-x` / `-y` | Position: `left`/`right`/`center`/`top`/`bottom` or a pixel offset |
| `-debug` | Write a debug log to `vilatile_debug.log` in the temp directory |

## Configuration

Config file location:

- Windows: `%APPDATA%\vilatile\config.yaml`
- Linux: `~/.config/vilatile/config.yaml`

```yaml
gvim_path: ""        # auto-detected when empty
columns: 90
lines: 10
position:
  x: right           # left | right | center | <pixel offset>
  y: bottom          # top | bottom | center | <pixel offset>
glazewm_port: 6123   # GlazeWM IPC port (used only to detect GlazeWM)
```

Command-line flags override config file values.

## Tiling window managers

### GlazeWM (Windows) — required setup

vilatile does **not** send floating commands over GlazeWM's IPC.
Untargeted `set-floating` commands act on the currently *focused* window,
which caused windows to be misidentified (and GVim to hang) when several
instances started at once. Instead, floating is delegated to a window rule.

Add this to your GlazeWM `config.yaml` and reload the config
(default `Alt+Shift+R`):

```yaml
window_rules:
  - commands: ['set-floating']
    match:
      - window_title: { regex: 'vilatile' }
```

If the title-based rule ever fails to trigger (the title is set shortly
after the window appears), match by process instead — note this floats
*every* GVim window:

```yaml
window_rules:
  - commands: ['set-floating']
    match:
      - window_process: { equals: 'gvim' }
```

Without this rule the window stays tiled while GlazeWM is running.

Two behaviors compensate for GlazeWM's floating placement, which resizes
the window right after it floats:

- GVim re-asserts the configured `columns`/`lines` for a few seconds after
  startup (timer in the embedded init.vim)
- The launcher watches for size changes for a few seconds and re-applies
  the configured position, so `right`/`bottom` anchoring stays correct

A brief flicker of the intermediate size on startup is expected. To avoid
it, you can additionally pin a pixel size in the window rule, e.g.
`set-floating --width 800px --height 300px` (character-based sizing is not
possible there since it depends on the font).

### Sway (Linux)

No setup needed: the launcher floats and positions the window via
`swaymsg` (`floating enable`, `move position`).

### Plain Windows (no tiling WM)

No setup needed: the launcher floats the window itself via the Win32 API
(tool-window style + `SetWindowPos`).
