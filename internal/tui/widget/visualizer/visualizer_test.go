package visualizer

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ygelfand/plexctl/internal/tui/player"
)

func TestVisualizerRespondsToAudioAndSettlesOnPause(t *testing.T) {
	var m Model
	frame := player.Spectrum{Valid: true, Left: -8, Right: -20}
	for i := range frame.Bands {
		frame.Bands[i] = -100
	}
	frame.Bands[3] = -12
	for range 10 {
		m.Step(frame, false, 50*time.Millisecond)
	}
	if m.levels[3] < 0.7 || m.levels[9] != 0 || m.left <= m.right {
		t.Fatal("display did not follow the measured frequency and stereo levels")
	}
	for range 100 {
		m.Step(frame, true, 50*time.Millisecond)
	}
	if m.levels[3] > 0.001 || m.peaks[3] > 0.001 {
		t.Fatal("paused audio kept animating")
	}
}

func TestVisualizerFitsTerminalSizes(t *testing.T) {
	for _, mode := range []string{"spectrum", "mirror"} {
		t.Run(mode, func(t *testing.T) {
			var m Model
			for _, width := range []int{1, 12, 40, 53, 130} {
				for _, height := range []int{1, 2, 6} {
					view := m.View(width, height, mode)
					if lipgloss.Height(view) != height+1 {
						t.Fatalf("%dx%d: unexpected height %d", width, height, lipgloss.Height(view))
					}
					for _, line := range strings.Split(view, "\n") {
						if lipgloss.Width(line) > width {
							t.Fatalf("%dx%d: line overflows terminal", width, height)
						}
					}
				}
			}
		})
	}
}
