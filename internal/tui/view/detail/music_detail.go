package detail

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LukeHagar/plexgo/models/components"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	tint "github.com/lrstanley/bubbletint"
	"github.com/ygelfand/plexctl/internal/plex"
	"github.com/ygelfand/plexctl/internal/ui"
)

// MusicDetailView navigates artist -> album -> track while retaining each
// parent's selection and scroll position when returning from a child.
type MusicDetailView struct {
	DetailBase
	items    list.Model
	listRect ui.Rect
	selected *MusicDetailView
}

func NewMusicDetailView(ratingKey string, theme tint.Tint) *MusicDetailView {
	return &MusicDetailView{DetailBase: NewDetailBase(ratingKey, theme), items: createBaseDetailList()}
}

func (v *MusicDetailView) Init() tea.Cmd { return v.fetchData }

func (v *MusicDetailView) Refresh() tea.Cmd {
	if v.selected != nil {
		return v.selected.Refresh()
	}
	v.Loading = true
	return v.fetchData
}

func (v *MusicDetailView) fetchData() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	meta, err := plex.GetMetadata(ctx, v.RatingKey, true)
	if err != nil {
		return err
	}
	var children []components.Metadata
	if meta.Type == "artist" || meta.Type == "album" {
		children, err = plex.GetChildren(ctx, v.RatingKey)
		if err != nil {
			return err
		}
	}
	return detailDataMsg{metadata: meta, children: children}
}

func (v *MusicDetailView) GetSelectedMetadata() *components.Metadata {
	if v.selected != nil {
		return v.selected.GetSelectedMetadata()
	}
	if item, ok := v.items.SelectedItem().(musicItem); ok {
		return &item.metadata
	}
	return v.Metadata
}

func (v *MusicDetailView) IsAtRoot() bool { return v.selected == nil }

func (v *MusicDetailView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if v.selected != nil {
		if _, back := msg.(BackMsg); back && v.selected.IsAtRoot() {
			v.selected = nil
			return v, nil
		}
		_, cmd := v.selected.Update(msg)
		return v, cmd
	}
	if mouse, ok := msg.(tea.MouseMsg); ok {
		if local, inside := v.listRect.Local(mouse); inside {
			if ui.ListMouse(&v.items, local) {
				return v.Update(ui.KeyPress("enter"))
			}
			return v, nil
		}
	}
	switch msg := msg.(type) {
	case detailDataMsg:
		items := make([]list.Item, 0, len(msg.children))
		for _, child := range msg.children {
			items = append(items, musicItem{metadataItem{metadata: child}})
		}
		v.items.SetItems(items)
		v.items.Title = "Tracks"
		if msg.metadata != nil && msg.metadata.Type == "artist" {
			v.items.Title = "Albums"
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := v.items.SelectedItem().(musicItem); ok && item.metadata.RatingKey != nil {
				v.selected = NewMusicDetailView(*item.metadata.RatingKey, v.Theme)
				return v, v.selected.Init()
			}
		case "esc", "backspace":
			return v, func() tea.Msg { return BackMsg{} }
		}
	}
	baseCmd := v.DetailBase.Update(msg)
	var listCmd tea.Cmd
	v.items, listCmd = v.items.Update(msg)
	return v, tea.Batch(baseCmd, listCmd)
}

func (v *MusicDetailView) HelpKeys() []ui.HelpKey {
	return []ui.HelpKey{
		{Key: "enter", Desc: "Open Album / Track Details"},
		{Key: "p", Desc: "Play Selected Track (First Track of an Album)"},
		{Key: "↑/↓", Desc: "Select Album / Track"},
		{Key: "esc", Desc: "Back"},
	}
}

func (v *MusicDetailView) View() string {
	if v.selected != nil {
		return v.selected.View()
	}
	if v.Loading {
		return "\n  Loading music..."
	}
	if v.Err != nil {
		return "\n  " + v.Err.Error() + "\n\n [esc] Back"
	}
	if v.Metadata == nil {
		return "\n  No music metadata available\n\n [esc] Back"
	}
	width := max(1, ui.GetLayout().InnerWidth()-4)
	infoWidth := width
	if v.Metadata.Type == "track" {
		infoWidth = v.SynopsisVP.Width
	}
	info := musicItem{metadataItem{metadata: *v.Metadata}}.Description()
	header := lipgloss.JoinVertical(lipgloss.Left,
		v.RenderHeader(infoWidth),
		lipgloss.NewStyle().Width(infoWidth).Foreground(v.Theme.BrightCyan()).Render(info),
	)
	v.listRect = ui.Rect{}
	footer := "[p] Play | [esc] Back"
	if v.Metadata.Type == "track" {
		header += "\n\n" + renderBadges(v.Metadata, v.Theme)
		header = v.RenderPosterAndInfo(header)
	} else {
		v.listRect = ui.Rect{X: 2, Y: 2 + lipgloss.Height(header), Width: width}
		v.listRect.Height = max(4, ui.GetLayout().ContentHeight()-v.listRect.Y-4)
		v.items.SetSize(width, v.listRect.Height)
		header += "\n\n" + v.items.View()
		footer = "[enter] Open | [p] Play | [esc] Back"
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(header) + "\n\n " +
		lipgloss.NewStyle().Foreground(v.Theme.BrightBlack()).Render(footer)
}

type musicItem struct{ metadataItem }

func (i musicItem) Title() string {
	if i.metadata.Type == "track" && i.metadata.Index != nil {
		return fmt.Sprintf("%02d. %s", *i.metadata.Index, i.metadata.Title)
	}
	return i.metadata.Title
}

func (i musicItem) Description() string {
	var parts []string
	if i.metadata.GrandparentTitle != nil {
		parts = append(parts, *i.metadata.GrandparentTitle)
	}
	if i.metadata.ParentTitle != nil {
		parts = append(parts, *i.metadata.ParentTitle)
	}
	if i.metadata.Type == "track" && i.metadata.Duration != nil {
		seconds := *i.metadata.Duration / 1000
		parts = append(parts, fmt.Sprintf("%d:%02d", seconds/60, seconds%60))
	} else if i.metadata.Year != nil {
		parts = append(parts, fmt.Sprint(*i.metadata.Year))
	}
	return strings.Join(parts, " · ")
}
