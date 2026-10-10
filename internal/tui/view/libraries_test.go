package view

import (
	"testing"

	"github.com/ygelfand/plexctl/internal/ui"
)

func TestLibrariesTableNavigation(t *testing.T) {
	tab := NewLibrariesTab(ui.PlexctlTheme)
	tab.Update(navigationRows(len(tab.table.Columns())))
	verifyTableNavigation(t, tab, &tab.table)
}
