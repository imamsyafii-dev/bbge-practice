# Step 31 — Memisahkan dan menguji lifecycle HTTP server

Pada Step 30, pembuatan `http.Server` sudah dipisahkan ke constructor. Step ini memisahkan proses menjalankan server, menangani kegagalan, graceful shutdown, dan fallback penutupan paksa ke fungsi `runHTTPServer` yang dapat diuji tanpa membuka port.

## Tujuan

Setelah menyelesaikan step ini, kita dapat:

- memisahkan lifecycle HTTP server dari `main()`;
- memahami dua kejadian yang dapat menghentikan server: `ListenAndServe` selesai atau application context dibatalkan;
- menjalankan graceful shutdown dengan batas waktu;
- memakai `Close` sebagai fallback ketika `Shutdown` gagal;
- mempertahankan beberapa error dengan `errors.Join`;
- menguji concurrency dengan channel dan hubungan happens-before tanpa `time.Sleep`;
- memastikan goroutine `ListenAndServe` selalu ditunggu pada jalur shutdown;
- menguji lifecycle tanpa port jaringan, signal OS, Docker, atau PostgreSQL.

## Prasyarat

Step 1 sampai Step 30 harus sudah selesai. `main.go` sudah mempunyai:

```text
newAccountUseCase
newHTTPHandler
newHTTPServer
```

Konfigurasi server telah memiliki `httpReadHeaderTimeout`, sedangkan lifecycle server sebelumnya masih ditulis langsung di dalam `main()`.

## File yang diubah

```text
srv-server/main.go
srv-server/main_test.go
```

Dokumentasi offline step ini berada di:

```text
learning-steps/01-foundation/09-server-lifecycle/step-31-test-http-server-lifecycle.md
```

## 1. Konstanta timeout

```go
const (
	httpReadHeaderTimeout = 5 * time.Second
	httpShutdownTimeout   = 10 * time.Second
)
```

`httpReadHeaderTimeout` tetap menjadi kebijakan untuk membaca header request. `httpShutdownTimeout` adalah waktu maksimum yang diberikan kepada graceful shutdown.

Keduanya sengaja memiliki tanggung jawab berbeda. Timeout membaca header tidak boleh dipakai sebagai timeout penghentian server.

## 2. Interface lifecycle yang sempit

```go
type httpServerLifecycle interface {
	ListenAndServe() error
	Shutdown(context.Context) error
	Close() error
}

var _ httpServerLifecycle = (*http.Server)(nil)
```

`runHTTPServer` tidak membutuhkan seluruh detail concrete `*http.Server`. Ia hanya membutuhkan tiga operasi lifecycle tersebut.

Compile-time assertion membuktikan bahwa `*http.Server` memenuhi kontrak. Test kemudian dapat memakai fake yang memenuhi interface yang sama tanpa membuka socket.

## 3. Alur `runHTTPServer`

Alur besarnya:

```text
mulai ListenAndServe di goroutine
                 │
                 ▼
menunggu server selesai ATAU appCtx dibatalkan
        │                         │
        │                         ├─ panggil callback shutdown
        │                         ├─ buat context timeout 10 detik
        │                         ├─ panggil Shutdown
        │                         ├─ jika gagal, panggil Close
        │                         └─ tetap ambil hasil ListenAndServe
        │
        ├─ nil / http.ErrServerClosed → selesai normal
        └─ error lain → bungkus dan kembalikan
```

### Branch pertama: server selesai lebih dahulu

Jika `ListenAndServe` mengembalikan hasil sebelum application context dibatalkan:

- `nil` dianggap selesai normal;
- `http.ErrServerClosed` juga dianggap selesai normal;
- error lain dibungkus sebagai `listen and serve HTTP: ...`.

Pada branch ini tidak ada alasan memanggil `Shutdown`, `Close`, atau callback, karena server sudah berhenti.

### Branch kedua: application context dibatalkan

Jika `appCtx.Done()` lebih dahulu siap:

1. callback `onShutdownRequested` dipanggil;
2. dibuat shutdown context baru dengan timeout 10 detik;
3. `Shutdown` menolak koneksi baru dan menunggu koneksi aktif selesai;
4. jika `Shutdown` gagal, `Close` dipakai sebagai fallback;
5. hasil `ListenAndServe` tetap diambil dari `serverErrors`;
6. semua error relevan dikembalikan.

## 4. Mengapa `serverErrors` buffered dan tetap harus di-drain

```go
serverErrors := make(chan error, 1)
```

Buffer berukuran satu memungkinkan goroutine pengirim menyimpan satu hasil `ListenAndServe` walaupun receiver belum sedang membaca. Ini penting bila hasil server muncul hampir bersamaan dengan pembatalan context.

Namun buffer bukan alasan untuk mengabaikan hasil tersebut. Pada jalur shutdown, kode tetap menjalankan:

```go
serveErr := <-serverErrors
```

Drain ini:

- menyinkronkan selesainya goroutine server dengan kembalinya `runHTTPServer`;
- memastikan error server yang muncul selama shutdown tidak hilang;
- mencegah lifecycle dilaporkan selesai sebelum `ListenAndServe` benar-benar selesai.

Test graceful shutdown secara khusus membuktikan fungsi belum kembali sebelum fake mengirim `http.ErrServerClosed`.

## 5. Callback memisahkan lifecycle dari signal OS

`runHTTPServer` menerima callback biasa:

```go
onShutdownRequested func()
```

Fungsi ini tidak mengimpor atau memahami `os.Signal`. `main()` tetap menjadi tempat yang mengetahui bahwa application context berasal dari `signal.NotifyContext`.

Callback di `main()` melakukan:

```go
func() {
	log.Printf("shutdown signal received")
	stopSignals()
}
```

`stopSignals` juga dipanggil melalui `defer`. Pemanggilan cancel function lebih dari sekali aman atau idempotent, sehingga callback dan defer tidak saling bertentangan.

## 6. Mengapa shutdown memakai `context.Background()`

```go
shutdownCtx, cancelShutdown := context.WithTimeout(
	context.Background(),
	httpShutdownTimeout,
)
```

`appCtx` sudah dibatalkan ketika branch shutdown dipilih. Jika context yang sudah batal itu dipakai sebagai parent, `Shutdown` akan langsung menerima context selesai dan tidak mendapat waktu untuk menunggu request aktif.

Karena itu graceful shutdown memperoleh context baru dari `context.Background()` dengan deadline sendiri. Setelah `Shutdown` yang sinkron selesai, `cancelShutdown()` segera dipanggil untuk melepaskan resource timer.

Pada test, context yang diterima fake harus:

- mempunyai deadline;
- memiliki sisa waktu lebih dari nol;
- tidak melebihi `httpShutdownTimeout`;
- berstatus `context.Canceled` setelah pemanggilan selesai karena cancel function telah dijalankan.

## 7. Fallback `Close`

`Shutdown` berusaha menghentikan server secara halus. Bila operasi itu gagal atau melewati deadline, kode memanggil:

```go
server.Close()
```

`Close` adalah fallback paksa untuk koneksi yang dikelola server. Ia hanya dipanggil ketika `Shutdown` mengembalikan error. Pada graceful shutdown yang berhasil, `Close` tidak boleh dipanggil.

Catatan penting dari perilaku `net/http`: `Shutdown` tidak menutup dan tidak menunggu koneksi yang sudah diambil alih melalui hijacking, misalnya koneksi WebSocket. Aplikasi yang kelak mempunyai koneksi seperti itu perlu melacak dan menutupnya sendiri, biasanya melalui mekanisme shutdown khusus.

## 8. Error wrapping, `errors.Join`, dan `errors.Is`

Error tunggal dibungkus memakai `%w`, misalnya:

```go
fmt.Errorf("shutdown HTTP server: %w", shutdownErr)
```

Pembungkusan menambahkan konteks tanpa menghilangkan identitas error asal. Pemanggil masih dapat memeriksanya dengan `errors.Is`.

Jika `Shutdown` dan `Close` sama-sama gagal, keduanya dipertahankan:

```go
lifecycleErr = errors.Join(
	lifecycleErr,
	fmt.Errorf("force close HTTP server: %w", closeErr),
)
```

Karena hasil `errors.Join` dapat memiliki beberapa penyebab, kedua pemeriksaan berikut dapat sama-sama bernilai benar:

```go
errors.Is(err, shutdownErr)
errors.Is(err, closeErr)
```

Error non-normal dari `ListenAndServe` yang muncul sesudah proses shutdown juga digabungkan. Sebaliknya, `http.ErrServerClosed` adalah hasil normal dari penutupan server dan tidak dilaporkan sebagai kegagalan.

## 9. Perubahan perilaku logging yang disengaja

Sebelumnya, jika `ListenAndServe` gagal lebih dahulu, `main()` mencetak error langsung pada branch tersebut lalu keluar tanpa menjalankan graceful shutdown karena server sudah berhenti. Setelah refactor, semua keputusan lifecycle berada di `runHTTPServer`.

`main()` sekarang hanya mencetak salah satu bentuk akhir:

```text
practice server stopped
```

atau:

```text
practice server stopped: <error lifecycle>
```

Dengan demikian, error lifecycle dibungkus dan dikembalikan terlebih dahulu. Logging tetap menjadi tanggung jawab composition root, sedangkan `runHTTPServer` tetap mudah diuji tanpa menangkap output log.

## Isi lengkap akhir `main.go`

```go
package main

import (
	"context"
	"errors"
	"fmt"
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

const (
	httpReadHeaderTimeout = 5 * time.Second
	httpShutdownTimeout   = 10 * time.Second
)

type httpServerLifecycle interface {
	ListenAndServe() error
	Shutdown(context.Context) error
	Close() error
}

var _ httpServerLifecycle = (*http.Server)(nil)

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

func runHTTPServer(
	appCtx context.Context,
	server httpServerLifecycle,
	onShutdownRequested func(),
) error {
	serverErrors := make(
		chan error,
		1,
	)

	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err == nil ||
			errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf(
			"listen and serve HTTP: %w",
			err,
		)

	case <-appCtx.Done():
		onShutdownRequested()
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		httpShutdownTimeout,
	)

	shutdownErr := server.Shutdown(
		shutdownCtx,
	)
	cancelShutdown()

	var lifecycleErr error

	if shutdownErr != nil {
		lifecycleErr = fmt.Errorf(
			"shutdown HTTP server: %w",
			shutdownErr,
		)

		if closeErr := server.Close(); closeErr != nil {
			lifecycleErr = errors.Join(
				lifecycleErr,
				fmt.Errorf(
					"force close HTTP server: %w",
					closeErr,
				),
			)
		}
	}

	serveErr := <-serverErrors
	if serveErr != nil &&
		!errors.Is(
			serveErr,
			http.ErrServerClosed,
		) {
		lifecycleErr = errors.Join(
			lifecycleErr,
			fmt.Errorf(
				"listen and serve HTTP: %w",
				serveErr,
			),
		)
	}

	return lifecycleErr
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

	if err := runHTTPServer(
		appCtx,
		server,
		func() {
			log.Printf("shutdown signal received")
			stopSignals()
		},
	); err != nil {
		log.Printf(
			"practice server stopped: %v",
			err,
		)

		return
	}

	log.Printf("practice server stopped")
}
```

## 10. Desain fake lifecycle berbasis channel

Fake menyediakan empat titik sinkronisasi:

```text
listenStarted  → memberi tahu bahwa ListenAndServe sudah berjalan
listenResult   → test menentukan kapan dan bagaimana ListenAndServe selesai
shutdownCalls  → merekam context yang diberikan kepada Shutdown
closeCalls     → merekam pemanggilan Close
```

`ListenAndServe` menutup `listenStarted` lalu menunggu dari `listenResult`. Penutupan channel membentuk hubungan happens-before: setelah test menerima dari `listenStarted`, test tahu goroutine sudah masuk ke method tersebut.

`shutdownCalls` dan `closeCalls` memiliki buffer satu agar fake dapat merekam pemanggilan tanpa bergantung pada ketepatan jadwal receiver.

Pengujian tidak memakai `time.Sleep`. Sinkronisasi utama dilakukan oleh channel. `time.After(lifecycleTestTimeout)` hanya menjadi batas pengaman agar kegagalan concurrency berubah menjadi pesan timeout, bukan test yang menggantung selamanya.

### Caveat cleanup pada failure path test

Fake `ListenAndServe` memblokir sampai test mengirim nilai ke `listenResult`. Pada happy execution, setiap test yang memasuki jalur shutdown selalu mengirim hasil sehingga goroutine selesai.

Jika test memanggil `t.Fatal` sebelum sempat mengirim hasil itu, goroutine fake dapat tertinggal sampai proses test berakhir. Timeout helper mencegah suite menggantung pada receive yang ditunggu oleh test, tetapi tidak otomatis membersihkan semua goroutine pada setiap failure path. Untuk suite yang lebih besar, cleanup dapat diperkuat dengan channel yang bisa ditutup sekali atau `t.Cleanup` yang memastikan fake selalu dilepas.

## 11. Tiga test lifecycle

### `TestRunHTTPServerReturnsListenError`

Test ini mengirim `listen failed` sebagai hasil `ListenAndServe` sebelum context dibatalkan.

Yang dibuktikan:

- hasil mengandung error asal melalui `errors.Is`;
- `Shutdown` tidak dipanggil;
- `Close` tidak dipanggil;
- callback shutdown tidak dipanggil.

Ini menguji branch ketika server selesai lebih dahulu.

### `TestRunHTTPServerGracefulShutdown`

Test membatalkan application context setelah `ListenAndServe` dipastikan mulai.

Yang dibuktikan:

- callback dipanggil tepat satu kali;
- `Shutdown` dipanggil tepat satu kali;
- shutdown context mempunyai deadline yang valid;
- context dibatalkan sesudah `Shutdown` selesai;
- `Close` tidak dipanggil;
- `runHTTPServer` belum selesai sebelum hasil `ListenAndServe` dikirim;
- `http.ErrServerClosed` diperlakukan sebagai hasil normal.

Ini sekaligus menguji mandatory drain terhadap `serverErrors`.

### `TestRunHTTPServerFallsBackToClose`

Fake dikonfigurasi agar `Shutdown` dan `Close` sama-sama gagal.

Yang dibuktikan:

- `Close` dipanggil setelah kegagalan `Shutdown`;
- callback tetap dipanggil;
- `Shutdown` dan `Close` masing-masing hanya sekali;
- hasil akhir masih dikenali sebagai `shutdownErr`;
- hasil akhir juga dikenali sebagai `closeErr`.

Ini membuktikan `errors.Join` tidak membuang salah satu penyebab.

## Isi lengkap akhir `main_test.go`

```go
package main

import (
	"context"
	"errors"
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

const lifecycleTestTimeout = time.Second

type fakeHTTPServerLifecycle struct {
	listenStarted chan struct{}
	listenResult  chan error
	shutdownCalls chan context.Context
	closeCalls    chan struct{}

	shutdownErr error
	closeErr    error
}

var _ httpServerLifecycle = (*fakeHTTPServerLifecycle)(nil)

func newFakeHTTPServerLifecycle() *fakeHTTPServerLifecycle {
	return &fakeHTTPServerLifecycle{
		listenStarted: make(chan struct{}),
		listenResult:  make(chan error),
		shutdownCalls: make(
			chan context.Context,
			1,
		),
		closeCalls: make(
			chan struct{},
			1,
		),
	}
}

func (f *fakeHTTPServerLifecycle) ListenAndServe() error {
	close(f.listenStarted)

	return <-f.listenResult
}

func (f *fakeHTTPServerLifecycle) Shutdown(
	ctx context.Context,
) error {
	f.shutdownCalls <- ctx

	return f.shutdownErr
}

func (f *fakeHTTPServerLifecycle) Close() error {
	f.closeCalls <- struct{}{}

	return f.closeErr
}

func waitForLifecycleStart(
	t *testing.T,
	server *fakeHTTPServerLifecycle,
) {
	t.Helper()

	select {
	case <-server.listenStarted:
	case <-time.After(lifecycleTestTimeout):
		t.Fatal(
			"timed out waiting for ListenAndServe",
		)
	}
}

func waitForLifecycleResult(
	t *testing.T,
	results <-chan error,
) error {
	t.Helper()

	select {
	case err := <-results:
		return err

	case <-time.After(lifecycleTestTimeout):
		t.Fatal(
			"timed out waiting for runHTTPServer",
		)

		return nil
	}
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

func TestRunHTTPServerReturnsListenError(
	t *testing.T,
) {
	listenErr := errors.New(
		"listen failed",
	)

	server := newFakeHTTPServerLifecycle()

	appCtx, cancelApp := context.WithCancel(
		context.Background(),
	)
	defer cancelApp()

	shutdownRequested := make(
		chan struct{},
		1,
	)
	results := make(
		chan error,
		1,
	)

	go func() {
		results <- runHTTPServer(
			appCtx,
			server,
			func() {
				shutdownRequested <- struct{}{}
			},
		)
	}()

	waitForLifecycleStart(
		t,
		server,
	)

	server.listenResult <- listenErr

	err := waitForLifecycleResult(
		t,
		results,
	)
	if !errors.Is(err, listenErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			listenErr,
		)
	}

	select {
	case <-server.shutdownCalls:
		t.Fatal(
			"Shutdown should not be called",
		)
	default:
	}

	select {
	case <-server.closeCalls:
		t.Fatal(
			"Close should not be called",
		)
	default:
	}

	select {
	case <-shutdownRequested:
		t.Fatal(
			"shutdown callback should not be called",
		)
	default:
	}
}

func TestRunHTTPServerGracefulShutdown(
	t *testing.T,
) {
	server := newFakeHTTPServerLifecycle()

	appCtx, cancelApp := context.WithCancel(
		context.Background(),
	)

	shutdownRequested := make(
		chan struct{},
		1,
	)
	results := make(
		chan error,
		1,
	)

	go func() {
		results <- runHTTPServer(
			appCtx,
			server,
			func() {
				shutdownRequested <- struct{}{}
			},
		)
	}()

	waitForLifecycleStart(
		t,
		server,
	)

	cancelApp()

	var shutdownCtx context.Context

	select {
	case shutdownCtx = <-server.shutdownCalls:
	case <-time.After(lifecycleTestTimeout):
		t.Fatal(
			"timed out waiting for Shutdown",
		)
	}

	select {
	case <-shutdownRequested:
	case <-time.After(lifecycleTestTimeout):
		t.Fatal(
			"shutdown callback was not called",
		)
	}

	deadline, ok := shutdownCtx.Deadline()
	if !ok {
		t.Fatal(
			"Shutdown context has no deadline",
		)
	}

	remaining := time.Until(deadline)
	if remaining <= 0 ||
		remaining > httpShutdownTimeout {
		t.Fatalf(
			"Shutdown deadline remaining = %s",
			remaining,
		)
	}

	select {
	case err := <-results:
		t.Fatalf(
			"runHTTPServer returned before ListenAndServe: %v",
			err,
		)
	default:
	}

	server.listenResult <- http.ErrServerClosed

	err := waitForLifecycleResult(
		t,
		results,
	)
	if err != nil {
		t.Fatalf(
			"runHTTPServer returned error: %v",
			err,
		)
	}

	if !errors.Is(
		shutdownCtx.Err(),
		context.Canceled,
	) {
		t.Fatalf(
			"Shutdown context error = %v, want %v",
			shutdownCtx.Err(),
			context.Canceled,
		)
	}

	select {
	case <-server.closeCalls:
		t.Fatal(
			"Close should not be called",
		)
	default:
	}

	select {
	case <-shutdownRequested:
		t.Fatal(
			"shutdown callback called more than once",
		)
	default:
	}

	select {
	case <-server.shutdownCalls:
		t.Fatal(
			"Shutdown called more than once",
		)
	default:
	}
}

func TestRunHTTPServerFallsBackToClose(
	t *testing.T,
) {
	shutdownErr := errors.New(
		"shutdown failed",
	)
	closeErr := errors.New(
		"close failed",
	)

	server := newFakeHTTPServerLifecycle()
	server.shutdownErr = shutdownErr
	server.closeErr = closeErr

	appCtx, cancelApp := context.WithCancel(
		context.Background(),
	)

	shutdownRequested := make(
		chan struct{},
		1,
	)
	results := make(
		chan error,
		1,
	)

	go func() {
		results <- runHTTPServer(
			appCtx,
			server,
			func() {
				shutdownRequested <- struct{}{}
			},
		)
	}()

	waitForLifecycleStart(
		t,
		server,
	)

	cancelApp()

	select {
	case <-server.shutdownCalls:
	case <-time.After(lifecycleTestTimeout):
		t.Fatal(
			"timed out waiting for Shutdown",
		)
	}

	select {
	case <-server.closeCalls:
	case <-time.After(lifecycleTestTimeout):
		t.Fatal(
			"timed out waiting for Close",
		)
	}

	server.listenResult <- http.ErrServerClosed

	err := waitForLifecycleResult(
		t,
		results,
	)

	if !errors.Is(err, shutdownErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			shutdownErr,
		)
	}

	if !errors.Is(err, closeErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			closeErr,
		)
	}

	select {
	case <-shutdownRequested:
	case <-time.After(lifecycleTestTimeout):
		t.Fatal(
			"shutdown callback was not called",
		)
	}

	select {
	case <-server.shutdownCalls:
		t.Fatal(
			"Shutdown called more than once",
		)
	default:
	}

	select {
	case <-server.closeCalls:
		t.Fatal(
			"Close called more than once",
		)
	default:
	}
}
```

## 12. Menjalankan test

Dari root module:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w main.go main_test.go

go test . \
  -run '^TestRunHTTPServer' \
  -v

go test -race . \
  -run '^TestRunHTTPServer'

go test ./...
```

Test ini tidak memerlukan:

- port HTTP;
- signal OS;
- Docker;
- PostgreSQL;
- request jaringan.

## Hasil verifikasi

Focused lifecycle test:

```text
PASS
ok  github.com/imamsyafii-dev/bbge-practice/srv-server  0.010s
```

Race detector:

```text
PASS
ok  github.com/imamsyafii-dev/bbge-practice/srv-server  1.016s
```

Full test dari root module:

```text
ok  github.com/imamsyafii-dev/bbge-practice/srv-server  0.009s
```

Semua package lain juga lulus. Package tanpa test dilaporkan sebagai `[no test files]`.

## Ringkasan perubahan

```text
main.go
├── menambahkan httpShutdownTimeout
├── menambahkan httpServerLifecycle
├── menambahkan compile-time assertion untuk *http.Server
├── memindahkan lifecycle ke runHTTPServer
├── menambahkan graceful Shutdown dan fallback Close
├── mempertahankan beberapa error dengan errors.Join
└── menyederhanakan logging akhir di main

main_test.go
├── menambahkan fakeHTTPServerLifecycle
├── menambahkan helper sinkronisasi dengan timeout
├── menguji listen error
├── menguji graceful shutdown dan drain
└── menguji fallback Close serta joined errors
```

## Konsep yang dipelajari

- narrow interface untuk boundary lifecycle;
- dependency inversion agar `*http.Server` dapat diganti fake;
- goroutine dan buffered channel;
- hubungan happens-before melalui channel;
- pemilihan branch dengan `select`;
- graceful shutdown dengan context deadline;
- alasan memakai parent context baru untuk cleanup;
- fallback dari `Shutdown` ke `Close`;
- error wrapping dan multi-error;
- `errors.Is` terhadap wrapped/joined errors;
- pemisahan OS signal, lifecycle, dan logging;
- deterministic concurrency test tanpa sleep.

## Batas Step 31

Step ini menguji orkestrasi lifecycle, bukan implementasi internal `net/http.Server`. Fake memastikan urutan dan hasil operasi yang diminta oleh kode kita. Test ini juga belum menjalankan signal OS nyata atau membuka port karena bagian tersebut akan membuat test lebih lambat dan lebih rapuh.

## Step 32 — Preview

Step berikutnya akan menguji konfigurasi aplikasi:

- nilai default HTTP dan PostgreSQL;
- override melalui environment variable;
- parsing port dan durasi bila digunakan;
- isolasi environment antar-test;
- memastikan prefix konfigurasi practice tetap konsisten.

Tujuannya membuat perilaku `config.Load()` terdokumentasi dan dapat diverifikasi tanpa menjalankan server.
