# TipReader

Windows 透明悬浮提词器（类音乐悬浮歌词）。完全透明、无边框、可移动、可缩放、可点击阅读、支持 OBS 捕获排除、支持 01/02 局域网同步。

## 技术栈（规格要求）

- 语言：Go `go1.26.4`（实际使用版本）
- GUI 框架：Wails v3（`v3.0.0-beta.22`）
- 前端：原生 HTML + CSS + JavaScript（无 React/Vue/npm）
- Windows 原生能力：`golang.org/x/sys/windows`
- 网络：原生 WebSocket（`github.com/gorilla/websocket`）
- 依赖：`go mod vendor` 已全部下载到本项目 `vendor/`，前端资源用 `go:embed` 打入 exe，最终为单文件便携程序，**不依赖 Node.js / Python / Go 运行库**。

## 目录结构

```text
TipReader.exe     可执行文件（全部资源已内嵌）
read.md           提词稿件（放在 exe 同目录）
conf/config.json  配置（便携，不含绝对路径）
```

## 角色

启动时选择本机角色（配置 `conf/config.json` 的 `role` 也会预选中）：

- `01 控制端`：本窗口即控制界面，可点击整行、上一行/下一行、重读、设置；同时在本机端口提供 WebSocket 服务，手机/浏览器访问 `http://电脑IP:端口/` 即成为被控端。
- `02 被控端`：只显示提词器，自动同步控制端行位置，**禁止用户手动滚动/改行**。

配置 `controllerIP` / `controllerPort` 指向控制端。

## 操作

- 点击任意行：设置当前阅读位置，该行及其前所有行变红
- `↑` / `↓` / 空格：上一行 / 下一行
- `Ctrl+R`：重新读取 read.md
- `Esc`：关闭窗口
- 顶部透明条：拖动窗口（四边 / 四角可缩放）

## 构建（开发机）

```cmd
go mod vendor          # 依赖全部落到本目录（离线可编译）
go build -mod=vendor -trimpath -ldflags "-s -w -H windowsgui" -o release\TipReader\TipReader.exe .
```

最终便携目录只需：`TipReader.exe`、`read.md`、`conf/config.json`，整文件夹复制到任意 Windows（10/11，amd64）即可运行。

Windows 防火墙放行（如需被局域网访问）：

```cmd
netsh advfirewall firewall add rule name="TipReader" dir=in action=allow protocol=TCP localport=7777
```

## OBS 捕获排除

启用 `conf/config.json` 的 `contentProtection: true`。程序通过 Windows 原生 `SetWindowDisplayAffinity(WDA_EXCLUDEFROMCAPTURE)` 使 OBS 的窗口捕获 / 显示器捕获无法录到本窗口，但本机用户仍可正常看到。该 API 主要对窗口与显示器捕获有效，第三方截图可能仍能看到。

## Markdown 说明

`read.md` 中的 Markdown 标记（`#` 标题、`**粗体**`、`*斜体*`、`` `代码` ``、`[链接](url)`、列表符号）都会被清理，标题统一字号，连续空行折叠为单个空行间距。程序只读不改写 `read.md`。