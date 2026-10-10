package poster

import (
	"testing"

	"github.com/LukeHagar/plexgo/models/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ygelfand/plexctl/internal/ui"
)

func TestPosterListResizeKeepsSelectionVisible(t *testing.T) {
	ui.GetLayout().Update(ui.SidebarTotalWidth+ui.MainOverhead+6*ui.PosterStepWidth, 50, false)
	m := NewPosterList(0, "Movies", make([]components.Metadata, 12), ui.PlexctlTheme)
	m.Cursor = 5
	m.Sync()
	ui.GetLayout().Update(ui.SidebarTotalWidth+ui.MainOverhead+3*ui.PosterStepWidth, 50, false)
	m.Update(tea.WindowSizeMsg{})
	if m.Cursor != 5 || m.Cursor < m.Offset || m.Cursor >= m.Offset+3 {
		t.Fatalf("resize left selection %d outside the row beginning at %d", m.Cursor, m.Offset)
	}
}
