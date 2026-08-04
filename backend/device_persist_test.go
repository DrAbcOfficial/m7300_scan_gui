package backend

import (
	"testing"
)

// TestDevicePersistence verifies that devices added via AddDevices survive a
// Load/Save round-trip (regression: frontend persist() must not overwrite them).
func TestDevicePersistence(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)

	app := &App{}

	devs := app.AddDevices([]Device{
		{Name: "M7300FDN series", Host: "192.168.160.214", Model: "m7300fdn"},
	})
	if len(devs) != 1 {
		t.Fatalf("AddDevices returned %d devices, want 1", len(devs))
	}

	// Same devices again -> no duplicates.
	devs = app.AddDevices([]Device{
		{Name: "M7300FDN series", Host: "192.168.160.214", Model: "m7300fdn"},
		{Name: "M7300FDW series", Host: "192.168.160.215", Model: "m7300fdw"},
	})
	if len(devs) != 2 {
		t.Fatalf("dedupe failed: %d devices, want 2", len(devs))
	}

	// Persisted on disk?
	s := LoadSettings()
	if len(s.Devices) != 2 || s.Devices[0].Host != "192.168.160.214" {
		t.Fatalf("devices not persisted: %+v", s.Devices)
	}

	// Rename.
	devs = app.RenameDevice("192.168.160.214", "办公室扫描仪")
	if devs[0].Name != "办公室扫描仪" {
		t.Fatalf("rename failed: %+v", devs[0])
	}

	// Active host.
	app.SetActiveDevice("192.168.160.215")
	if got := LoadSettings().ActiveHost; got != "192.168.160.215" {
		t.Fatalf("active host not saved: %q", got)
	}

	// Remove.
	devs = app.RemoveDevice("192.168.160.214")
	if len(devs) != 1 || devs[0].Host != "192.168.160.215" {
		t.Fatalf("remove failed: %+v", devs)
	}
	if got := LoadSettings(); len(got.Devices) != 1 || got.Devices[0].Name != "M7300FDW series" {
		t.Fatalf("post-remove state wrong: %+v", got.Devices)
	}
}
