package backend

import (
	"testing"
	"time"
)

// TestDiscoverDevicesLive runs a real WSD discovery on the local network and
// checks that at least the configured device is found.
func TestDiscoverDevicesLive(t *testing.T) {
	hosts := readConfHosts("m7300fdn")
	if len(hosts) == 0 {
		t.Skip("m7300fdn.conf has no host configured")
	}
	done := make(chan []ModelInfo, 1)
	go func() {
		done <- DiscoverDevices()
	}()
	var devs []ModelInfo
	select {
	case devs = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DiscoverDevices did not return within 30s")
	}
	t.Logf("discovered %d device(s)", len(devs))
	for _, d := range devs {
		t.Logf("  %s %s (%s)", d.Model, d.ModelName, d.Host)
	}
	if len(devs) == 0 {
		t.Fatal("no devices discovered")
	}
	foundConfigured := false
	for _, d := range devs {
		if d.Host == hosts[0] {
			foundConfigured = true
		}
	}
	if !foundConfigured {
		t.Errorf("configured host %s not found in discovery", hosts[0])
	}
}
