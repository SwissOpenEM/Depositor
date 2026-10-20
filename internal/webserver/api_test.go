package webserver

import (
	"context"
	"testing"

	"github.com/SwissOpenEM/Depositor/internal/core"
)

func TestNewDepositorWebServer(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	server, err := NewDepositorWebServer("test-version", &core.Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("NewDepositorWebServer() error = %v", err)
	}
	if server.Version != "test-version" {
		t.Errorf("Version = %q, want %q", server.Version, "test-version")
	}
	if server.DataDir != dataDir {
		t.Errorf("DataDir = %q, want %q", server.DataDir, dataDir)
	}
}

func TestOtherControllerGetVersion(t *testing.T) {
	t.Parallel()

	server := &DepositorWebServerImplementation{Version: "test-version"}
	response, err := server.OtherControllerGetVersion(
		context.Background(),
		OtherControllerGetVersionRequestObject{},
	)
	if err != nil {
		t.Fatalf("OtherControllerGetVersion() error = %v", err)
	}

	got, ok := response.(OtherControllerGetVersion200JSONResponse)
	if !ok {
		t.Fatalf("response type = %T, want OtherControllerGetVersion200JSONResponse", response)
	}
	if string(got) != "test-version" {
		t.Errorf("response = %q, want %q", got, "test-version")
	}
}
