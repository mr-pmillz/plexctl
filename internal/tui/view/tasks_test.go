package view

import (
	"testing"

	"github.com/ygelfand/plexctl/internal/ui"
)

func TestTasksTableNavigation(t *testing.T) {
	tab := NewTasksTab(ui.PlexctlTheme)
	tab.Update(navigationRows(len(tab.table.Columns())))
	verifyTableNavigation(t, tab, &tab.table)
}
