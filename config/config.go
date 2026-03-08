package config

import (
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	NATS     NATSConfig
	Logger   LoggerConfig
	OAuth    OAuthConfig
	SMTP     SMTPConfig
	AI       AIConfig
	Google   GoogleConfig
	Scraper  ScraperConfig
}

type AppConfig struct {
	Name string `envconfig:"APP_NAME" default:"seo-rank-guardian"`
	Env  string `envconfig:"APP_ENV" default:"development"`
}

type HTTPConfig struct {
	Host string `envconfig:"HTTP_HOST" default:"0.0.0.0"`
	Port string `envconfig:"HTTP_PORT" default:"8080"`
}

type PostgresConfig struct {
	Host     string `envconfig:"POSTGRES_HOST" default:"localhost"`
	Port     string `envconfig:"POSTGRES_PORT" default:"5432"`
	User     string `envconfig:"POSTGRES_USER" default:"srg"`
	Password string `envconfig:"POSTGRES_PASSWORD" default:"srg_secret"`
	DB       string `envconfig:"POSTGRES_DB" default:"srg_db"`
	SSLMode  string `envconfig:"POSTGRES_SSLMODE" default:"disable"`
}

type RedisConfig struct {
	Host     string `envconfig:"REDIS_HOST" default:"localhost"`
	Port     string `envconfig:"REDIS_PORT" default:"6379"`
	Password string `envconfig:"REDIS_PASSWORD" default:""`
	DB       int    `envconfig:"REDIS_DB" default:"0"`
}

type NATSConfig struct {
	URL string `envconfig:"NATS_URL" default:"nats://localhost:4222"`
}

type LoggerConfig struct {
	Level      string `envconfig:"LOG_LEVEL" default:"debug"`
	File       string `envconfig:"LOG_FILE" default:"logs/srg.log"`
	MaxSize    int    `envconfig:"LOG_MAX_SIZE" default:"100"`
	MaxBackups int    `envconfig:"LOG_MAX_BACKUPS" default:"3"`
	MaxAge     int    `envconfig:"LOG_MAX_AGE" default:"30"`
}

type OAuthConfig struct {
	AccessSecret  string `envconfig:"OAUTH_ACCESS_SECRET" default:"access-secret"`
	RefreshSecret string `envconfig:"OAUTH_REFRESH_SECRET" default:"refresh-secret"`
	AccessExpiry  string `envconfig:"OAUTH_ACCESS_EXPIRY" default:"15m"`
	RefreshExpiry string `envconfig:"OAUTH_REFRESH_EXPIRY" default:"168h"`
}

type SMTPConfig struct {
	Host     string `envconfig:"SMTP_HOST" default:"smtp.example.com"`
	Port     int    `envconfig:"SMTP_PORT" default:"587"`
	Username string `envconfig:"SMTP_USERNAME" default:""`
	Password string `envconfig:"SMTP_PASSWORD" default:""`
	From     string `envconfig:"SMTP_FROM" default:"noreply@example.com"`
}

type AIConfig struct {
	Provider        string `envconfig:"AI_PROVIDER" default:"gemini"`
	APIKey          string `envconfig:"AI_API_KEY" default:""`
	Model           string `envconfig:"AI_MODEL" default:"gemini-2.0-flash"`
	SearchGrounding bool   `envconfig:"AI_SEARCH_GROUNDING" default:"true"`
	ReportMode      string `envconfig:"REPORT_MODE" default:"api"`
	WebTimeout      int    `envconfig:"GEMINI_WEB_TIMEOUT_SEC" default:"300"`
}

type GoogleConfig struct {
	ClientID     string `envconfig:"GOOGLE_CLIENT_ID" default:""`
	ClientSecret string `envconfig:"GOOGLE_CLIENT_SECRET" default:""`
	RedirectURL  string `envconfig:"GOOGLE_REDIRECT_URL" default:"http://localhost:8080/api/v1/auth/google/callback"`
}

type ScraperConfig struct {
	ResultLimit int  `envconfig:"SCRAPE_RESULT_LIMIT" default:"10"`
	MinDelayMs  int  `envconfig:"SCRAPE_MIN_DELAY_MS" default:"10000"`
	MaxDelayMs  int  `envconfig:"SCRAPE_MAX_DELAY_MS" default:"20000"`
	MaxRetries  int  `envconfig:"SCRAPE_MAX_RETRIES" default:"3"`
	RodEnabled  bool `envconfig:"SCRAPE_ROD_ENABLED" default:"true"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}

	if err := envconfig.Process("", &cfg.App); err != nil {
		return nil, err
	}
	if err := envconfig.Process("", &cfg.HTTP); err != nil {
		return nil, err
	}
	if err := envconfig.Process("", &cfg.Postgres); err != nil {
		return nil, err
	}
	if err := envconfig.Process("", &cfg.Redis); err != nil {
		return nil, err
	}
	if err := envconfig.Process("", &cfg.NATS); err != nil {
		return nil, err
	}
	if err := envconfig.Process("", &cfg.Logger); err != nil {
		return nil, err
	}
	if err := envconfig.Process("", &cfg.OAuth); err != nil {
		return nil, err
	}
	if err := envconfig.Process("", &cfg.SMTP); err != nil {
		return nil, err
	}
	if err := envconfig.Process("", &cfg.AI); err != nil {
		return nil, err
	}
	if err := envconfig.Process("", &cfg.Google); err != nil {
		return nil, err
	}
	if err := envconfig.Process("", &cfg.Scraper); err != nil {
		return nil, err
	}

	return cfg, nil
}
