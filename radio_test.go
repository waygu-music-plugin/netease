package netease

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/waygu-music-plugin/netease/internal/client"
	"github.com/waygu-music-plugin/sdk/pluginapi"
)

// Restoring a paused radio queue reattaches its feed at the saved track and position.
func TestNetEaseRestoresPausedRadio(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.pl.Reset)
	a.hostTest.savedSession = &pluginapi.PlaybackSession{
		Queue: []pluginapi.Track{{ID: 7}, {ID: 8, Duration: 200000}, {ID: 9}, {ID: 10}},
		Order: []int{0, 1, 2, 3}, At: 1, Position: 48000, Radio: "fm:DEFAULT",
	}
	a.restoreSession()
	current := a.pl.Current()
	if current == nil || current.ID != 8 || a.pl.Playing() || a.pl.Position() != 48*time.Second || a.pl.Radio() != "fm:DEFAULT" {
		t.Fatalf("restored track %v at %v, playing=%v, radio=%q", current, a.pl.Position(), a.pl.Playing(), a.pl.Radio())
	}
}

// A saved radio with a corrupt play order cannot attach its feed to an unrelated current song.
func TestNetEaseRejectsRadioWithInvalidSavedOrder(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.pl.Reset)
	a.pl.Cue([]pluginapi.Track{{ID: 9}})
	a.hostTest.savedSession = &pluginapi.PlaybackSession{
		Queue: []pluginapi.Track{{ID: 7}, {ID: 8}}, Order: []int{0, 0}, Radio: heartRadio,
	}
	a.restoreSession()
	if current := a.pl.Current(); current == nil || current.ID != 9 || a.pl.Radio() != "" {
		t.Fatalf("invalid saved radio replaced the current queue: track %v, radio %q", current, a.pl.Radio())
	}
}

// A heart-mode refill uses the queue sent by the host even if the window still displays an older snapshot.
func TestNetEaseHeartRefillUsesHostQueue(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.pl.Reset)
	a.pl.Cue([]pluginapi.Track{{ID: 3}, {ID: 4}})
	a.mine = []netease.Playlist{{ID: 51, SpecialType: 5}}
	previous := fetchHeartMode
	defer func() { fetchHeartMode = previous }()
	requested := make(chan [3]int64, 1)
	fetchHeartMode = func(_ context.Context, last, first, playlist int64) ([]netease.Track, error) {
		requested <- [3]int64{last, first, playlist}
		return nil, pluginapi.ErrQueueFeedUnavailable
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	a.radioFeed(heartRadio)(ctx, pluginapi.QueueFeedCursor{Page: 2, FirstID: 17, LastID: 23}, func([]netease.Track, error) {})
	select {
	case ids := <-requested:
		if ids != [3]int64{23, 17, 51} {
			t.Fatalf("heart mode used the wrong seed, tail or playlist: %v", ids)
		}
	case <-ctx.Done():
		t.Fatal("heart mode did not request a refill")
	}
}

func TestCommentTime(t *testing.T) {
	now := time.Date(2026, 10, 25, 22, 0, 0, 0, time.Local)
	for _, c := range []struct {
		t    time.Time
		want string
	}{
		{now.Add(-20 * time.Second), "刚刚"},
		{now.Add(-42 * time.Minute), "42分钟前"},
		{now.Add(-5 * time.Hour), "17:00"},
		{now.Add(-23 * time.Hour), "昨天 23:00"},
		{time.Date(2026, 3, 8, 9, 0, 0, 0, time.Local), "3月8日"},
		{time.Date(2021, 4, 10, 9, 0, 0, 0, time.Local), "2021年4月10日"},
	} {
		if got := commentTime(c.t, now); got != c.want {
			t.Errorf("%v: %q, want %q", c.t, got, c.want)
		}
	}
}

// Heart mode playing, its card and Now Playing offer to leave it, which
// keeps the song playing.
func TestExitHeartMode(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	a.pl.Cue([]netease.Track{{ID: 1, Name: "晚风"}, {ID: 2, Name: "推荐的歌"}})
	a.pl.AttachRadio(heartRadio, func(context.Context, pluginapi.QueueFeedCursor, func([]netease.Track, error)) {})
	a.route = route{page: pageFM}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("退出心动模式"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.pl.Radio() != "" || a.pl.Current().ID != 1 || len(a.pl.Queue()) != 1 {
		t.Fatalf("radio %q, current %v, queue %d", a.pl.Radio(), a.pl.Current(), len(a.pl.Queue()))
	}
	if !tt.HasText("开启心动模式") {
		t.Errorf("card still on: %q", tt.Texts())
	}

	a.pl.AttachRadio(heartRadio, func(context.Context, pluginapi.QueueFeedCursor, func([]netease.Track, error)) {})
	a.nowPlaying = true
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("退出心动模式"); err != nil {
		t.Fatal(err)
	}
	if a.pl.Radio() != "" || a.pl.Current().ID != 1 {
		t.Fatalf("from Now Playing: radio %q, current %v", a.pl.Radio(), a.pl.Current())
	}
}
