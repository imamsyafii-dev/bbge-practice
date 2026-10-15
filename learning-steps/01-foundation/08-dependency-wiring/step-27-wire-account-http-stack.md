# Step 27 — Wiring account ke server HTTP

Pada Step 25 dan Step 26, HTTP account handler sudah dibuat dan diuji secara terpisah. Pada step ini semua layer account dirangkai di `main.go`, lalu alurnya diuji sampai benar-benar membaca dan menulis data PostgreSQL.

## Tujuan

Setelah menyelesaikan step ini, kita dapat:

- memahami fungsi `main.go` sebagai **composition root**;
- menghubungkan PostgreSQL, SQLC, domain service, application use case, dan HTTP handler;
- mendaftarkan health handler dan account handler pada `http.ServeMux` yang sama;
- menjalankan migrasi sebelum dependency account dipakai;
- menguji alur HTTP sampai PostgreSQL dengan `curl`;
- mematikan server dengan graceful shutdown dan memastikan port sudah bebas.

Kontrak account latihan tetap hanya menggunakan `user_id` dan `name`. Tidak ada field `email` pada request.

## Prasyarat

Step 1 sampai Step 26 harus sudah selesai. Komponen berikut sudah tersedia:

```text
config/config.go
db/migrate.go
internal/store/
internal/domain/account/service.go
internal/app/account/usecase.go
internal/router/account.go
pkg/postgres/postgres.go
docker/docker-compose.yml
```

Docker Desktop juga harus aktif agar container PostgreSQL dapat dijalankan.

## File yang diubah

```text
srv-server/main.go
```

Tidak ada file source lain yang perlu diubah pada step ini.

## Isi lengkap `main.go`

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
	accountapp "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account"
	accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
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

	queries := store.New(database)

	accountService := accountdomain.NewService(
		queries,
	)

	accountUseCase := accountapp.NewUseCase(
		accountService,
	)

	accountHandler := router.NewAccountHandler(
		accountUseCase,
	)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler,
	)
	accountHandler.Register(mux)

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

## Mengapa alias import diperlukan?

Application package dan domain package sama-sama bernama `account`. Jika keduanya di-import tanpa alias, nama `account` akan bertabrakan di dalam satu file.

Karena itu digunakan:

```go
accountapp "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account"
accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
```

Pemakaiannya menjadi jelas:

```go
accountService := accountdomain.NewService(queries)
accountUseCase := accountapp.NewUseCase(accountService)
```

Alias hanya mengubah nama yang dipakai di file ini. Alias tidak mengubah nama package, lokasi package, atau perilaku program.

## `main.go` sebagai composition root

**Composition root** adalah tempat semua object konkret dibuat dan disambungkan. Pada aplikasi ini, tempat tersebut adalah `main()`.

Dependency chain yang dibuat adalah:

```text
PostgreSQL pool
    ↓
SQLC *store.Queries
    ↓
domain *account.Service
    ↓
application *account.UseCase
    ↓
HTTP *router.AccountHandler
    ↓
http.ServeMux
```

Kode konkretnya:

```go
queries := store.New(database)
accountService := accountdomain.NewService(queries)
accountUseCase := accountapp.NewUseCase(accountService)
accountHandler := router.NewAccountHandler(accountUseCase)
accountHandler.Register(mux)
```

`main.go` boleh mengetahui constructor dan type konkret karena tugasnya memang memilih implementasi aplikasi yang akan digunakan saat runtime. Sebaliknya, layer domain, application, dan router menerima **narrow interface** yang hanya berisi method yang mereka perlukan.

Contohnya:

- domain service menerima interface store yang menyediakan `CreateAccount` dan `GetAccountByUserID`;
- application use case menerima interface account service yang menyediakan `Create` dan `GetByUserID`;
- HTTP handler menerima interface account use case yang menyediakan `Create` dan `GetByUserID`.

Keuntungannya, layer tidak terikat langsung pada seluruh implementasi konkret. Test dapat memasukkan fake yang kecil, sedangkan `main.go` memasukkan implementasi nyata.

## Arti setiap sambungan dependency

### 1. PostgreSQL pool ke SQLC Queries

```go
queries := store.New(database)
```

`database` adalah `*pgxpool.Pool` yang sudah terhubung ke PostgreSQL. `store.New` membungkus pool tersebut menjadi `*store.Queries`, sehingga query hasil generate SQLC dapat dijalankan.

### 2. SQLC Queries ke domain service

```go
accountService := accountdomain.NewService(queries)
```

`*store.Queries` memenuhi interface store milik domain account. Domain service menggunakan dependency ini untuk menyimpan atau mengambil account, sambil tetap menangani aturan seperti trimming dan validasi `user_id` serta `name`.

### 3. Domain service ke application use case

```go
accountUseCase := accountapp.NewUseCase(accountService)
```

`*accountdomain.Service` memenuhi interface `AccountService` milik application layer. Use case menjadi pintu masuk operasi aplikasi dan meneruskan pekerjaan domain yang diperlukan.

### 4. Application use case ke HTTP handler

```go
accountHandler := router.NewAccountHandler(accountUseCase)
```

`*accountapp.UseCase` memenuhi interface `AccountUseCase` milik router. Handler bertugas menerjemahkan HTTP request menjadi input use case dan menerjemahkan hasil atau error menjadi HTTP response.

### 5. Handler ke ServeMux yang sama

```go
mux := http.NewServeMux()

mux.HandleFunc(
	"GET /health",
	healthHandler,
)
accountHandler.Register(mux)
```

Health route dan account route didaftarkan pada `mux` yang sama. Kemudian object yang sama dipasang sebagai handler server:

```go
server := &http.Server{
	Addr:              cfg.HTTPAddr,
	Handler:           mux,
	ReadHeaderTimeout: 5 * time.Second,
}
```

Dengan demikian server melayani:

```text
GET  /health
POST /accounts
GET  /accounts/{userID}
```

## Mengapa wiring dilakukan setelah migration berhasil?

Urutan startup adalah:

```text
load config
→ buka dan ping PostgreSQL
→ jalankan migration
→ buat dependency account
→ daftarkan route
→ mulai HTTP server
```

Wiring account ditempatkan setelah:

```go
if err := db.AutoMigrate(database); err != nil {
	// log error lalu return
}
```

Ini memastikan tabel `account` dan object database yang diperlukan sudah tersedia sebelum server menerima request. Jika migration gagal, program berhenti sebelum membuka HTTP listener. Server tidak terlihat siap ketika schema database sebenarnya belum siap.

## Alur satu request

Untuk request:

```text
POST /accounts
```

alurnya adalah:

```text
client
→ http.Server
→ ServeMux mencocokkan "POST /accounts"
→ AccountHandler.create
→ application UseCase.Create
→ domain Service.Create
→ SQLC Queries.CreateAccount
→ pgx pool
→ PostgreSQL
```

Hasil dari PostgreSQL berjalan kembali melalui arah sebaliknya. Router mengubah `store.Account` menjadi JSON response dengan status `201 Created`.

Untuk `GET /accounts/{userID}`, jalurnya sama tetapi method yang dipanggil adalah `GetByUserID` sampai query `GetAccountByUserID`.

## Format dan test

Dari root `srv-server`, jalankan:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w main.go
go test ./...
```

Hasil test yang sudah diamati:

```text
?    github.com/imamsyafii-dev/bbge-practice/srv-server                         [no test files]
?    github.com/imamsyafii-dev/bbge-practice/srv-server/config                  [no test files]
?    github.com/imamsyafii-dev/bbge-practice/srv-server/db                      [no test files]
ok   github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account    (cached)
ok   github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account (cached)
ok   github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router         (cached)
ok   github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store          (cached)
?    github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres            [no test files]
```

Semua package lulus. Package tanpa file test ditandai `[no test files]`, bukan gagal.

## Menjalankan PostgreSQL practice

Pastikan Docker Desktop sudah aktif, lalu jalankan:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server/docker

docker compose up -d
docker compose ps
```

Container yang diharapkan:

```text
practice-postgres   Up (healthy)   0.0.0.0:25432->5432/tcp
```

Nama Compose project, container, database, user, port, volume, dan network menggunakan prefix `practice` agar tidak berbenturan dengan BBGE utama:

```text
Compose project : practice-srv-server
container       : practice-postgres
database        : practice_server
user            : practice_user
host port       : 25432
volume          : practice-postgres-data
network         : practice-network
```

## Menjalankan server pada port 18082

Dari terminal pertama, jalankan:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

PRACTICE_HTTP_ADDR=:18082 go run .
```

Environment variable tersebut hanya mengganti alamat HTTP untuk proses ini. Konfigurasi PostgreSQL tetap memakai nilai default practice.

Log startup yang diharapkan:

```text
connected to PostgreSQL database practice_server
database migrations applied
practice server listening on http://localhost:18082
```

Biarkan terminal pertama tetap menjalankan server. Jalankan perintah pengujian berikut dari terminal kedua.

## Pengujian API secara langsung

### 1. Health check

```bash
curl -i http://localhost:18082/health
```

Respons yang diamati adalah `200 OK` dengan body:

```json
{"status":"ok"}
```

### 2. Memastikan account belum ada

Sebelum membuat data, request berikut pernah dijalankan:

```bash
curl -i \
  http://localhost:18082/accounts/practice-user-027
```

Respons awal yang diamati adalah `404 Not Found` karena `practice-user-027` belum ada:

```json
{"error":{"code":"account_not_found","message":"account not found"}}
```

### 3. Membuat account

```bash
curl -i \
  -X POST http://localhost:18082/accounts \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"practice-user-027","name":"Step 27 Practice User"}'
```

Request berhasil dengan `201 Created`. Tidak ada `email` di dalam payload.

Data yang tersimpan menggunakan:

```text
user_id : practice-user-027
name    : Step 27 Practice User
```

### 4. Mengambil kembali account

```bash
curl -i \
  http://localhost:18082/accounts/practice-user-027
```

Respons yang benar-benar diamati adalah `200 OK` dengan data berikut:

```json
{
  "account": {
    "id": 3,
    "user_id": "practice-user-027",
    "name": "Step 27 Practice User",
    "status": 1,
    "version": 0,
    "created_at": "2026-10-15T13:12:30.098247Z",
    "updated_at": "2026-10-15T13:12:30.098247Z"
  }
}
```

Nilai `id` yang diamati adalah `3`. Pada database baru atau setelah data berubah, nilai `id` dapat berbeda karena PostgreSQL sequence terus berjalan.

Hasil ini membuktikan alur berikut bekerja secara nyata:

```text
HTTP → handler → use case → domain service → SQLC store → PostgreSQL
```

## Batasan account pada tahap ini

### `user_id` harus unik

Migration memberi constraint unik pada kolom `user_id`. Mengirim POST kedua dengan `practice-user-027` akan ditolak oleh PostgreSQL.

Pada tahap ini, error duplicate belum dipetakan ke error domain khusus. Akibatnya handler memasukkannya ke cabang error internal dan client menerima response generik:

```text
500 Internal Server Error
```

```json
{"error":{"code":"internal_error","message":"internal server error"}}
```

Pesan detail PostgreSQL tidak dibocorkan kepada client. Penanganan duplicate secara lebih tepat dapat ditambahkan pada step lanjutan, misalnya dipetakan menjadi `409 Conflict`.

### Panjang `user_id` maksimal 40 karakter

Schema PostgreSQL menggunakan:

```sql
user_id VARCHAR(40) NOT NULL
```

Karena itu `user_id` yang tersimpan mempunyai batas maksimal 40 karakter. Domain saat ini memeriksa bahwa nilainya tidak kosong, tetapi belum memvalidasi batas panjang sebelum query. Pelanggaran batas database saat ini juga akan menjadi error internal generik.

### Data tetap ada setelah container dihentikan

Compose memasang named volume:

```text
practice-postgres-data
```

Menjalankan `docker compose stop`, `docker compose down`, atau me-restart Docker tidak otomatis menghapus named volume. Karena itu account yang sudah dibuat tetap dapat ditemukan ketika container dijalankan lagi.

Jangan menambahkan `-v` pada `docker compose down` jika data latihan ingin dipertahankan. Opsi `-v` menghapus volume Compose beserta data PostgreSQL di dalamnya.

## Graceful shutdown

Kembali ke terminal yang menjalankan server dan tekan:

```text
Ctrl+C
```

Signal interrupt membatalkan `appCtx`. Program berhenti menerima request baru, memberikan waktu paling lama 10 detik untuk request aktif selesai, lalu menutup HTTP server.

Log yang diharapkan:

```text
shutdown signal received
practice server stopped
```

Container PostgreSQL tidak ikut dihentikan. `defer database.Close()` hanya menutup pool milik proses Go, bukan mematikan PostgreSQL.

Jika terminal server tidak ditemukan, cari proses yang benar-benar mendengarkan port `18082`:

```bash
lsof -nP -iTCP:18082 -sTCP:LISTEN
```

Ambil PID binary `srv-server` pada hasil tersebut, lalu kirim signal interrupt ke PID listener itu:

```bash
kill -INT <PID_SRV_SERVER>
```

`go run .` memakai proses wrapper yang menjalankan binary anak. Mengirim signal hanya ke PID wrapper belum tentu menghentikan binary yang memegang port. Karena itu targetkan PID `srv-server` yang ditemukan oleh `lsof`. Signal `INT` tetap melewati alur graceful shutdown. Hindari `kill -9` untuk pengujian ini karena proses tidak mendapat kesempatan menjalankan shutdown dengan rapi.

## Memastikan port sudah tertutup

Setelah server berhenti, jalankan:

```bash
lsof -nP -iTCP:18082 -sTCP:LISTEN
```

Jika port sudah bebas, perintah tidak menampilkan proses listener. Pemeriksaan alternatif:

```bash
curl -i http://localhost:18082/health
```

Request seharusnya gagal terhubung karena server sudah tidak berjalan. Kegagalan koneksi setelah shutdown adalah hasil yang benar untuk pemeriksaan ini.

## Yang dipelajari

Pada step ini kita mempelajari bahwa:

- wiring adalah proses membuat object dan memasukkan dependency yang dibutuhkan;
- `main.go` adalah composition root yang memilih implementasi konkret;
- layer internal tetap bergantung pada narrow interface agar mudah diuji dan tidak terlalu terikat;
- alias import membedakan package application account dan domain account;
- satu pool PostgreSQL dapat diberikan kepada SQLC dan dipakai oleh dependency chain account;
- migration harus berhasil sebelum server dinyatakan siap;
- health handler dan account handler harus terdaftar pada `ServeMux` yang dipasang ke server;
- test unit membuktikan perilaku tiap layer secara terpisah, sedangkan `curl` membuktikan integrasi seluruh stack;
- Docker named volume membuat data PostgreSQL tetap persisten;
- graceful shutdown menutup HTTP server dan pool aplikasi tanpa mematikan container database.

## Preview Step 28

Pada Step 28 kita akan melanjutkan fondasi server setelah wiring pertama berhasil. Fokusnya adalah menambah pengujian atau penyempurnaan pada composition root/server startup agar dependency dan route yang sudah dirangkai tidak hanya diverifikasi secara manual, tetapi juga lebih mudah diuji secara otomatis.
