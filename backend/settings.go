package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Device is a saved scanner discovered via WSD/USB or added manually.
type Device struct {
	Name  string `json:"name"`  // user-visible name
	Host  string `json:"host"`  // IP address or usb[:bus:addr]
	Model string `json:"model"` // m7300fdn | m7300fdw
}

// Settings holds the last-used scanner options. The GUI saves it to
// ~/.config/pantum-scan-gui.json so every choice is restored on start.
type Settings struct {
	Model      string   `json:"model"`      // auto | m7300fdn | m7300fdw
	Host       string   `json:"host"`       // device IP / hostname; "" = read from SANE config
	Devices    []Device `json:"devices"`    // saved scanner devices
	ActiveHost string   `json:"activeHost"` // currently selected device host
	Source     string   `json:"source"`     // platen | adf | adf-duplex
	Resolution int      `json:"resolution"`
	Mode       string   `json:"mode"` // color | gray | lineart
	RegionFull bool     `json:"regionFull"`
	TlX        int      `json:"tlX"` // scan region in mm (used when !RegionFull)
	TlY        int      `json:"tlY"`
	BrX        int      `json:"brX"`
	BrY        int      `json:"brY"`
	Brightness int      `json:"brightness"`
	Contrast   int      `json:"contrast"`
	Threshold  int      `json:"threshold"`
	Format     string   `json:"format"` // png | jpg | pdf | pdf-page
	Quality    int      `json:"quality"`
	MaxPages   int      `json:"maxPages"`
	OutputDir  string   `json:"outputDir"`
	OutputBase string   `json:"outputBase"`
	Verbose    bool     `json:"verbose"`
	Language   string   `json:"language"` // zh-CN | en-US
}

// DefaultSettings returns sane defaults for a fresh install.
func DefaultSettings() Settings {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return Settings{
		Model:      "auto",
		Source:     "platen",
		Resolution: 300,
		Mode:       "color",
		RegionFull: true,
		TlX:        0, TlY: 0, BrX: 210, BrY: 297,
		Brightness: 0,
		Contrast:   0,
		Threshold:  128,
		Format:     "png",
		Quality:    90,
		MaxPages:   500,
		OutputDir:  home,
		OutputBase: "scan",
		Verbose:    true,
		Language:   "zh-CN",
	}
}

func settingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "pantum-scan-gui.json")
}

// LoadSettings reads the saved settings; returns defaults when missing/corrupt.
func LoadSettings() Settings {
	s := DefaultSettings()
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	s.sanitize()
	return s
}

// SaveSettings persists the settings for the next start.
func SaveSettings(s Settings) {
	s.sanitize()
	dir := filepath.Dir(settingsPath())
	_ = os.MkdirAll(dir, 0o755)
	data, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(settingsPath(), data, 0o644)
}

// sanitize clamps values to the ranges the driver CLI accepts.
func (s *Settings) sanitize() {
	if s.Model != "m7300fdn" && s.Model != "m7300fdw" {
		s.Model = "auto"
	}
	switch s.Source {
	case "adf", "adf-duplex":
	default:
		s.Source = "platen"
	}
	if s.Resolution != 75 && s.Resolution != 150 && s.Resolution != 300 {
		s.Resolution = 300
	}
	switch s.Mode {
	case "gray", "lineart":
	default:
		s.Mode = "color"
	}
	switch s.Format {
	case "jpg", "pdf", "pdf-page":
	default:
		s.Format = "png"
	}
	clamp := func(v, lo, hi int) int {
		if v < lo {
			return lo
		}
		if v > hi {
			return hi
		}
		return v
	}
	s.TlX = clamp(s.TlX, 0, 300)
	s.TlY = clamp(s.TlY, 0, 420)
	s.BrX = clamp(s.BrX, 1, 400)
	s.BrY = clamp(s.BrY, 1, 600)
	s.Brightness = clamp(s.Brightness, -100, 100)
	s.Contrast = clamp(s.Contrast, -100, 100)
	s.Threshold = clamp(s.Threshold, 0, 255)
	s.Quality = clamp(s.Quality, 0, 100)
	s.MaxPages = clamp(s.MaxPages, 1, 5000)
	if s.OutputBase == "" {
		s.OutputBase = "scan"
	}
	if s.Language != "en-US" {
		s.Language = "zh-CN"
	}
}
