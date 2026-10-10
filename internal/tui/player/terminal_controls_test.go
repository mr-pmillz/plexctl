package player

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/LukeHagar/plexgo/models/operations"
	"github.com/ygelfand/plexctl/internal/termgraphics"
)

func TestVideoControlsMouse(t *testing.T) {
	for _, frame := range []termgraphics.Frame{
		{Width: 960, Height: 540},
		{X: 12, Y: 6, Width: 720, Height: 406},
		{X: 35, Y: 1, Width: 450, Height: 800},
	} {
		c := terminalControls{focus: -1, hover: -1, speed: 1}
		status := PlayerStatus{Time: 30, Duration: 120, Volume: 80}
		l := c.layout(frame, status)
		at := func(x, y float64, button int, pressed bool) []any {
			m := playbackMouse{Button: button, Pressed: pressed,
				X: frame.X + int(x*float64(frame.Width)/float64(l.w)/9),
				Y: frame.Y + int(y*float64(frame.Height)/float64(l.h)/18)}
			return c.input(playbackInput{Mouse: &m}, l, frame, 9, 18, status)
		}
		for i, want := range [][]any{
			{"cycle", "pause"}, {"seek", -10, "relative+exact"}, {"seek", 10, "relative+exact"},
			{"cycle", "mute"}, {"add", "volume", -5}, {"add", "volume", 5},
			{"cycle", "sub-visibility"}, {"set_property", "speed", 1.25}, {"quit"},
		} {
			button := l.buttons[i]
			got := at(button.x+button.w/2, button.y+button.h/2, 0, true)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("frame %+v button %s: got %v want %v", frame, button.label, got, want)
			}
			if got := at(button.x+button.w/2, button.y+button.h/2, 0, false); len(got) != 0 {
				t.Fatal("release activated a button twice", got)
			}
		}
		for i, fraction := range []float64{.25, .6, .75} {
			button, pressed := 0, true
			if i == 1 {
				button = 32
			}
			if i == 2 {
				pressed = false
			}
			got := at(l.seek.x+l.seek.w*fraction, l.seek.y+l.seek.h/2, button, pressed)
			if len(got) != 3 || got[0] != "seek" || got[2] != "absolute+exact" || math.Abs(got[1].(float64)-fraction*120) > 3 {
				t.Fatalf("click/drag/release seek fraction %v, frame %+v: %v", fraction, frame, got)
			}
		}
		if c.dragging {
			t.Fatal("mouse release left seek dragging active")
		}
		status.Duration = 0
		if got := at(l.seek.x+l.seek.w/2, l.seek.y+l.seek.h/2, 0, true); len(got) != 0 {
			t.Fatal("seek without a duration", got)
		}
	}
}

func TestVideoControlsKeyboardAndDisplay(t *testing.T) {
	c := terminalControls{focus: -1, hover: -1, speed: 1}
	frame := termgraphics.Frame{Width: 1280, Height: 720}
	status := PlayerStatus{Title: "Film {\\pos(0,0)}\nsecret", Duration: 7200, Time: 3661, Volume: 75, State: operations.StatePaused}
	l := c.layout(frame, status)
	key := func(s string) []any { return c.input(playbackInput{Key: s}, l, frame, 8, 16, status) }
	overlay := c.overlay(l, status)
	for _, text := range []string{"Play", "Return", "Vol 75%", "1:01:01 / 2:00:00", "h: hide"} {
		if !strings.Contains(overlay, text) {
			t.Errorf("missing control/state %q", text)
		}
	}
	if strings.Contains(overlay, "Film {\\pos") || strings.Contains(overlay, "\nsecret") {
		t.Fatal("media title injected ASS markup")
	}
	key("TAB")
	if got := key("ENTER"); !reflect.DeepEqual(got, []any{"cycle", "pause"}) {
		t.Fatal("keyboard pause", got)
	}
	key("Shift+TAB")
	if got := key("ENTER"); !reflect.DeepEqual(got, []any{"quit"}) {
		t.Fatal("keyboard return", got)
	}
	key("h")
	if c.overlay(l, status) != "" {
		t.Fatal("hidden overlay still visible")
	}
	key("TAB")
	if c.hidden {
		t.Fatal("Tab did not reveal controls")
	}
	if got := key("RIGHT"); !reflect.DeepEqual(got, []any{"keypress", "RIGHT"}) {
		t.Fatal("mpv arrow binding was lost")
	}
}
