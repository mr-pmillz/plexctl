package termgraphics

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// PaneZoom tracks playback's explicit fullscreen changes. Creating it only
// reads the current layout; playback starts at the pane's existing size.
// Its methods belong to the playback event loop, not concurrent callers.
type PaneZoom struct {
	pane    string
	run     func(...string) ([]byte, error)
	initial bool
	changed bool
}

// NewPaneZoom returns nil outside an actual tmux pane.
func NewPaneZoom() (*PaneZoom, error) {
	if !InTmux() {
		return nil, nil
	}
	return newPaneZoom(os.Getenv("TMUX_PANE"), func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return exec.CommandContext(ctx, "tmux", args...).Output()
	})
}

func newPaneZoom(pane string, run func(...string) ([]byte, error)) (*PaneZoom, error) {
	p := &PaneZoom{pane: pane, run: run}
	initial, err := p.Zoomed()
	if err != nil {
		return nil, err
	}
	p.initial = initial
	return p, nil
}

func (p *PaneZoom) Zoomed() (bool, error) {
	value, err := p.run("display-message", "-p", "-t", p.pane, "#{window_zoomed_flag}")
	if err != nil {
		return false, fmt.Errorf("inspect tmux pane zoom: %w", err)
	}
	switch strings.TrimSpace(string(value)) {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("tmux did not report the playback pane's zoom state")
	}
}

// Origin is the pane's position in the attached terminal, including a status
// bar above the window. tmux does not position the cursor for raw passthrough.
func (p *PaneZoom) Origin() (int, int, error) {
	value, err := p.run("display-message", "-p", "-t", p.pane, "#{pane_left} #{pane_top} #{status-position} #{status}")
	if err != nil {
		return 0, 0, fmt.Errorf("inspect tmux pane position: %w", err)
	}
	fields := strings.Fields(string(value))
	if len(fields) != 4 {
		return 0, 0, fmt.Errorf("tmux did not report the playback pane's position")
	}
	x, xerr := strconv.Atoi(fields[0])
	y, yerr := strconv.Atoi(fields[1])
	if xerr != nil || yerr != nil || x < 0 || y < 0 {
		return 0, 0, fmt.Errorf("tmux reported an invalid playback pane position")
	}
	if fields[2] == "top" {
		lines := 0
		switch fields[3] {
		case "on":
			lines = 1
		case "off":
		default:
			lines, err = strconv.Atoi(fields[3])
			if err != nil || lines < 0 || lines > 5 {
				return 0, 0, fmt.Errorf("tmux reported an invalid status bar height")
			}
		}
		y += lines
	}
	return x, y, nil
}

func (p *PaneZoom) Toggle() (bool, error) {
	if _, err := p.run("resize-pane", "-Z", "-t", p.pane); err != nil {
		return false, fmt.Errorf("toggle terminal playback fullscreen: %w", err)
	}
	p.changed = true
	// tmux leaves a one-pane window unzoomed. Read the actual result instead
	// of assuming every successful command changed the state.
	return p.Zoomed()
}

// Restore preserves a pre-existing zoom and leaves external changes alone if
// playback never used its fullscreen control.
func (p *PaneZoom) Restore() error {
	if !p.changed {
		return nil
	}
	zoomed, err := p.Zoomed()
	if err != nil {
		return err
	}
	if zoomed != p.initial {
		if _, err := p.Toggle(); err != nil {
			return err
		}
	}
	p.changed = false
	return nil
}
