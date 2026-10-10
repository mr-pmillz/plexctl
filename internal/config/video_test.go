package config

import "testing"

func TestParseTerminalVideo(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"auto", "kitty", "sixel", "tct", "", "gpu", "kitty,sixel", "--vo=gpu"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTerminalVideo(value)
			valid := value == "auto" || value == "kitty" || value == "sixel" || value == "tct"
			if valid && (err != nil || string(got) != value) {
				t.Fatalf("ParseTerminalVideo(%q) = %q, %v", value, got, err)
			}
			if !valid && err == nil {
				t.Fatalf("accepted invalid renderer %q", value)
			}
		})
	}
}
