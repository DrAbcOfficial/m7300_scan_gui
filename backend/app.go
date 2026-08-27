package backend

import (
	"context"
	"os/exec"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
) // App is the Wails application entry; its exported methods are bound to the
// frontend and callable from JavaScript.
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Startup is the Wails startup hook entry point.
func (a *App) Startup(ctx context.Context) {
	a.startup(ctx)
}

// ScanDevices discovers supported USB and WSD scanners and returns them
// with a default name (device-reported model name).
func (a *App) ScanDevices() []Device {
	infos := DiscoverDevices()
	devices := make([]Device, 0, len(infos))
	for _, info := range infos {
		devices = append(devices, Device{
			Name:  info.ModelName,
			Host:  info.Host,
			Model: info.Model,
		})
	}
	return devices
}

// AddDevices appends devices (deduplicated by host) and persists them.
// Returns the full device list.
func (a *App) AddDevices(newDevices []Device) []Device {
	s := LoadSettings()
	existing := map[string]bool{}
	for _, d := range s.Devices {
		existing[d.Host] = true
	}
	for _, d := range newDevices {
		if d.Host == "" || existing[d.Host] {
			continue
		}
		if d.Name == "" {
			d.Name = d.Model
		}
		existing[d.Host] = true
		s.Devices = append(s.Devices, d)
	}
	SaveSettings(s)
	return s.Devices
}

// RenameDevice renames a saved device by host. Returns the full device list.
func (a *App) RenameDevice(host, name string) []Device {
	s := LoadSettings()
	name = strings.TrimSpace(name)
	for i := range s.Devices {
		if s.Devices[i].Host == host {
			if name != "" {
				s.Devices[i].Name = name
			}
			break
		}
	}
	SaveSettings(s)
	return s.Devices
}

// RemoveDevice deletes a saved device by host. Returns the full device list.
func (a *App) RemoveDevice(host string) []Device {
	s := LoadSettings()
	out := s.Devices[:0]
	for _, d := range s.Devices {
		if d.Host != host {
			out = append(out, d)
		}
	}
	s.Devices = out
	if s.ActiveHost == host {
		s.ActiveHost = ""
	}
	SaveSettings(s)
	return s.Devices
}

// SetActiveDevice remembers the currently selected device.
func (a *App) SetActiveDevice(host string) {
	s := LoadSettings()
	s.ActiveHost = host
	SaveSettings(s)
}

// FindBinary returns the path of the <model>-scan executable.
func (a *App) FindBinary(model string) string {
	return FindBinary(model)
}

// LoadSettings restores the last-used settings.
func (a *App) LoadSettings() Settings {
	return LoadSettings()
}

// SaveSettings persists the current settings.
func (a *App) SaveSettings(s Settings) {
	SaveSettings(s)
}

// StartScan launches the scan; progress is pushed via scan:* events.
func (a *App) StartScan(s Settings) error {
	return StartScan(a.ctx, s)
}

// CancelScan terminates the running scan.
func (a *App) CancelScan() {
	CancelScan()
}

// OpenFolder opens a directory in the system file manager.
func (a *App) OpenFolder(path string) string {
	return openWith(a.ctx, path)
}

// OpenFile opens a file with the system default application.
func (a *App) OpenFile(path string) string {
	return openWith(a.ctx, path)
}

// BrowseDirectory shows a native folder picker and returns the selection.
func (a *App) BrowseDirectory(defaultDir string) string {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		DefaultDirectory:     defaultDir,
		CanCreateDirectories: true,
	})
	if err != nil || dir == "" {
		return ""
	}
	return dir
}

func openWith(ctx context.Context, path string) string {
	if path == "" {
		return "empty path"
	}
	cmd := exec.CommandContext(ctx, "xdg-open", path)
	if err := cmd.Run(); err != nil {
		runtime.LogWarning(ctx, "xdg-open "+path+": "+err.Error())
		return err.Error()
	}
	return ""
}
