package tui

import (
	"testing"
	"time"

	"github.com/LukeHagar/plexgo/models/operations"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ygelfand/plexctl/internal/tui/player"
	"github.com/ygelfand/plexctl/internal/tui/view"
	"github.com/ygelfand/plexctl/internal/ui"
	"go.dalton.dog/bubbleup"
)

func TestRightClickNavigatesBackFromAnotherPane(t *testing.T) {
	for _, tt := range []struct {
		name  string
		focus paneFocus
	}{
		{name: "sidebar", focus: paneSidebar},
		{name: "player", focus: panePlayer},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := focusController()
			c.alert = *bubbleup.NewAlertModel(40, true, time.Second)
			c.playerStatus = player.PlayerStatus{State: operations.StatePlaying}
			c.focusPane(tt.focus)
			ui.GetLayout().Update(100, 40, true)
			content := c.tabManager.ActiveModel().(*mouseOverlay)
			c.Update(tea.MouseMsg{
				X: ui.SidebarTotalWidth + 4, Y: 2,
				Button: tea.MouseButtonRight, Action: tea.MouseActionPress,
			})
			key, ok := content.last.(tea.KeyMsg)
			if !ok || key.Type != tea.KeyEsc {
				t.Fatal("right click did not send Back to the content pane")
			}
			if c.focus != paneContent {
				t.Fatal("right click did not restore content focus")
			}
		})
	}
}

type letterTab struct {
	mouseOverlay
	letter string
}

func (m *letterTab) CanJumpLetters() bool { return true }
func (m *letterTab) JumpToLetter(letter string) tea.Cmd {
	m.letter = letter
	return nil
}

func TestNavigationFocusesItsDestination(t *testing.T) {
	for _, tt := range []struct {
		name string
		msg  tea.Msg
		want int
	}{
		{name: "next library", msg: tea.KeyMsg{Type: tea.KeyTab}, want: 1},
		{name: "previous library", msg: tea.KeyMsg{Type: tea.KeyShiftTab}, want: 2},
		{name: "search result", msg: ui.JumpToDetailMsg{SectionID: "music", RatingKey: "500", Type: "artist"}, want: 2},
		{name: "detail in current library", msg: ui.JumpToDetailMsg{RatingKey: "500", Type: "artist"}, want: 2},
		{name: "alphabet destination", msg: ui.JumpToLetterMsg{Letter: "Z"}, want: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := focusController()
			c.tabManager.tabModels[2] = view.NewMediaView("music", "Music", ui.PlexctlTheme)
			if tt.name == "detail in current library" {
				c.tabManager.SetActive(2)
			}
			letter := &letterTab{}
			if tt.name == "alphabet destination" {
				c.tabManager.tabModels[0] = letter
			}
			c.focusPane(paneSidebar)
			c.Update(tt.msg)
			if c.focus != paneContent || c.tabManager.activeTabIdx != tt.want {
				t.Fatal("navigation left keyboard focus outside the destination library")
			}
			if tt.name == "alphabet destination" && letter.letter != "Z" {
				t.Fatal("alphabet jump did not reach the destination")
			}
		})
	}
}
