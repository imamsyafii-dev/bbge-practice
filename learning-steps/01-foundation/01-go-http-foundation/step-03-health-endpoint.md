# Step 3 — Membuat HTTP server dan health endpoint

Ganti isi `srv-server/main.go` menjadi:

```go
package main

import (
	"log"
	"net/http"
	"time"
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
	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler,
	)

	server := &http.Server{
		Addr:              ":18080",
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

Format dan jalankan:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w main.go
go run .
```

Terminal akan tetap aktif karena server sedang berjalan.

Dari terminal kedua, jalankan:

```bash
curl -i http://localhost:18080/health
```

Hasil penting yang diharapkan:

```text
HTTP/1.1 200 OK
Content-Type: application/json

{"status":"ok"}
```

Kembali ke terminal server dan tekan `Control + C`. Kemudian jalankan:

```bash
go test ./...
```

Hasilnya kurang lebih:

```text
?    github.com/imamsyafii-dev/bbge-practice/srv-server    [no test files]
```
