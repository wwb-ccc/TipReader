package main

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WireMsg is the JSON message exchanged over WebSocket (spec §36).
type WireMsg struct {
	Type     string    `json:"type"` // "state" | "cursor" | "settings" | "error"
	Cursor   int       `json:"cursor,omitempty"`
	State    *State    `json:"state,omitempty"`
	Settings *Settings `json:"settings,omitempty"`
	Message  string    `json:"message,omitempty"`
}

// Hub owns the native WebSocket layer for the 01/02 roles. As a controller it
// runs a small HTTP+WS server exposing a web viewer; as a viewer it dials the
// configured controller and applies its state (blocking local scroll).
type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
	server  *http.Server
	ln      net.Listener
	app     *App
	role    string
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// LAN devices with arbitrary origins must be able to connect (spec §37).
	CheckOrigin: func(*http.Request) bool { return true },
}

// start launches the role-specific network behaviour.
func (h *Hub) start(a *App, cfg Config, parent context.Context) {
	h.mu.Lock()
	h.app = a
	h.role = cfg.Role
	h.mu.Unlock()

	if cfg.Role == "01" {
		h.startController(a, cfg, parent)
	} else {
		go h.startViewerClient(a, cfg, parent)
	}
}

// stop shuts down the controller server (if any) and drops all clients.
func (h *Hub) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		_ = conn.Close()
	}
	h.clients = make(map[*websocket.Conn]bool)
	if h.server != nil {
		_ = h.server.Close()
	}
	if h.ln != nil {
		_ = h.ln.Close()
	}
}

// --- controller role (01) ---

func (h *Hub) startController(a *App, cfg Config, ctx context.Context) {
	addr := "0.0.0.0:" + itoa(cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// spec §40: a bound port must surface a clear error, not fail silently.
		a.setRemoteErr("端口被占用 无法监听 " + addr + ": " + err.Error())
		return
	}
	h.mu.Lock()
	h.ln = ln
	h.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveEmbedded(w, r, "webextra/viewer.html")
	})
	mux.HandleFunc("/tb.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		serveEmbedded(w, r, "frontend/tb.png")
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.register(conn)
		st := a.snapshot()
		_ = conn.WriteJSON(WireMsg{Type: "state", State: &st})
		h.viewerLoop(conn)
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	h.mu.Lock()
	h.server = server
	h.mu.Unlock()

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			a.setRemoteErr("HTTP服务错误: " + err.Error())
		}
	}()
}

func (h *Hub) register(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
}

func (h *Hub) unregister(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	_ = conn.Close()
}

// viewerLoop reads from a viewer so the connection stays healthy and it can
// request fresh state.
func (h *Hub) viewerLoop(conn *websocket.Conn) {
	defer h.unregister(conn)
	for {
		var msg WireMsg
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		if msg.Type == "state" || msg.Type == "settings" {
			_ = conn.WriteJSON(WireMsg{Type: "state", State: allState(h.app)})
		}
	}
}

func allState(a *App) *State {
	st := a.snapshot()
	return &st
}

// broadcastState pushes the full state to every connected viewer.
func (h *Hub) broadcastState(a *App) {
	st := a.snapshot()
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		_ = conn.WriteJSON(WireMsg{Type: "state", State: &st})
	}
}

func (h *Hub) broadcastCursor(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		_ = conn.WriteJSON(WireMsg{Type: "cursor", Cursor: id})
	}
}

func (h *Hub) broadcastSettings(a *App) {
	s := toSettings(a.cfg.Get())
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		_ = conn.WriteJSON(WireMsg{Type: "settings", Settings: &s})
	}
}

// --- viewer role (02) ---

func (h *Hub) startViewerClient(a *App, cfg Config, ctx context.Context) {
	addr := cfg.ControllerIP + ":" + itoa(cfg.ControllerPort)
	url := "ws://" + addr + "/ws"
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			a.setRemoteErr("无法连接控制端 " + url)
			time.Sleep(2 * time.Second)
			continue
		}
		a.setRemoteErr("")
		h.consume(a, conn)
		time.Sleep(time.Second)
	}
}

func (h *Hub) consume(a *App, conn *websocket.Conn) {
	defer conn.Close()
	for {
		var msg WireMsg
		if err := conn.ReadJSON(&msg); err != nil {
			a.setRemoteErr("连接已断开")
			return
		}
		switch msg.Type {
		case "state":
			if msg.State != nil {
				a.applyRemote(*msg.State)
			}
		case "cursor":
			a.applyRemoteCursor(msg.Cursor)
		case "settings":
			if msg.Settings != nil {
				a.applyRemoteSettings(*msg.Settings)
			}
		case "error":
			if msg.Message != "" {
				a.setRemoteErr(msg.Message)
			}
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}