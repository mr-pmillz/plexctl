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

Alternatively, if you have Go installed:

```bash
go install github.com/ygelfand/plexctl@latest
```

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

Movie and TV cover posters automatically use high-resolution images in Kitty
0.28+ (also through tmux), with text posters as a fallback in other terminals.
See the [TUI guide](docs/content/tui.mdx) for configuration and playback details.

Music libraries support artist → album → track navigation. Press **p** to play
the selected song; playing an artist or album starts its first playable track.

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
