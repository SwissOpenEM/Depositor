package main

import (
	_ "embed"
	"fmt"
	"os"

	"log/slog"

	core "github.com/SwissOpenEM/Depositor/internal/core"
	"github.com/SwissOpenEM/Depositor/internal/webserver"
)

// String can be overwritten by using linker flags: -ldflags "-X main.version=VERSION"
var version string = "DEVELOPMENT_VERSION"

func setupLogging(logLevel string) {
	level := slog.LevelDebug
	switch logLevel {
	case "Info":
		level = slog.LevelInfo
	case "Debug":
		level = slog.LevelDebug
	case "Error":
		level = slog.LevelError
	case "Warning":
		level = slog.LevelWarn
	}

	opts := &slog.HandlerOptions{Level: level}
	h := slog.NewTextHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(h))
}

func main() {
	config, err := core.GetConfig()

	if err != nil {
		slog.Info("Reading config from Env")
		panic(fmt.Errorf("failed to read config from Env: %w", err))
	}

	slog.Info("Starting Depositor service", "Version", version)
	setupLogging(config.LogLevel)

	webserver.SetupAndRun(&config, version)
}
