package netease

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/waygu-music-plugin/netease/internal/client"
)

// Cueing a song keeps the sidebar's scroll range so the old playing card cannot crowd out playlists.
func TestSidebarScrollRangeStaysStableWithCurrentTrack(t *testing.T) {
	a := editTestApp()
	for i := int64(100); i < 130; i++ {
		a.mine = append(a.mine, netease.Playlist{ID: i, Name: fmt.Sprintf("歌单 %d", i)})
	}
	saved := a.saved[0]
	a.playlists[saved.ID] = &saved
	tt := ui.NewTester(a.view, 1200, 700)
	tt.Frame()
	header, ok := tt.Find("我的歌单")
	if !ok || a.navScroll.MaxY <= 0 {
		t.Fatal("the sidebar fixture needs a scrollable playlist section")
	}
	before := a.navScroll.MaxY
	a.pl.Cue(a.playlists[10].Tracks)
	tt.Frame()
	if a.navScroll.MaxY != before {
		t.Fatalf("selecting a song changed the sidebar's scroll range from %v to %v", before, a.navScroll.MaxY)
	}

	tt.Scroll(header.X+header.W/2, header.Y+header.H/2, 0, before+100)
	tt.Frame()
	if err := tt.Click(saved.Name); err != nil {
		t.Fatalf("the last playlist cannot be opened after scrolling: %v", err)
	}
	if a.route != (route{page: pagePlaylist, id: saved.ID}) {
		t.Fatalf("opened %+v, want playlist %d", a.route, saved.ID)
	}
}

// Each playlist header hides only its own rows without changing the page
// or queue, and Enter restores their navigation.
func TestSidebarPlaylistCollapse(t *testing.T) {
	for _, tc := range []struct {
		name, header, row, other string
		playlist                 int64
	}{
		{"mine", "我的歌单", "睡前", "飙升榜", 11},
		{"saved", "收藏的歌单", "飙升榜", "睡前", 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := editTestApp()
			a.playlists[tc.playlist] = &netease.Playlist{ID: tc.playlist, Name: tc.row}
			a.pl.Cue(a.playlists[10].Tracks)
			before := a.route
			tt := ui.NewTester(a.view, 1200, 800)
			tt.Frame()

			if err := tt.Click(tc.header); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if _, ok := tt.Find(tc.row); ok {
				t.Error("the collapsed section still builds its playlist rows")
			}
			if _, ok := tt.Find(tc.other); !ok {
				t.Error("collapsing one section hid the other section's rows")
			}
			if a.route != before || len(a.pl.Queue()) != 2 || len(a.mine) != 3 || len(a.saved) != 1 {
				t.Error("collapsing the section changed the page, queue or playlists")
			}

			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if err := tt.Click(tc.row); err != nil {
				t.Fatalf("the restored playlist cannot be opened: %v", err)
			}
			tt.Frame()
			if a.route != (route{page: pagePlaylist, id: tc.playlist}) {
				t.Errorf("opened %+v, want playlist %d", a.route, tc.playlist)
			}
		})
	}
}

// The plus still opens the naming dialog with both sections collapsed, without expanding either one.
func TestNewPlaylistInCollapsedSidebar(t *testing.T) {
	a := editTestApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	for _, header := range []string{"我的歌单", "收藏的歌单"} {
		if err := tt.Click(header); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	if !a.mineCollapsed || !a.savedCollapsed {
		t.Fatal("the sections did not collapse independently")
	}
	if err := tt.Click("新建歌单"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.naming.open {
		t.Error("the plus did not open the naming dialog")
	}
	if !a.mineCollapsed || !a.savedCollapsed {
		t.Error("the plus also toggled a playlist section")
	}
}
