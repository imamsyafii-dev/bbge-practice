# Step 29 — Memisahkan komposisi account use case

Pada Step 28, komposisi route HTTP sudah dipindahkan dari `main()` ke `newHTTPHandler`. Pada step ini, rangkaian dependency account juga dipindahkan ke fungsi kecil bernama `newAccountUseCase`, lalu wiring tersebut diuji memakai fake store.

## Tujuan

Setelah menyelesaikan step ini, kita dapat:

- memisahkan komposisi domain dan application layer dari proses startup server;
- memahami prinsip **accept interface, return concrete type**;
- memilih interface `accountdomain.Store` yang sesuai kebutuhan domain;
- memahami mengapa `store.New(database)` tetap berada di `main()`;
- membuktikan alur `use case → domain service → store` dengan fake;
- menguji wiring tanpa HTTP, PostgreSQL, Docker, atau membuka port.

## Prasyarat

Step 1 sampai Step 28 harus sudah selesai. Komponen berikut sudah tersedia:

```text
internal/app/account/usecase.go
internal/domain/account/service.go
internal/router/account.go
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

## Pemisahan tanggung jawab

Setelah perubahan ini, composition root terbagi menjadi beberapa bagian kecil:

```text
main()
├── membaca config
├── membuat signal context
├── membuka PostgreSQL
├── menjalankan migration
├── store.New(database)       → adapter infrastruktur SQLC
├── newAccountUseCase(store)  → domain service + application use case
├── newHTTPHandler(useCase)   → router + ServeMux
└── menjalankan dan menghentikan http.Server
```

`newAccountUseCase` hanya bertanggung jawab merangkai domain dan application layer. `newHTTPHandler` hanya bertanggung jawab merangkai HTTP handler dan route. Siklus hidup server—mulai, menunggu signal, graceful shutdown—tetap menjadi tanggung jawab `main()`.

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

## Cara kerja `newAccountUseCase`

Fungsi baru menerima store account:

```go
func newAccountUseCase(
	accountStore accountdomain.Store,
) *accountapp.UseCase
```

Lalu fungsi itu merangkai dependency berikut:

```text
accountdomain.Store
        ↓
accountdomain.NewService
        ↓
accountapp.NewUseCase
        ↓
*accountapp.UseCase
```

Domain service menjadi tempat aturan domain, seperti normalisasi dan validasi. Application use case mengatur pemanggilan operasi domain dari kebutuhan aplikasi. Factory ini tidak menjalankan query dan tidak membuat database; ia hanya menyambungkan objek.

## Accept interface, return concrete type

Parameter `newAccountUseCase` berupa interface:

```go
accountStore accountdomain.Store
```

Artinya fungsi menerima implementasi apa pun yang memiliki operasi store yang dibutuhkan domain account. Saat runtime, implementasinya adalah `*store.Queries`. Saat test, implementasinya adalah `*fakeRootAccountStore`.

Return value-nya berupa concrete type:

```go
*accountapp.UseCase
```

Pemanggil mengetahui objek nyata yang dibuat oleh factory, sementara dependency masuk tetap fleksibel. Inilah maksud prinsip:

```text
accept interface, return concrete type
```

Prinsip ini bukan aturan mutlak untuk semua situasi, tetapi cocok di sini karena interface diperlukan pada batas dependency masuk, sedangkan factory memang membuat satu implementasi use case tertentu.

## Mengapa memakai `accountdomain.Store`, bukan `store.DBTX`?

`accountdomain.Store` menggambarkan kebutuhan domain account secara langsung:

```text
CreateAccount
GetAccountByUserID
```

Sebaliknya, `store.DBTX` adalah kontrak infrastruktur tingkat rendah yang dibutuhkan kode hasil SQLC untuk menjalankan operasi database seperti `Exec`, `Query`, dan `QueryRow`. Jika factory menerima `store.DBTX`, maka:

- factory akan mengetahui detail database yang tidak diperlukan;
- test harus memalsukan API database tingkat rendah;
- batas antara domain dan infrastruktur menjadi kurang jelas.

Dengan `accountdomain.Store`, fake test hanya perlu menyediakan operasi account yang benar-benar dipakai domain.

## Mengapa `store.New(database)` tetap di `main()`?

Baris berikut tetap berada di `main()`:

```go
queries := store.New(database)
```

`store.New` mengubah koneksi PostgreSQL menjadi adapter SQLC `*store.Queries`. Pembuatan adapter ini adalah wiring infrastruktur karena membutuhkan objek database konkret. Setelah dibuat, `queries` memenuhi `accountdomain.Store` dan dapat dikirim ke `newAccountUseCase`.

Pemisahannya menjadi:

```text
main():               database → SQLC adapter
newAccountUseCase():  store interface → domain service → app use case
newHTTPHandler():     app use case → HTTP handler → routes
```

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

## Fake store dan compile-time assertion

Fake menyimpan jumlah panggilan dan parameter terakhir:

```go
type fakeRootAccountStore struct {
	createAccountCalls  int
	createAccountParams store.CreateAccountParams
}
```

Assertion berikut diperiksa compiler:

```go
var _ accountdomain.Store = (*fakeRootAccountStore)(nil)
```

Baris tersebut tidak membuat objek saat runtime. Ia hanya memastikan `*fakeRootAccountStore` memiliki semua method yang diwajibkan `accountdomain.Store`. Jika satu method hilang atau signature-nya salah, kompilasi gagal.

`GetAccountByUserID` tetap dibuat agar fake memenuhi interface, tetapi berisi `panic`:

```go
panic("unexpected GetAccountByUserID call")
```

Test ini hanya seharusnya menjalankan alur create. Jika wiring tanpa sengaja memanggil get, test langsung gagal dan menunjukkan panggilan yang tidak diharapkan.

## Alur focused test

Test `TestNewAccountUseCaseWiresDependencies` menjalankan alur:

```text
Test
  ↓
newAccountUseCase(fakeRootAccountStore)
  ↓
*accountapp.UseCase.Create
  ↓
*accountdomain.Service.Create
  ↓
fakeRootAccountStore.CreateAccount
```

Input yang dikirim sengaja memiliki spasi:

```go
UserID: "  practice-user-029  "
Name:   "  Step 29 Practice User  "
```

Fake kemudian harus menerima:

```go
store.CreateAccountParams{
	UserID: "practice-user-029",
	Name:   "Step 29 Practice User",
}
```

Perbedaan ini menjadi bukti bahwa factory tidak menghubungkan use case langsung ke store. Domain service asli benar-benar berada di tengah dan menjalankan normalisasi.

Test juga memeriksa:

- `CreateAccount` dipanggil tepat satu kali;
- parameter yang diterima fake sudah dinormalisasi;
- hasil dari fake berjalan kembali melewati service dan use case;
- hasil akhir memiliki `ID`, `UserID`, `Name`, dan `Status` yang diharapkan.

## Batas test pada step ini

Test ini tidak menjalankan:

```text
HTTP request
httptest untuk focused test
http.Server
TCP port
PostgreSQL
Docker
migration
```

Test sengaja hanya memakai satu happy path. Validasi input, error propagation, dan operasi `GetByUserID` sudah diuji pada test domain dan application use case. Detail status HTTP, JSON, dan error mapping sudah diuji pada test router. Mengulang semuanya di sini akan membuat test composition tumpang tindih dengan tanggung jawab layer lain.

## Perintah verifikasi

Dari folder `practice/srv-server`, jalankan:

```bash
gofmt -w main.go main_test.go

go test . \
  -run '^TestNewAccountUseCaseWiresDependencies$' \
  -v

go test . -v
go test ./...
```

## Hasil verifikasi yang diamati

Focused test lulus:

```text
=== RUN   TestNewAccountUseCaseWiresDependencies
--- PASS: TestNewAccountUseCaseWiresDependencies (0.00s)
PASS
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server	0.009s
```

Seluruh test package root lulus dalam `0.010s`, termasuk test wiring Step 28 dan Step 29:

```text
PASS
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server	0.010s
```

Perintah `go test ./...` juga lulus untuk seluruh package. Tidak ada Docker, PostgreSQL, server HTTP, atau port yang digunakan selama verifikasi ini.

## Ringkasan perubahan dan konsep

File yang berubah:

```text
srv-server/main.go
srv-server/main_test.go
```

Konsep yang dipelajari:

- composition root dapat dipecah menjadi factory kecil;
- dependency masuk dapat berupa interface sempit;
- factory dapat mengembalikan concrete type yang dibuatnya;
- adapter infrastruktur dibuat di batas aplikasi;
- fake membuktikan wiring tanpa database;
- normalisasi input menjadi bukti domain service benar-benar terpasang;
- test composition tidak perlu mengulang seluruh test domain, application, dan HTTP.

## Step berikutnya

Step 30 direncanakan membahas constructor `http.Server` dan pengujian konfigurasi server. Detail akhirnya akan disesuaikan dengan struktur kode saat step tersebut dimulai, sehingga kita belum mengunci bentuk implementasinya sekarang.
