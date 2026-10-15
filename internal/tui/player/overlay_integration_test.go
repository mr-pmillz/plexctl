//go:build integration && !windows

package player

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ygelfand/plexctl/internal/config"
	"github.com/ygelfand/plexctl/internal/termgraphics"
	"github.com/ygelfand/plexctl/internal/tui/artwork"
	"github.com/ygelfand/plexctl/internal/ui"
)

func TestOverlayPlaybackWithRealMPV(t *testing.T) {
	mpv, err := exec.LookPath("mpv")
	if override := os.Getenv("PLEXCTL_TEST_MPV"); override != "" {
		mpv, err = exec.LookPath(override)
	}
	if err != nil {
		t.Skip("mpv is not installed")
	}
	wasTUI, previous := config.IsTUI, *config.Get()
	config.IsTUI = true
	config.Get().TerminalVideoMode = "overlay"
	config.Get().WindowMusicVisualizer = "waves"
	config.Get().SixelVideoWidth = 160
	t.Cleanup(func() { config.IsTUI = wasTUI; *config.Get() = previous })
	dir := t.TempDir()
	config.Get().CacheDir = dir
	media := testAudio(t, dir)
	// Force a silent audio sink without depending on the user's mpv settings.
	wrapper := filepath.Join(dir, "mpv")
	script := "#!/bin/sh\nif [ \"$PLEXCTL_FAIL_KITTY\" = 1 ]; then\n  for arg; do [ \"$arg\" = --vo=kitty ] && exit 3; done\nfi\nexec \"$PLEXCTL_REAL_MPV\" --no-config --ao=null \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLEXCTL_REAL_MPV", mpv)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	layout := ui.GetLayout()
	w, h := layout.TotalWidth(), layout.TotalHeight()
	layout.Update(78, 24, false)
	t.Cleanup(func() { layout.Update(w, h, false) })
	for _, output := range []config.VideoOutput{config.VideoKitty, config.VideoSixel, config.VideoTCT, config.VideoAuto} {
		t.Run(string(output), func(t *testing.T) {
			if output == config.VideoAuto {
				t.Setenv("TERM", "xterm-kitty")
				t.Setenv("TERM_PROGRAM", "kitty")
				t.Setenv("PLEXCTL_FAIL_KITTY", "1")
			}
			if _, _, err := terminalOutput(t.Context(), output, os.Getenv); err != nil {
				t.Skip(err)
			}
			file, err := os.CreateTemp(t.TempDir(), "screen")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = file.Close() }()
			screen, err := artwork.OpenRenderer(file, "halfcell", termgraphics.Capabilities{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = screen.Close() }()
			pm := newPlayerManager(filepath.Join(t.TempDir(), "ipc"))
			defer pm.Close()
			defer func() { pm.StopPlayback()() }()
			req := &PlayRequest{Music: true, NoReport: true, Items: []PlayItem{{URL: media, Title: "Overlay", RatingKey: "overlay"}}}
			msg := pm.PlayRequest(req, output, 0)()
			if output == config.VideoAuto {
				if fallback, ok := msg.(VideoFallbackMsg); !ok || fallback.From != config.VideoKitty {
					t.Fatalf("missing automatic graphics fallback notification: %v", msg)
				}
			} else if msg != nil {
				t.Fatalf("start overlay: %v", msg)
			}
			status := waitAudio(t, pm, "overlay starts", func(s PlayerStatus) bool { return s.Overlay && s.Time > .15 })
			if status.AudioOnly {
				t.Fatal("overlay lost the visualizer video stream")
			}
			pm.ioMu.Lock()
			var current string
			err = pm.conn.get("current-vo", &current)
			pm.ioMu.Unlock()
			want := output
			if output == config.VideoAuto {
				want = config.VideoTCT
			}
			if err != nil || current != string(want) {
				t.Fatalf("video output = %q, %v", current, err)
			}
			pm.TogglePause()()
			waitAudio(t, pm, "pause overlay", func(s PlayerStatus) bool { return s.Paused() })
			pm.SetOverlayVisible(false)
			pm.SetOverlayVisible(true)
			before, _ := file.Stat()
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				after, _ := file.Stat()
				if after.Size() > before.Size() {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			after, _ := file.Stat()
			if after.Size() <= before.Size() {
				t.Fatal("paused overlay did not repaint after the dialog closed")
			}
			pm.StopPlayback()()
			data, err := os.ReadFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}
			if len(data) == 0 || strings.Contains(string(data), "\x1b[?1049") || strings.Contains(string(data), "\x1b[2J") {
				t.Fatal("overlay switched or cleared the TUI screen")
			}
		})
	}
}

func TestMusicVideoGraphsWithRealMPV(t *testing.T) {
	mpv, err := exec.LookPath("mpv")
	if err != nil {
		t.Skip("mpv is not installed")
	}
	media := testAudio(t, t.TempDir())
	for _, style := range []string{"cqt", "spectrum", "waves", "scope"} {
		t.Run(style, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, mpv, "--no-config", "--vo=null", "--ao=null", "--length=0.3", "--untimed",
				"--lavfi-complex="+musicVideoGraph(style), media)
			data, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(data), "VO: [null]") || !strings.Contains(string(data), "AO: [null]") {
				t.Fatalf("visualizer did not produce both audio and video: %v\n%s", err, data)
			}
		})
	}
}
