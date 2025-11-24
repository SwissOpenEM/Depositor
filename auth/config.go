package auth

import (
	"fmt"
	"os"
)

// AuthConfig holds the configuration for OIDC authentication
type AuthConfig struct {
	// OIDC provider settings
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string

	// Session settings
	SessionDuration uint // in seconds
	SecureCookies   bool

	// Frontend and backend URLs
	FrontendURL string
	BackendURL  string

	// Disable auth (for testing)
	Disable bool
}

// LoadAuthConfigFromEnv loads auth configuration from environment variables
func LoadAuthConfigFromEnv() AuthConfig {
	return AuthConfig{
		IssuerURL:       getEnv("OIDC_ISSUER_URL", ""),
		ClientID:        getEnv("OIDC_CLIENT_ID", ""),
		ClientSecret:    getEnv("OIDC_CLIENT_SECRET", ""),
		RedirectURL:     getEnv("OIDC_REDIRECT_URL", "http://localhost:8080/auth/callback"),
		SessionDuration: getEnvAsUint("SESSION_DURATION", 3600),
		SecureCookies:   getEnvAsBool("SECURE_COOKIES", false),
		FrontendURL:     getEnv("FRONTEND_URL", "http://localhost:4200"),
		BackendURL:      getEnv("BACKEND_URL", "http://localhost:8080"),
		Disable:         getEnvAsBool("DISABLE_AUTH", false),
	}
}

// Validate checks if the auth configuration is valid
func (c *AuthConfig) Validate() error {
	if c.Disable {
		return nil
	}

	if c.IssuerURL == "" {
		return fmt.Errorf("OIDC_ISSUER_URL is required")
	}
	if c.ClientID == "" {
		return fmt.Errorf("OIDC_CLIENT_ID is required")
	}
	if c.RedirectURL == "" {
		return fmt.Errorf("OIDC_REDIRECT_URL is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsUint(key string, defaultValue uint) uint {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	var value uint
	_, err := fmt.Sscanf(valueStr, "%d", &value)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	return valueStr == "true" || valueStr == "1"
}
