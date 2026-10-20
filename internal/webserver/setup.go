package webserver

import (
	"log"
	"log/slog"

	"github.com/SwissOpenEM/Depositor/internal/core"
	"github.com/oapi-codegen/runtime"
)

// Should run once at the start of the program to set up the runtime for oapi-codegen
func initAPIRuntime() {
	// Opt-in to OpenAPI 3.1 type arrays
	runtime.NarrowUnionNumericFormats = true
}

func SetupAndRun(config *core.Config, version string) *DepositorWebServerImplementation {
	initAPIRuntime()
	depositor, err := NewDepositorWebServer(version, config)
	if err != nil {
		log.Fatal(err)
	}

	s := NewDepositorServer(config, depositor)

	slog.Info("Starting Depositor HTTP server", "port", config.Port, "version", version)
	log.Fatal(s.ListenAndServe())

	return depositor
}
