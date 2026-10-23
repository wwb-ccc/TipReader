package main

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Settings is the frontend-facing display configuration.
type Settings struct {
	FontFamily    string `json:"fontFamily"`
	FontSize      int    `json:"fontSize"`
	TextColor     string `json:"textColor"`
	ReadColor     string `json:"readColor"`
	CenterRatio   float64 `json:"centerRatio"`
	Role          string `json:"role"`
	Port          int    `json:"port"`
	ControllerIP  string `json:"controllerIP"`
	ControllerPort int   `json:"controllerPort"`
	AlwaysOnTop   bool   `json:"alwaysOnTop"`
	ContentProtect bool  `json:"contentProtect"`
}

// State is the full snapshot the frontend renders from.
type State struct {
	Lines    []ScriptLine `json:"lines"`
	Cursor   int          `json:"cursor"` // current line ID (>=0), -1 when unset
	Settings Settings     `json:"settings"`
	Error    string       `json:"error,omitempty"`
}

// App is the single bound Go service (spec §47). It owns the central state
// shared by the window, the network hub and the web viewers.
type App struct {
	mu sync.RWMutex

	app     *application.App
	win     *application.WebviewWindow
	cfg     *configStore
	mdPath  string
	appDir  string

	lines  []ScriptLine
	cursor int

	remoteMu sync.RWMutex
	remote   *State     // state received from the controller (role 02)
	remoteErr string

	hub *Hub // native WebSocket hub (controller role) / client (viewer role)

	netCtx    context.Context
	netCancel context.CancelFunc
	booted    bool
}

// setRemoteErr records a connection/error message surfaced to the viewer UI.
func (a *App) setRemoteErr(msg string) {
	a.remoteMu.Lock()
	a.remoteErr = msg
	a.remoteMu.Unlock()
}

// remoteError returns the current viewer-side status message.
func (a *App) remoteError() string {
	a.remoteMu.RLock()
	defer a.remoteMu.RUnlock()
	return a.remoteErr
}

// applyRemote stores the full state pushed by a controller.
func (a *App) applyRemote(st State) {
	a.remoteMu.Lock()
	cp := st
	a.remote = &cp
	a.remoteMu.Unlock()
}

// applyRemoteCursor updates only the cursor from a remote "cursor" message.
func (a *App) applyRemoteCursor(id int) {
	a.remoteMu.Lock()
	if a.remote == nil {
		a.remote = &State{Settings: toSettings(a.cfg.Get())}
	}
	a.remote.Cursor = id
	a.remoteMu.Unlock()
}

// applyRemoteSettings applies display settings broadcast by a controller.
func (a *App) applyRemoteSettings(s Settings) {
	a.remoteMu.Lock()
	if a.remote == nil {
		a.remote = &State{}
	}
	a.remote.Settings = s
	a.remoteMu.Unlock()
}

// effectiveSnapshot returns the state the window should render: in viewer mode
// it reflects the remote lines/cursor and the remote or local settings.
func (a *App) effectiveSnapshot() State {
	a.remoteMu.RLock()
	remote := a.remote
	errMsg := a.remoteErr
	a.remoteMu.RUnlock()

	if a.cfg.Get().Role == "02" && remote != nil {
		out := *remote
		if out.Settings.FontSize == 0 {
			out.Settings = toSettings(a.cfg.Get())
		}
		out.Settings.Role = "02"
		return out
	}
	_ = errMsg
	st := a.snapshot()
	return st
}

// NewApp builds the central service.
func NewApp(appDir string, cfg *configStore) *App {
	return &App{
		cfg:    cfg,
		appDir: appDir,
		mdPath: filepath.Join(appDir, "read.md"),
		cursor: -1,
		hub:    NewHub(),
	}
}

// bindWindow wires the created window into the service so window APIs can be
// driven from bound methods.
func (a *App) bindWindow(app *application.App, win *application.WebviewWindow) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.app = app
	a.win = win
}

// --- script loading ---

// Load reads read.md once at startup (controller) or on demand.
func (a *App) Load() error {
	lines, err := loadScript(a.mdPath)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.lines = lines
	a.mu.Unlock()
	return nil
}

func (a *App) snapshot() State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	c := a.cfg.Get()
	return State{
		Lines:    a.lines,
		Cursor:   a.cursor,
		Settings: toSettings(c),
	}
}

func toSettings(c Config) Settings {
	return Settings{
		FontFamily:     c.FontFamily,
		FontSize:       c.FontSize,
		TextColor:      c.TextColor,
		ReadColor:      c.ReadColor,
		CenterRatio:    c.ScreenCenterRatioX,
		Role:           c.Role,
		Port:           c.Port,
		ControllerIP:   c.ControllerIP,
		ControllerPort: c.ControllerPort,
		AlwaysOnTop:    c.AlwaysOnTop,
		ContentProtect: c.ContentProtection,
	}
}

// --- bound methods (callable from the frontend) ---

// GetScript returns the initial state snapshot.
func (a *App) GetScript() State {
	st := a.effectiveSnapshot()
	st.Error = a.remoteError()
	return st
}

// PullState returns the current effective state, including any status message.
func (a *App) PullState() State {
	return a.GetScript()
}

// Reload re-reads read.md and returns the new state.
func (a *App) Reload() State {
	_ = a.Load()
	a.hub.broadcastState(a)
	return a.snapshot()
}

// SetCursor sets the current reading line. ID == -1 means "no selection".
func (a *App) SetCursor(id int) State {
	a.mu.Lock()
	last := a.lastLineIDLocked()
	if id < -1 {
		id = -1
	}
	if id > last {
		id = last
	}
	a.cursor = id
	a.mu.Unlock()
	a.hub.broadcastCursor(id)
	return a.snapshot()
}

// CursorNext moves the cursor to the following reading line.
func (a *App) CursorNext() State {
	a.mu.RLock()
	ids := a.lineIDsLocked()
	var target int
	if a.cursor < 0 {
		if len(ids) > 0 {
			target = ids[0]
		} else {
			target = -1
		}
	} else {
		target = -1
		for _, id := range ids {
			if id > a.cursor {
				target = id
				break
			}
		}
		if target == -1 && len(ids) > 0 {
			target = ids[len(ids)-1]
		}
	}
	a.mu.RUnlock()
	return a.SetCursor(target)
}

// CursorPrev moves the cursor to the previous reading line.
func (a *App) CursorPrev() State {
	a.mu.RLock()
	ids := a.lineIDsLocked()
	var target int
	if a.cursor < 0 {
		if len(ids) > 0 {
			target = ids[0]
		} else {
			target = -1
		}
	} else {
		prev := -1
		for _, id := range ids {
			if id >= a.cursor {
				break
			}
			prev = id
		}
		target = prev
	}
	a.mu.RUnlock()
	return a.SetCursor(target)
}

// GetSettings returns the current display settings.
func (a *App) GetSettings() Settings { return toSettings(a.cfg.Get()) }

// SaveSettings stores new settings and applies live window changes where needed.
func (a *App) SaveSettings(s Settings) Settings {
	c := a.cfg.Get()
	c.FontFamily = s.FontFamily
	c.FontSize = s.FontSize
	c.TextColor = s.TextColor
	c.ReadColor = s.ReadColor
	c.ScreenCenterRatioX = s.CenterRatio
	c.Role = s.Role
	c.Port = s.Port
	c.ControllerIP = s.ControllerIP
	c.ControllerPort = s.ControllerPort
	c.AlwaysOnTop = s.AlwaysOnTop
	c.ContentProtection = s.ContentProtect
	_ = a.cfg.Set(c)
	a.ApplyWindowSettings()
	a.hub.broadcastSettings(a)
	return toSettings(a.cfg.Get())
}

// ApplyWindowSettings applies always-on-top / content-protection live.
func (a *App) ApplyWindowSettings() {
	a.mu.RLock()
	win := a.win
	a.mu.RUnlock()
	if win == nil {
		return
	}
	c := a.cfg.Get()
	win.SetAlwaysOnTop(c.AlwaysOnTop)
	// ContentProtection (WDA_EXCLUDEFROMCAPTURE) is applied at creation in
	// main.go; toggling it live is only meaningful when the window exists.
}

// ResizeWindow exposes a window resize to the frontend (spec §64-phase 4).
func (a *App) ResizeWindow(w, h int) {
	a.mu.RLock()
	win := a.win
	a.mu.RUnlock()
	if win == nil {
		return
	}
	if w < 200 {
		w = 200
	}
	if h < 120 {
		h = 120
	}
	win.SetSize(w, h)
}

// CloseApp closes the whole application.
func (a *App) CloseApp() {
	a.mu.RLock()
	app := a.app
	a.mu.RUnlock()
	if app != nil {
		app.Quit()
	}
}

// lineIDsLocked returns the ordered IDs of clickable (non-space) lines.
func (a *App) lineIDsLocked() []int {
	var ids []int
	for _, l := range a.lines {
		if l.ID >= 0 {
			ids = append(ids, l.ID)
		}
	}
	return ids
}

func (a *App) lastLineIDLocked() int {
	max := -1
	for _, l := range a.lines {
		if l.ID > max {
			max = l.ID
		}
	}
	return max
}

// ServiceStartup is the Wails v3 lifecycle hook: boot the network layer with
// the configured role when the application starts.
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.StartNetworking(ctx)
	return nil
}

// StartNetworking begins the correct network role (controller hub or viewer
// client) based on the current config role. It is idempotent.
func (a *App) StartNetworking(parent context.Context) {
	a.mu.Lock()
	if a.booted {
		a.mu.Unlock()
		return
	}
	a.booted = true
	ctx, cancel := context.WithCancel(parent)
	a.netCtx = ctx
	a.netCancel = cancel
	c := a.cfg.Get()
	a.mu.Unlock()
	a.hub.start(a, c, ctx)
}

// SetRole persists the chosen role and (re)starts the network layer.
func (a *App) SetRole(role string) Settings {
	if role != "01" && role != "02" {
		return toSettings(a.cfg.Get())
	}
	c := a.cfg.Get()
	c.Role = role
	_ = a.cfg.Set(c)
	a.restartNetworking()
	return toSettings(a.cfg.Get())
}

// restartNetworking stops any running hub and starts one for the new role.
func (a *App) restartNetworking() {
	a.mu.Lock()
	a.hub.stop()
	if a.netCancel != nil {
		a.netCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.netCtx = ctx
	a.netCancel = cancel
	c := a.cfg.Get()
	a.booted = true
	a.mu.Unlock()
	a.hub.start(a, c, ctx)
}