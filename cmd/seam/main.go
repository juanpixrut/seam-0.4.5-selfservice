// Seam AMS - Asset Management System
//
// Seam AMS is a multi-tenant IoT asset management platform that tracks
// physical assets using QR codes, sensors, and connectivity gateways.
//
// Usage:
//
//	seam [flags]
//
// Flags:
//
//	-config string    Path to configuration file (default "config.json")
//	-port int         HTTP port to listen on (overrides config)
//	-xolu-url string   XOLU server URL (overrides config)
//	-debug            Enable debug logging
//	-version          Print version and exit
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ha1tch/seam-ui/internal/config"
	"github.com/ha1tch/seam-ui/internal/server"
	appversion "github.com/ha1tch/seam-ui/internal/version"
)

// Build information (commit and buildDate set via ldflags; version comes from
// internal/version so it is always in sync with the VERSION file).
var (
	version   = appversion.Version
	commit    = "dev"
	buildDate = "unknown"
)

func main() {
	// Parse command-line flags
	var (
		configPath  = flag.String("config", "config.json", "Path to configuration file")
		port        = flag.Int("port", 0, "HTTP port (overrides config)")
		xoluURL     = flag.String("xolu-url", "", "XOLU server URL (overrides config)")
		debug       = flag.Bool("debug", false, "Enable debug logging")
		showVersion = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("seam-ams %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Apply environment variables
	cfg.LoadFromEnv()

	// Apply command-line overrides
	if *port > 0 {
		cfg.Server.Port = *port
	}
	if *xoluURL != "" {
		cfg.XOLU.URL = *xoluURL
	}
	if *debug {
		cfg.Log.Level = "debug"
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Create and start server
	srv, err := server.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
