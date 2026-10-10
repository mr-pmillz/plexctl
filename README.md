# plexctl

A robust Plex CLI and TUI written in Go.

> **Note**: This project is a **Work in Progress**. Expect breaking changes and incomplete features.

![plexctl demo](assets/demo.gif)

## Prerequisites

`plexctl` uses **mpv** for media playback. You must have it installed and available in your PATH.

### Installation Instructions

- **macOS**: `brew install mpv`
- **Linux**: `sudo apt install mpv` or `sudo pacman -S mpv`
- **Windows**: Download from [mpv.io](https://mpv.io/)

## Installation

Download the latest binary for your platform from the [Releases](https://github.com/ygelfand/plexctl/releases) page.

### Manual Installation (macOS/Linux)

If you download the binary directly, you will need to make it executable and move it to your path:

```bash
# Rename the downloaded binary (example for macOS ARM)
mv plexctl_darwin_arm64 plexctl

# Make it executable
chmod +x plexctl

# Move to a directory in your PATH (optional)
sudo mv plexctl /usr/local/bin/
```

Alternatively, with Go **1.27.2 or newer** installed:

```bash
go install github.com/ygelfand/plexctl@latest
```

Builds made with Go 1.27 require macOS 13 Ventura or newer on macOS.

## Development checks

Use Go 1.27.2 or newer and golangci-lint 2.14.0, built with the same Go version:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
go build ./...
go test -race -tags=integration ./...
golangci-lint run
golangci-lint run --build-tags integration
```

The audio integration test uses a local mpv installation and skips when mpv is unavailable.
Pull requests run these checks and `govulncheck` in GitHub Actions.

## Features

- **Interactive TUI**: Navigate your Plex library with ease.
- **Fuzzy Search**: Find exactly what you're looking for instantly.
- **Cross-Platform**: Supports macOS, Linux, and Windows.
- **Flexible Output**: CLI commands support table, JSON, YAML, and CSV formats.

## Usage

### TUI (Interactive)
Simply run `plexctl` to launch the interactive terminal interface.

Press **v** on a movie or episode to choose **Kitty**, **Sixel**, or **Text**
playback. **Ctrl+P** starts terminal playback using your saved preference.
Kitty video includes a visible control bar: play/pause, a draggable seek bar,
skip backward/forward, volume, mute, subtitles, playback speed, and **Return**.
Click the controls or use **Tab / Shift+Tab** and **Enter**; **h** hides or shows
the bar. The controls also work through tmux and SSH into a Kitty terminal.

The player uses mpv's Kitty or Sixel graphics output when detected, with TCT text
output as a fallback. Press **Space** to pause, use the arrow keys to seek, and
press **q** to return to the library. Regular **p** opens the video window.

Choose a renderer in **Ctrl+S → Terminal Video**, or override it at launch:

```bash
plexctl tui --terminal-video kitty
plexctl tui --terminal-video sixel
plexctl play 12345 --terminal-video auto
```

Graphics playback requires a compatible terminal and an mpv build that lists the
chosen renderer in `mpv --vo=help`. plexctl probes the attached terminal, including
through tmux, and uses text output when graphics are unavailable. In tmux 3.3+,
playback temporarily zooms its pane and restores the layout on exit.

Mouse support is enabled automatically: click posters, navigation, and dialog
options; scroll with the wheel; right click to go back. During terminal video,
click to pause, scroll to seek, and right click to return. **V** switches library
list/poster views.

**Ctrl+H/J/K/L** moves focus left/down/up/right between the sidebar, library,
alphabet rail, and active player. The focused pane is highlighted. In the sidebar,
use **j/k** or the arrows to select a library, then **Enter** to open it.
**L** opens library settings; **Ctrl+S** opens global settings.

Movie and TV cover posters automatically use high-resolution images in Kitty
0.28+ (also through tmux), with text posters as a fallback in other terminals.
See the [TUI guide](docs/content/tui.mdx) for configuration and playback details.

Music libraries support artist → album → track navigation. Press **p** to play
the selected song in the TUI, with a live stereo waveform and no extra
window. **v** chooses terminal audio or a separate window; **Ctrl+S → Music
Player** saves the default. Album covers appear in the artist gallery and beside
the track list. Scrolling automatically loads more artists. Playing an artist or
album starts its first playable track.

Click the right-hand **A–Z rail**, or press **g then a letter**, to jump through
movies, TV shows, artists, albums, and track titles. Missing library pages load
automatically. From the library, **Ctrl+L** focuses the rail: **j/k** or **Down/Up**
select a letter, and **Enter** jumps to it and returns focus to the content.
**Ctrl+H** or **Esc** returns without jumping. In the content pane,
**Home / End** reach the first/last item; **Page Up / Page Down**
move by a page. The library footer shows how many items have loaded.

Use **Space** to pause, **[ / ]** to seek, **+ / -** for volume, **m** to mute,
**z** to cycle waveform/spectrum/mirror/off, and **x** to stop. The default
waveform draws the playing audio with cyan/violet traces and fading trails.
It uses ordinary terminal cells, including through tmux and
SSH. **/** searches Plex for artists, albums, and track names without requiring
a local index first.

### CLI (Command Line)
`plexctl` provides a comprehensive command-line interface for automation and quick tasks. Every command supports the `-h` or `--help` flag for detailed usage instructions.

```bash
# Explore available commands
plexctl -h

# List all libraries on your server
plexctl library list

# Perform a fuzzy search
plexctl search find "Inception"

# Manage active playback sessions
plexctl session list

# Start or stop background server tasks
plexctl tasks list
```

All CLI commands support the `-o` or `--output` flag to return data in `table`, `json`, `json-pretty`, `yaml`, `csv`, or `txt` formats.

## Configuration

## License

MIT
