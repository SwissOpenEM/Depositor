package core

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGetConfigDefaults(t *testing.T) {
	unsetConfigEnv(t)

	got, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	want := Config{
		Port:         8080,
		AllowOrigins: []string{"http://localhost:4200"},
		BaseUrl:      "https://onedep-depui-test.wwpdb.org/deposition/api/v1",
		LogLevel:     "Debug",
		DataDir:      "data",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetConfig() = %#v, want %#v", got, want)
	}
}

func TestGetConfigEnvironmentOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("ALLOW_ORIGINS", "https://example.org")
	t.Setenv("ONEDEP_API_BASE_URL", "https://api.example.org")
	t.Setenv("LOG_LEVEL", "Info")
	t.Setenv("DATA_DIR", "/var/lib/depositor")

	got, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	want := Config{
		Port:         9090,
		AllowOrigins: []string{"https://example.org"},
		BaseUrl:      "https://api.example.org",
		LogLevel:     "Info",
		DataDir:      "/var/lib/depositor",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetConfig() = %#v, want %#v", got, want)
	}
}

func TestGetConfigInvalidPort(t *testing.T) {
	t.Setenv("PORT", "not-a-port")

	got, err := GetConfig()
	if err == nil {
		t.Fatal("GetConfig() error = nil, want invalid PORT error")
	}
	if !strings.Contains(err.Error(), "invalid PORT") {
		t.Errorf("GetConfig() error = %q, want it to mention invalid PORT", err)
	}
	if !reflect.DeepEqual(got, Config{}) {
		t.Errorf("GetConfig() config = %#v, want zero Config on error", got)
	}
}

func unsetConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PORT",
		"ALLOW_ORIGINS",
		"ONEDEP_API_BASE_URL",
		"LOG_LEVEL",
		"DATA_DIR",
	} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
}
