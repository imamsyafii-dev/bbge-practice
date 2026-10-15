package config

import "testing"

func TestLoadUsesDefaults(
	t *testing.T,
) {
	environmentKeys := []string{
		"PRACTICE_HTTP_ADDR",
		"PRACTICE_POSTGRES_HOST",
		"PRACTICE_POSTGRES_PORT",
		"PRACTICE_POSTGRES_USER",
		"PRACTICE_POSTGRES_PASSWORD",
		"PRACTICE_POSTGRES_DB",
		"PRACTICE_POSTGRES_SSLMODE",
	}

	for _, key := range environmentKeys {
		t.Setenv(key, "")
	}

	got := Load()

	want := Config{
		HTTPAddr: ":18080",
		Postgres: PostgresConfig{
			Host:     "127.0.0.1",
			Port:     "25432",
			User:     "practice_user",
			Password: "practice_password",
			Database: "practice_server",
			SSLMode:  "disable",
		},
	}

	if got != want {
		t.Fatalf(
			"Load() = %#v, want %#v",
			got,
			want,
		)
	}
}

func TestLoadUsesEnvironmentOverrides(
	t *testing.T,
) {
	t.Setenv(
		"PRACTICE_HTTP_ADDR",
		":28080",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_HOST",
		"practice-postgres-host",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_PORT",
		"35432",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_USER",
		"practice_override_user",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_PASSWORD",
		"practice_override_password",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_DB",
		"practice_override_database",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_SSLMODE",
		"require",
	)

	got := Load()

	want := Config{
		HTTPAddr: ":28080",
		Postgres: PostgresConfig{
			Host:     "practice-postgres-host",
			Port:     "35432",
			User:     "practice_override_user",
			Password: "practice_override_password",
			Database: "practice_override_database",
			SSLMode:  "require",
		},
	}

	if got != want {
		t.Fatalf(
			"Load() = %#v, want %#v",
			got,
			want,
		)
	}
}
