package backend

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BuildScanArgs converts the GUI settings into m7300fdn-scan/m7300fdw-scan
// arguments. The semantics match the driver CLI (scanner/src/cli/tool_main.cpp).
func BuildScanArgs(s Settings) []string {
	args := []string{}

	if s.Host != "" {
		args = append(args, "-H", s.Host)
	}
	args = append(args, "-s", s.Source)

	res := s.Resolution
	if res != 75 && res != 150 && res != 300 {
		res = 300
	}
	args = append(args, "-r", fmt.Sprintf("%d", res))

	mode := s.Mode
	if mode != "gray" && mode != "lineart" {
		mode = "color"
	}
	args = append(args, "-m", mode)

	if !s.RegionFull {
		args = append(args,
			"--tl-x", fmt.Sprintf("%d", s.TlX),
			"--tl-y", fmt.Sprintf("%d", s.TlY),
			"--br-x", fmt.Sprintf("%d", s.BrX),
			"--br-y", fmt.Sprintf("%d", s.BrY))
	}
	if s.Brightness != 0 {
		args = append(args, "-b", fmt.Sprintf("%d", s.Brightness))
	}
	if s.Contrast != 0 {
		args = append(args, "-c", fmt.Sprintf("%d", s.Contrast))
	}
	if mode == "lineart" {
		args = append(args, "-t", fmt.Sprintf("%d", s.Threshold))
	}

	base := s.OutputBase
	if base == "" {
		base = "scan"
	}
	args = append(args, "-o", filepath.Join(s.OutputDir, base))

	switch s.Format {
	case "jpg":
		args = append(args, "-f", "jpg")
	case "pdf":
		args = append(args, "--pdf")
	case "pdf-page":
		args = append(args, "--pdf-per-page")
	default:
		args = append(args, "-f", "png")
	}

	q := s.Quality
	if q < 0 || q > 100 {
		q = 90
	}
	args = append(args, "-q", fmt.Sprintf("%d", q))

	n := s.MaxPages
	if n <= 0 {
		n = 500
	}
	args = append(args, "-n", fmt.Sprintf("%d", n))

	if s.Verbose {
		args = append(args, "-v")
	}
	return args
}

// PreviewCommand renders the command line shown in the GUI.
func PreviewCommand(model string, s Settings) string {
	if model == "" {
		model = s.Model
	}
	if model == "" || model == "auto" {
		model = "m7300fdn"
	}
	return model + "-scan " + strings.Join(BuildScanArgs(s), " ")
}
