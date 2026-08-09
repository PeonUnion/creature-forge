// Command gocore-server is the standalone CreatureForge API server. It embeds
// the built Vue front-end (internal/server/static via //go:embed) so a single
// binary serves both the API and the SPA. The CLI (cmd/gocore) stays free of
// the front-end.
//
// Usage:
//
//	gocore-server [--config ../config.yaml] [--port 8765] [--host 127.0.0.1] [--data-dir data] [--dev]
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/PeonUnion/creature-forge/gocore/internal/config"
	"github.com/PeonUnion/creature-forge/gocore/internal/logging"
	"github.com/PeonUnion/creature-forge/gocore/internal/server"
)

func main() {
	var port int
	var host, dataDir, configPath string
	var dev bool
	flag.IntVar(&port, "port", 8765, "listen port")
	flag.StringVar(&host, "host", "127.0.0.1", "listen host")
	flag.StringVar(&dataDir, "data-dir", "", "data root (default from config or ./data)")
	flag.BoolVar(&dev, "dev", false, "dev mode (CORS; front-end served from the Vite dev server)")
	flag.StringVar(&configPath, "config", "", "config.yaml path (viper)")
	flag.Parse()

	cfg := config.Default()
	if configPath != "" {
		loaded, err := config.Load(configPath)
		if err != nil {
			fatal(fmt.Errorf("load config: %w", err))
		}
		cfg = loaded
	}
	if port != 8765 {
		cfg.Server.Port = port
	}
	if host != "127.0.0.1" {
		cfg.Server.Host = host
	}
	if dev {
		cfg.Server.Dev = true
	}
	if dataDir != "" {
		cfg.Data.Root = dataDir
	}

	log := logging.New(logging.Config{
		Level:  logging.Level(cfg.Log.Level),
		Format: logging.Format(cfg.Log.Format),
		Output: cfg.Log.Output,
	})
	logging.SetDefault(log)

	srv := server.New(cfg, log)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	mode := "prod"
	if cfg.Server.Dev {
		mode = "dev"
	}
	log.Info("CreatureForge server started", "mode", mode, "addr", addr,
		"data_root", cfg.Data.Root, "frontend", "embedded")
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gocore-server:", err)
	os.Exit(1)
}
