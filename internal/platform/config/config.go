package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Logger    LoggerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Auth      AuthConfig
	Google    GoogleConfig
	Upload    UploadConfig
	Schedule  ScheduleConfig
	RateLimit RateLimitConfig
}

type AppConfig struct {
	Port    string
	Env     string
	BaseURL string
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

type AuthConfig struct {
	AllowedOrigins  []string
	CookieSecure    bool
	SessionTTL      time.Duration
	DevLoginEnabled bool
	TrustProxy      bool
	TokenEncKey     string
}

// SessionCookieName returns the `__Host-` prefixed name when the cookie is
// Secure (the prefix is rejected by browsers otherwise).
func (a AuthConfig) SessionCookieName() string {
	if a.CookieSecure {
		return "__Host-session"
	}
	return "session"
}

type GoogleConfig struct {
	ClientID            string
	ClientSecret        string
	RedirectURL         string
	CalendarRedirectURL string
	CalendarAPIBase     string
}

type UploadConfig struct {
	Dir      string
	MaxBytes int64
}

type ScheduleConfig struct {
	GranularityMin int
}

type RateLimitConfig struct {
	AuthPerMin     int
	InvitePerMin   int
	UploadPerMin   int
	SchedulePerMin int
}

func Load() Config {
	_ = godotenv.Load()

	apiBase := "http://localhost:" + env("APP_PORT", "8080")
	// Without a frontend the API itself is the "app": logins land on it.
	baseURL := strings.TrimRight(env("APP_BASE_URL", apiBase), "/")

	return Config{
		App: AppConfig{
			Port:    env("APP_PORT", "8080"),
			Env:     env("APP_ENV", "development"),
			BaseURL: baseURL,
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
			// Empty disables Redis; it is only checked by /health/ready.
			Addr:     env("REDIS_ADDR", ""),
			Username: env("REDIS_USERNAME", ""),
			Password: env("REDIS_PASSWORD", ""),
			DB:       intEnv("REDIS_DB", 0),
		},
		Auth: AuthConfig{
			AllowedOrigins:  allowedOrigins(baseURL),
			CookieSecure:    boolEnv("COOKIE_SECURE", strings.HasPrefix(baseURL, "https://")),
			SessionTTL:      durationEnv("SESSION_TTL", 30*24*time.Hour),
			DevLoginEnabled: boolEnv("DEV_LOGIN_ENABLED", false),
			TrustProxy:      boolEnv("TRUST_PROXY", false),
			TokenEncKey:     env("TOKEN_ENC_KEY", ""),
		},
		Google: GoogleConfig{
			ClientID:            env("GOOGLE_CLIENT_ID", ""),
			ClientSecret:        env("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:         env("GOOGLE_REDIRECT_URL", apiBase+"/api/v1/auth/google/callback"),
			CalendarRedirectURL: env("GOOGLE_CALENDAR_REDIRECT_URL", apiBase+"/api/v1/calendar/callback"),
			CalendarAPIBase:     strings.TrimRight(env("GOOGLE_CALENDAR_API_BASE", "https://www.googleapis.com/calendar/v3"), "/"),
		},
		Upload: UploadConfig{
			Dir:      env("UPLOAD_DIR", "uploads"),
			MaxBytes: int64(intEnv("MAX_UPLOAD_BYTES", 5*1024*1024)),
		},
		Schedule: ScheduleConfig{
			GranularityMin: intEnv("SCHEDULE_GRANULARITY_MIN", 5),
		},
		RateLimit: RateLimitConfig{
			AuthPerMin:     intEnv("RATE_LIMIT_AUTH_PER_MIN", 20),
			InvitePerMin:   intEnv("RATE_LIMIT_INVITE_PER_MIN", 60),
			UploadPerMin:   intEnv("RATE_LIMIT_UPLOAD_PER_MIN", 30),
			SchedulePerMin: intEnv("RATE_LIMIT_SCHEDULE_PER_MIN", 240),
		},
	}
}

// allowedOrigins merges ALLOWED_ORIGINS (comma separated) with the origin of
// APP_BASE_URL so the frontend is always allowed.
func allowedOrigins(baseURL string) []string {
	seen := map[string]struct{}{}
	var origins []string

	add := func(value string) {
		value = strings.ToLower(strings.TrimRight(strings.TrimSpace(value), "/"))
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		origins = append(origins, value)
	}

	if parsed, err := url.Parse(baseURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		add(parsed.Scheme + "://" + parsed.Host)
	}
	for _, origin := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		add(origin)
	}

	return origins
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

func boolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
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
