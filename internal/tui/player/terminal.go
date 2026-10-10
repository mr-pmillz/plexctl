package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/LukeHagar/plexgo/models/operations"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ygelfand/plexctl/internal/config"
	"github.com/ygelfand/plexctl/internal/termgraphics"
	"golang.org/x/term"
)

// terminalPlayback owns a foreground mpv process. Bubble Tea releases its input
// and renderer for the entire Run call, then restores the library view.
// Its status is private: IPC events and progress reporting run on the same loop.
type terminalPlayback struct {
	mpv        string
	output     config.VideoOutput
	preference config.VideoOutput
	status     PlayerStatus
	subtitles  []ExternalSubtitle
	stdin      io.Reader
	stdout     io.Writer
	report     func(context.Context, PlayerStatus)
}

var _ tea.ExecCommand = (*terminalPlayback)(nil)

func (pm *PlayerManager) playTerminal(url, title, ratingKey string, noReport bool, preference config.VideoOutput, startOffset int64, subtitles []ExternalSubtitle) tea.Cmd {
	return func() tea.Msg {
		if !config.IsTUI {
			termgraphics.Probe(os.Stdin, os.Stdout)
		}
		mpv, output, err := terminalOutput(context.Background(), preference, os.Getenv)
		if err != nil {
			return err
		}
		playback := &terminalPlayback{
			mpv: mpv, output: output, preference: preference, subtitles: subtitles, report: reportProgress,
			status: PlayerStatus{
				Title: title, File: url, Key: ratingKey, NoReport: noReport,
				Time: float64(startOffset) / 1000,
			},
		}
		// A separate socket prevents a later TUI from attaching to a foreground
		// player whose terminal belongs to this session.
		pm.StopPlayback()()
		finished := func(err error) tea.Msg {
			if err != nil {
				return err
			}
			return PlayerStatusMsg{}
		}
		if config.IsTUI {
			return tea.Exec(playback, finished)()
		}
		playback.SetStdin(os.Stdin)
		playback.SetStdout(os.Stdout)
		return finished(playback.Run())
	}
}

func (p *terminalPlayback) SetStdin(r io.Reader) { p.stdin = r }
func (p *terminalPlayback) SetStdout(w io.Writer) {
	if output, ok := w.(interface{ TerminalFile() *os.File }); ok {
		w = output.TerminalFile()
	}
	p.stdout = w
}

// mpv diagnostics can contain authenticated media URLs and corrupt graphics.
// Failures are returned to the TUI after it has reclaimed the terminal.
func (p *terminalPlayback) SetStderr(io.Writer) {}

func (p *terminalPlayback) args(socket string) []string {
	args := []string{
		"--idle=once", "--keep-open=no", "--no-resume-playback",
		"--pause=no", "--loop-file=no", "--loop-playlist=no",
		"--force-window=no", "--terminal=yes", "--input-terminal=yes",
		"--input-default-bindings=yes", "--really-quiet",
		"--vo=" + string(p.output), "--input-ipc-server=" + socket,
		"--force-media-title=" + p.status.Title,
		fmt.Sprintf("--start=%.3f", p.status.Time),
	}
	if p.output == config.VideoKitty {
		// Escape-code transport also works over SSH; shared memory does not.
		args = append(args, "--vo-kitty-use-shm=no")
		// Our control bar uses mpv's OSD compositor and IPC, independently
		// of the Lua OSC and its window-system mouse events.
		args = append(args, "--osc=no")
	}
	return args
}

func (p *terminalPlayback) Run() error {
	for _, stream := range []any{p.stdin, p.stdout} {
		if f, ok := stream.(interface{ Fd() uintptr }); ok && !term.IsTerminal(int(f.Fd())) {
			return fmt.Errorf("terminal video requires an interactive terminal")
		}
	}
	if input, ok := p.stdin.(*os.File); ok {
		caps := termgraphics.Probe(input, p.stdout)
		if p.preference != config.VideoWindow {
			var err error
			p.mpv, p.output, err = terminalOutput(context.Background(), p.preference, os.Getenv)
			if err != nil {
				return err
			}
		}
		if caps.Responded && ((p.output == config.VideoKitty && !caps.Kitty) || (p.output == config.VideoSixel && !caps.Sixel)) {
			name := caps.Name
			if name == "" {
				name = "the attached terminal"
			}
			return fmt.Errorf("%s does not advertise %s graphics; open plexctl in a supporting terminal or select Text", name, p.output)
		}
	}
	dir, err := os.MkdirTemp("", "plexctl-mpv-")
	if err != nil {
		return fmt.Errorf("create terminal player socket directory: %w", err)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "ipc")
	if runtime.GOOS == "windows" {
		socket = `\\.\pipe\` + filepath.Base(dir)
	}

	cmd := exec.Command(p.mpv, p.args(socket)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = p.stdin, p.stdout, io.Discard
	var keys <-chan string
	var graphics *termgraphics.Writer
	var controls *terminalControls
	var cellW, cellH float64
	if termgraphics.InTmux() && (p.output == config.VideoKitty || p.output == config.VideoSixel) {
		restore, err := termgraphics.EnablePassthrough()
		if err != nil {
			return err
		}
		defer restore()
		unzoom, err := termgraphics.ZoomPane()
		if err != nil {
			return err
		}
		defer unzoom()
		if p.output == config.VideoSixel {
			cmd.Stdout = &termgraphics.Writer{Output: p.stdout}
			defer func() {
				_, _ = io.WriteString(p.stdout, termgraphics.Wrap("\x1b[2J\x1b[H"))
			}()
		}
	}
	if p.output == config.VideoKitty {
		controls = &terminalControls{focus: -1, hover: -1, speed: 1, subtitles: true}
		p.status.Volume = 100
		graphics = &termgraphics.Writer{Output: p.stdout, Direct: !termgraphics.InTmux(), ImageID: (1 << 30) + rand.IntN(1<<20)}
		cmd.Stdout = graphics
		defer func() { _, _ = io.WriteString(p.stdout, termgraphics.Wrap(termgraphics.DeleteImage(graphics.ImageID))) }()
	}
	// Our wrapper handles actual tmux panes. Also remove stale tmux variables
	// from direct Kitty windows so mpv cannot wrap their graphics by mistake.
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "TMUX=") && !strings.HasPrefix(entry, "TMUX_PANE=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	if input, ok := p.stdin.(*os.File); ok && term.IsTerminal(int(input.Fd())) {
		// Own input while mpv plays: this works with piped graphics in tmux
		// and provides the same mouse controls in every terminal renderer.
		cmd.Args = append(cmd.Args, "--input-terminal=no")
		var stop func()
		keys, stop, err = readPlaybackKeys(p.stdin)
		if err != nil {
			return err
		}
		defer stop()
		_, _ = io.WriteString(p.stdout, "\x1b[?1000h\x1b[?1006h")
		defer func() { _, _ = io.WriteString(p.stdout, "\x1b[?1000l\x1b[?1006l") }()
		if controls != nil {
			cellW, cellH = terminalCellSize(input)
			// Motion supports seek dragging and button highlights. Ask the
			// actual terminal for cell metrics even over SSH.
			_, _ = io.WriteString(p.stdout, "\x1b[?1003h"+termgraphics.Wrap("\x1b[16t"))
			defer func() { _, _ = io.WriteString(p.stdout, "\x1b[?1003l") }()
		}
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start terminal player: %w", err)
	}
	done := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(done)
	}()
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			<-done
		}
	}()

	conn, err := connectTerminal(socket, done)
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan terminalEvent, 64)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		readTerminalEvents(ctx, conn, events)
	}()
	defer func() {
		cancel()
		_ = conn.Close()
		<-readerDone
	}()

	properties := []string{"time-pos", "pause", "duration"}
	if controls != nil {
		properties = append(properties, "volume", "mute", "speed", "sub-visibility")
	}
	for id, property := range properties {
		if err := sendTerminalCommand(conn, 0, "observe_property", id+1, property); err != nil {
			return err
		}
	}
	// Request ID 1 lets us catch a rejected load even when mpv stays idle.
	if err := sendTerminalCommand(conn, 1, "loadfile", p.status.File, "replace"); err != nil {
		return err
	}

	loaded := false
	defer func() {
		if loaded {
			p.status.State = operations.StateStopped
			p.reportStatus()
		}
	}()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	controlTicker := time.NewTicker(100 * time.Millisecond)
	defer controlTicker.Stop()
	var controlColumns, controlRows int
	keyTimer := time.NewTimer(time.Hour)
	keyTimer.Stop()
	defer keyTimer.Stop()
	pendingKeys := ""
	forwardKeys := func(flush bool) error {
		var decoded []playbackInput
		decoded, pendingKeys = playbackInputs(pendingKeys, flush)
		for _, input := range decoded {
			if input.CellWidth > 0 {
				cellW, cellH = input.CellWidth, input.CellHeight
				continue
			}
			var command []any
			if controls != nil {
				// ioctl follows window and font changes when pixel metrics are
				// available; otherwise retain the terminal's cell-size reply.
				if f, ok := p.stdin.(*os.File); ok {
					if w, h := terminalCellSize(f); w > 0 {
						cellW, cellH = w, h
					}
				}
				frame := graphics.Frame()
				command = controls.input(input, controls.layout(frame, p.status), frame, cellW, cellH, p.status)
			} else {
				key := input.Key
				if input.Mouse != nil {
					key = input.Mouse.key()
				}
				if key != "" {
					command = []any{"keypress", key}
				}
			}
			if len(command) == 0 {
				continue
			}
			if err := sendTerminalCommand(conn, 0, command...); err != nil {
				return err
			}
		}
		if pendingKeys != "" {
			keyTimer.Reset(50 * time.Millisecond)
		}
		return nil
	}
	for {
		select {
		case data, ok := <-keys:
			if !ok {
				keys = nil
				continue
			}
			pendingKeys += data
			keyTimer.Stop()
			if err := forwardKeys(false); err != nil {
				return err
			}
		case <-keyTimer.C:
			if err := forwardKeys(true); err != nil {
				return err
			}
		case <-controlTicker.C:
			if controls == nil || !loaded {
				continue
			}
			layout := controls.layout(graphics.Frame(), p.status)
			resized := false
			if input, ok := p.stdin.(*os.File); ok {
				if columns, rows, err := term.GetSize(int(input.Fd())); err == nil {
					resized = columns != controlColumns || rows != controlRows
					controlColumns, controlRows = columns, rows
				}
			}
			overlay := controls.overlay(layout, p.status)
			// A paused Kitty VO needs an OSD redraw to process SIGWINCH.
			// Resending the overlay preserves pause and the playback position.
			if overlay == controls.lastOverlay && !resized {
				continue
			}
			if err := sendTerminalCommand(conn, 2, "osd-overlay", 1, "ass-events", overlay, layout.w, layout.h); err != nil {
				return err
			}
			controls.lastOverlay = overlay
		case ev, ok := <-events:
			if !ok {
				// IPC normally closes just before Wait returns on EOF or q.
				select {
				case <-done:
					waited = true
					return p.exitError(waitErr)
				case <-time.After(2 * time.Second):
					return fmt.Errorf("terminal player disconnected from IPC")
				}
			}
			if ev.RequestID == 1 && ev.Error != "success" {
				return fmt.Errorf("mpv rejected the media stream")
			}
			if ev.RequestID == 2 && ev.Error != "success" {
				return fmt.Errorf("mpv could not draw video controls; use an mpv build with libass OSD support")
			}
			switch ev.Event {
			case "file-loaded":
				loaded = true
				p.status.Play()
				for _, sub := range p.subtitles {
					if err := sendTerminalCommand(conn, 0, "sub-add", sub.URL, "auto", sub.Title, sub.Language); err != nil {
						return err
					}
				}
				p.reportStatus()
			case "property-change":
				oldState := p.status.State
				p.applyProperty(ev)
				if controls != nil && len(ev.Data) > 0 && string(ev.Data) != "null" {
					switch ev.ID {
					case 6:
						_ = json.Unmarshal(ev.Data, &controls.speed)
					case 7:
						_ = json.Unmarshal(ev.Data, &controls.subtitles)
					}
				}
				if loaded && p.status.State != oldState {
					p.reportStatus()
				}
			case "end-file":
				if ev.Reason == "error" {
					return fmt.Errorf("mpv could not play the stream using %s; check the server connection and terminal graphics support", p.output)
				}
				if ev.Reason == "eof" && p.status.Duration > 0 {
					p.status.Time = p.status.Duration
				}
			case "shutdown":
				// Continue draining properties until the process exits.
			}
		case <-ticker.C:
			if loaded {
				p.reportStatus()
			}
		}
	}
}

func (p *terminalPlayback) exitError(err error) error {
	var exit *exec.ExitError
	// mpv uses status 4 for a user interrupt (Ctrl+C).
	if errors.As(err, &exit) && exit.ExitCode() == 4 {
		return nil
	}
	if err != nil {
		return fmt.Errorf("terminal playback with %s failed; check terminal support and 'mpv --vo=help': %w", p.output, err)
	}
	return nil
}

func (p *terminalPlayback) reportStatus() {
	if p.report == nil || p.status.NoReport || p.status.Key == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	p.report(ctx, p.status)
}

type terminalEvent struct {
	Event     string          `json:"event"`
	ID        int             `json:"id"`
	Data      json.RawMessage `json:"data"`
	Reason    string          `json:"reason"`
	RequestID int             `json:"request_id"`
	Error     string          `json:"error"`
}

func (p *terminalPlayback) applyProperty(ev terminalEvent) {
	if len(ev.Data) == 0 || string(ev.Data) == "null" {
		return
	}
	switch ev.ID {
	case 1:
		_ = json.Unmarshal(ev.Data, &p.status.Time)
	case 2:
		var paused bool
		if json.Unmarshal(ev.Data, &paused) == nil {
			if paused {
				p.status.State = operations.StatePaused
			} else {
				p.status.Play()
			}
		}
	case 3:
		_ = json.Unmarshal(ev.Data, &p.status.Duration)
	case 4:
		_ = json.Unmarshal(ev.Data, &p.status.Volume)
	case 5:
		_ = json.Unmarshal(ev.Data, &p.status.Muted)
	}
}

func connectTerminal(socket string, done <-chan struct{}) (net.Conn, error) {
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	retry := time.NewTicker(50 * time.Millisecond)
	defer retry.Stop()
	for {
		select {
		case <-done:
			return nil, fmt.Errorf("mpv exited before terminal playback started; check 'mpv --vo=help'")
		case <-timeout.C:
			return nil, fmt.Errorf("timed out connecting to the terminal player")
		case <-retry.C:
			if conn, err := dialTerminal(socket); err == nil {
				return conn, nil
			}
		}
	}
}

func sendTerminalCommand(conn net.Conn, requestID int, args ...any) error {
	if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return fmt.Errorf("set player IPC deadline: %w", err)
	}
	command := struct {
		Command   []any `json:"command"`
		RequestID int   `json:"request_id"`
	}{args, requestID}
	if err := json.NewEncoder(conn).Encode(command); err != nil {
		return fmt.Errorf("send player IPC command: %w", err)
	}
	return nil
}

func readTerminalEvents(ctx context.Context, conn net.Conn, events chan<- terminalEvent) {
	defer close(events)
	decoder := json.NewDecoder(conn)
	for {
		var ev terminalEvent
		if err := decoder.Decode(&ev); err != nil {
			return
		}
		select {
		case events <- ev:
		case <-ctx.Done():
			return
		}
	}
}
