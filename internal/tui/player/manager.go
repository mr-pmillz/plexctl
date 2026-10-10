package player

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/LukeHagar/plexgo/models/operations"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ygelfand/plexctl/internal/config"
	"github.com/ygelfand/plexctl/internal/plex"
)

type PlayerManager struct {
	// ioMu serializes commands and lifecycle changes. Rendering only takes
	// statusMu, so a slow IPC request never blocks the terminal UI.
	ioMu       sync.Mutex
	conn       *ipcClient
	cancel     context.CancelFunc
	socketPath string
	statusMu   sync.RWMutex
	status     PlayerStatus
	updates    chan tea.Msg
	done       chan struct{}
	closeOnce  sync.Once
	workers    sync.WaitGroup
}

var (
	instance *PlayerManager
	once     sync.Once
)

func GetPlayerManager() *PlayerManager {
	once.Do(func() {
		socket := filepath.Join(config.Get().CacheDir, "mpv.sock")
		if runtime.GOOS == "windows" {
			socket = "\\\\.\\pipe\\mpv-socket"
		} else {
			_ = os.MkdirAll(filepath.Dir(socket), 0o755)
		}
		instance = newPlayerManager(socket)
	})
	return instance
}

func newPlayerManager(socket string) *PlayerManager {
	return &PlayerManager{socketPath: socket, updates: make(chan tea.Msg, 1), done: make(chan struct{})}
}

func (pm *PlayerManager) Status() PlayerStatus {
	pm.statusMu.RLock()
	defer pm.statusMu.RUnlock()
	return pm.status
}

func (pm *PlayerManager) setStatus(status PlayerStatus) {
	pm.statusMu.Lock()
	pm.status = status
	pm.statusMu.Unlock()
}

func (pm *PlayerManager) VerifyConnection() bool {
	pm.ioMu.Lock()
	defer pm.ioMu.Unlock()
	return pm.conn != nil && pm.conn.get("mpv-version", nil) == nil
}

func (pm *PlayerManager) Play(
	url, title, ratingKey string, noReport bool, output config.VideoOutput,
	startOffset int64, subtitles []ExternalSubtitle,
) tea.Cmd {
	if output != config.VideoWindow && output != config.AudioTerminal {
		return pm.playTerminal(url, title, ratingKey, noReport, output, startOffset, subtitles)
	}
	return func() tea.Msg {
		previous, err := pm.start(url, title, ratingKey, noReport, output, startOffset, subtitles)
		if err != nil {
			return err
		}
		if previous.Active() {
			previous.State = operations.StateStopped
			reportProgress(context.Background(), previous)
		}
		pm.sendUpdate()
		reportProgress(context.Background(), pm.Status())
		return nil
	}
}

func (pm *PlayerManager) start(
	url, title, ratingKey string, noReport bool, output config.VideoOutput,
	startOffset int64, subtitles []ExternalSubtitle,
) (PlayerStatus, error) {
	pm.ioMu.Lock()
	defer pm.ioMu.Unlock()
	previous := pm.Status()
	select {
	case <-pm.done:
		return previous, fmt.Errorf("music player is closed")
	default:
	}
	if err := pm.connect(); err != nil {
		return previous, err
	}
	audioOnly := output == config.AudioTerminal
	// Remove only our filter; preserve the user's other audio filters.
	_ = pm.conn.call("af", "remove", "@plexctl-spectrum")
	if audioOnly {
		// Minimal builds may lack lavfi. Playback still works without meters.
		_ = pm.conn.call("af", "add", spectrumFilter())
	}
	if err := pm.conn.command(mediaLoadCommand(url, startOffset, audioOnly), nil); err != nil {
		return previous, err
	}
	_ = pm.conn.call("set_property", "pause", false)
	_ = pm.conn.call("set_property", "force-media-title", title)
	_ = pm.conn.call("set_property", "user-data/plex-rating-key", ratingKey)
	_ = pm.conn.call("set_property", "user-data/plex-title", title)
	_ = pm.conn.call("set_property", "user-data/plex-no-report", noReport)
	_ = pm.conn.call("set_property", "user-data/plex-audio-only", audioOnly)
	for _, sub := range subtitles {
		_ = pm.conn.call("sub-add", sub.URL, "auto", sub.Title, sub.Language)
	}
	pm.setStatus(PlayerStatus{
		Title: title, File: url, Key: ratingKey, NoReport: noReport, AudioOnly: audioOnly,
		Time: float64(startOffset) / 1000, State: operations.StatePlaying, Volume: previous.Volume,
	})
	return previous, nil
}

// connect is called with ioMu held. Connection identity prevents a stale
// monitor from clearing a replacement player.
func (pm *PlayerManager) connect() error {
	if pm.conn != nil && pm.conn.get("mpv-version", nil) == nil {
		return nil
	}
	pm.disconnect()
	if conn, err := openIPC(pm.socketPath); err == nil {
		if conn.get("mpv-version", nil) == nil {
			pm.attach(conn)
			return nil
		}
		_ = conn.conn.Close()
	}
	mpv, err := exec.LookPath("mpv")
	if err != nil {
		return fmt.Errorf("mpv command not found; please install mpv: %w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Remove(pm.socketPath)
	}
	// Embedded album art must never create a window for in-terminal music.
	cmd := exec.Command(mpv, "--idle=yes", "--no-resume-playback", "--force-window=no",
		"--terminal=no", "--input-terminal=no", "--input-ipc-server="+pm.socketPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start mpv: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	retry := time.NewTicker(50 * time.Millisecond)
	defer retry.Stop()
	for {
		select {
		case <-exited:
			return fmt.Errorf("mpv exited before opening its playback connection")
		case <-deadline.C:
			_ = cmd.Process.Kill()
			<-exited
			return fmt.Errorf("timed out connecting to mpv")
		case <-retry.C:
			conn, err := openIPC(pm.socketPath)
			if err != nil {
				continue
			}
			if conn.get("mpv-version", nil) == nil {
				pm.attach(conn)
				return nil
			}
			_ = conn.conn.Close()
		}
	}
}

func (pm *PlayerManager) attach(conn *ipcClient) {
	ctx, cancel := context.WithCancel(context.Background())
	pm.conn, pm.cancel = conn, cancel
	reports := make(chan PlayerStatus, 4)
	pm.workers.Add(2)
	go func() {
		defer pm.workers.Done()
		pm.monitor(ctx, conn, reports)
	}()
	go func() {
		defer pm.workers.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case status := <-reports:
				reportProgress(ctx, status)
			}
		}
	}()
}

func (pm *PlayerManager) Reconnect() tea.Cmd {
	return func() tea.Msg {
		pm.ioMu.Lock()
		defer pm.ioMu.Unlock()
		select {
		case <-pm.done:
			return nil
		default:
		}
		if pm.conn != nil {
			return nil
		}
		conn, err := openIPC(pm.socketPath)
		if err != nil {
			return nil
		}
		if conn.get("mpv-version", nil) != nil {
			_ = conn.conn.Close()
			return nil
		}
		var status PlayerStatus
		_ = conn.get("user-data/plex-title", &status.Title)
		_ = conn.get("user-data/plex-rating-key", &status.Key)
		_ = conn.get("user-data/plex-no-report", &status.NoReport)
		_ = conn.get("user-data/plex-audio-only", &status.AudioOnly)
		_ = conn.get("path", &status.File)
		pm.setStatus(status)
		pm.attach(conn)
		return nil
	}
}

func (pm *PlayerManager) control(args ...any) tea.Cmd {
	return func() tea.Msg {
		pm.ioMu.Lock()
		defer pm.ioMu.Unlock()
		if pm.conn == nil {
			return nil
		}
		if err := pm.conn.call(args...); err != nil {
			return fmt.Errorf("player control failed: %w", err)
		}
		return nil
	}
}

func (pm *PlayerManager) TogglePause() tea.Cmd { return pm.control("cycle", "pause") }
func (pm *PlayerManager) Seek(seconds float64) tea.Cmd {
	return pm.control("seek", seconds, "relative")
}
func (pm *PlayerManager) SeekPercent(percent float64) tea.Cmd {
	return pm.control("seek", max(0, min(100, percent)), "absolute-percent")
}
func (pm *PlayerManager) Volume(delta float64) tea.Cmd { return pm.control("add", "volume", delta) }
func (pm *PlayerManager) ToggleMute() tea.Cmd          { return pm.control("cycle", "mute") }

func (pm *PlayerManager) StopPlayback() tea.Cmd {
	return func() tea.Msg {
		pm.ioMu.Lock()
		previous := pm.Status()
		if pm.conn != nil {
			_ = pm.conn.call("quit")
		}
		pm.disconnect()
		pm.setStatus(PlayerStatus{})
		pm.ioMu.Unlock()
		previous.State = operations.StateStopped
		reportProgress(context.Background(), previous)
		pm.sendUpdate()
		return nil
	}
}

func (pm *PlayerManager) Stop() tea.Cmd {
	return func() tea.Msg { pm.StopPlayback()(); return tea.Quit() }
}

// Close stops in-terminal music and releases IPC. An external player the user
// elected to keep running can continue after the TUI exits.
func (pm *PlayerManager) Close() {
	pm.closeOnce.Do(func() {
		close(pm.done)
		pm.ioMu.Lock()
		status := pm.Status()
		if status.AudioOnly && pm.conn != nil {
			_ = pm.conn.call("quit")
		}
		pm.disconnect()
		pm.ioMu.Unlock()
		pm.workers.Wait()
	})
}

func (pm *PlayerManager) disconnect() {
	if pm.cancel != nil {
		pm.cancel()
		pm.cancel = nil
	}
	if pm.conn != nil {
		_ = pm.conn.conn.Close()
		pm.conn = nil
	}
}

func (pm *PlayerManager) monitor(ctx context.Context, conn *ipcClient, reports chan PlayerStatus) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	lastReport := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.ioMu.Lock()
			if ctx.Err() != nil || pm.conn != conn {
				pm.ioMu.Unlock()
				return
			}
			previous := pm.Status()
			status := previous
			var idle, paused bool
			err := conn.get("idle-active", &idle)
			if err == nil && !idle {
				_ = conn.get("pause", &paused)
				status.State = operations.StatePlaying
				if paused {
					status.State = operations.StatePaused
				}
				_ = conn.get("time-pos", &status.Time)
				_ = conn.get("duration", &status.Duration)
				_ = conn.get("volume", &status.Volume)
				_ = conn.get("mute", &status.Muted)
				if status.AudioOnly && !paused {
					var metadata map[string]string
					_ = conn.get("af-metadata/plexctl-spectrum", &metadata)
					status.Spectrum = readSpectrum(metadata)
				}
			} else {
				status.State = operations.StateStopped
				status.Spectrum = Spectrum{}
			}
			pm.setStatus(status)
			if err != nil {
				pm.disconnect()
			}
			pm.ioMu.Unlock()
			if previous.State != status.State {
				pm.sendUpdate()
			}
			if previous.State != status.State || time.Since(lastReport) >= 10*time.Second {
				// Plex HTTP latency must never stall visualization or controls.
				select {
				case reports <- status:
				default:
				}
				lastReport = time.Now()
			}
			if err != nil {
				// The connection context has been canceled; report the final
				// position with its own bounded request after an external exit.
				if previous.Active() {
					reportProgress(context.Background(), status)
				}
				return
			}
		}
	}
}

func (pm *PlayerManager) PollUpdates() tea.Cmd {
	interval := 250 * time.Millisecond
	if status := pm.Status(); status.Active() && status.AudioOnly {
		interval = 50 * time.Millisecond
	}
	return tea.Tick(interval, func(time.Time) tea.Msg { return PlayerStatusMsgPoller{} })
}

func (pm *PlayerManager) sendUpdate() {
	select {
	case pm.updates <- PlayerStatusMsg{}:
	default:
	}
}

func (pm *PlayerManager) WaitForUpdates() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-pm.updates:
			return msg
		case <-pm.done:
			return nil
		}
	}
}

func reportProgress(ctx context.Context, status PlayerStatus) {
	if status.NoReport || status.Key == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	client, err := plex.NewClient()
	if err != nil {
		return
	}
	timePos, duration := int64(status.Time*1000), int64(status.Duration*1000)
	device := "PlexCTL"
	_, _ = client.SDK.Timeline.Report(ctx, operations.ReportRequest{
		State: &status.State, Time: &timePos, Duration: &duration,
		Key: &status.Key, RatingKey: &status.Key, Device: &device,
	})
}
