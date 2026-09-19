package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFrontendConfigValidate(t *testing.T) {
	valid := FrontendConfig{
		API: FrontendAPIConfig{URL: "http://localhost:8080/api", Timeout: time.Second},
		HTTP: FrontendHTTPConfig{
			Addr:              ":3000",
			ReadHeaderTimeout: time.Second,
			ReadTimeout:       time.Second,
			WriteTimeout:      time.Second,
			IdleTimeout:       time.Second,
			ShutdownTimeout:   time.Second,
			MaxHeaderBytes:    1024,
			MaxFormBytes:      1024,
		},
		Session: FrontendSessionConfig{CookieName: "session", RefreshCookieName: "refresh", TTL: time.Minute},
	}
	require.NoError(t, valid.Validate())

	tests := map[string]func(*FrontendConfig){
		"empty address":               func(c *FrontendConfig) { c.HTTP.Addr = "" },
		"relative API URL":            func(c *FrontendConfig) { c.API.URL = "/api" },
		"invalid API URL":             func(c *FrontendConfig) { c.API.URL = "://bad" },
		"invalid API timeout":         func(c *FrontendConfig) { c.API.Timeout = 0 },
		"invalid read header timeout": func(c *FrontendConfig) { c.HTTP.ReadHeaderTimeout = 0 },
		"invalid read timeout":        func(c *FrontendConfig) { c.HTTP.ReadTimeout = 0 },
		"invalid write timeout":       func(c *FrontendConfig) { c.HTTP.WriteTimeout = 0 },
		"invalid idle timeout":        func(c *FrontendConfig) { c.HTTP.IdleTimeout = 0 },
		"invalid shutdown timeout":    func(c *FrontendConfig) { c.HTTP.ShutdownTimeout = 0 },
		"invalid max header bytes":    func(c *FrontendConfig) { c.HTTP.MaxHeaderBytes = 0 },
		"invalid max form bytes":      func(c *FrontendConfig) { c.HTTP.MaxFormBytes = 0 },
		"empty cookie name":           func(c *FrontendConfig) { c.Session.CookieName = "" },
		"empty refresh cookie name":   func(c *FrontendConfig) { c.Session.RefreshCookieName = "" },
		"duplicate cookie names":      func(c *FrontendConfig) { c.Session.RefreshCookieName = c.Session.CookieName },
		"invalid cookie TTL":          func(c *FrontendConfig) { c.Session.TTL = 0 },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := valid
			mutate(&cfg)
			require.Error(t, cfg.Validate())
		})
	}
}

func TestLoadFrontend(t *testing.T) {
	t.Setenv("FRONTEND_API_URL", "http://api:8000/api")
	configPath := filepath.Join(t.TempDir(), "frontend.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
api:
  url: http://localhost:8080/api
  timeout: 10s
http:
  addr: :3000
  read_header_timeout: 5s
  read_timeout: 15s
  write_timeout: 30s
  idle_timeout: 2m
  shutdown_timeout: 10s
  max_header_bytes: 1048576
  max_form_bytes: 1048576
session:
  cookie_name: conduit_session
  refresh_cookie_name: conduit_refresh
  secure: false
  ttl: 720h
logger:
  level: info
  format: text
`), 0o600))

	cfg, err := LoadFrontend(configPath)
	require.NoError(t, err)
	require.Equal(t, "http://api:8000/api", cfg.API.URL)
	require.Equal(t, 10*time.Second, cfg.API.Timeout)
	require.Equal(t, ":3000", cfg.HTTP.Addr)
	require.Equal(t, "info", cfg.Logger.Level)

	_, err = LoadFrontend(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "read frontend config")

	t.Setenv("FRONTEND_API_TIMEOUT", "invalid")
	_, err = LoadFrontend(configPath)
	require.ErrorContains(t, err, "read frontend config")

	t.Setenv("FRONTEND_API_TIMEOUT", "10s")
	t.Setenv("FRONTEND_API_URL", "/api")
	_, err = LoadFrontend(configPath)
	require.ErrorContains(t, err, "validate frontend config")
}
