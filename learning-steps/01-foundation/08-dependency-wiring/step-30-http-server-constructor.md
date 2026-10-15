# Step 30 — Memisahkan constructor HTTP server

Pada Step 29, pembuatan account use case sudah dipisahkan dari `main()`. Pada step ini, pembuatan `http.Server` dipindahkan ke constructor `newHTTPServer` agar konfigurasi server dapat diuji tanpa membuka port jaringan.

## Tujuan

Setelah menyelesaikan step ini, kita dapat:

- memisahkan pembuatan objek `http.Server` dari lifecycle server;
- memahami kegunaan `ReadHeaderTimeout`;
- memberikan hanya address dan handler yang benar-benar diperlukan constructor;
- menerapkan pola **accept narrow values/interfaces, return concrete type**;
- menguji `Addr`, `Handler`, dan `ReadHeaderTimeout` tanpa PostgreSQL, Docker, atau port jaringan;
- memahami batas pengujian constructor pada step ini.

## Prasyarat

Step 1 sampai Step 29 harus sudah selesai. Komponen berikut sudah tersedia:

```text
config/config.go
internal/app/account/
internal/domain/account/
internal/router/
internal/store/
main.go
main_test.go
```

Kontrak account latihan tetap menggunakan `user_id` dan `name`. Tidak ada field `email`.

## File yang diubah

```text
srv-server/main.go
srv-server/main_test.go
```

## Perubahan pada `main.go`

Kebijakan timeout diberi nama melalui konstanta:

```go
const httpReadHeaderTimeout = 5 * time.Second
```

Lalu constructor berikut membuat server:

```go
func newHTTPServer(
	address string,
	handler http.Handler,
) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}
}
```

`newHTTPServer` hanya membentuk nilai `*http.Server`. Fungsi ini tidak memanggil `ListenAndServe`, tidak membuka port, tidak menerima request, dan tidak mengakses database.

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

const httpReadHeaderTimeout = 5 * time.Second

func newHTTPServer(
	address string,
	handler http.Handler,
) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}
}

func newAccountUseCase(
	accountStore accountdomain.Store,
) *accountapp.UseCase {
	accountService := accountdomain.NewService(
		accountStore,
	)

	return accountapp.NewUseCase(
		accountService,
	)
}

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

	accountUseCase := newAccountUseCase(
		queries,
	)

	httpHandler := newHTTPHandler(
		accountUseCase,
	)

	server := newHTTPServer(
		cfg.HTTPAddr,
		httpHandler,
	)

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

## Accept narrow values/interfaces, return concrete type

Constructor menerima dua hal yang benar-benar dibutuhkan:

```go
address string
handler http.Handler
```

`address` adalah nilai sempit dan `http.Handler` adalah interface sempit untuk objek yang dapat menangani HTTP. Constructor tidak menerima seluruh `config.Config`, sebab bagian PostgreSQL dan konfigurasi lain tidak diperlukan untuk membuat server.

Hasil constructor adalah concrete type:

```go
*http.Server
```

Dengan begitu, dependency masuk tetap sekecil mungkin dan pemanggil mendapatkan objek server nyata yang dapat dikonfigurasi, dijalankan, atau dihentikan.

## Constructor berbeda dari lifecycle

Constructor hanya mengatur keadaan awal objek:

```text
address + handler
        ↓
newHTTPServer
        ↓
*http.Server yang sudah dikonfigurasi
```

Lifecycle masih berada di `main()`:

```text
ListenAndServe
→ menunggu signal atau server error
→ Shutdown
→ Close jika graceful shutdown gagal
```

Pemisahan ini membuat konfigurasi server dapat diuji tanpa menyalakan server. Pemisahan lifecycle agar bebas dari signal sistem akan menjadi pekerjaan Step 31.

## Kegunaan `ReadHeaderTimeout`

`ReadHeaderTimeout` membatasi waktu yang boleh dipakai client untuk mengirim header request HTTP. Tanpa batas ini, client yang sangat lambat atau sengaja menahan pengiriman header dapat mempertahankan koneksi dan memakai resource server terlalu lama.

Nilainya dijadikan konstanta aplikasi:

```go
const httpReadHeaderTimeout = 5 * time.Second
```

Step ini sengaja belum menambahkan:

- `ReadTimeout` untuk keseluruhan proses membaca request;
- `WriteTimeout` untuk menulis response;
- `IdleTimeout` untuk koneksi keep-alive yang sedang idle.

Ketiganya adalah keputusan kebijakan terpisah dan berada di luar scope Step 30.

## Test constructor

Test membuat `*http.ServeMux`, memberikan address latihan, lalu memeriksa field hasil constructor:

```go
func TestNewHTTPServerConfiguresServer(
	t *testing.T,
) {
	handler := http.NewServeMux()

	server := newHTTPServer(
		":28080",
		handler,
	)

	if server == nil {
		t.Fatal(
			"newHTTPServer returned nil",
		)
	}

	if server.Addr != ":28080" {
		t.Fatalf(
			"Addr = %q, want %q",
			server.Addr,
			":28080",
		)
	}

	if server.Handler != handler {
		t.Fatal(
			"Handler does not preserve the supplied handler",
		)
	}

	if server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf(
			"ReadHeaderTimeout = %s, want %s",
			server.ReadHeaderTimeout,
			5*time.Second,
		)
	}
}
```

Assertion timeout memakai literal `5*time.Second`, bukan `httpReadHeaderTimeout`. Jika constructor keliru memasang nilai lain tetapi konstanta yang sama dipakai sebagai nilai harapan test, kesalahan tersebut dapat tersembunyi. Literal membuat perilaku lima detik yang diinginkan terlihat langsung pada test.

## Mengapa handler memakai `*http.ServeMux`?

`http.NewServeMux()` mengembalikan pointer:

```go
handler := http.NewServeMux()
```

Pointer dapat dibandingkan. Karena itu test dapat memastikan constructor mempertahankan objek yang sama:

```go
if server.Handler != handler
```

Sebaliknya, `http.HandlerFunc` memiliki underlying value berupa fungsi. Nilai fungsi tidak dapat dibandingkan selain terhadap `nil`. Memasukkan `HandlerFunc` ke interface lalu membandingkan kedua interface dapat menyebabkan panic. `*http.ServeMux` menghindari masalah itu sekaligus mewakili handler HTTP nyata.

## Isi lengkap akhir `main_test.go`

```go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	accountapp "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account"
	accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
)

type fakeRootAccountUseCase struct {
	createCalls      int
	getByUserIDCalls int
}

type fakeRootAccountStore struct {
	createAccountCalls  int
	createAccountParams store.CreateAccountParams
}

var _ accountdomain.Store = (*fakeRootAccountStore)(nil)

func (f *fakeRootAccountStore) CreateAccount(
	_ context.Context,
	params store.CreateAccountParams,
) (store.Account, error) {
	f.createAccountCalls++
	f.createAccountParams = params

	return store.Account{
		ID:     29,
		UserID: params.UserID,
		Name:   params.Name,
		Status: 1,
	}, nil
}

func (f *fakeRootAccountStore) GetAccountByUserID(
	context.Context,
	string,
) (store.Account, error) {
	panic("unexpected GetAccountByUserID call")
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

func TestNewHTTPServerConfiguresServer(
	t *testing.T,
) {
	handler := http.NewServeMux()

	server := newHTTPServer(
		":28080",
		handler,
	)

	if server == nil {
		t.Fatal(
			"newHTTPServer returned nil",
		)
	}

	if server.Addr != ":28080" {
		t.Fatalf(
			"Addr = %q, want %q",
			server.Addr,
			":28080",
		)
	}

	if server.Handler != handler {
		t.Fatal(
			"Handler does not preserve the supplied handler",
		)
	}

	if server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf(
			"ReadHeaderTimeout = %s, want %s",
			server.ReadHeaderTimeout,
			5*time.Second,
		)
	}
}

func TestNewAccountUseCaseWiresDependencies(
	t *testing.T,
) {
	fake := &fakeRootAccountStore{}

	useCase := newAccountUseCase(
		fake,
	)

	account, err := useCase.Create(
		context.Background(),
		accountapp.CreateInput{
			UserID: "  practice-user-029  ",
			Name:   "  Step 29 Practice User  ",
		},
	)
	if err != nil {
		t.Fatalf(
			"Create returned error: %v",
			err,
		)
	}

	if fake.createAccountCalls != 1 {
		t.Fatalf(
			"CreateAccount calls = %d, want 1",
			fake.createAccountCalls,
		)
	}

	expectedParams := store.CreateAccountParams{
		UserID: "practice-user-029",
		Name:   "Step 29 Practice User",
	}

	if fake.createAccountParams != expectedParams {
		t.Fatalf(
			"CreateAccount params = %+v, want %+v",
			fake.createAccountParams,
			expectedParams,
		)
	}

	if account.ID != 29 ||
		account.UserID != expectedParams.UserID ||
		account.Name != expectedParams.Name ||
		account.Status != 1 {
		t.Fatalf(
			"account = %+v, want composed account result",
			account,
		)
	}
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

## Perilaku yang sengaja berada di luar scope

Jika `handler` bernilai `nil`, standar library akan memakai `http.DefaultServeMux` ketika server dijalankan. Constructor saat ini tidak menolak `nil`, dan test tidak mengubah perilaku bawaan tersebut. Runtime aplikasi tetap memberikan handler non-nil dari `newHTTPHandler`.

Default address dari config adalah `:18080`. Bentuk ini berarti server bind pada port 18080 di seluruh interface yang sesuai. Log yang ada saat ini menyusun `http://localhost%s`, sehingga untuk address default tampil sebagai `http://localhost:18080`. Jika kelak `PRACTICE_HTTP_ADDR` berisi host lengkap seperti `127.0.0.1:18080`, format log tersebut perlu dipertimbangkan kembali karena dapat menjadi `http://localhost127.0.0.1:18080`. Step 30 tidak mengubah binding ataupun format log itu.

## Menjalankan test

Dari root `srv-server`, format dan jalankan focused test:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w main.go main_test.go

go test . \
  -run '^TestNewHTTPServerConfiguresServer$' \
  -v
```

Hasil yang diamati:

```text
=== RUN   TestNewHTTPServerConfiguresServer
--- PASS: TestNewHTTPServerConfiguresServer (0.00s)
PASS
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server	0.010s
```

Kemudian jalankan seluruh test:

```bash
go test ./...
```

Hasil penting yang diamati:

```text
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server	0.009s
```

Seluruh package lulus. Test ini tidak menjalankan Docker, tidak terhubung ke PostgreSQL, dan tidak membuka port. Address `:28080` hanya disimpan sebagai string karena `ListenAndServe` tidak pernah dipanggil.

## Ringkasan perubahan

Pada Step 30 kita:

- menambahkan konstanta `httpReadHeaderTimeout`;
- menambahkan constructor `newHTTPServer`;
- mengganti struct literal server di `main()` dengan pemanggilan constructor;
- menambahkan `TestNewHTTPServerConfiguresServer`;
- memastikan address, identitas handler, dan timeout terpasang dengan benar;
- menjaga lifecycle server tetap sama.

Konsep utamanya:

```text
constructor = membentuk dan mengonfigurasi objek
lifecycle   = menjalankan, menunggu, dan menghentikan objek
```

## Step berikutnya

Step 31 akan mengekstrak lifecycle HTTP server agar tidak bergantung langsung pada signal sistem. Setelah dipisahkan, alur start, server error, shutdown, dan force close dapat diuji dengan dependency terkontrol tanpa mengirim signal ke proses test.
