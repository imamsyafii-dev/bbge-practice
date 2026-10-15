# Step 10 — Menghubungkan Server ke PostgreSQL

Pada langkah ini, `main.go` membuka PostgreSQL connection pool saat aplikasi mulai dan menutupnya ketika fungsi `main` selesai.

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
	ctx := context.Background()

	database, err := postgres.Open(
		ctx,
		cfg.Postgres,
	)
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

## Membuka koneksi saat startup

`context.Background()` membuat context dasar yang tidak memiliki deadline atau cancellation. Context ini cukup untuk tahap startup sederhana karena aplikasi belum memiliki lifecycle shutdown sendiri. Context tersebut diteruskan ke `postgres.Open`, lalu digunakan oleh `pgxpool.New` dan `pool.Ping` seperti yang dibuat pada Step 9.

Alur startup sekarang adalah:

1. `config.Load()` membaca konfigurasi HTTP dan PostgreSQL.
2. `context.Background()` menyediakan context untuk proses membuka database.
3. `postgres.Open(ctx, cfg.Postgres)` membuat connection pool dan menjalankan `Ping`.
4. Jika PostgreSQL tidak dapat dijangkau atau konfigurasinya salah, `log.Fatal(err)` menghentikan aplikasi sebelum HTTP server dijalankan.
5. Jika koneksi berhasil, aplikasi mencatat nama database lalu menjalankan HTTP server seperti sebelumnya.

Pemeriksaan `Ping` membuat aplikasi memakai pola **fail-fast startup**. Server tidak terlihat siap jika dependency utamanya, yaitu PostgreSQL, belum siap. Contohnya, port yang salah, password yang salah, atau container PostgreSQL yang belum berjalan akan langsung menghasilkan error saat startup.

## Menutup connection pool

Setelah `postgres.Open` berhasil, baris berikut mendaftarkan penutupan pool:

```go
defer database.Close()
```

`defer` dijalankan ketika fungsi `main` kembali secara normal. Dengan demikian, semua koneksi yang dikelola pool dilepaskan ketika HTTP server berhenti dan alur program kembali dari `main`.

Error dari `postgres.Open` masih ditangani dengan `log.Fatal` karena pada titik itu startup gagal dan belum ada pool berhasil yang perlu ditutup oleh `main`. Sebaliknya, error dari `server.ListenAndServe()` dicatat dengan `log.Printf`, bukan `log.Fatal`. `log.Fatal` memanggil `os.Exit(1)`, sehingga fungsi-fungsi `defer` tidak dijalankan. Dengan `log.Printf`, fungsi `main` dapat mencapai akhir dan `defer database.Close()` dapat berjalan ketika `ListenAndServe` kembali.

Penanganan ini belum merupakan graceful shutdown. Menekan `Ctrl+C` mengirim signal proses dan belum ada kode yang menangkap signal tersebut, memanggil `server.Shutdown`, serta menunggu request aktif selesai. Karena itu, pada tahap ini `defer database.Close()` tidak boleh dianggap terjamin berjalan saat proses dihentikan dengan `Ctrl+C`. Penanganan signal, shutdown HTTP server, context cancellation, dan timeout shutdown akan ditambahkan pada langkah berikutnya.

## Memastikan health endpoint tetap tersedia

Pembuatan `http.ServeMux`, pendaftaran `GET /health`, dan konfigurasi `http.Server` tetap dipertahankan. Penambahan PostgreSQL hanya mengubah urutan startup: database harus berhasil terhubung sebelum server mulai menerima request.

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

Dengan konfigurasi default, log startup akan memuat dua baris berikut. Timestamp di awal baris akan berbeda sesuai waktu eksekusi:

```text
2026/10/15 14:30:00 connected to PostgreSQL database practice_server
2026/10/15 14:30:00 practice server listening on http://localhost:18080
```

Selama `go run .` masih berjalan, buka terminal lain dan periksa health endpoint:

```bash
curl -i http://localhost:18080/health
```

Response yang diharapkan memiliki status `200 OK`, content type JSON, dan body berikut:

```text
HTTP/1.1 200 OK
Content-Type: application/json
...

{"status":"ok"}
```

Jika PostgreSQL dihentikan sebelum `go run .`, aplikasi harus gagal pada `postgres.Open` dan tidak mulai mendengarkan port HTTP. Step ini hanya membuktikan koneksi database saat startup serta mempertahankan health endpoint. Graceful shutdown, timeout startup/shutdown, dan pemeriksaan kesehatan database selama aplikasi berjalan merupakan tanggung jawab langkah-langkah berikutnya.
