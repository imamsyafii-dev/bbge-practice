# Step 11 — Membatasi Waktu Koneksi PostgreSQL Saat Startup

Pada Step 10, server sudah membuka PostgreSQL connection pool sebelum mulai menerima request. Namun, `context.Background()` yang langsung diberikan kepada `postgres.Open` tidak memiliki batas waktu. Pada langkah ini, proses koneksi awal dibatasi selama 5 detik agar startup tidak menunggu tanpa batas ketika PostgreSQL sulit dijangkau.

Buka file server dari direktori project:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
```

Ubah `main.go` menjadi:

```go
package main

import (
	"context"
	"log"
	"net/http"
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

	startupCtx, cancelStartup := context.WithTimeout(
		context.Background(),
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

	if err := server.ListenAndServe(); err != nil {
		log.Printf(
			"practice server stopped: %v",
			err,
		)
	}
}
```

## Membuat startup context dengan timeout

`context.Background()` tetap menjadi parent context paling dasar. Context ini tidak memiliki deadline dan tidak dapat dibatalkan sendiri. `context.WithTimeout` membuat child context bernama `startupCtx` yang memiliki deadline 5 detik:

```go
startupCtx, cancelStartup := context.WithTimeout(
	context.Background(),
	5*time.Second,
)
```

`startupCtx` kemudian diberikan kepada `postgres.Open`. Fungsi `Open` meneruskan context yang sama ke `pgxpool.New` dan `pool.Ping`. Dengan demikian, pembuatan pool dan pemeriksaan koneksi awal berada di bawah batas waktu startup yang sama.

Jika seluruh proses selesai sebelum 5 detik, `postgres.Open` mengembalikan pool yang siap dipakai. Jika deadline tercapai lebih dahulu, operasi menerima cancellation dari context dan `Open` mengembalikan error. Server menerapkan pola fail-fast seperti pada Step 10: error tersebut ditangani oleh `log.Fatal`, sehingga HTTP server tidak dijalankan tanpa koneksi awal PostgreSQL yang berhasil.

Timeout 5 detik adalah batas maksimum untuk operasi yang menghormati context, bukan waktu tunggu minimum. Error seperti `connection refused` dapat dikembalikan jauh lebih cepat.

## Membatalkan startup context segera

`context.WithTimeout` mengembalikan fungsi `cancelStartup`. Fungsi ini dipanggil tepat setelah `postgres.Open` selesai dan sebelum error diperiksa:

```go
database, err := postgres.Open(
	startupCtx,
	cfg.Postgres,
)
cancelStartup()

if err != nil {
	log.Fatal(err)
}
```

Pemanggilan langsung tersebut melepaskan timer dan resource milik startup context sesegera mungkin, baik `Open` berhasil maupun gagal. Membatalkan `startupCtx` setelah `Open` berhasil tidak menutup connection pool. Pool memiliki lifecycle sendiri dan tetap ditutup oleh `defer database.Close()`.

Jangan mengganti pemanggilan langsung tersebut dengan `defer cancelStartup()` di fungsi `main`. Fungsi `main` menjalankan HTTP server dalam waktu lama, sehingga defer baru berjalan ketika server berhenti. Deadline memang akan membatalkan context setelah 5 detik, tetapi pada koneksi yang berhasil lebih cepat, pemanggilan langsung tetap melepaskan resource timer tanpa menunggu deadline. Selain itu, jalur error memakai `log.Fatal`, yang memanggil `os.Exit(1)` dan tidak menjalankan fungsi defer. Memanggil `cancelStartup()` sebelum pemeriksaan error memastikan cleanup tetap dilakukan pada kedua jalur.

## Perubahan tetap terbatas pada main

Step ini tidak memerlukan perubahan pada `config/config.go`. Durasi 5 detik digunakan langsung untuk lifecycle startup sederhana ini dan bukan konfigurasi koneksi PostgreSQL seperti host, port, user, password, database, atau SSL mode.

`pkg/postgres/postgres.go` juga tidak berubah. Fungsi `postgres.Open` sudah menerima `context.Context` dari pemanggil dan meneruskannya ke pembuatan pool serta `Ping`. Tanggung jawab membuat deadline berada di `main`, sedangkan package PostgreSQL hanya menghormati context yang diterimanya.

## Memformat, menguji, dan menjalankan server

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

Dengan konfigurasi default dan PostgreSQL yang siap, log startup tetap memuat dua baris berikut. Timestamp di awal baris akan berbeda sesuai waktu eksekusi:

```text
2026/10/15 15:45:00 connected to PostgreSQL database practice_server
2026/10/15 15:45:00 practice server listening on http://localhost:18080
```

Selama `go run .` masih berjalan, buka terminal lain dan periksa health endpoint:

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

Step ini hanya menambahkan timeout pada koneksi awal PostgreSQL. Penanganan signal proses, pemanggilan `server.Shutdown`, penantian request aktif, dan timeout graceful shutdown tetap menjadi tanggung jawab Step 12.
