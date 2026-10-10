package player

import (
	"slices"
	"testing"
)

func TestPlaybackKeys(t *testing.T) {
	keys, pending := playbackKeys(" \x1b[Cq\x1b[", false)
	if !slices.Equal(keys, []string{"SPACE", "RIGHT", "q"}) || pending != "\x1b[" {
		t.Fatalf("keys=%v pending=%q", keys, pending)
	}
	keys, pending = playbackKeys(pending+"D\x03", false)
	if !slices.Equal(keys, []string{"LEFT", "q"}) || pending != "" {
		t.Fatalf("split arrow: %v, %q", keys, pending)
	}
	keys, pending = playbackKeys("\x1b", true)
	if !slices.Equal(keys, []string{"ESC"}) || pending != "" {
		t.Fatalf("escape timeout: %v, %q", keys, pending)
	}
}

func TestPlaybackMouseReports(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"\x1b[<0;20;8M", "SPACE"},
		{"\x1b[<0;20;8m", ""},
		{"\x1b[<32;20;8M", ""},
		{"\x1b[<64;20;8M", "RIGHT"},
		{"\x1b[<65;20;8M", "LEFT"},
		{"\x1b[<2;20;8M", "q"},
		{"\x1b[<0;bad;8M", ""},
	} {
		for split := 1; split < len(tc.input); split++ {
			first, pending := playbackKeys(tc.input[:split], false)
			last, pending := playbackKeys(pending+tc.input[split:], false)
			keys := append(first, last...)
			if pending != "" || (tc.want == "" && len(keys) != 0) || (tc.want != "" && !slices.Equal(keys, []string{tc.want})) {
				t.Fatalf("report %q split %d: keys=%v pending=%q", tc.input, split, keys, pending)
			}
		}
	}
}
