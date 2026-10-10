package tui

import (
	"testing"

	"github.com/LukeHagar/plexgo/models/operations"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ygelfand/plexctl/internal/tui/player"
	"github.com/ygelfand/plexctl/internal/tui/widget/navbar"
	"github.com/ygelfand/plexctl/internal/ui"
)

func focusController() *Controller {
	nav := navbar.NewNavbar([]navbar.NavSection{{Items: []navbar.NavItem{
		{Title: "Home"}, {Title: "Movies"}, {Title: "Music"},
	}}}, ui.PlexctlTheme)
	return &Controller{
		theme: ui.PlexctlTheme,
		tabManager: &TabManager{
			navbar: nav, tabModels: []tea.Model{&mouseOverlay{}, &mouseOverlay{}, &mouseOverlay{}},
		},
		navigator: NewNavigator(ui.PlexctlTheme),
	}
}

func TestDirectionalPaneFocusAndKeyRepeat(t *testing.T) {
	c := focusController()
	c.playerStatus = player.PlayerStatus{State: operations.StatePlaying, AudioOnly: true}
	for _, step := range []struct {
		key  tea.KeyType
		pane paneFocus
	}{
		{tea.KeyCtrlH, paneSidebar}, {tea.KeyCtrlL, paneContent},
		{tea.KeyCtrlJ, panePlayer}, {tea.KeyCtrlK, paneContent},
		{tea.KeyCtrlJ, panePlayer}, {tea.KeyCtrlH, paneSidebar},
		{tea.KeyCtrlJ, paneSidebar}, {tea.KeyCtrlK, paneSidebar},
		{tea.KeyCtrlL, paneContent},
	} {
		for range 8 {
			c.Update(tea.KeyMsg{Type: step.key})
			if c.focus != step.pane {
				t.Fatalf("%v focused %v, want %v", step.key, c.focus, step.pane)
			}
		}
	}
	if c.navigator.ActiveOverlay() != nil {
		t.Fatal("pane navigation opened a dialog")
	}
	c.playerStatus = player.PlayerStatus{}
	c.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if c.focus != paneContent {
		t.Fatal("focused a missing player")
	}
}

func TestSidebarSelectionDoesNotLeakIntoContent(t *testing.T) {
	c := focusController()
	content := c.tabManager.ActiveModel().(*mouseOverlay)
	c.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	c.Update(ui.KeyPress("j"))
	c.Update(ui.KeyPress("down"))
	c.Update(ui.KeyPress("k"))
	if c.tabManager.navbar.Cursor() != 1 || c.tabManager.activeTabIdx != 0 || content.last != nil {
		t.Fatal("sidebar navigation changed content before activation")
	}
	c.Update(ui.KeyPress("enter"))
	if c.focus != paneContent || c.tabManager.activeTabIdx != 1 {
		t.Fatal("Enter did not open the selected library and focus it")
	}
	c.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	if c.tabManager.navbar.Cursor() != 1 {
		t.Fatal("returning to navigation lost the active library")
	}
}

func TestPaneKeysRespectDialogsAndMouseFocus(t *testing.T) {
	c := focusController()
	overlay := &mouseOverlay{}
	c.navigator.Push(overlay)
	c.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	if c.focus != paneContent || overlay.last.(tea.KeyMsg).Type != tea.KeyCtrlH {
		t.Fatal("pane key escaped a modal text field")
	}
	c.navigator.Pop()
	ui.GetLayout().Update(100, 40, false)
	c.renderBaseView()
	c.handleMouse(tea.MouseMsg{X: 4, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if c.focus != paneSidebar {
		t.Fatal("sidebar click did not focus navigation")
	}
	c.handleMouse(tea.MouseMsg{
		X: ui.SidebarTotalWidth + 4, Y: 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	if c.focus != paneContent {
		t.Fatal("content click did not restore keyboard focus")
	}
}
