# Step 9 — Membuat PostgreSQL Connection Pool

Buat package PostgreSQL dari direktori server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
mkdir -p pkg/postgres
touch pkg/postgres/postgres.go
```

Isi `pkg/postgres/postgres.go` dengan kode berikut:

```go
package postgres

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(
	ctx context.Context,
	cfg config.PostgresConfig,
) (*pgxpool.Pool, error) {
	connectionURL := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			cfg.User,
			cfg.Password,
		),
		Host: net.JoinHostPort(
			cfg.Host,
			cfg.Port,
		),
		Path: cfg.Database,
	}

	query := connectionURL.Query()
	query.Set("sslmode", cfg.SSLMode)
	connectionURL.RawQuery = query.Encode()

	pool, err := pgxpool.New(
		ctx,
		connectionURL.String(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create PostgreSQL pool: %w",
			err,
		)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf(
			"ping PostgreSQL: %w",
			err,
		)
	}

	return pool, nil
}
```

Alur fungsi `Open` adalah `config.PostgresConfig` → connection URL → `pgxpool.Pool` → `Ping`:

1. Nilai konfigurasi diubah menjadi URL PostgreSQL.
2. URL diberikan kepada `pgxpool.New` untuk membuat connection pool.
3. `pool.Ping` memastikan database benar-benar dapat dijangkau.
4. Pool yang siap dipakai dikembalikan kepada pemanggil.

`url.UserPassword` menyusun bagian username dan password serta melakukan escaping karakter khusus dengan benar. `net.JoinHostPort` menggabungkan host dan port tanpa merakit tanda titik dua secara manual, termasuk ketika host berupa alamat IPv6. Nilai `sslmode` ditambahkan sebagai query parameter melalui `url.Values`, sehingga URL dibentuk oleh standard library, bukan melalui penggabungan string.

`pgxpool.New` membuat dan mengonfigurasi pool, tetapi tidak menjamin koneksi hidup sudah berhasil dibuat pada saat fungsi tersebut kembali. `pool.Ping(ctx)` memaksa pemeriksaan koneksi ke PostgreSQL, sehingga password yang salah, port yang tidak tersedia, atau database yang tidak dapat dijangkau diketahui saat `Open` dipanggil.

Jika `Ping` gagal, `pool.Close()` harus dipanggil sebelum error dikembalikan. Hal ini melepaskan resource dan koneksi yang mungkin sudah dibuat oleh pool. Error hanya diberi konteks operasi seperti `create PostgreSQL pool` atau `ping PostgreSQL`; jangan menyertakan `connectionURL.String()` dalam error atau log karena URL tersebut mengandung password database.

Format source code, rapikan dependency, lalu jalankan test:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
gofmt -w pkg/postgres/postgres.go
go mod tidy
go test ./...
```

Pada Step 9, `go mod tidy` sudah aman dijalankan karena `postgres.go` telah mengimpor `pgxpool`. Dependency `github.com/jackc/pgx/v5` sekarang menjadi dependency langsung, sedangkan dependency pendukung pool tetap ditandai `// indirect`.

Langkah ini hanya membuat fungsi pembuka connection pool. Jangan memanggil `postgres.Open` dari `main.go` pada Step 9, sehingga server belum membuka koneksi database sungguhan ketika dijalankan. Wiring konfigurasi, pembukaan pool saat startup, dan penutupan pool saat shutdown dilakukan pada Step 10.
