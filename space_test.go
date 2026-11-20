package netease

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/waygu-music-plugin/netease/internal/client"
)

// Space pauses right after a click opened Now Playing: the button clicked
// has the focus then, and would take Space as another click.
func TestNetEaseSpaceAfterOpeningNowPlaying(t *testing.T) {
	checkSpaceAfterOpeningNowPlaying(t, newTestApp())
}

func checkSpaceAfterOpeningNowPlaying(t *testing.T, a *testApp) {
	t.Helper()
	tr := netease.Track{ID: 7, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000}
	a.pl.Cue([]netease.Track{tr})
	a.pl.SetLyrics(nil)
	a.pl.Hold(0)

	tt := ui.NewTester(a.view, 1200, 800)
	if err := tt.Click("打开正在播放"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if _, ok := tt.Find("关闭"); !ok {
		t.Fatal("now playing did not open")
	}
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if a.pl.Playing() {
		t.Error("Space did not pause")
	}
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if !a.pl.Playing() {
		t.Error("Space did not play again")
	}
}
