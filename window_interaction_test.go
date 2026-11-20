package netease

import (
	"github.com/waygu-music-plugin/sdk/plugintest"
	"testing"
)

// Font and lock controls affect only an unlocked desktop lyrics window under the pointer.
func TestDesktopLyricsToolbar(t *testing.T) {
	plugintest.CheckDesktopLyricsToolbar(t, plugintest.Start(t, Registration()))
}
