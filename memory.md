# TipReader - 项目记忆库
最后更新：2026-10-23 11:00
维护者：Coding Agent + 开发团队

## 一、项目概述
一句话描述：Windows 透明悬浮提词器（类音乐悬浮歌词），支持点击阅读、OBS 捕获排除、01/02 局域网 WebSocket 同步。
目标用户：需要提词的主播/演讲者（Windows 10/11 amd64，免安装便携）。

## 二、技术栈
- 语言：Go 1.26.4
- GUI：Wails v3（v3.0.0-beta.22），WebView2
- 前端：原生 HTML/CSS/JS，go:embed 内嵌（禁 Vue/React/npm）
- 网络：gorilla/websocket（原生 WebSocket，禁 SSE）
- Windows API：golang.org/x/sys/windows（WDA_EXCLUDEFROMCAPTURE 等）
- 图标：TB.png → build/appicon.ico（16~256 多尺寸，PNG 条目）→ windres 编译 rsrc_windows_amd64.syso（msys64 ucrt64）
- 依赖：go mod vendor 全部落到 vendor/，离线可编译

## 三、项目结构
- main.go        入口：配置加载、ensureReadMD、窗口创建（Frameless + 透明）
- app.go         唯一中央服务 App（AppState）；实现 Wails ServiceStartup 钩子启动网络层
- config.go      conf/config.json 便携配置（无绝对路径）
- script.go      read.md 解析（cleanLine 去 Markdown 标记）+ defaultReadMD/ensureReadMD 默认引导稿件
- network.go     Hub：01=HTTP+WS 服务（/ → webextra/viewer.html，/tb.png → frontend/tb.png，/ws）；02=拨号客户端
- frontend/      主窗口（index.html / app.js / style.css / tb.png）
- webextra/      被控端网页 viewer.html
- build/         resource.rc + appicon.ico（图标源文件）
- rsrc_windows_amd64.syso  windres 产物，go build 自动链接进 exe

## 四、启动与开发
- 构建：`go build -mod=vendor -trimpath -ldflags "-s -w -H windowsgui" -o TipReader.exe .`
- 重新生成图标资源（改图标时）：`windres -O coff -c 65001 build\resource.rc -o rsrc_windows_amd64.syso`（在项目根执行；windres 位于 C:\msys64\ucrt64\bin；**必须带 -c 65001**，否则 rc 内中文按本地代码页解析会乱码）
- 发布便携目录仅需：TipReader.exe、read.md、conf/config.json
- 防火墙放行（局域网被控端）：`netsh advfirewall firewall add rule name="TipReader" dir=in action=allow protocol=TCP localport=7777`

## 五、架构设计
- 单一 App 服务持有全部状态；游标同步以 Cursor（行 ID）为准，不是 scrollTop
- 透明实现：窗口/背景透明，文字完全不透明 + 深色描边
- Wails v3 前端调用：`import "/wails/runtime.js"` + `Call.ByName("main.App.方法")`（无 window.runtime 全局对象）
- 窗口拖动：`.drag-area` 用 `--wails-draggable: drag`（自写 JS+Go 拖动在透明无边框窗口上不可靠）
- 设置面板需 max-height + overflow-y:auto，否则可能超高遮挡控件

## 六、编码规范
- 注释与 UI 文案使用中文；Go 标准 gofmt；最小改动/极简优先原则

## 七、重要决策记录（ADR）
- [2026-10-23] exe 图标方案：PNG→ICO(PNG 压缩条目)→windres COFF .syso。理由：日常 `go build` 无需任何额外工具链；.syso 随源码保存，克隆即可编译。
- [2026-10-23] read.md 首次启动自动生成引导文档：ensureReadMD 仅在文件缺失时创建，永不覆盖已有稿件。
- [2026-10-23] 修复两处原有接线缺陷：① App 此前未实现 Wails ServiceStartup 钩子，StartNetworking 无人调用，01/02 同步整体失效；② viewer 路由 embed 路径少了 `webextra/` 前缀，被控端页面 404。

## 八、开发进度与规划
- 已完成：透明提词、点击/键盘游标、重读、设置面板（字号/颜色/行距/端口）、OBS 排除、01/02 局域网同步、exe 图标、read.md 自动生成。
- 2026-10-23 详细实现：
  - 图标：build/appicon.ico + rsrc_windows_amd64.syso（exe/任务栏/资源管理器）；frontend/tb.png（index.html 与 viewer.html favicon）；network.go 新增 /tb.png 路由
  - read.md：script.go 新增 defaultReadMD + ensureReadMD，main.go 启动时调用
  - 验证：图标提取 32x32 成功；curl 实测 / → 200、/tb.png → 200 (image/png)、/ws 非握手 → 400；read.md 自动生成且自定义稿件不被覆盖（51→51 字节）

## 九、常见问题与解决方案（FAQ）
- 本机 127.0.0.1:7777 被华硕 ArmouryCrateControlInterface 占用：回环请求全被它截走（表现为连接重置 / "接收时发生错误"）。用本机局域网 IP 测试（如 http://172.26.192.1:7777/），或改 conf/config.json 的 port。
- PowerShell 5.1 Invoke-WebRequest 报"基础连接已经关闭：接收时发生错误"：先怀疑代理或端口占用，用 `curl.exe --noproxy "*"` 复测。
- go:embed 的路径带目录前缀：embeddedAssets.Open 必须用 "webextra/viewer.html"、"frontend/tb.png"，不能只写文件名。
