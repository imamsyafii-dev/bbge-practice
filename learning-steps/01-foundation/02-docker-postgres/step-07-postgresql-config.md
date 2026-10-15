# Step 7 — Menambahkan Konfigurasi PostgreSQL

Ubah `srv-server/config/config.go` menjadi:

```go
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
```

`PostgresConfig` mengelompokkan seluruh konfigurasi PostgreSQL. Struct tersebut disimpan sebagai field `Postgres` di dalam `Config`, sehingga pemanggil nantinya dapat mengakses nilai seperti `cfg.Postgres.Host` tanpa mencampurkannya dengan konfigurasi HTTP.

Helper `envOrDefault` membaca environment variable dengan `os.Getenv`. Jika nilainya kosong atau tidak diatur, helper mengembalikan nilai bawaan. Helper yang sama sekarang dipakai untuk konfigurasi HTTP dan PostgreSQL agar aturan fallback tetap konsisten.

Environment variable dan nilai bawaannya adalah:

- `PRACTICE_HTTP_ADDR`: `:18080`
- `PRACTICE_POSTGRES_HOST`: `127.0.0.1`
- `PRACTICE_POSTGRES_PORT`: `25432`
- `PRACTICE_POSTGRES_USER`: `practice_user`
- `PRACTICE_POSTGRES_PASSWORD`: `practice_password`
- `PRACTICE_POSTGRES_DB`: `practice_server`
- `PRACTICE_POSTGRES_SSLMODE`: `disable`

Alamat bawaan `127.0.0.1:25432` dipakai ketika server Go berjalan langsung di Mac. Docker meneruskan port host `25432` ke port PostgreSQL `5432` di dalam container. Jika server Go nantinya dijalankan sebagai container pada network Docker yang sama, host-nya adalah nama service `practice-postgres` dan port-nya `5432`, bukan `127.0.0.1:25432`.

Nilai `sslmode=disable` hanya sesuai untuk PostgreSQL lokal dalam latihan ini. Jangan memakai pengaturan tersebut untuk production; gunakan mode SSL dan sertifikat sesuai konfigurasi database production.

Format dan uji dari direktori server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
gofmt -w config/config.go
go test ./...
```

Pada langkah ini belum ada penambahan driver PostgreSQL, pembukaan koneksi database, atau perubahan pada `main.go`. Integrasi koneksi tersebut dilakukan pada Step 8.
