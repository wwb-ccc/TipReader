<div align="center">

<img src="TB.png" width="160" alt="TipReader" />

# TipReader

**Windows 透明悬浮提词器**

完全透明 · 无边框 · 可拖动缩放 · 点击阅读 · OBS 捕获排除 · 局域网多端同步

[![Release](https://img.shields.io/badge/release-v1.1.0-blue)](../../releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-green)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Windows%2010%2F11-lightgrey)]()
[![Built with](https://img.shields.io/badge/built%20with-Go%20%2B%20Wails%20v3-00ADD8)]()

[下载最新版](../../releases) · [快速上手](#快速上手) · [从源码构建](#从源码构建)

</div>

---

## 功能特性

- **透明悬浮**：窗口背景 100% 透明，文字完全不透明并带深色描边，任何背景下都清晰可读
- **点击阅读**：点击任意一行设置阅读位置，该行及之前的行自动标红
- **键盘控制**：`↑` `↓` `空格` 切换行，`Ctrl+R` 重读稿件，`Esc` 关闭
- **无边框窗口**：顶部透明条拖动，四边/四角缩放，可置顶
- **OBS 捕获排除**：通过 `SetWindowDisplayAffinity(WDA_EXCLUDEFROMCAPTURE)` 让 OBS 窗口/显示器捕获录不到本窗口，本机用户正常观看
- **局域网同步**：`01 控制端` + `02 被控端`（或手机浏览器）实时同步行位置，基于原生 WebSocket
- **便携单文件**：资源全部内嵌 exe，无需安装、无需运行库，首次启动自动生成 `read.md` 引导稿件与 `conf/config.json` 默认配置
- **Markdown 稿件**：自动清理 `#` 标题、`**粗体**`、`*斜体*`、`` `代码` ``、`[链接]()`、列表符号，标题统一字号，连续空行折叠

## 下载安装

1. 到 [Releases](../../releases) 下载 `TipReader.exe`（v1.1.0）
2. 放到任意文件夹，双击运行
3. 首次启动会自动生成 `read.md`（引导示例）和 `conf/config.json`（默认配置）
4. 用记事本把 `read.md` 内容换成你的台词，按 `Ctrl+R` 重读即可

> 需要 WebView2 Runtime（Windows 10/11 一般已内置）。

## 快速上手

```text
TipReader.exe     主程序（图标/资源已内嵌）
read.md           提词稿件（每行一句；缺失时自动生成引导示例）
conf/config.json  配置（字号/颜色/端口/窗口尺寸等，自动生成）
```

## 操作

| 操作 | 效果 |
| --- | --- |
| 点击任意一行 | 从该行开始阅读，该行及之前标红 |
| `↑` / `↓` / `空格` | 上一行 / 下一行 |
| `Ctrl+R` | 重新读取 read.md（改完稿件无需重启） |
| `Esc` | 关闭窗口 |
| 拖动顶部透明条 | 移动窗口（四边/四角可缩放） |

## 角色（局域网同步）

- **01 控制端**：本窗口即控制界面；同时在本机端口提供 WebSocket 服务
- **02 被控端**：只显示提词器，自动跟随控制端行位置（禁止手动滚动）

手机或同一局域网电脑浏览器访问 `http://控制端IP:端口/` 即成为网页被控端。

Windows 防火墙放行（控制端如需被局域网访问）：

```cmd
netsh advfirewall firewall add rule name="TipReader" dir=in action=allow protocol=TCP localport=7777
```

## 从源码构建

环境要求：Go 1.26+、MinGW windres（仅重新生成图标资源时需要）

```cmd
go mod vendor
go build -mod=vendor -trimpath -ldflags "-s -w -H windowsgui" -o TipReader.exe .
```

更换应用图标：替换根目录 `TB.png` 后，在本地 `build/` 目录维护 `resource.rc`（引用 `appicon.ico`，含 VERSIONINFO 与图标资源 ID 1）并重新生成：

```cmd
windres -O coff -c 65001 build\resource.rc -o rsrc_windows_amd64.syso
```

再执行 `go build` 即可（`-c 65001` 必须带上，否则中文资源会乱码）。`build/` 不入库，但编译好的 `rsrc_windows_amd64.syso` 已入库，克隆后可直接 `go build`。

## 常见问题

- **本机浏览器访问 `127.0.0.1:7777` 失败**：可能被本机其他软件占用（如华硕 Armoury Crate），改用局域网 IP 访问，或在设置中更换端口
- **exe 图标显示旧的**：Windows 图标缓存残留，重启资源管理器或注销重登即可
- **修改稿件后不生效**：按 `Ctrl+R` 重读，无需重启软件

## 许可证

[Apache License 2.0](LICENSE)
