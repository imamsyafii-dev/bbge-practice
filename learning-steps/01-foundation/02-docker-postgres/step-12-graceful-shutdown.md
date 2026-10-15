# Step 12 — Menambahkan Graceful Shutdown

Pada Step 11, server sudah membatasi koneksi awal PostgreSQL selama 5 detik. Namun, proses masih belum mengatur penghentian HTTP server ketika menerima signal dari sistem operasi. Pada langkah ini, server menangani `Ctrl+C` dan `SIGTERM`, memberi waktu bagi request aktif untuk selesai, lalu menutup PostgreSQL connection pool melalui alur keluar normal dari fungsi `main`.

Buka file server dari direktori project:

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

## Menangkap signal proses

`signal.NotifyContext` membuat `appCtx` yang dibatalkan ketika proses menerima salah satu signal berikut:

- `os.Interrupt`, yang biasanya dikirim ketika `Ctrl+C` ditekan di terminal.
- `syscall.SIGTERM`, yang umum dipakai oleh Docker, Kubernetes, dan process manager untuk meminta aplikasi berhenti.

Parent dari `appCtx` adalah `context.Background()`, sehingga lifecycle aplikasi dimulai tanpa deadline. Signal yang terdaftar menjadi sumber cancellation untuk seluruh alur aplikasi:

```go
appCtx, stopSignals := signal.NotifyContext(
	context.Background(),
	os.Interrupt,
	syscall.SIGTERM,
)
defer stopSignals()
```

`defer stopSignals()` memastikan pendaftaran signal dibersihkan ketika `main` kembali. Pada jalur signal, `stopSignals()` juga dipanggil langsung setelah signal pertama diterima. Pemanggilan langsung ini mengembalikan perilaku default signal, sehingga interrupt kedua dapat menghentikan proses jika graceful shutdown macet. Memanggil fungsi tersebut lagi melalui `defer` aman.

## Menurunkan startup context dari app context

Timeout koneksi PostgreSQL dari Step 11 tetap 5 detik, tetapi parent-nya sekarang adalah `appCtx`:

```go
startupCtx, cancelStartup := context.WithTimeout(
	appCtx,
	5*time.Second,
)
```

Dengan susunan ini, startup berhenti bukan hanya ketika deadline 5 detik tercapai, tetapi juga ketika aplikasi menerima `os.Interrupt` atau `SIGTERM`. `startupCtx` tetap diteruskan ke `postgres.Open`, lalu dipakai oleh `pgxpool.New` dan `pool.Ping`. `cancelStartup()` tetap dipanggil langsung setelah `Open` selesai agar resource timer dilepaskan sebelum jalur sukses atau error dilanjutkan.

Jika `postgres.Open` gagal, belum ada pool yang berhasil dikembalikan kepada `main`, sehingga `log.Fatal` masih dipakai untuk menghentikan startup. Setelah pool berhasil dibuat, kode tidak memakai `log.Fatal` lagi karena `log.Fatal` memanggil `os.Exit` dan akan melewati fungsi-fungsi `defer`.

## Menjalankan HTTP server secara concurrent

`ListenAndServe` memblokir sampai HTTP server berhenti. Jika dipanggil langsung pada goroutine utama, `main` tidak dapat menunggu signal pada waktu yang sama. Karena itu, HTTP server dijalankan pada goroutine terpisah:

```go
serverErrors := make(chan error, 1)

go func() {
	serverErrors <- server.ListenAndServe()
}()
```

Channel `serverErrors` memiliki buffer satu elemen. Jika `ListenAndServe` selesai sebelum `main` mulai menerima hasilnya, goroutine tetap dapat mengirim satu error tanpa tertahan. Goroutine tidak memanggil `log.Fatal`; semua keputusan untuk keluar tetap dilakukan pada goroutine utama agar fungsi `main` dapat kembali secara normal.

## Memilih signal atau kegagalan server

`select` menunggu dua kemungkinan:

1. `appCtx.Done()` ditutup karena `os.Interrupt` atau `SIGTERM`. Program melanjutkan proses graceful shutdown.
2. `ListenAndServe` selesai lebih dahulu. Error yang tidak diharapkan dicatat, lalu `main` kembali karena HTTP server sudah tidak berjalan.

```go
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
```

Contoh kegagalan awal HTTP server adalah port `18080` yang sudah dipakai proses lain. Jalur ini tidak mencoba memanggil `Shutdown` karena listener sudah gagal berjalan. `return` mengakhiri `main` secara normal, sehingga `defer database.Close()` tetap dijalankan.

## Memberi waktu 10 detik untuk shutdown

Setelah signal diterima, kode membuat context baru dengan timeout 10 detik:

```go
shutdownCtx, cancelShutdown := context.WithTimeout(
	context.Background(),
	10*time.Second,
)

shutdownErr := server.Shutdown(shutdownCtx)
cancelShutdown()
```

Shutdown context memakai `context.Background()`, bukan `appCtx`. Pada titik ini `appCtx` sudah dibatalkan oleh signal; jika dipakai sebagai parent, shutdown context juga langsung batal dan tidak memberi waktu kepada request aktif untuk selesai.

`server.Shutdown` berhenti menerima koneksi baru, menutup koneksi idle, lalu menunggu request aktif selesai sampai batas waktu 10 detik. `cancelShutdown()` dipanggil segera setelah `Shutdown` kembali untuk melepaskan resource timer tanpa menunggu fungsi `main` selesai.

Jika timeout tercapai atau `Shutdown` gagal karena alasan lain, kode mencatat error lalu memanggil `server.Close()`. Berbeda dari `Shutdown`, `Close` memaksa koneksi aktif ditutup. Fallback ini memastikan aplikasi tidak terus menunggu server setelah batas graceful shutdown habis.

## Menangani hasil ListenAndServe

Ketika `Shutdown` menutup listener, `ListenAndServe` mengembalikan `http.ErrServerClosed`. Error ini adalah hasil yang diharapkan, bukan kegagalan aplikasi. Pemeriksaan berikut mengabaikannya dengan `errors.Is`:

```go
if err := <-serverErrors; err != nil &&
	!errors.Is(err, http.ErrServerClosed) {
	log.Printf(
		"practice server stopped: %v",
		err,
	)
}
```

Menerima hasil dari `serverErrors` setelah `Shutdown` juga memastikan goroutine HTTP server sudah selesai sebelum `main` kembali. Setelah log `practice server stopped`, fungsi `main` mencapai akhir. `defer database.Close()` kemudian menutup PostgreSQL connection pool dan `defer stopSignals()` membersihkan pendaftaran signal.

## Memformat dan menguji source code

Pastikan container PostgreSQL dari Step 6 masih berstatus `healthy`, lalu format dan test source code:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
gofmt -w main.go
go test ./...
```

Jalankan server:

```bash
go run .
```

Dengan konfigurasi default dan PostgreSQL yang siap, log startup memuat dua baris berikut. Timestamp di awal baris akan berbeda sesuai waktu eksekusi:

```text
2026/10/15 16:30:00 connected to PostgreSQL database practice_server
2026/10/15 16:30:00 practice server listening on http://localhost:18080
```

Selama server berjalan, buka terminal lain dan periksa health endpoint:

```bash
curl -i http://localhost:18080/health
```

Response yang diharapkan tetap memiliki status `200 OK`, content type JSON, dan body berikut:

```text
HTTP/1.1 200 OK
Content-Type: application/json
...

{"status":"ok"}
```

Kembali ke terminal yang menjalankan server, lalu tekan `Ctrl+C`. Log shutdown yang diharapkan adalah:

```text
^C2026/10/15 16:31:00 shutdown signal received
2026/10/15 16:31:00 practice server stopped
```

Server harus berhenti tanpa mencatat `http: Server closed` sebagai error.

## Verifikasi opsional

Untuk memeriksa `SIGTERM` secara langsung, build dan jalankan binary di background:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
go build -o /tmp/practice-srv-server .
/tmp/practice-srv-server &
server_pid=$!
```

Tunggu sampai log `practice server listening` muncul, lalu kirim `SIGTERM` dan tunggu proses selesai:

```bash
kill -TERM "$server_pid"
wait "$server_pid"
```

Log shutdown harus kembali memuat `shutdown signal received` dan `practice server stopped`.

Untuk memeriksa jalur kegagalan awal HTTP server, jalankan satu instance server, lalu jalankan `go run .` lagi dari terminal berbeda. Instance kedua harus gagal memakai port yang sama dan mencatat error seperti:

```text
practice server stopped: listen tcp :18080: bind: address already in use
```

Instance kedua kemudian kembali dari `main` dan menutup pool database miliknya. Hentikan instance pertama dengan `Ctrl+C` setelah verifikasi selesai.

## Batas scope Step 12

Step ini hanya mengatur lifecycle satu HTTP server dan PostgreSQL connection pool ketika proses berhenti. Health endpoint masih hanya menyatakan proses HTTP hidup; endpoint tersebut belum memeriksa PostgreSQL pada setiap request. Konfigurasi timeout melalui environment variable, readiness check database, observability tambahan, dan koordinasi beberapa background worker atau beberapa server tetap menjadi topik langkah berikutnya.
