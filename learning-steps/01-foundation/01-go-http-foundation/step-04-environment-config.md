# Step 4 — Membaca konfigurasi dari environment variable

Buat direktori dan file `srv-server/config/config.go`:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
mkdir -p config
```

```go
package config

import "os"

const defaultHTTPAddr = ":18080"

type Config struct {
	HTTPAddr string
}

func Load() Config {
	httpAddr := os.Getenv("PRACTICE_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}

	return Config{
		HTTPAddr: httpAddr,
	}
}
```

Kemudian ubah `srv-server/main.go` menjadi:

```go
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/config"
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
		log.Fatal(err)
	}
}
```

Format dan uji:

```bash
gofmt -w main.go config/config.go
go test ./...
```

Jalankan tanpa environment variable untuk memakai alamat bawaan `:18080`:

```bash
go run .
```

Dari terminal kedua, periksa health endpoint:

```bash
curl -i http://localhost:18080/health
```

Tekan `Control + C` pada terminal server. Kemudian jalankan dengan alamat lain:

```bash
PRACTICE_HTTP_ADDR=:18082 go run .
```

Dari terminal kedua, periksa alamat baru:

```bash
curl -i http://localhost:18082/health
```

Kedua pemeriksaan akan menghasilkan status `200 OK` dan body:

```json
{"status":"ok"}
```

Gunakan `:18082` untuk latihan override ini. Jangan gunakan `:18081` karena port tersebut dipakai oleh `bbge-pgweb`.
