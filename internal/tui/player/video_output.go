package player

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ygelfand/plexctl/internal/config"
	"github.com/ygelfand/plexctl/internal/termgraphics"
)

func terminalOutput(ctx context.Context, preference config.VideoOutput, getenv func(string) string) (string, config.VideoOutput, error) {
	if _, err := config.ParseTerminalVideo(string(preference)); err != nil {
		return "", config.VideoWindow, err
	}
	mpv, err := exec.LookPath("mpv")
	if err != nil {
		return "", config.VideoWindow, fmt.Errorf("mpv command not found; please install mpv: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	help, err := exec.CommandContext(ctx, mpv, "--no-config", "--vo=help").Output()
	if err != nil {
		return "", config.VideoWindow, fmt.Errorf("could not list mpv video outputs: %w", err)
	}
	output, err := selectTerminalOutput(preference, string(help), getenv, termgraphics.Current())
	return mpv, output, err
}

func selectTerminalOutput(preference config.VideoOutput, help string, getenv func(string) string, caps termgraphics.Capabilities) (config.VideoOutput, error) {
	available := make(map[config.VideoOutput]bool)
	for line := range strings.SplitSeq(help, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			available[config.VideoOutput(fields[0])] = true
		}
	}
	if preference != config.VideoAuto {
		if !available[preference] {
			return config.VideoWindow, fmt.Errorf("mpv does not provide the %s video output; check 'mpv --vo=help' or select another Terminal Video renderer in Settings", preference)
		}
		return preference, nil
	}

	term := strings.ToLower(getenv("TERM"))
	program := strings.ToLower(getenv("TERM_PROGRAM"))
	var candidates []config.VideoOutput
	if caps.Kitty {
		candidates = append(candidates, config.VideoKitty)
	}
	if caps.Sixel {
		candidates = append(candidates, config.VideoSixel)
	}
	// Multiplexers can hide capabilities or require passthrough configuration.
	// Do not emit graphics automatically based on the outer terminal's env vars.
	multiplexer := getenv("TMUX") != "" || getenv("STY") != "" || strings.HasPrefix(term, "screen") || strings.HasPrefix(term, "tmux")
	if !multiplexer && !caps.Responded {
		switch {
		case strings.Contains(term, "kitty"), getenv("KITTY_WINDOW_ID") != "", program == "kitty", program == "ghostty", term == "xterm-ghostty":
			candidates = append(candidates, config.VideoKitty)
		case getenv("KONSOLE_VERSION") != "":
			candidates = append(candidates, config.VideoKitty, config.VideoSixel)
		case strings.Contains(term, "sixel"), strings.HasPrefix(term, "foot"), strings.HasPrefix(term, "mlterm"), program == "wezterm", program == "contour", program == "mintty":
			candidates = append(candidates, config.VideoSixel)
		}
	}
	candidates = append(candidates, config.VideoTCT)
	for _, output := range candidates {
		if available[output] {
			return output, nil
		}
	}
	return config.VideoWindow, fmt.Errorf("no compatible terminal video output found; check 'mpv --vo=help' and select kitty, sixel, or tct in Settings")
}

// sixelSizeArgs bounds the pixel work without stretching the physical cells.
// Both the pixel size and matching cell geometry must describe the same area.
func sixelSizeArgs(limit, columns, rows, cellW, cellH int) []string {
	if limit <= 0 {
		return nil
	}
	if cellW <= 0 || cellH <= 0 {
		cellW, cellH = 8, 16
	}
	columns, rows = max(1, columns), max(2, rows)
	width := min(columns*cellW, limit)
	cols := max(1, width/cellW)
	return []string{
		fmt.Sprintf("--vo-sixel-width=%d", width),
		fmt.Sprintf("--vo-sixel-height=%d", rows*cellH),
		fmt.Sprintf("--vo-sixel-cols=%d", cols),
		fmt.Sprintf("--vo-sixel-rows=%d", rows),
		fmt.Sprintf("--vo-sixel-left=%d", (columns-cols)/2+1),
	}
}
