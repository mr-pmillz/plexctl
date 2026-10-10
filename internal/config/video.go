package config

import "fmt"

// VideoOutput selects an mpv video renderer. The zero value opens a video window.
type VideoOutput string

const (
	VideoWindow VideoOutput = ""
	VideoAuto   VideoOutput = "auto"
	VideoKitty  VideoOutput = "kitty"
	VideoSixel  VideoOutput = "sixel"
	VideoTCT    VideoOutput = "tct"
	// AudioTerminal keeps the TUI active and disables mpv's video window.
	AudioTerminal VideoOutput = "audio"
)

// ParseTerminalVideo validates a terminal renderer preference.
func ParseTerminalVideo(value string) (VideoOutput, error) {
	switch output := VideoOutput(value); output {
	case VideoAuto, VideoKitty, VideoSixel, VideoTCT:
		return output, nil
	default:
		return VideoWindow, fmt.Errorf("invalid terminal video renderer %q: choose auto, kitty, sixel, or tct", value)
	}
}
