package config

import "os"

const (
	defaultHTTPAddr = ":18080"

	defaultPostgresHost     = "127.0.0.1"
	defaultPostgresPort     = "25432"
	defaultPostgresUser     = "practice_user"
	defaultPostgresPassword = "practice_password"
	defaultPostgresDatabase = "practice_server"
	defaultPostgresSSLMode  = "disable"
)

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

type Config struct {
	HTTPAddr string
	Postgres PostgresConfig
}

func Load() Config {
	return Config{
		HTTPAddr: envOrDefault(
			"PRACTICE_HTTP_ADDR",
			defaultHTTPAddr,
		),
		Postgres: PostgresConfig{
			Host: envOrDefault(
				"PRACTICE_POSTGRES_HOST",
				defaultPostgresHost,
			),
			Port: envOrDefault(
				"PRACTICE_POSTGRES_PORT",
				defaultPostgresPort,
			),
			User: envOrDefault(
				"PRACTICE_POSTGRES_USER",
				defaultPostgresUser,
			),
			Password: envOrDefault(
				"PRACTICE_POSTGRES_PASSWORD",
				defaultPostgresPassword,
			),
			Database: envOrDefault(
				"PRACTICE_POSTGRES_DB",
				defaultPostgresDatabase,
			),
			SSLMode: envOrDefault(
				"PRACTICE_POSTGRES_SSLMODE",
				defaultPostgresSSLMode,
			),
		},
	}
}

func envOrDefault(
	key string,
	fallback string,
) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
