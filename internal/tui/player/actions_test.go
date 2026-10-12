package player

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LukeHagar/plexgo/models/operations"
	"github.com/ygelfand/plexctl/internal/config"
	"github.com/ygelfand/plexctl/internal/ui"
)

func TestSharedResumeAndProgressKeepOriginatingServer(t *testing.T) {
	var reports int
	friend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "friend-token" {
			t.Error("playback used a different account or server token")
			http.Error(w, "denied", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/:/timeline" {
			reports++
			_, _ = w.Write([]byte(`{"MediaContainer":{}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"MediaContainer": map[string]any{"Metadata": []any{map[string]any{
			"ratingKey": "1", "title": "Shared Film", "type": "movie", "viewOffset": 5000,
			"Media": []any{map[string]any{"Part": []any{map[string]any{"key": "/video"}}}},
		}}}})
	}))
	defer friend.Close()
	target := config.ServerTarget{ID: "friend", URL: friend.URL, Token: "friend-token", Shared: true}
	msg := FetchAndPlay("1", config.VideoWindow, target)()
	resume, ok := msg.(ui.ResumeChoiceMsg)
	if !ok {
		t.Fatalf("expected resume choice, got %T: %v", msg, msg)
	}
	if resume.Server != target || *resume.Metadata.ViewOffset != 5000 {
		t.Fatal("resume lost the shared server or offset")
	}
	reportProgress(context.Background(), PlayerStatus{Server: resume.Server, Key: "1", Time: 6, Duration: 60, State: operations.StatePlaying})
	if reports != 1 {
		t.Fatal("watch progress was not reported to the friend's server")
	}
}
