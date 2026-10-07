package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	Logger   LoggerConfig
	Database DatabaseConfig
	Redis    RedisConfig
}

type AppConfig struct {
	Port string
}

type LoggerConfig struct {
	Level  string
	LogDir string
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

type RedisConfig struct {
	Addr     string
	Username string
	Password string
	DB       int
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		App: AppConfig{
			Port: env("APP_PORT", "8080"),
		},
		Logger: LoggerConfig{
			Level:  env("LOG_LEVEL", "info"),
			LogDir: env("LOG_DIR", ".logs"),
		},
		Database: DatabaseConfig{
			URL:             env("DATABASE_URL", "postgres://open_suite:open_suite@localhost:5432/open_suite?sslmode=disable"),
			MaxOpenConns:    int32Env("DATABASE_MAX_OPEN_CONNS", 10),
			MinConns:        int32Env("DATABASE_MIN_CONNS", 1),
			MaxConnLifetime: durationEnv("DATABASE_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime: durationEnv("DATABASE_MAX_CONN_IDLE_TIME", 30*time.Minute),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Username: env("REDIS_USERNAME", ""),
			Password: env("REDIS_PASSWORD", ""),
			DB:       intEnv("REDIS_DB", 0),
		},
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func int32Env(key string, fallback int32) int32 {
	return int32(intEnv(key, int(fallback)))
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}
