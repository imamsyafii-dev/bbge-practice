# Step 15 — Menjalankan Migration Saat Startup

Pada Step 14, migration runner dan migration `0001_create_trigger_function.sql` sudah dapat dikompilasi, tetapi belum pernah dipanggil oleh aplikasi. Pada langkah ini, `main.go` menjalankan semua migration yang belum diterapkan setelah koneksi PostgreSQL berhasil dibuat dan sebelum HTTP server menerima request.

## Menghubungkan migration runner ke main

Masuk ke direktori server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
```

Ubah `main.go` menjadi:

```go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/config"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/db"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres"
)

func healthHandler(
	w http.ResponseWriter,
	_ *http.Request,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write(
		[]byte(`{"status":"ok"}`),
	)
}

func main() {
	cfg := config.Load()

	appCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	startupCtx, cancelStartup := context.WithTimeout(
		appCtx,
		5*time.Second,
	)

	database, err := postgres.Open(
		startupCtx,
		cfg.Postgres,
	)
	cancelStartup()

	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	log.Printf(
		"connected to PostgreSQL database %s",
		cfg.Postgres.Database,
	)

	if err := db.AutoMigrate(database); err != nil {
		log.Printf(
			"database migration failed: %v",
			err,
		)

		return
	}

	log.Printf("database migrations applied")

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler,
	)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf(
		"practice server listening on http://localhost%s",
		server.Addr,
	)

	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-appCtx.Done():
		log.Printf("shutdown signal received")
		stopSignals()

	case err := <-serverErrors:
		if err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Printf(
				"practice server stopped: %v",
				err,
			)
		}

		return
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	shutdownErr := server.Shutdown(shutdownCtx)
	cancelShutdown()

	if shutdownErr != nil {
		log.Printf(
			"practice server shutdown failed: %v",
			shutdownErr,
		)

		if err := server.Close(); err != nil {
			log.Printf(
				"practice server force close failed: %v",
				err,
			)
		}
	}

	if err := <-serverErrors; err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Printf(
			"practice server stopped: %v",
			err,
		)
	}

	log.Printf("practice server stopped")
}
```

## Menambahkan import package db

Import berikut memberi `main` akses ke migration runner yang dibuat pada Step 14:

```go
"github.com/imamsyafii-dev/bbge-practice/srv-server/db"
```

Package `db` bertanggung jawab atas embedded migration dan cara menjalankan Goose. `main` tidak perlu mengetahui nama file SQL, tabel versi Goose, atau cara menjembatani `pgxpool` ke `database/sql`.

## Menempatkan AutoMigrate pada urutan startup

Pemanggilan migration diletakkan setelah PostgreSQL pool berhasil dibuat, `defer database.Close()` sudah didaftarkan, dan koneksi berhasil dicatat:

```go
defer database.Close()

log.Printf(
	"connected to PostgreSQL database %s",
	cfg.Postgres.Database,
)

if err := db.AutoMigrate(database); err != nil {
	log.Printf(
		"database migration failed: %v",
		err,
	)

	return
}

log.Printf("database migrations applied")
```

Urutan startup sekarang adalah:

1. Memuat konfigurasi.
2. Membuat application context yang menerima `os.Interrupt` dan `SIGTERM`.
3. Membuka serta memeriksa PostgreSQL pool dengan timeout 5 detik.
4. Mendaftarkan penutupan pool.
5. Menjalankan semua embedded migration yang belum diterapkan.
6. Membuat mux dan HTTP server.
7. Mulai menerima request.

Migration harus selesai sebelum mux dan HTTP server dibuat. Dengan demikian, server tidak terlihat siap ketika schema yang dibutuhkan belum tersedia.

## Fail-fast tanpa melewati cleanup

Jika `AutoMigrate` gagal, aplikasi mencatat error dan kembali dari `main` sebelum HTTP server dijalankan. Ini adalah fail-fast startup: aplikasi tidak melayani request menggunakan schema yang gagal disiapkan.

Jalur ini memakai `log.Printf` lalu `return`, bukan `log.Fatal`. Pada titik tersebut PostgreSQL pool sudah berhasil diperoleh dan `defer database.Close()` sudah terdaftar. `log.Fatal` memanggil `os.Exit(1)`, sehingga semua fungsi `defer` dilewati. Dengan `return`, pool ditutup dan `stopSignals()` tetap dijalankan melalui alur keluar normal.

Konsekuensinya, implementasi Step 15 mengakhiri proses dengan status `0` ketika migration gagal karena fungsi `main` kembali secara normal. Cleanup tetap berjalan, tetapi supervisor proses tidak dapat membedakan kegagalan startup tersebut dari penghentian yang sukses. Perbaikan yang mempertahankan cleanup sekaligus menghasilkan exit status non-zero memerlukan refactor entrypoint menjadi fungsi seperti `run() error`: semua resource dan defer dimiliki `run`, lalu `main` mengubah error yang sudah kembali setelah cleanup menjadi status proses non-zero. Refactor tersebut berada di luar scope Step 15.

`log.Fatal` pada error `postgres.Open` tetap berada sebelum pool berhasil dimiliki oleh `main`. Jika `Open` gagal setelah membuat pool sementara, package PostgreSQL sudah menutup pool tersebut sebelum mengembalikan error.

## Batas timeout migration saat ini

Timeout 5 detik masih hanya membatasi pembukaan pool dan `Ping`:

```go
database, err := postgres.Open(
	startupCtx,
	cfg.Postgres,
)
cancelStartup()
```

`startupCtx` sudah dibatalkan sebelum `db.AutoMigrate` dipanggil. Runner pada Step 14 memakai `goose.Up`, yang tidak menerima context dari `main`. Artinya, migration saat ini tidak memiliki timeout sendiri dan tidak langsung dibatalkan oleh `appCtx` ketika signal diterima.

Jika signal tiba ketika Goose sedang bekerja, `appCtx` menjadi cancelled, tetapi migration tetap berjalan sampai `goose.Up` kembali. Setelah itu, alur utama akan melihat cancellation ketika mencapai `select` dan langsung melakukan graceful shutdown. Menambahkan migration timeout atau API Goose berbasis context berada di luar scope Step 15.

## Memformat, menguji, dan menjalankan server

Pastikan container PostgreSQL berstatus `healthy`, lalu format dan test source code:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
gofmt -w main.go
go test ./...
```

Jalankan server:

```bash
go run .
```

Pada database latihan yang masih bersih, output startup akan memuat alur seperti berikut. Timestamp dan durasi migration dapat berbeda:

```text
2026/10/15 17:30:00 connected to PostgreSQL database practice_server
2026/10/15 17:30:00 OK   0001_create_trigger_function.sql (...)
2026/10/15 17:30:00 goose: successfully migrated database to version: 1
2026/10/15 17:30:00 database migrations applied
2026/10/15 17:30:00 practice server listening on http://localhost:18080
```

Selama server berjalan, buka terminal lain dan periksa health endpoint:

```bash
curl -i http://localhost:18080/health
```

Response yang diharapkan tetap memiliki status `200 OK` dan body:

```text
HTTP/1.1 200 OK
Content-Type: application/json
...

{"status":"ok"}
```

## Memverifikasi hasil migration di PostgreSQL

Masuk ke direktori Compose:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server/docker
```

Periksa versi migration terakhir yang sedang diterapkan:

```bash
docker compose exec practice-postgres \
  psql -U practice_user -d practice_server \
  -c "SELECT MAX(version_id) AS current_version FROM goose_db_version WHERE is_applied;"
```

Hasil yang diharapkan:

```text
 current_version
-----------------
               1
```

Periksa function yang dibuat oleh migration `0001`:

```bash
docker compose exec practice-postgres \
  psql -U practice_user -d practice_server \
  -c "SELECT to_regprocedure('public.auto_update_at_column()') AS function_name;"
```

Hasil yang diharapkan:

```text
        function_name
-----------------------------
 auto_update_at_column()
```

Step 15 belum membuat tabel `account`. Buktikan bahwa object tersebut masih `NULL`:

```bash
docker compose exec practice-postgres \
  psql -U practice_user -d practice_server \
  -c "SELECT to_regclass('public.account') AS account_table;"
```

Kolom `account_table` pada hasil yang diharapkan kosong karena PostgreSQL menampilkan nilai `NULL` sebagai sel kosong:

```text
 account_table
---------------

```

## Memverifikasi startup yang idempotent

Hentikan server dengan `Ctrl+C`. Alur graceful shutdown dari Step 12 tetap berlaku:

```text
^C2026/10/15 17:31:00 shutdown signal received
2026/10/15 17:31:00 practice server stopped
```

Jalankan kembali:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
go run .
```

Pada startup kedua, Goose membaca `goose_db_version`, mengetahui version 1 sudah diterapkan, dan tidak menjalankan SQL `0001` lagi. Output akan memuat alur seperti:

```text
2026/10/15 17:32:00 connected to PostgreSQL database practice_server
2026/10/15 17:32:00 goose: no migrations to run. current version: 1
2026/10/15 17:32:00 database migrations applied
2026/10/15 17:32:00 practice server listening on http://localhost:18080
```

Query `current_version` tetap menghasilkan `1`, function tetap tersedia, dan server tetap merespons health endpoint. Inilah sifat idempotent runner: memanggil `goose.Up` berulang kali hanya menerapkan migration yang belum tercatat.

## Menjaga migration yang sudah diterapkan

Setelah migration `0001` diterapkan dan tercatat, jangan mengubah isinya untuk menambahkan schema baru. Goose mencatat version migration dan status penerapannya, bukan checksum isi file. Perubahan pada file yang sudah diterapkan tidak akan terdeteksi: database lama tidak menjalankan ulang version tersebut, sedangkan database baru menjalankan SQL yang sudah diubah. Perbedaan tersebut menyebabkan schema drift antar-database.

Perubahan schema berikutnya harus dibuat sebagai file dengan version baru. Pada Step 16, kita akan menambahkan migration `0002` untuk tabel `account`. Schema latihan tersebut tidak menggunakan kolom email; identitas account akan mengikuti model `user_id` yang digunakan project latihan.
