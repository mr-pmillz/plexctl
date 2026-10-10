package player

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/LukeHagar/plexgo/models/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ygelfand/plexctl/internal/config"
	"github.com/ygelfand/plexctl/internal/plex"
	"github.com/ygelfand/plexctl/internal/ui"
)

func PlayMedia(metadata *components.Metadata, noReport bool, output config.VideoOutput, startOffset int64) tea.Cmd {
	part := plex.PlayablePart(metadata)
	if part == nil {
		return func() tea.Msg { return fmt.Errorf("no playable media found") }
	}

	cfg := config.Get()
	_, serverCfg, ok := cfg.GetActiveServer()
	if !ok {
		return func() tea.Msg { return fmt.Errorf("no active server") }
	}

	separator := "?"
	if strings.Contains(part.Key, "?") {
		separator = "&"
	}
	playURL := fmt.Sprintf("%s%s%sX-Plex-Token=%s", serverCfg.URL, part.Key, separator, cfg.Token)

	var subtitles []ExternalSubtitle
	for _, stream := range part.Stream {
		if stream.StreamType == components.StreamTypeSubtitle && stream.Key != "" {
			sep := "?"
			if strings.Contains(stream.Key, "?") {
				sep = "&"
			}
			subURL := fmt.Sprintf("%s%s%sX-Plex-Token=%s", serverCfg.URL, stream.Key, sep, cfg.Token)
			lang := ""
			if stream.LanguageCode != nil {
				lang = *stream.LanguageCode
			}
			subtitles = append(subtitles, ExternalSubtitle{
				URL:      subURL,
				Title:    stream.DisplayTitle,
				Language: lang,
			})
		}
	}

	slog.Debug("Playing media", "output", output, "offset", startOffset, "subtitles", len(subtitles))
	rk := ""
	if metadata.RatingKey != nil {
		rk = *metadata.RatingKey
	}

	title := metadata.Title
	if noReport {
		title = "Trailer: " + title
	}

	return GetPlayerManager().Play(playURL, title, rk, noReport, output, startOffset, subtitles)
}

// FetchAndPlay handles the full playback logic: fetch full metadata, check for resume, then play
func FetchAndPlay(ratingKey string, output config.VideoOutput) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		meta, err := plex.GetMetadata(ctx, ratingKey, true)
		if err != nil {
			return err
		}
		meta, err = plex.ResolvePlayable(ctx, meta)
		if err != nil {
			return err
		}

		if meta.ViewOffset != nil && *meta.ViewOffset > 0 {
			return ui.ResumeChoiceMsg{
				Metadata: meta,
				Output:   output,
			}
		}

		cmd := PlayMedia(meta, false, output, 0)
		if cmd != nil {
			return cmd()
		}
		return nil
	}
}

// FetchAndPlayTrailer handles finding and playing the correct trailer for a media item
func FetchAndPlayTrailer(parentMeta *components.Metadata, output config.VideoOutput) tea.Cmd {
	return func() tea.Msg {
		targetKey := ""
		if parentMeta.PrimaryExtraKey != nil {
			targetKey = *parentMeta.PrimaryExtraKey
		} else if parentMeta.Extras != nil {
			for _, extra := range parentMeta.Extras.Metadata {
				if extra.Subtype != nil && *extra.Subtype == "trailer" {
					if extra.RatingKey != nil {
						targetKey = *extra.RatingKey
						break
					}
				}
			}
		}

		if targetKey == "" {
			return fmt.Errorf("no trailer found for this item")
		}

		// Normalize key (some extra keys are full paths)
		parts := strings.Split(targetKey, "/")
		ratingKey := parts[len(parts)-1]

		slog.Debug("FetchAndPlayTrailer: Resolved trailer key", "parent", parentMeta.Title, "target", ratingKey)

		meta, err := plex.GetMetadata(context.Background(), ratingKey, false)
		if err != nil {
			return err
		}

		cmd := PlayMedia(meta, true, output, 0)
		if cmd != nil {
			return cmd()
		}
		return nil
	}
}
