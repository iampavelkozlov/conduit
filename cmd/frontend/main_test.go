package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	t.Run("graceful shutdown", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := run(ctx, []string{"-config", frontendConfigFile(t, ":0", "http://localhost:8080/api", "info")})
		require.NoError(t, err)
	})

	t.Run("invalid flag", func(t *testing.T) {
		err := run(t.Context(), []string{"-unknown"})
		require.ErrorContains(t, err, "parse flags")
	})

	t.Run("invalid config", func(t *testing.T) {
		err := run(t.Context(), []string{"-config", filepath.Join(t.TempDir(), "missing.yaml")})
		require.ErrorContains(t, err, "read frontend config")
	})

	t.Run("invalid logger", func(t *testing.T) {
		err := run(t.Context(), []string{"-config", frontendConfigFile(t, ":0", "http://localhost:8080/api", "trace")})
		require.ErrorContains(t, err, "initialize frontend logger")
	})

	t.Run("listen failure", func(t *testing.T) {
		err := run(t.Context(), []string{"-config", frontendConfigFile(t, "invalid address", "http://localhost:8080/api", "info")})
		require.ErrorContains(t, err, "serve frontend")
	})
}

func TestMainError(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	os.Args = []string{"conduit-frontend", "-unknown"}
	require.ErrorContains(t, mainError(), "parse flags")
}

func frontendConfigFile(t *testing.T, addr, apiURL, logLevel string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "frontend.yaml")
	contents := fmt.Sprintf(`
api:
  url: %q
  timeout: 1s
http:
  addr: %q
  read_header_timeout: 1s
  read_timeout: 1s
  write_timeout: 1s
  idle_timeout: 1s
  shutdown_timeout: 1s
  max_header_bytes: 1024
  max_form_bytes: 1024
session:
  cookie_name: conduit_session
  secure: false
  ttl: 1m
logger:
  level: %q
  format: text
`, apiURL, addr, logLevel)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}
