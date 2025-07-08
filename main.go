package main

import (
	"flag"
	"goFSL/config"
	"goFSL/db"
	"goFSL/id"
	"log/slog"
	"os"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{AddSource: true})))

	dataDir := flag.String("dataDir", "data/", "path to data directory")
	configFile := flag.String("configFile", "", "path to config file (optional)")
	httpPort := flag.Int("httpPort", 8080, "http port to listen on")

	flag.Parse()

	config.DataDir = ensureTrailingSlash(*dataDir)

	if err := config.ReadConfig(*configFile); err != nil {
		slog.Error("Error reading config file", "error", err)
		os.Exit(1)
	}

	if err := db.InitDB(config.DataDir); err != nil {
		slog.Error("error initializing database", "err", err)
		os.Exit(1)
	}

	if err := id.InitializeIDKey(); err != nil {
		slog.Error("error initializing id_key", "err", err)
		os.Exit(1)
	}

	if err := createDirectory(config.DataDir, "files/"); err != nil {
		slog.Error("error creating directory", "err", err)
		os.Exit(1)
	}

	if err := createDirectory(config.DataDir, "files/temp"); err != nil {
		slog.Error("error creating directory", "err", err)
		os.Exit(1)
	}

	DeleteAllExpiredFiles()
	go ExpiryObserver()

	slog.Info("Initialization completed", "http_port", *httpPort)

	if err := startHTTPServer(*httpPort); err != nil {
		slog.Error("error starting http server", "err", err)
	}
}
