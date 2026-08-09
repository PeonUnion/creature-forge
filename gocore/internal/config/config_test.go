package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	c := Default()
	if c.Server.Port != 8765 || c.Server.Host != "127.0.0.1" {
		t.Errorf("default server: %+v", c.Server)
	}
	if c.Log.Level != "info" {
		t.Errorf("default log level: %s", c.Log.Level)
	}
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `server:
  host: 0.0.0.0
  port: 9000
  dev: false
data:
  root: /tmp/cf-data
log:
  level: debug
  format: json
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Port != 9000 || c.Server.Host != "0.0.0.0" || c.Server.Dev {
		t.Errorf("server: %+v", c.Server)
	}
	if c.Data.Root != "/tmp/cf-data" {
		t.Errorf("data root: %s", c.Data.Root)
	}
	if c.Log.Level != "debug" || c.Log.Format != "json" {
		t.Errorf("log: %+v", c.Log)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("CFG_SERVER_PORT", "9999")
	t.Setenv("CFG_LOG_LEVEL", "warn")
	t.Setenv("CFG_DATA_ROOT", "/env/data")
	c, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Port != 9999 {
		t.Errorf("env port: %d", c.Server.Port)
	}
	if c.Log.Level != "warn" {
		t.Errorf("env log level: %s", c.Log.Level)
	}
	if c.Data.Root != "/env/data" {
		t.Errorf("env data root: %s", c.Data.Root)
	}
}

func TestMissingFileFallsBack(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("missing file should fall back to defaults: %v", err)
	}
	if c.Server.Port != 8765 {
		t.Errorf("default port: %d", c.Server.Port)
	}
}
