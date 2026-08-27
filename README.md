# pantum-scan-gui

A graphical network-scanning application for Pantum M7300FDN and M7300FDW multifunction printers.

The application is built with [Wails v2](https://wails.io), Go, Vue 3, and vue-i18n. It automatically detects the scanner model and invokes the matching `m7300fdn-scan` or `m7300fdw-scan` driver CLI. The interface is available in English and Simplified Chinese, and remembers the most recently used settings.

## Features

- **Device management:** Discover USB scanners and WSD network scanners. Add multiple devices, rename or remove them, and remember the active device.
- **Persistent settings:** Store all application options in `~/.config/pantum-scan-gui.json`.
- **Complete scan controls:** Select platen, ADF, or duplex ADF input; 75, 150, or 300 DPI; color, grayscale, or line-art mode; brightness, contrast, threshold, and a custom scan area in millimeters.
- **Multiple output formats:** Save PNG, JPEG, merged PDF, or one PDF per page. Extensionless files produced by older driver builds are automatically renamed with the correct extension.
- **Progress and results:** Display page progress, support cancellation, list completed files, and open a file or its containing folder directly from the application.
- **Internationalization:** Switch between English and Simplified Chinese without restarting the application.

## Requirements

- Linux on ARM64, such as Kylin V10 or Ubuntu 20.04 and later.
- The `m7300fdn-scan` or `m7300fdw-scan` driver CLI installed and available to the application.
- A scanner reachable over WSD or USB (VID `0x232B`).
- The scanner address configured in `/etc/sane.d/m7300fdn.conf` or `/etc/sane.d/m7300fdw.conf`, or added through the application (`usb[:bus:addr]` for USB).

For PNG output, build the scanner driver with `-DENABLE_PNG_SUPPORT=ON`.

## Download

Prebuilt packages are published from the [m7300_bundle](https://github.com/DrAbcOfficial/m7300_bundle) repository.

The target system must provide the WebKitGTK runtime required by Wails. Package names vary by distribution and release, commonly `libwebkit2gtk-4.0` or `libwebkit2gtk-4.1`.

## Build from Source

The current source tree requires Go 1.25 or later, Node.js 22 or later, the Wails v2 CLI, GTK 3 development files, WebKitGTK development files, and `pkg-config`.

On recent Ubuntu releases, install the native build dependencies with:

```sh
sudo apt update
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config
```

Install Wails and build the application:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0

git clone https://github.com/DrAbcOfficial/m7300_scan_gui.git
cd m7300_scan_gui
wails build -platform linux/arm64 -tags webkit2_41
```

The resulting executable is written to `build/bin/pantum-scan-gui`.

The `webkit2_41` build tag tells Wails to link against WebKitGTK 4.1. On distributions that only provide WebKitGTK 4.0 development files, omit this tag and install the corresponding 4.0 development package instead.

## Installation

Run the included installation script from a source checkout or an extracted distribution directory:

```sh
sudo ./install.sh
```

The script installs the GUI executable and desktop entry. When a compatible driver build directory is available, it also installs the `m7300fdn-scan` and `m7300fdw-scan` binaries, preferring builds with PNG support.

## Development

Start the Wails development server with live frontend reloading:

```sh
wails dev
```

The Go backend is located in `backend/`:

- `app.go` exposes application bindings.
- `model_detect.go` handles USB/WSD discovery, configuration fallback, and driver binary lookup.
- `scanner_runner.go` runs scans and emits progress events.
- `settings.go` persists application settings.
- `cli_args.go` builds driver CLI arguments.

The Vue frontend is located in `frontend/src/`, with translations under `frontend/src/i18n/`.

## Tests

```sh
go test ./backend
```

Release packages are published from the [m7300_bundle](https://github.com/DrAbcOfficial/m7300_bundle) repository.
