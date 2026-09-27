package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config mirrors conf/config.json. It only stores portable values (no absolute
// paths) so the whole folder can be copied to another machine.
type Config struct {
	FontFamily         string `json:"fontFamily"`
	FontSize           int    `json:"fontSize"`
	TextColor          string `json:"textColor"`
	ReadColor          string `json:"readColor"`
	AlwaysOnTop        bool   `json:"alwaysOnTop"`
	WindowWidth        int    `json:"windowWidth"`
	WindowHeight       int    `json:"windowHeight"`
	Role               string `json:"role"`               // "01" controller / "02" controlled
	Port               int    `json:"port"`               // controller listen port (default 7777)
	ControllerIP       string `json:"controllerIP"`       // used when Role == "02"
	ControllerPort     int    `json:"controllerPort"`     // used when Role == "02"
	ContentProtection  bool   `json:"contentProtection"`  // WDA_EXCLUDEFROMCAPTURE
	ScreenCenterRatioX float64 `json:"screenCenterRatio"` // 0..1, keeps current line near this height
}

// DefaultConfig returns the spec defaults (§30, §32, §40).
func DefaultConfig() Config {
	return Config{
		FontFamily:         "Microsoft YaHei",
		FontSize:           16,
		TextColor:          "#FFFFFF",
		ReadColor:          "#FF0000",
		AlwaysOnTop:        true,
		WindowWidth:        800,
		WindowHeight:       300,
		Role:               "01",
		Port:               7777,
		ControllerIP:       "127.0.0.1",
		ControllerPort:     7777,
		ContentProtection:  true,
		ScreenCenterRatioX: 0.45,
	}
}

// configStore guards access to the in-memory config and its persistence.
type configStore struct {
	mu     sync.RWMutex
	path   string
	config Config
}

// newConfigStore loads conf/config.json from the application directory,
// creating it with defaults when missing.
func newConfigStore(appDir string) (*configStore, error) {
	dir := filepath.Join(appDir, "conf")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	s := &configStore{
		path:   filepath.Join(dir, "config.json"),
		config: DefaultConfig(),
	}
	data, err := os.ReadFile(s.path)
	if err == nil {
		_ = json.Unmarshal(data, &s.config)
	} else {
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	}
	s.normalize()
	return s, nil
}

// normalize clamps values into sensible ranges.
func (s *configStore) normalize() {
	if s.config.FontSize < 10 {
		s.config.FontSize = 32
	}
	if s.config.TextColor == "" {
		s.config.TextColor = "#FFFFFF"
	}
	if s.config.ReadColor == "" {
		s.config.ReadColor = "#FF0000"
	}
	if s.config.FontFamily == "" {
		s.config.FontFamily = "Microsoft YaHei"
	}
	if s.config.Role != "02" {
		s.config.Role = "01"
	}
	if s.config.Port <= 0 {
		s.config.Port = 7777
	}
	if s.config.ControllerPort <= 0 {
		s.config.ControllerPort = 7777
	}
	if s.config.WindowWidth <= 0 {
		s.config.WindowWidth = 800
	}
	if s.config.WindowHeight <= 0 {
		s.config.WindowHeight = 300
	}
	if s.config.ScreenCenterRatioX <= 0 || s.config.ScreenCenterRatioX > 1 {
		s.config.ScreenCenterRatioX = 0.45
	}
}

func (s *configStore) saveLocked() error {
	data, err := json.MarshalIndent(s.config, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

// Get returns a copy of the current config.
func (s *configStore) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// Set updates the config in memory, normalises it and persists it.
func (s *configStore) Set(c Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = c
	s.normalize()
	return s.saveLocked()
}