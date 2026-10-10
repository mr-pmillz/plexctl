package detail

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LukeHagar/plexgo/models/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/ygelfand/plexctl/internal/ui"
)

func TestMusicNavigationAndMouse(t *testing.T) {
	ui.GetLayout().Update(100, 40, false)
	artist := NewMusicDetailView("artist", ui.PlexctlTheme)
	artist.Update(detailDataMsg{
		metadata: &components.Metadata{Type: "artist", Title: "Artist"},
		children: []components.Metadata{{RatingKey: ui.Ptr("album"), Type: "album", Title: "The album"}},
	})
	artist.Update(ui.KeyPress("enter"))
	album := artist.selected
	if album == nil || album.RatingKey != "album" {
		t.Fatal("enter did not open the selected album")
	}
	var tracks []components.Metadata
	for i := range 20 {
		tracks = append(tracks, components.Metadata{RatingKey: ui.Ptr(fmt.Sprint(i)), Type: "track", Title: fmt.Sprintf("Song %02d", i), Index: ui.Ptr(i + 1)})
	}
	artist.Update(detailDataMsg{metadata: &components.Metadata{Type: "album", Title: "The album"}, children: tracks})
	album.View() // Establish the list's actual viewport size.
	for range 10 {
		album.Update(tea.MouseMsg{X: album.listRect.X + 3, Y: album.listRect.Y + 3, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	}
	view := ansi.Strip(album.View())
	clicked := false
	for y, line := range strings.Split(view, "\n") {
		if x := strings.Index(line, "Song "); x >= 0 {
			title := line[x : x+len("Song 00")]
			album.Update(tea.MouseMsg{X: lipgloss.Width(line[:x]) + 1, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			selected := album.items.SelectedItem().(musicItem)
			if album.selected == nil || selected.metadata.Title != title || album.selected.RatingKey != *selected.metadata.RatingKey {
				t.Fatalf("clicked %q but opened %+v", title, album.selected)
			}
			clicked = true
			break
		}
	}
	if !clicked {
		t.Fatal("no tracks rendered")
	}
	manager := DetailManager{View: artist}
	manager.Update(BackMsg{})
	if artist.selected != album || album.selected != nil {
		t.Fatal("back from track skipped its album")
	}
	if got := artist.GetSelectedMetadata(); got == nil || got.Type != "track" {
		t.Fatal("album did not expose its selected track for playback")
	}
	manager.Update(BackMsg{})
	if manager.View != artist || artist.selected != nil {
		t.Fatal("back from album skipped its artist")
	}
	manager.Update(BackMsg{})
	if manager.Active() {
		t.Fatal("back from artist did not return to the library")
	}
}
