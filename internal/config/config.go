package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port               string
	DatabaseURL        string
	AuthSecret         string
	GoogleClientID     string
	GoogleClientSecret string
	PublicBaseURL      string
	R2AccountID        string
	R2AccessKeyID      string
	R2SecretAccessKey  string
	R2Bucket           string
	R2PublicBaseURL    string
	ResendAPIKey       string
	ResendFrom         string
	InternalSecret     string
	MediaMTXURL        string
	CookieSecure       bool
	RedisAddr          string
	StagingDir         string
}

func Load() (Config, error) {
	cfg := Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		AuthSecret:         os.Getenv("AUTH_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		PublicBaseURL:      getEnv("PUBLIC_BASE_URL", "http://localhost"),
		R2AccountID:        os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:      os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:  os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:           os.Getenv("R2_BUCKET"),
		R2PublicBaseURL:    os.Getenv("R2_PUBLIC_BASE_URL"),
		ResendAPIKey:       os.Getenv("RESEND_API_KEY"),
		ResendFrom:         getEnv("RESEND_FROM", "noreply@localhost"),
		InternalSecret:     os.Getenv("INTERNAL_SECRET"),
		MediaMTXURL:        getEnv("MEDIAMTX_URL", ""),
		CookieSecure:       getEnv("COOKIE_SECURE", "true") == "true",
		RedisAddr:          redisAddr(),
		StagingDir:         getEnv("STAGING_DIR", "/staging"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.AuthSecret == "" {
		return Config{}, fmt.Errorf("config: AUTH_SECRET is required")
	}

	return cfg, nil
}

func (c Config) GoogleEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
}

func (c Config) ResendEnabled() bool {
	return c.ResendAPIKey != ""
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func redisAddr() string {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = os.Getenv("REDIS_URL")
	}
	return strings.TrimPrefix(addr, "redis://")
}
