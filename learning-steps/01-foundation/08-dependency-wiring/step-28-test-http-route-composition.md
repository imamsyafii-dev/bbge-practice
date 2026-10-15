# Step 28 — Menguji komposisi route HTTP

Pada Step 27, seluruh dependency account sudah dirangkai di `main.go` dan diuji secara manual sampai PostgreSQL. Pada step ini, pembuatan route dipisahkan ke fungsi kecil bernama `newHTTPHandler`, lalu komposisi route diuji otomatis tanpa membuka port dan tanpa memakai database.

## Tujuan

Setelah menyelesaikan step ini, kita dapat:

- memahami **composition root** dan **test seam**;
- memisahkan pembuatan HTTP handler dari proses startup aplikasi;
- menguji bahwa route health dan account benar-benar terdaftar;
- mengganti use case asli dengan fake melalui interface `router.AccountUseCase`;
- menggunakan `httptest` tanpa menjalankan server sungguhan;
- membedakan tanggung jawab test composition dari test detail HTTP handler pada Step 26.

## Prasyarat

Step 1 sampai Step 27 harus sudah selesai. Secara khusus, komponen berikut sudah tersedia:

```text
internal/app/account/usecase.go
internal/domain/account/service.go
internal/router/account.go
internal/router/account_test.go
internal/store/
main.go
```

Docker dan PostgreSQL tidak diperlukan pada step ini. Test tidak menjalankan fungsi `main()`.

## File yang diubah dan dibuat

```text
srv-server/main.go       # diubah
srv-server/main_test.go  # dibuat
```

Kontrak account latihan tetap memakai `user_id` dan `name`. Tidak ada field `email`.

## Masalah sebelum refactor

Pada Step 27, `main()` melakukan dua jenis pekerjaan sekaligus:

1. menjalankan proses startup, seperti membaca config, membuka PostgreSQL, menjalankan migration, dan membuat server;
2. membuat `http.ServeMux` serta mendaftarkan route.

Jika test memanggil `main()`, test juga akan mencoba membuka database dan port. Itu membuat test route menjadi lambat dan bergantung pada infrastruktur eksternal.

Solusinya adalah membuat **test seam**, yaitu batas kecil pada kode yang memungkinkan dependency asli diganti dan bagian tertentu diuji secara terpisah. Pada step ini, test seam tersebut adalah:

```go
func newHTTPHandler(
	accountUseCase router.AccountUseCase,
) http.Handler
```

Fungsi `main()` tetap menjadi composition root utama yang membuat dependency konkret. Namun, detail komposisi HTTP dipindahkan ke fungsi yang dapat dipanggil langsung oleh test.

## Isi lengkap akhir `main.go`

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

func newHTTPHandler(
	accountUseCase router.AccountUseCase,
) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler,
	)

	accountHandler := router.NewAccountHandler(
		accountUseCase,
	)
	accountHandler.Register(mux)

	return mux
}

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

	httpHandler := newHTTPHandler(
		accountUseCase,
	)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpHandler,
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

## Mengapa `newHTTPHandler` menerima `router.AccountUseCase`?

Parameter fungsi bukan `*accountapp.UseCase`, melainkan interface sempit:

```go
accountUseCase router.AccountUseCase
```

HTTP composition hanya membutuhkan dua kemampuan yang didefinisikan oleh interface tersebut:

```text
Create
GetByUserID
```

Saat aplikasi berjalan, argument-nya adalah `*accountapp.UseCase`. Saat test berjalan, argument-nya adalah `*fakeRootAccountUseCase`. Keduanya dapat digunakan karena memenuhi method set yang sama.

Manfaatnya:

- `newHTTPHandler` tidak bergantung pada implementasi use case tertentu;
- test tidak perlu membuat domain service, SQLC store, atau PostgreSQL;
- fake cukup mengimplementasikan fungsi yang diwajibkan interface;
- dependency yang digunakan handler terlihat jelas pada parameter fungsi.

Ini bukan berarti semua fungsi use case harus dibuat ulang. Fake hanya memenuhi kontrak interface yang memang dibutuhkan oleh HTTP handler.

## Mengapa return type-nya `http.Handler`?

Fungsi tersebut mengembalikan:

```go
http.Handler
```

bukan:

```go
*http.ServeMux
```

`http.Server` dan test hanya membutuhkan satu perilaku:

```go
ServeHTTP(http.ResponseWriter, *http.Request)
```

Perilaku itu adalah kontrak `http.Handler`. Pemanggil tidak perlu mengetahui bahwa implementasi saat ini adalah `*http.ServeMux`. Dengan demikian, implementasi router dapat diganti nanti tanpa mengubah kontrak fungsi.

## Alur runtime setelah refactor

Refactor ini tidak mengubah perilaku server. Alur produksi tetap:

```text
main()
  → config.Load()
  → postgres.Open(...)
  → db.AutoMigrate(...)
  → store.New(database)
  → accountdomain.NewService(queries)
  → accountapp.NewUseCase(accountService)
  → newHTTPHandler(accountUseCase)
      → http.NewServeMux()
      → register GET /health
      → router.NewAccountHandler(accountUseCase)
      → register POST /accounts
      → register GET /accounts/{userID}
  → http.Server{Handler: httpHandler}
  → ListenAndServe()
```

Perbedaannya hanya pembuatan `ServeMux` dan pendaftaran route kini dikelompokkan dalam `newHTTPHandler`.

## Isi lengkap akhir `main_test.go`

```go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accountapp "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
)

type fakeRootAccountUseCase struct {
	createCalls      int
	getByUserIDCalls int
}

var _ router.AccountUseCase = (*fakeRootAccountUseCase)(nil)

func (f *fakeRootAccountUseCase) Create(
	_ context.Context,
	input accountapp.CreateInput,
) (store.Account, error) {
	f.createCalls++

	return store.Account{
		ID:     28,
		UserID: input.UserID,
		Name:   input.Name,
		Status: 1,
	}, nil
}

func (f *fakeRootAccountUseCase) GetByUserID(
	_ context.Context,
	userID string,
) (store.Account, error) {
	f.getByUserIDCalls++

	return store.Account{
		ID:     28,
		UserID: userID,
		Name:   "Step 28 Practice User",
		Status: 1,
	}, nil
}

func TestNewHTTPHandlerRegistersHealthRoute(
	t *testing.T,
) {
	fake := &fakeRootAccountUseCase{}
	handler := newHTTPHandler(fake)

	request := httptest.NewRequest(
		http.MethodGet,
		"/health",
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if contentType := recorder.Header().Get(
		"Content-Type",
	); contentType != "application/json" {
		t.Fatalf(
			"Content-Type = %q, want application/json",
			contentType,
		)
	}

	if body := recorder.Body.String(); body != `{"status":"ok"}` {
		t.Fatalf(
			"body = %q, want %q",
			body,
			`{"status":"ok"}`,
		)
	}

	if fake.createCalls != 0 ||
		fake.getByUserIDCalls != 0 {
		t.Fatal(
			"account use case should not be called",
		)
	}
}

func TestNewHTTPHandlerRegistersAccountRoutes(
	t *testing.T,
) {
	tests := []struct {
		name             string
		method           string
		target           string
		body             string
		contentType      string
		wantStatus       int
		wantCreateCalls  int
		wantGetByIDCalls int
	}{
		{
			name:            "create account",
			method:          http.MethodPost,
			target:          "/accounts",
			body:            `{"user_id":"practice-user-028","name":"Step 28 Practice User"}`,
			contentType:     "application/json",
			wantStatus:      http.StatusCreated,
			wantCreateCalls: 1,
		},
		{
			name:             "get account",
			method:           http.MethodGet,
			target:           "/accounts/practice-user-028",
			wantStatus:       http.StatusOK,
			wantGetByIDCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeRootAccountUseCase{}
			handler := newHTTPHandler(fake)

			request := httptest.NewRequest(
				test.method,
				test.target,
				strings.NewReader(test.body),
			)

			if test.contentType != "" {
				request.Header.Set(
					"Content-Type",
					test.contentType,
				)
			}

			recorder := httptest.NewRecorder()

			handler.ServeHTTP(
				recorder,
				request,
			)

			if recorder.Code != test.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					recorder.Code,
					test.wantStatus,
					recorder.Body.String(),
				)
			}

			if fake.createCalls != test.wantCreateCalls {
				t.Fatalf(
					"Create calls = %d, want %d",
					fake.createCalls,
					test.wantCreateCalls,
				)
			}

			if fake.getByUserIDCalls != test.wantGetByIDCalls {
				t.Fatalf(
					"GetByUserID calls = %d, want %d",
					fake.getByUserIDCalls,
					test.wantGetByIDCalls,
				)
			}
		})
	}
}
```

## Mengapa test memakai `package main`?

Baris pertama test adalah:

```go
package main
```

Karena `main_test.go` berada dalam package yang sama dengan `main.go`, test dapat memanggil fungsi yang tidak diekspor:

```go
handler := newHTTPHandler(fake)
```

Nama dengan huruf kecil seperti `newHTTPHandler` hanya dapat diakses dari package yang sama. Jika test menggunakan `package main_test`, test hanya dapat mengakses identifier yang diekspor dan `newHTTPHandler` tidak akan terlihat.

Memakai package yang sama tepat untuk white-box test kecil ini karena yang ingin diuji memang fungsi composition internal.

## Fake use case dan compile-time interface assertion

Fake menyimpan dua counter:

```go
type fakeRootAccountUseCase struct {
	createCalls      int
	getByUserIDCalls int
}
```

Setiap method menaikkan counter yang sesuai:

```text
Create(...)      → createCalls++
GetByUserID(...) → getByUserIDCalls++
```

Counter tidak hanya membuktikan response HTTP berhasil. Counter juga membuktikan route meneruskan request ke method use case yang benar.

Baris berikut adalah compile-time assertion:

```go
var _ router.AccountUseCase = (*fakeRootAccountUseCase)(nil)
```

Maknanya: compiler diminta memastikan bahwa pointer `*fakeRootAccountUseCase` memenuhi interface `router.AccountUseCase`.

- `_` berarti hasil assignment tidak perlu disimpan.
- `(*fakeRootAccountUseCase)(nil)` membuat nilai pointer bertipe tanpa membuat object nyata.
- Jika method `Create` atau `GetByUserID` hilang atau signature-nya salah, kompilasi langsung gagal.
- Baris ini tidak menjalankan method dan tidak menyebabkan panic.

## Test route health

Test pertama menjalankan alur:

```text
fake use case
  → newHTTPHandler(fake)
  → httptest.NewRequest(GET /health)
  → handler.ServeHTTP(...)
  → healthHandler
  → httptest.ResponseRecorder
```

Yang diperiksa adalah:

- status `200 OK`;
- header `Content-Type` bernilai `application/json`;
- body tepat `{"status":"ok"}`;
- `Create` tidak dipanggil;
- `GetByUserID` tidak dipanggil.

Pemeriksaan counter bernilai nol penting karena `/health` seharusnya mandiri dan tidak melewati account use case.

## Table-driven test route account

Test kedua memakai slice tabel agar pola Arrange–Act–Assert yang sama dapat dipakai untuk beberapa route:

```text
POST /accounts
GET  /accounts/{userID}
```

Setiap kasus mendefinisikan:

- nama subtest;
- method HTTP;
- target URL;
- body request;
- Content-Type;
- status yang diharapkan;
- jumlah pemanggilan `Create`;
- jumlah pemanggilan `GetByUserID`.

Untuk setiap kasus, test membuat fake dan handler baru. Karena itu counter antar-subtest tidak saling bocor.

### Kasus POST `/accounts`

Alurnya:

```text
POST /accounts
  → ServeMux memilih account create handler
  → JSON request di-decode
  → fakeRootAccountUseCase.Create(...)
  → fake mengembalikan store.Account
  → handler menulis 201 Created
```

Test mengharapkan:

```text
status             = 201
Create calls       = 1
GetByUserID calls  = 0
```

Fake memakai `input.UserID` dan `input.Name` dalam result. Hal ini cukup agar account handler dapat membentuk response sukses tanpa database.

### Kasus GET `/accounts/{userID}`

Alurnya:

```text
GET /accounts/practice-user-028
  → ServeMux memilih account get handler
  → PathValue("userID") dibaca
  → fakeRootAccountUseCase.GetByUserID(...)
  → fake mengembalikan store.Account
  → handler menulis 200 OK
```

Test mengharapkan:

```text
status             = 200
Create calls       = 0
GetByUserID calls  = 1
```

## Mengapa test ini tidak menduplikasi Step 26?

Step 26 menguji perilaku detail `AccountHandler`, antara lain:

- validasi `Content-Type`;
- JSON malformed, unknown field, dan multiple JSON values;
- batas ukuran request body;
- pemetaan domain/application error ke status HTTP;
- response JSON;
- routing `HEAD`, `405 Method Not Allowed`, dan header `Allow`.

Step 28 menguji batas yang berbeda, yaitu **route composition pada root aplikasi**:

- apakah `/health` benar-benar didaftarkan oleh `newHTTPHandler`;
- apakah route POST account terhubung ke dependency yang diberikan;
- apakah route GET account terhubung ke dependency yang diberikan;
- apakah hasil `newHTTPHandler` dapat langsung dipakai sebagai `http.Handler`.

Jadi Step 26 menjawab, “apakah account handler berperilaku benar?”, sedangkan Step 28 menjawab, “apakah root aplikasi memasang handler yang benar pada route yang benar?”

Mengulang seluruh edge case Step 26 di sini akan membuat test lebih panjang tanpa menambah perlindungan yang berarti.

## Alur test tanpa database dan tanpa port

Alur test lengkap adalah:

```text
go test
  → membuat fakeRootAccountUseCase di memory
  → memanggil newHTTPHandler(fake)
  → membuat request dengan httptest.NewRequest
  → menjalankan handler.ServeHTTP secara langsung
  → merekam response dengan httptest.ResponseRecorder
  → memeriksa status, header, body, dan call counter
```

Yang **tidak** terjadi:

```text
main() tidak dipanggil
config.Load() tidak dipanggil
postgres.Open() tidak dipanggil
db.AutoMigrate() tidak dipanggil
ListenAndServe() tidak dipanggil
tidak ada TCP port yang dibuka
tidak ada container Docker yang diperlukan
```

`httptest.NewRequest` dan `httptest.NewRecorder` mensimulasikan sisi request/response di dalam proses Go. Karena tidak ada `ListenAndServe`, test tidak melakukan komunikasi jaringan sungguhan.

## Format dan menjalankan test

Dari root `practice/srv-server`, jalankan:

```bash
gofmt -w main.go main_test.go
go test . -v
go test ./...
```

Perintah pertama merapikan source. Perintah kedua hanya menguji package root sehingga mudah melihat subtest Step 28. Perintah ketiga memastikan perubahan tidak merusak package lain.

## Hasil focused test

Hasil yang diamati untuk `go test . -v`:

```text
=== RUN   TestNewHTTPHandlerRegistersHealthRoute
--- PASS: TestNewHTTPHandlerRegistersHealthRoute (0.00s)
=== RUN   TestNewHTTPHandlerRegistersAccountRoutes
=== RUN   TestNewHTTPHandlerRegistersAccountRoutes/create_account
=== RUN   TestNewHTTPHandlerRegistersAccountRoutes/get_account
--- PASS: TestNewHTTPHandlerRegistersAccountRoutes (0.00s)
    --- PASS: TestNewHTTPHandlerRegistersAccountRoutes/create_account (0.00s)
    --- PASS: TestNewHTTPHandlerRegistersAccountRoutes/get_account (0.00s)
PASS
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server	0.015s
```

Nama subtest berubah dari spasi menjadi underscore pada output Go test:

```text
"create account" → create_account
"get account"    → get_account
```

## Hasil seluruh test

`go test ./...` juga lulus. Ringkasan yang diamati:

```text
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server	0.023s
```

Package config, database migration, PostgreSQL helper, SQLC store, domain service, application use case, dan HTTP router tetap berhasil dikompilasi atau diuji. Tidak ada kegagalan pada package lain.

## Ringkasan perubahan

### `main.go`

- menambahkan `newHTTPHandler`;
- memindahkan pembuatan `http.ServeMux` ke fungsi tersebut;
- memindahkan pendaftaran health dan account route ke fungsi tersebut;
- memakai hasilnya sebagai `server.Handler`;
- tidak mengubah dependency chain PostgreSQL sampai account use case;
- tidak mengubah graceful shutdown.

### `main_test.go`

- menambahkan fake untuk `router.AccountUseCase`;
- menambahkan compile-time interface assertion;
- menguji route health;
- menguji route create account;
- menguji route get account;
- memastikan route memanggil method fake yang tepat.

## Konsep yang dipelajari

```text
composition root
test seam
dependency injection melalui parameter
interface sempit
http.Handler
httptest.NewRequest
httptest.ResponseRecorder
fake dependency
compile-time interface assertion
call counter
table-driven test
focused test
route composition test
```

## Preview Step 29

Pada Step 29, fondasi HTTP akan dilanjutkan dengan memisahkan pembuatan dependency aplikasi dari proses eksekusi server. Tujuannya adalah membuat startup/composition semakin jelas dan dapat diuji tanpa mengubah perilaku runtime. Instruksi final Step 29 tetap akan mengikuti source dan roadmap yang diperiksa pada awal step berikutnya.
