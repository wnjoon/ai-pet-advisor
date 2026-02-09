package config

import (
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port    string
	GinMode string

	// Database
	DatabaseURL string

	// Google ADK / Gemini
	GoogleAPIKey string
	GeminiModel  string

	// Session
	SessionTimeoutMin int

	// Tier
	DefaultMaxItems int

	// KakaoTalk
	KakaoSkillAPIKey string

	// Webview
	WebviewBaseURL string
}

// Load reads configuration from environment variables and returns a Config.
// It uses default values when environment variables are not set.
func Load() *Config {
	return &Config{
		Port:              getEnv("PORT", "8080"),
		GinMode:           getEnv("GIN_MODE", "debug"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/ai_pet_advisor?sslmode=disable"),
		GoogleAPIKey:      getEnv("GOOGLE_API_KEY", ""),
		GeminiModel:       getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
		SessionTimeoutMin: getEnvInt("SESSION_TIMEOUT_MIN", 30),
		DefaultMaxItems:   getEnvInt("DEFAULT_MAX_ITEMS", 30),
		KakaoSkillAPIKey:  getEnv("KAKAO_SKILL_API_KEY", ""),
		WebviewBaseURL:    getEnv("WEBVIEW_BASE_URL", "http://localhost:3000"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
