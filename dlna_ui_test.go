package netease

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/waygu-music-plugin/netease/internal/client"
	"github.com/waygu-music-plugin/sdk/pluginapi"
)

// Tests that clicking the cast button toggles the DLNA device selection popover.
func TestCastButtonTogglesPopover(t *testing.T) {
	a := newTestApp()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "Test Track", Duration: 240000}})

	tt := ui.NewTester(func(c *ui.Context) {
		a.playerBar(c)
	}, 1200, 200)

	if a.castOpen {
		t.Error("expected castOpen to be false initially")
	}

	// Click cast button
	if err := tt.Click("无线投送 (DLNA)"); err != nil {
		t.Fatalf("expected to click cast button: %v", err)
	}

	if !a.castOpen {
		t.Error("expected castOpen to be true after clicking cast button")
	}

	// Popover should now display local device option
	if err := tt.Click("本机播放"); err != nil {
		t.Fatalf("expected to click '本机播放' option in cast popover: %v", err)
	}

	if a.castOpen {
		t.Error("expected castOpen to close after selecting an option")
	}
	if a.hostTest.casting.Sending {
		t.Error("expected player not to be casting when local device is chosen")
	}
}

// Tests that an active DLNA device is displayed in the popover even while scanning.
func TestActiveCastDeviceVisibleWhileScanning(t *testing.T) {
	a := newTestApp()
	device := pluginapi.CastingDevice{ID: "fixture-speaker", Name: "Bedroom Speaker"}
	a.hostTest.casting.Devices = []pluginapi.CastingDevice{device}
	a.hostTest.casting.Active = &device
	a.hostTest.casting.Sending = true

	tt := ui.NewTester(func(c *ui.Context) {
		a.playerBar(c)
	}, 1200, 200)

	// Set scanning flag to true to simulate background scan
	a.hostTest.casting.Scanning = true

	// Open popover
	if err := tt.Click("无线投送 (DLNA)"); err != nil {
		t.Fatalf("expected to click cast button: %v", err)
	}

	if !a.castOpen {
		t.Error("expected castOpen to be true")
	}

	// Device must still be present and clickable despite ongoing scan
	if err := tt.Click("Bedroom Speaker"); err != nil {
		t.Fatalf("expected Bedroom Speaker to be visible and clickable in popover: %v", err)
	}
	if a.hostTest.lastOutput != device.ID {
		t.Fatalf("selected output %q, want %q", a.hostTest.lastOutput, device.ID)
	}
}

// Tests that clicking the equalizer button on the player bar toggles the EQ popover when MPV is ready.
func TestEqualizerButtonTogglesPopover(t *testing.T) {
	a := newTestApp()
	a.settings.UseMPV = true
	a.hostTest.useMPV = true
	a.pl.Cue([]netease.Track{{ID: 1, Name: "Test Track", Duration: 240000}})

	tt := ui.NewTester(func(c *ui.Context) {
		a.playerBar(c)
	}, 1200, 200)

	if a.eqOpen {
		t.Error("expected eqOpen to be false initially")
	}

	if err := tt.Click("图形均衡器 (EQ)"); err != nil {
		t.Fatalf("expected to click equalizer button: %v", err)
	}

	if !a.eqOpen {
		t.Error("expected eqOpen to be true after clicking equalizer button")
	}
}
