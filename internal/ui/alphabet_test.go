package ui

import (
	"strings"
	"testing"

	"github.com/LukeHagar/plexgo/models/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestAlphabetRailMouseMatchesRenderedLettersAfterResize(t *testing.T) {
	for _, height := range []int{40, 27, 26, 14, 9, 6, 5} {
		rail := NewAlphabetRail(50, 4, height)
		seen := ""
		for y, line := range strings.Split(ansi.Strip(rail.View("M", PlexctlTheme)), "\n") {
			for x, letter := range line {
				if letter == ' ' {
					continue
				}
				mouse := tea.MouseMsg{X: rail.X + x, Y: rail.Y + y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
				if got := rail.LetterAt(mouse); got != string(letter) {
					t.Fatalf("height %d: clicked %c, got %q", height, letter, got)
				}
				mouse.Action = tea.MouseActionRelease
				if rail.LetterAt(mouse) != "" {
					t.Fatal("release activated a letter")
				}
				seen += string(letter)
			}
		}
		if len(seen) != 27 {
			t.Fatalf("height %d: only %d letters visible", height, len(seen))
		}
	}
}

func TestAlphabetRespectsSortTitleAccentsAndNumbers(t *testing.T) {
	for _, test := range []struct {
		meta components.Metadata
		want string
	}{
		{components.Metadata{Title: "The Matrix", TitleSort: Ptr("Matrix")}, "M"},
		{components.Metadata{Title: " Émilie"}, "E"},
		{components.Metadata{Title: "123"}, "#"},
		{components.Metadata{Title: "東京"}, "#"},
		{components.Metadata{}, "#"},
	} {
		if got := AlphabetLetter(test.meta); got != test.want {
			t.Fatalf("%q: got %q, want %q", test.meta.Title, got, test.want)
		}
	}
}
