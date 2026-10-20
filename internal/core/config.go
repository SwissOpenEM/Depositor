package core

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port         int
	AllowOrigins []string
	BaseUrl      string
	LogLevel     string
	DataDir      string
}

func GetConfig() (Config, error) {
	port, err := strconv.Atoi(getEnv("PORT", "8080"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid PORT: %w", err)
	}

	return Config{
		Port:         port,
		AllowOrigins: []string{getEnv("ALLOW_ORIGINS", "http://localhost:4200")},
		BaseUrl:      getEnv("ONEDEP_API_BASE_URL", "https://onedep-depui-test.wwpdb.org/deposition/api/v1"),
		LogLevel:     getEnv("LOG_LEVEL", "Debug"),
		DataDir:      getEnv("DATA_DIR", "data"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
