#!/usr/bin/env bash
# Install pantum-scan-gui and (re)install the m7300fdn/fdw scan CLI binaries.
# Run as root:  sudo ./install.sh
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"
APP_NAME="pantum-scan-gui"
GUI_BIN="$HERE/build/bin/$APP_NAME"
DRIVER_SRC="${DRIVER_SRC:-$HOME/下载/pantum/m7300fdn_driver}"
SHARE_DIR="/usr/share/$APP_NAME"

if [[ $EUID -ne 0 ]]; then
    echo "请以 root 运行: sudo $0" >&2
    exit 1
fi

# 1) GUI binary
if [[ ! -x "$GUI_BIN" ]]; then
    echo "错误: 未找到 $GUI_BIN, 请先执行 wails build" >&2
    exit 1
fi
install -m 0755 "$GUI_BIN" "$BIN_DIR/$APP_NAME"
echo "已安装: $BIN_DIR/$APP_NAME"

# 2) scanner CLI binaries (m7300fdn-scan / m7300fdw-scan).
#    Prefer a PNG-enabled build (build2), fall back to build or the installed one.
CLI_BIN=""
for d in "$DRIVER_SRC/build2/scanner" "$DRIVER_SRC/build/scanner"; do
    if [[ -d "$d" ]]; then
        for m in m7300fdn-scan m7300fdw-scan; do
            if [[ -x "$d/$m" ]]; then
                install -m 0755 "$d/$m" "$BIN_DIR/$m"
                echo "已安装: $BIN_DIR/$m  (来自 $d)"
                CLI_BIN=1
            fi
        done
        break
    fi
done
if [[ -z "$CLI_BIN" ]]; then
    echo "提示: 未找到驱动构建目录, 跳过 m7300fdn/fdw-scan 安装(如已安装可忽略)" >&2
fi

# 3) desktop entry
mkdir -p "$SHARE_DIR"
install -m 0644 "$HERE/build/appicon.png" "$SHARE_DIR/pantum-scan-gui.png" 2>/dev/null || true
cat > "/usr/share/applications/$APP_NAME.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Pantum Scanner
Name[zh_CN]=奔图扫描工具
Comment=Scan with M7300FDN / M7300FDW
Comment[zh_CN]=M7300FDN / M7300FDW 扫描
Exec=$BIN_DIR/$APP_NAME
Icon=$SHARE_DIR/pantum-scan-gui.png
Terminal=false
Categories=Graphics;Scanning;
EOF
echo "已安装: /usr/share/applications/$APP_NAME.desktop"
echo
echo "完成。可从开始菜单启动 \"奔图扫描工具\", 或直接运行 $BIN_DIR/$APP_NAME"
