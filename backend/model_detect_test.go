package backend

import (
	"strings"
	"testing"
)

func TestBuildScanArgs(t *testing.T) {
	s := DefaultSettings()
	s.RegionFull = false
	s.Brightness = 30
	s.Mode = "lineart"
	args := BuildScanArgs(s)
	got := strings.Join(args, " ")
	for _, want := range []string{
		"-s platen", "-r 300", "-m lineart",
		"--tl-x 0 --tl-y 0 --br-x 210 --br-y 297",
		"-b 30", "-t 128", "-f png", "-q 90", "-n 500", "-v",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("args missing %q in %s", want, got)
		}
	}
	s2 := DefaultSettings()
	s2.Format = "pdf"
	if got := strings.Join(BuildScanArgs(s2), " "); !strings.Contains(got, "--pdf") {
		t.Errorf("pdf format missing --pdf: %s", got)
	}
	s3 := DefaultSettings()
	s3.Format = "pdf-page"
	if got := strings.Join(BuildScanArgs(s3), " "); !strings.Contains(got, "--pdf-per-page") {
		t.Errorf("pdf-page format missing --pdf-per-page: %s", got)
	}
}

// TestDetectModelLive probes the real device on the LAN. Requires the scanner
// to be reachable; skipped automatically when it is not configured.
func TestDetectModelLive(t *testing.T) {
	hosts := readConfHosts("m7300fdn")
	if len(hosts) == 0 {
		t.Skip("m7300fdn.conf has no host configured")
	}
	info := DetectModel(hosts[0])
	t.Logf("DetectModel(%s) = %+v", hosts[0], info)
	if info.Model == "" {
		t.Errorf("model not detected: %+v", info)
	}
	if !strings.Contains(info.ModelName, strings.ToUpper(info.Model)) {
		t.Errorf("model name %q does not contain model id %q", info.ModelName, info.Model)
	}
	if info.Source != "wsd" {
		t.Errorf("expected wsd detection, got %q", info.Source)
	}
}
