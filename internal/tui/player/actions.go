package player

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/LukeHagar/plexgo/models/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ygelfand/plexctl/internal/config"
	"github.com/ygelfand/plexctl/internal/plex"
	"github.com/ygelfand/plexctl/internal/ui"
)

func PlayMedia(metadata *components.Metadata, noReport bool, output config.VideoOutput, startOffset int64, servers ...config.ServerTarget) tea.Cmd {
	server := config.MediaServer(servers...)
	part := plex.PlayablePart(metadata)
	if part == nil {
		return func() tea.Msg { return fmt.Errorf("no playable media found") }
	}

	if server.URL == "" {
		return func() tea.Msg { return fmt.Errorf("no active server") }
	}

	separator := "?"
	if strings.Contains(part.Key, "?") {
		separator = "&"
	}
	token := url.QueryEscape(server.Token)
	playURL := fmt.Sprintf("%s%s%sX-Plex-Token=%s", server.URL, part.Key, separator, token)

	var subtitles []ExternalSubtitle
	for _, stream := range part.Stream {
		if stream.StreamType == components.StreamTypeSubtitle && stream.Key != "" {
			sep := "?"
			if strings.Contains(stream.Key, "?") {
				sep = "&"
			}
			subURL := fmt.Sprintf("%s%s%sX-Plex-Token=%s", server.URL, stream.Key, sep, token)
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
	if metadata.Type == "track" && metadata.GrandparentTitle != nil {
		title = *metadata.GrandparentTitle + " — " + title
	}
	if noReport {
		title = "Trailer: " + title
	}

	return GetPlayerManager().Play(playURL, title, rk, noReport, output, startOffset, subtitles, server)
}

// FetchAndPlay handles the full playback logic: fetch full metadata, check for resume, then play
func FetchAndPlay(ratingKey string, output config.VideoOutput, servers ...config.ServerTarget) tea.Cmd {
	server := config.MediaServer(servers...)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(server.Context(context.Background()), 30*time.Second)
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
			return ui.ResumeChoiceMsg{Server: server,
				Metadata: meta,
				Output:   output,
			}
		}

		cmd := PlayMedia(meta, false, output, 0, server)
		if cmd != nil {
			return cmd()
		}
		return nil
	}
}

// FetchAndPlayTrailer handles finding and playing the correct trailer for a media item
func FetchAndPlayTrailer(parentMeta *components.Metadata, output config.VideoOutput, servers ...config.ServerTarget) tea.Cmd {
	server := config.MediaServer(servers...)
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

		meta, err := plex.GetMetadata(server.Context(context.Background()), ratingKey, false)
		if err != nil {
			return err
		}

		cmd := PlayMedia(meta, true, output, 0, server)
		if cmd != nil {
			return cmd()
		}
		return nil
	}
}
