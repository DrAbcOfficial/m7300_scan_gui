package backend

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ScanProgress is emitted while the CLI reports pages.
type ScanProgress struct {
	Page int    `json:"page"`
	Info string `json:"info"`
}

// ScanResult is emitted when the scan finishes.
type ScanResult struct {
	OK      bool     `json:"ok"`
	Pages   int      `json:"pages"`
	Files   []string `json:"files"`
	Error   string   `json:"error"`
	Command string   `json:"command"`
}

var (
	pageRe = regexp.MustCompile(`Page (\d+):`)
	doneRe = regexp.MustCompile(`Complete; (\d+) pages`)
)

type scanState struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

var currentScan scanState

// StartScan launches <model>-scan with the given settings. Progress, log
// lines and the final result are pushed to the frontend as Wails events.
// Returns an error when a scan is already running or the binary is missing.
func StartScan(appCtx context.Context, s Settings) error {
	currentScan.mu.Lock()
	defer currentScan.mu.Unlock()
	if currentScan.cmd != nil {
		return fmt.Errorf("a scan is already running")
	}
	model := s.Model
	if model != "m7300fdn" && model != "m7300fdw" {
		// No explicit model: resolve from the active device, then WSD probe,
		// then fall back to the config files.
		model = ""
		if s.Host != "" {
			if info := DetectModel(s.Host); info.Model != "" {
				model = info.Model
			}
		}
		if model == "" {
			for _, m := range []string{"m7300fdn", "m7300fdw"} {
				if len(readConfHosts(m)) > 0 {
					model = m
					break
				}
			}
		}
		if model == "" {
			model = "m7300fdn"
		}
	}
	bin := FindBinary(model)
	if bin == "" {
		return fmt.Errorf("%s-scan not found", model)
	}
	if s.Format != "pdf" {
		if err := os.MkdirAll(filepath.Dir(scanBasePath(s)), 0o755); err != nil {
			return fmt.Errorf("create output folder: %w", err)
		}
	}

	args := BuildScanArgs(s)
	cmdLine := PreviewCommand(model, s)
	runCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(runCtx, bin, args...)
	currentScan.cancel = cancel
	currentScan.cmd = cmd

	go runScan(appCtx, runCtx, cmd, s, cmdLine)
	return nil
}

// CancelScan terminates the running scan.
func CancelScan() {
	currentScan.mu.Lock()
	defer currentScan.mu.Unlock()
	if currentScan.cancel != nil {
		currentScan.cancel()
	}
}

func runScan(appCtx, runCtx context.Context, cmd *exec.Cmd, s Settings, cmdLine string) {
	runtime.EventsEmit(appCtx, "scan:start", cmdLine)
	emitLog := func(line string) { runtime.EventsEmit(appCtx, "scan:log", line) }
	emitProgress := func(p ScanProgress) { runtime.EventsEmit(appCtx, "scan:progress", p) }

	stdout, errOut := cmd.StdoutPipe()
	stderr, errErr := cmd.StderrPipe()
	if errOut != nil || errErr != nil {
		finishScan(appCtx, ScanResult{OK: false, Error: "failed to capture output", Command: cmdLine})
		return
	}
	if err := cmd.Start(); err != nil {
		finishScan(appCtx, ScanResult{OK: false, Error: err.Error(), Command: cmdLine})
		return
	}

	var lastPage = 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	handle := func(rd io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(rd)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			emitLog(line)
			if m := pageRe.FindStringSubmatch(line); m != nil {
				mu.Lock()
				lastPage = atoi(m[1])
				mu.Unlock()
				emitProgress(ScanProgress{Page: lastPage, Info: line})
			}
		}
	}
	wg.Add(2)
	go handle(stdout)
	go handle(stderr)

	err := cmd.Wait()
	wg.Wait()

	result := ScanResult{Command: cmdLine, Pages: lastPage}
	if err == nil {
		result.OK = true
		result.Files = collectOutputFiles(s)
	} else if runCtx.Err() != nil {
		result.Error = "canceled"
		emitLog("canceled")
	} else {
		result.Error = err.Error()
	}
	finishScan(appCtx, result)
}

func finishScan(appCtx context.Context, result ScanResult) {
	currentScan.mu.Lock()
	currentScan.cancel = nil
	currentScan.cmd = nil
	currentScan.mu.Unlock()
	runtime.EventsEmit(appCtx, "scan:done", result)
}

// collectOutputFiles lists the files produced by the scan:
//   - merged PDF: exactly the output path
//   - images / per-page PDF: <base>_NNNN.<ext> inside the base sub-folder
func collectOutputFiles(s Settings) []string {
	base := scanBasePath(s)
	if s.Format == "pdf" {
		if st, err := os.Stat(base); err == nil && !st.IsDir() {
			return []string{base}
		}
		return nil
	}
	ext := "png"
	if s.Format == "jpg" {
		ext = "jpg"
	} else if s.Format == "pdf-page" {
		ext = "pdf"
	}
	matches, _ := filepath.Glob(base + "_*." + ext)
	sort.Strings(matches)
	return matches
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
