# pantum-scan-gui

奔图 M7300FDN / M7300FDW 网络扫描仪 GUI 工具。

基于 [Wails v2](https://wails.io) (Go + Vue 3 + vue-i18n) 构建, 自动识别设备型号并调用
`m7300fdn-scan` / `m7300fdw-scan` 驱动 CLI, 支持中英文界面与设置记忆。

## 功能

- **设备管理**: 点击"添加设备"打开扫描窗口,通过 WSD 协议(组播 + 广播)扫描局域网,
  自动识别 M7300FDN / M7300FDW 设备;可多选添加,支持重命名、删除;
  设备列表持久化,记住上次选中的设备。
- **记住上次设置**: 所有选项保存在 `~/.config/pantum-scan-gui.json`。
- **完整扫描参数**: 平板 / ADF / ADF 双面, 75/150/300 DPI, 彩色/灰度/黑白,
  亮度/对比度/黑白阈值, 自定义区域 (mm), PNG/JPG/PDF(合并)/PDF(每页) ,
  质量, 最大页数, 详细进度。
- **执行与反馈**: 扫描过程页数进度, 可取消;完成后列出产物, 一键打开文件/文件夹。
- **i18n**: 中文 / English, 界面内即时切换。

## 构建

要求: Go ≥ 1.18, Node ≥ 18, webkit2gtk-4.0-dev (Debian/Ubuntu/Kylin:
`sudo apt install golang-1.22 webkit2gtk-4.0-dev libgtk-3-dev pkg-config`)。

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest   # 需要 ~/go/bin 在 PATH
npm config set registry https://registry.npmmirror.com      # 国内网络可选

cd pantum-scan-gui
wails build
# 产物: build/bin/pantum-scan-gui (单文件, aarch64)
```

## 安装 / 分发

```sh
sudo ./install.sh     # 安装 GUI 到 /usr/local/bin + 桌面入口
                      # 并优先从驱动 build2/scanner 安装带 PNG 的 m7300fdn-scan / m7300fdw-scan
```

目标机要求: Kylin V10 / Ubuntu 20.04+ 等带 `libwebkit2gtk-4.0` 运行时即可运行单二进制。

## 前置条件

- 已安装驱动 CLI: `m7300fdn-scan` / `m7300fdw-scan` (见 m7300fdn_driver 项目,
  建议 `-DENABLE_PNG_SUPPORT=ON` 编译以支持 PNG 输出)。
- 设备 IP 已配置在 `/etc/sane.d/m7300fdn.conf` 或 `m7300fdw.conf`, 或在界面手动填写。
- 扫描仪需支持 WSD (Web Services on Devices) 扫描服务。

## 开发

```sh
wails dev    # 热重载开发
```

Go 后端: `app.go`(绑定入口), `model_detect.go`(WSD 探测/conf 兜底/二进制查找),
`scanner_runner.go`(进程执行与事件), `settings.go`(持久化), `cli_args.go`(CLI 参数构造)。
前端: `frontend/src/App.vue` + `frontend/src/i18n/`。
