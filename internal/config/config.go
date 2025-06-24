package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string        `json:"port"`
	Environment    string        `json:"environment"`
	DebugMode      bool          `json:"debug_mode"`
	DatabaseURL    string        `json:"database_url"`
	MaxConnections int           `json:"max_connections"`
	Timeout        time.Duration `json:"timeout"`

	Paytrail struct {
		MerchantID  string `json:"merchant_id"`
		SecretKey   string `json:"secret_key"`
		BaseURL     string `json:"base_url"`
		CallbackURL string `json:"callback_url"`
		SuccessURL  string `json:"success_url"`
		CancelURL   string `json:"cancel_url"`
	} `json:"paytrail"`

	Kong struct {
		InternalAuth string   `json:"internal_auth"`
		AllowedIPs   []string `json:"allowed_ips"`
		AdminAPIURL  string   `json:"admin_api_url"`
		ServiceURL   string   `json:"service_url"`
	} `json:"kong"`

	JWT struct {
		Secret             string        `json:"secret"`
		AccessTokenExpiry  time.Duration `json:"access_token_expiry"`
		RefreshTokenExpiry time.Duration `json:"refresh_token_expiry"`
	} `json:"jwt"`
}

func Load() (*Config, error) {
	cfg := &Config{}

	// Server configuration
	cfg.Port = getEnv("PORT", "8080")
	cfg.Environment = getEnv("ENVIRONMENT", "development")
	cfg.DebugMode = getEnvAsBool("DEBUG_MODE", false)
	cfg.MaxConnections = getEnvAsInt("MAX_CONNECTIONS", 100)
	cfg.Timeout = getEnvAsDuration("TIMEOUT", 30*time.Second)

	// Database
	cfg.DatabaseURL = getEnv("DATABASE_URL", "")
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	// Paytrail
	if err := loadPaytrailConfig(cfg); err != nil {
		return nil, fmt.Errorf("paytrail configuration error: %w", err)
	}

	// Kong
	if err := loadKongConfig(cfg); err != nil {
		return nil, fmt.Errorf("kong configuration error: %w", err)
	}

	// JWT
	if err := loadJWTConfig(cfg); err != nil {
		return nil, fmt.Errorf("jwt configuration error: %w", err)
	}

	// validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

func loadPaytrailConfig(cfg *Config) error {
	cfg.Paytrail.MerchantID = getEnv("PAYTRAIL_MERCHANT_ID", "")
	if cfg.Paytrail.MerchantID == "" {
		return fmt.Errorf("PAYTRAIL_MERCHANT_ID is required")
	}

	cfg.Paytrail.SecretKey = getEnv("PAYTRAIL_SECRET_KEY", "")
	if cfg.Paytrail.SecretKey == "" {
		return fmt.Errorf("PAYTRAIL_SECRET_KEY is required")
	}

	cfg.Paytrail.BaseURL = getEnv("PAYTRAIL_BASE_URL", "https://services.paytrail.com")
	cfg.Paytrail.CallbackURL = getEnv("PAYTRAIL_CALLBACK_URL", "")
	cfg.Paytrail.SuccessURL = getEnv("PAYTRAIL_SUCCESS_URL", "")
	cfg.Paytrail.CancelURL = getEnv("PAYTRAIL_CANCEL_URL", "")

	// validate required URLs
	if cfg.Paytrail.CallbackURL == "" {
		return fmt.Errorf("PAYTRAIL_CALLBACK_URL is required")
	}
	if cfg.Paytrail.SuccessURL == "" {
		return fmt.Errorf("PAYTRAIL_SUCCESS_URL is required")
	}
	if cfg.Paytrail.CancelURL == "" {
		return fmt.Errorf("PAYTRAIL_CANCEL_URL is required")
	}

	return nil
}

func loadKongConfig(cfg *Config) error {
	cfg.Kong.InternalAuth = getEnv("KONG_INTERNAL_AUTH", "")
	if cfg.Kong.InternalAuth == "" {
		return fmt.Errorf("KONG_INTERNAL_AUTH is required")
	}

	cfg.Kong.AllowedIPs = getEnvAsSlice("KONG_ALLOWED_IPS", ",", []string{})
	cfg.Kong.AdminAPIURL = getEnv("KONG_ADMIN_API_URL", "")
	cfg.Kong.ServiceURL = getEnv("KONG_SERVICE_URL", "http://localhost:8080")

	return nil
}

func loadJWTConfig(cfg *Config) error {
	cfg.JWT.Secret = getEnv("JWT_SECRET", "")
	if cfg.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	cfg.JWT.AccessTokenExpiry = getEnvAsDuration("JWT_ACCESS_TOKEN_EXPIRY", 15*time.Minute)
	cfg.JWT.RefreshTokenExpiry = getEnvAsDuration("JWT_REFRESH_TOKEN_EXPIRY", 7*24*time.Hour)

	return nil
}

func (c *Config) validate() error {
	// validate environment
	validEnvs := map[string]bool{
		"development": true,
		"staging":     true,
		"production":  true,
	}
	if !validEnvs[c.Environment] {
		return fmt.Errorf("invalid environment: %s (must be development, staging, or production)", c.Environment)
	}

	// production-specific validations
	if c.Environment == "production" {
		if c.DebugMode {
			return fmt.Errorf("debug mode should not be enabled in production")
		}
		if c.JWT.AccessTokenExpiry > time.Hour {
			return fmt.Errorf("access token expiry too long for production environment")
		}
	}

	// validate timeouts
	if c.Timeout < time.Second {
		return fmt.Errorf("timeout must be at least 1 second")
	}

	// validate max connections
	if c.MaxConnections < 1 {
		return fmt.Errorf("max connections must be at least 1")
	}

	return nil
}

// returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	strValue := getEnv(key, "")
	if strValue == "" {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(strValue)
	if err != nil {
		fmt.Printf("Warning: Could not parse %s as boolean, using default %v. Error: %v\n", key, defaultValue, err)
		return defaultValue
	}
	return boolValue
}

func getEnvAsInt(key string, defaultValue int) int {
	strValue := getEnv(key, "")
	if strValue == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(strValue)
	if err != nil {
		fmt.Printf("Warning: Could not parse %s as integer, using default %d. Error: %v\n", key, defaultValue, err)
		return defaultValue
	}
	return intValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	strValue := getEnv(key, "")
	if strValue == "" {
		return defaultValue
	}
	durationValue, err := time.ParseDuration(strValue)
	if err != nil {
		fmt.Printf("Warning: Could not parse %s as duration, using default %s. Error: %v\n", key, defaultValue.String(), err)
		return defaultValue
	}
	return durationValue
}

func getEnvAsSlice(key, delimiter string, defaultValue []string) []string {
	strValue := getEnv(key, "")
	if strValue == "" {
		return defaultValue
	}
	parts := strings.Split(strValue, delimiter)
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}
