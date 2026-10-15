package player

import (
	"testing"

	"github.com/LukeHagar/plexgo/models/operations"
	"github.com/ygelfand/plexctl/internal/config"
)

func TestQueueEventsReportEveryTrack(t *testing.T) {
	server := config.ServerTarget{ID: "friend", URL: "https://friend.invalid", Token: "private"}
	first := PlayItem{URL: "repeated-file", RatingKey: "one", Title: "First", Duration: 3}
	second := PlayItem{URL: "repeated-file", RatingKey: "two", Title: "Second", Duration: 4}
	pm := newPlayerManager("")
	pm.queue = []queueEntry{{ID: 10, PlayItem: first}, {ID: 11, PlayItem: second}}
	pm.conn = &ipcClient{events: []terminalEvent{
		{Event: "end-file", PlaylistEntryID: 10, Reason: "eof"},
		{Event: "start-file", PlaylistEntryID: 11},
		{Event: "end-file", PlaylistEntryID: 11, Reason: "eof"},
		{Event: "end-file", PlaylistEntryID: 999, Reason: "eof"},
	}}
	status := PlayerStatus{Server: server, AudioOnly: true, NoReport: true, State: operations.StatePlaying}
	setTrack(&status, first)
	reports := pm.queueEvents(&status)
	if len(reports) != 3 {
		t.Fatalf("missing track boundaries: %+v", reports)
	}
	for i, report := range reports {
		if report.Server != server || !report.NoReport || !report.AudioOnly {
			t.Fatalf("track %d lost playback context: %+v", i, report)
		}
	}
	if reports[0].Key != "one" || reports[0].Time != 3 || reports[0].State != operations.StateStopped {
		t.Fatalf("first track not completed: %+v", reports[0])
	}
	if reports[1].Key != "two" || reports[1].Time != 0 || reports[1].Title != "Second" {
		t.Fatalf("second track not reset: %+v", reports[1])
	}
	if status.Key != "two" || status.Time != 4 || status.State != operations.StateStopped {
		t.Fatalf("last track not completed: %+v", status)
	}
	if len(pm.queueEvents(&status)) != 0 {
		t.Fatal("track boundaries were reported twice")
	}
}
