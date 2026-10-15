# Step 25 — HTTP Handler dan Router Account

Pada Step 23–24, application layer sudah menyediakan `UseCase` account dan unit test-nya. Step 25 menambahkan batas HTTP di bagian terluar aplikasi. Batas ini menerima request HTTP, mengubahnya menjadi input application, memanggil use case, lalu mengubah result atau error menjadi response HTTP.

Tujuan Step 25:

1. membuat endpoint `POST /accounts` dan `GET /accounts/{userID}`;
2. menjaga handler hanya bergantung pada interface use case yang kecil;
3. memisahkan bentuk data HTTP dari model generated store;
4. membatasi dan memvalidasi JSON request secara ketat;
5. menerjemahkan error application/domain menjadi status dan error JSON yang stabil;
6. menjaga detail error internal agar tidak bocor kepada client;
7. menyiapkan handler agar dapat diuji tanpa server dan PostgreSQL pada Step 26.

Step ini membuka kelompok baru `07-http-router` karena alur belajar sudah berpindah dari application layer ke transport layer. Kelompok sebelumnya membangun dependency dari dalam ke luar:

```text
PostgreSQL
    |
    v
generated sqlc store
    |
    v
domain Service
    |
    v
application UseCase
```

Kelompok HTTP menambahkan lapisan terluar:

```text
HTTP request
    |
    v
AccountHandler
    |
    v
application UseCase
    |
    v
domain Service
    |
    v
generated sqlc store
    |
    v
PostgreSQL
```

Handler tidak memanggil domain service atau store secara langsung. Dengan batas ini, perubahan transport HTTP tidak harus mengubah aturan domain, dan perubahan penyimpanan data tidak harus mengubah kontrak HTTP.

## Perbandingan dengan BBGE production

Latihan ini menggunakan standard library Go:

```text
net/http + encoding/json
```

BBGE production menggunakan pola transport yang lebih besar, termasuk Gin dan protobuf pada bagian-bagian yang sesuai dengan kontrak production. Step ini tidak bermaksud menyalin framework production secara penuh. Standard library dipilih agar mekanisme dasar terlihat jelas: pencocokan route, pembacaan body, decoding JSON, pemanggilan use case, pemetaan error, dan penulisan response.

Prinsip arsitekturnya tetap sama:

```text
transport -> application -> domain -> persistence
```

Setelah konsep dasarnya dipahami, framework atau format transport dapat diganti tanpa memindahkan aturan bisnis ke handler.

## File yang dibuat

Buat file:

```text
srv-server/internal/router/account.go
```

## Isi final `account.go`

```go
package router

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	accountapp "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account"
	accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
)

const maxRequestBodyBytes = 1 << 20

var errMultipleJSONValues = errors.New(
	"request body must contain one JSON value",
)

type AccountUseCase interface {
	Create(
		ctx context.Context,
		input accountapp.CreateInput,
	) (store.Account, error)

	GetByUserID(
		ctx context.Context,
		userID string,
	) (store.Account, error)
}

var _ AccountUseCase = (*accountapp.UseCase)(nil)

type AccountHandler struct {
	useCase AccountUseCase
}

type createAccountRequest struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}

type accountResponse struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Status    int16     `json:"status"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type accountResponseEnvelope struct {
	Account accountResponse `json:"account"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewAccountHandler(
	accountUseCase AccountUseCase,
) *AccountHandler {
	return &AccountHandler{
		useCase: accountUseCase,
	}
}

func (h *AccountHandler) Register(
	mux *http.ServeMux,
) {
	mux.HandleFunc(
		"POST /accounts",
		h.create,
	)

	mux.HandleFunc(
		"GET /accounts/{userID}",
		h.getByUserID,
	)
}

func (h *AccountHandler) create(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h == nil || h.useCase == nil {
		writeAPIError(
			w,
			http.StatusServiceUnavailable,
			"account_unavailable",
			"account service unavailable",
		)
		return
	}

	if !hasJSONContentType(r) {
		writeAPIError(
			w,
			http.StatusUnsupportedMediaType,
			"unsupported_media_type",
			"Content-Type must be application/json",
		)
		return
	}

	var request createAccountRequest

	if err := decodeJSON(
		w,
		r,
		&request,
	); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeAPIError(
				w,
				http.StatusRequestEntityTooLarge,
				"request_too_large",
				"request body is too large",
			)
			return
		}

		writeAPIError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"invalid request body",
		)
		return
	}

	account, err := h.useCase.Create(
		r.Context(),
		accountapp.CreateInput{
			UserID: request.UserID,
			Name:   request.Name,
		},
	)
	if err != nil {
		writeAccountError(
			w,
			err,
		)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		accountResponseEnvelope{
			Account: newAccountResponse(account),
		},
	)
}

func (h *AccountHandler) getByUserID(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h == nil || h.useCase == nil {
		writeAPIError(
			w,
			http.StatusServiceUnavailable,
			"account_unavailable",
			"account service unavailable",
		)
		return
	}

	account, err := h.useCase.GetByUserID(
		r.Context(),
		r.PathValue("userID"),
	)
	if err != nil {
		writeAccountError(
			w,
			err,
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		accountResponseEnvelope{
			Account: newAccountResponse(account),
		},
	)
}

func decodeJSON(
	w http.ResponseWriter,
	r *http.Request,
	destination any,
) error {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxRequestBodyBytes,
	)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}

	var extra any

	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errMultipleJSONValues
	}

	return err
}

func hasJSONContentType(
	r *http.Request,
) bool {
	mediaType, _, err := mime.ParseMediaType(
		r.Header.Get("Content-Type"),
	)

	return err == nil &&
		mediaType == "application/json"
}

func newAccountResponse(
	account store.Account,
) accountResponse {
	return accountResponse{
		ID:        account.ID,
		UserID:    account.UserID,
		Name:      account.Name,
		Status:    account.Status,
		Version:   account.Version,
		CreatedAt: account.CreatedAt.Time,
		UpdatedAt: account.UpdatedAt.Time,
	}
}

func writeAccountError(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(
		err,
		accountdomain.ErrUserIDRequired,
	):
		writeAPIError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			accountdomain.ErrUserIDRequired.Error(),
		)

	case errors.Is(
		err,
		accountdomain.ErrNameRequired,
	):
		writeAPIError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			accountdomain.ErrNameRequired.Error(),
		)

	case errors.Is(
		err,
		accountdomain.ErrAccountNotFound,
	):
		writeAPIError(
			w,
			http.StatusNotFound,
			"account_not_found",
			accountdomain.ErrAccountNotFound.Error(),
		)

	case errors.Is(
		err,
		accountapp.ErrServiceNotConfigured,
	), errors.Is(
		err,
		accountdomain.ErrStoreNotConfigured,
	):
		writeAPIError(
			w,
			http.StatusServiceUnavailable,
			"account_unavailable",
			"account service unavailable",
		)

	default:
		writeAPIError(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"internal server error",
		)
	}
}

func writeAPIError(
	w http.ResponseWriter,
	status int,
	code string,
	message string,
) {
	writeJSON(
		w,
		status,
		errorResponse{
			Error: apiError{
				Code:    code,
				Message: message,
			},
		},
	)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}
```

## Penjelasan setiap import

```go
import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"
)
```

- `context` dipakai dalam signature `AccountUseCase`. Context dari request diteruskan ke application agar pembatalan request dan deadline dapat ikut mengalir.
- `encoding/json` melakukan decode request dan encode response.
- `errors` membuat sentinel untuk multiple JSON values, memeriksa error chain dengan `errors.Is`, dan mengenali tipe `*http.MaxBytesError` dengan `errors.As`.
- `io` menyediakan `io.EOF`, yaitu hasil yang diharapkan ketika tidak ada nilai JSON kedua.
- `mime` mengurai `Content-Type` secara benar, termasuk parameter seperti `charset=utf-8`.
- `net/http` menyediakan handler, request, response writer, ServeMux, status code, pembatas body, dan router pattern.
- `time` digunakan oleh DTO response. Transport mengekspos timestamp sebagai `time.Time`, bukan tipe PostgreSQL milik pgx.

Tiga import internal memiliki tanggung jawab berbeda:

- `accountapp` menyediakan input application, use case konkret, dan error konfigurasi application;
- `accountdomain` menyediakan error domain yang perlu diterjemahkan menjadi kontrak HTTP;
- `store` menyediakan result account yang saat ini dikembalikan oleh application/domain.

## Batas ukuran body: 1 MiB

```go
const maxRequestBodyBytes = 1 << 20
```

Operasi bit shift tersebut menghasilkan `1,048,576` byte atau 1 MiB. Batas diterapkan sebelum decoder membaca body:

```go
r.Body = http.MaxBytesReader(
	w,
	r.Body,
	maxRequestBodyBytes,
)
```

Tanpa batas, client dapat mengirim body sangat besar dan memaksa server memakai resource berlebihan. Ketika batas dilewati, pembacaan menghasilkan `*http.MaxBytesError`. Handler mengenalinya menggunakan `errors.As` dan mengembalikan `413 Request Entity Too Large`.

## Decoder JSON yang ketat

Decoder memiliki dua perlindungan utama.

Pertama, field yang tidak dikenal ditolak:

```go
decoder.DisallowUnknownFields()
```

Contoh berikut ditolak karena `email` tidak ada di `createAccountRequest`:

```json
{
  "user_id": "practice-user-025",
  "name": "Practice User",
  "email": "unused@example.com"
}
```

Hal ini juga sesuai dengan latihan account yang tidak menggunakan email.

Kedua, body harus berisi tepat satu nilai JSON. Decode pertama membaca request. Decode kedua harus menghasilkan `io.EOF`:

```go
var extra any

err := decoder.Decode(&extra)
if errors.Is(err, io.EOF) {
	return nil
}
```

Body berikut ditolak walaupun nilai pertamanya valid:

```text
{"user_id":"practice-user-025","name":"Practice User"}
{"user_id":"practice-user-026","name":"Another User"}
```

Jika decode kedua berhasil, berarti terdapat nilai kedua dan fungsi mengembalikan `errMultipleJSONValues`. Jika decode kedua menghasilkan syntax error, error tersebut juga diteruskan. Empty body, JSON terpotong, bentuk JSON salah, unknown field, dan multiple values akhirnya menjadi `400 invalid_request`.

## Pemeriksaan Content-Type dengan parser MIME

Handler tidak membandingkan header sebagai string mentah. Ia memakai:

```go
mediaType, _, err := mime.ParseMediaType(
	r.Header.Get("Content-Type"),
)
```

Akibatnya:

- `application/json` diterima;
- `application/json; charset=utf-8` diterima karena media type utamanya tetap `application/json`;
- header kosong ditolak;
- header dengan format tidak valid ditolak;
- `text/plain` ditolak;
- vendor media type seperti `application/vnd.example+json` belum diterima oleh kontrak step ini.

Request POST dengan content type yang tidak didukung mendapat `415 Unsupported Media Type` sebelum body di-decode atau use case dipanggil.

## Interface use case yang sempit

```go
type AccountUseCase interface {
	Create(
		ctx context.Context,
		input accountapp.CreateInput,
	) (store.Account, error)

	GetByUserID(
		ctx context.Context,
		userID string,
	) (store.Account, error)
}
```

Router hanya meminta dua kemampuan yang benar-benar digunakannya. Router tidak perlu bergantung pada semua kemungkinan workflow application yang akan ditambahkan nanti.

```go
var _ AccountUseCase = (*accountapp.UseCase)(nil)
```

Baris ini adalah compile-time assertion. Ia tidak membuat object dan tidak menjalankan method. Compiler hanya memastikan bahwa method set `*accountapp.UseCase` memenuhi `AccountUseCase`. Jika signature use case berubah tetapi interface router tidak diperbarui, proses compile langsung gagal.

Interface kecil juga memungkinkan Step 26 mengganti use case asli dengan fake tanpa database, domain service, atau store.

## Handler dan constructor

```go
type AccountHandler struct {
	useCase AccountUseCase
}
```

Handler menyimpan dependency application di dalam interface. Constructor memasukkannya:

```go
func NewAccountHandler(
	accountUseCase AccountUseCase,
) *AccountHandler
```

Constructor saat ini tetap dapat menerima `nil`. Karena itu, setiap handler memiliki guard:

```go
if h == nil || h.useCase == nil {
```

Guard melindungi dua keadaan:

1. method dipanggil melalui receiver `(*AccountHandler)(nil)`;
2. handler dibuat menggunakan `NewAccountHandler(nil)`.

Keduanya menghasilkan `503 account_unavailable`, bukan panic.

Ada satu caveat interface Go: interface yang berisi typed nil pointer tidak sama dengan `nil`.

```go
var useCase *accountapp.UseCase = nil
handler := NewAccountHandler(useCase)
```

Pada contoh tersebut `handler.useCase != nil` karena interface menyimpan informasi tipe `*accountapp.UseCase`, walaupun pointer nil. Guard handler tidak mendeteksi typed nil itu. `UseCase` latihan sendiri memiliki nil-receiver guard sehingga pemanggilannya tetap menghasilkan error konfigurasi, tetapi secara umum dependency injection sebaiknya tidak memasukkan typed nil ke interface.

## Registrasi route

```go
func (h *AccountHandler) Register(
	mux *http.ServeMux,
)
```

Method ini mendaftarkan dua pattern yang method-aware:

```text
POST /accounts
GET  /accounts/{userID}
```

Syntax method dan wildcard ini didukung oleh `http.ServeMux` modern. Method request menjadi bagian dari pencocokan route. Pattern `POST /accounts` hanya menerima POST. Ada perilaku khusus untuk GET: pattern `GET /accounts/{userID}` juga cocok dengan request HEAD, sehingga `HEAD /accounts/{userID}` tetap masuk ke `getByUserID`. Method lain yang tidak didukung pada path yang dikenal ditangani ServeMux sebagai `405 Method Not Allowed`, beserta header `Allow` yang sesuai. Handler account tidak perlu menulis pengecekan method manual.

Pada route GET, ServeMux menangkap segmen `{userID}`. Handler mengambilnya dengan:

```go
r.PathValue("userID")
```

Nil `mux` belum dilindungi dan akan panic ketika `Register` mencoba memanggil `HandleFunc`. Pada wiring nanti, caller wajib menyediakan `*http.ServeMux` yang valid.

## DTO request terpisah

```go
type createAccountRequest struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}
```

Struct ini adalah kontrak transport masuk. Handler tidak decode JSON langsung ke `accountapp.CreateInput` atau `store.CreateAccountParams`. Pemisahan ini menjaga transport bebas mengubah nama field, versi API, atau metadata HTTP tanpa menjadikan application dan persistence sebagai kontrak JSON publik.

Handler hanya memetakan nilai:

```go
accountapp.CreateInput{
	UserID: request.UserID,
	Name:   request.Name,
}
```

Handler sengaja tidak memanggil `strings.TrimSpace` dan tidak melakukan aturan bisnis `user_id`/`name`. Input, termasuk spasi, diteruskan ke use case. Domain service tetap menjadi satu tempat untuk normalisasi dan validasi.

## DTO response dan envelope

Response sukses memiliki envelope:

```go
type accountResponseEnvelope struct {
	Account accountResponse `json:"account"`
}
```

Contoh:

```json
{
  "account": {
    "id": 25,
    "user_id": "practice-user-025",
    "name": "Practice User",
    "status": 1,
    "version": 0,
    "created_at": "2026-10-15T10:00:00Z",
    "updated_at": "2026-10-15T10:00:00Z"
  }
}
```

Error juga memiliki envelope stabil:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "user id is required"
  }
}
```

`accountResponse` sengaja berbeda dari `store.Account`. Generated model store berisi tipe database seperti `pgtype.Timestamp`. Mengekspos generated model secara langsung akan mengikat API publik pada driver PostgreSQL dan bentuk hasil generator. DTO response menjaga JSON tetap sederhana serta memberi ruang untuk menyembunyikan atau mengganti field.

## Mapping timestamp

```go
CreatedAt: account.CreatedAt.Time,
UpdatedAt: account.UpdatedAt.Time,
```

`store.Account` memakai `pgtype.Timestamp`, yang memiliki `Time` dan `Valid`. Response hanya memakai `time.Time`.

Kode Step 25 tidak memeriksa `Valid`. Ini aman berdasarkan asumsi schema saat ini karena migration mendefinisikan:

```sql
created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
```

Artinya row account yang valid selalu memiliki kedua timestamp tersebut. Jika schema kelak mengizinkan `NULL`, mapping ini harus diubah agar kontrak HTTP dapat menyatakan nilai kosong secara eksplisit, misalnya menggunakan pointer atau nullable response field.

## Alur POST `/accounts`

Urutannya:

```text
cek dependency
    |
cek Content-Type
    |
batasi dan decode tepat satu JSON value
    |
map request DTO -> accountapp.CreateInput
    |
useCase.Create(request context, input)
    |
map error atau account -> JSON response
```

Pada sukses, statusnya `201 Created`. Context yang dipakai adalah `r.Context()`, sehingga pembatalan koneksi client dapat diteruskan melalui application, domain, dan database.

Kontrak POST:

| Kondisi | Status | Code |
|---|---:|---|
| Account berhasil dibuat | `201` | response sukses |
| Content-Type bukan JSON | `415` | `unsupported_media_type` |
| Body lebih dari 1 MiB | `413` | `request_too_large` |
| JSON invalid/unknown field/multiple values | `400` | `invalid_request` |
| `user_id` kosong menurut domain | `400` | `invalid_request` |
| `name` kosong menurut domain | `400` | `invalid_request` |
| Dependency tidak tersedia | `503` | `account_unavailable` |
| Error lainnya | `500` | `internal_error` |

## Alur GET `/accounts/{userID}`

Urutannya:

```text
cek dependency
    |
ambil r.PathValue("userID")
    |
useCase.GetByUserID(request context, userID)
    |
map error atau account -> JSON response
```

Pada sukses, statusnya `200 OK`. Handler tidak men-trim path value. Normalisasi dan validasi tetap dilakukan domain service.

Kontrak GET:

| Kondisi | Status | Code |
|---|---:|---|
| Account ditemukan | `200` | response sukses |
| `userID` invalid menurut domain | `400` | `invalid_request` |
| Account tidak ditemukan | `404` | `account_not_found` |
| Dependency tidak tersedia | `503` | `account_unavailable` |
| Error lainnya | `500` | `internal_error` |

Path `/accounts/` tanpa nilai `userID` tidak cocok dengan pattern `{userID}` dan tidak masuk ke handler GET.

## Pemetaan error dengan `errors.Is`

`writeAccountError` menggunakan `errors.Is`, bukan perbandingan string. Karena itu, sentinel tetap dikenali walaupun dibungkus menggunakan `%w` di layer bawah.

Mapping saat ini:

| Error | HTTP | Code | Message |
|---|---:|---|---|
| `accountdomain.ErrUserIDRequired` | `400` | `invalid_request` | `user id is required` |
| `accountdomain.ErrNameRequired` | `400` | `invalid_request` | `name is required` |
| `accountdomain.ErrAccountNotFound` | `404` | `account_not_found` | `account not found` |
| `accountapp.ErrServiceNotConfigured` | `503` | `account_unavailable` | `account service unavailable` |
| `accountdomain.ErrStoreNotConfigured` | `503` | `account_unavailable` | `account service unavailable` |
| Error lain | `500` | `internal_error` | `internal server error` |

Pesan error database tidak dikirim kepada client. Misalnya error koneksi, nama constraint, atau SQL tidak boleh muncul di response. Response generik menjaga detail internal dan data infrastruktur tidak bocor. Logging internal belum ditambahkan dalam step ini.

Ada keterbatasan yang disengaja: database memiliki unique constraint pada `account.user_id`, tetapi domain belum memiliki sentinel seperti `ErrAccountAlreadyExists`. Akibatnya, percobaan membuat `user_id` duplikat saat ini jatuh ke default dan menghasilkan `500 internal_error`. Pada step lanjutan, error PostgreSQL unique violation dapat diterjemahkan di layer yang tepat menjadi error domain, lalu dipetakan misalnya ke `409 Conflict`.

## Helper penulisan JSON

`writeAPIError` membuat envelope error lalu memakai helper yang sama dengan response sukses:

```go
func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
)
```

Helper selalu memasang:

```text
Content-Type: application/json
```

kemudian menulis status dan menjalankan encoder. `Encoder.Encode` juga menambahkan newline di akhir JSON; itu valid dan normal.

Keterbatasannya, error dari encoder saat ini diabaikan:

```go
_ = json.NewEncoder(w).Encode(value)
```

Selain itu, status sudah dikirim oleh `WriteHeader` sebelum encode dilakukan. Jika encoding atau penulisan koneksi gagal, handler tidak dapat mengganti response menjadi status lain karena header mungkin sudah terkirim. Untuk DTO sederhana pada step ini risiko encoding sangat kecil, tetapi production biasanya mencatat kegagalan write/encode atau melakukan encoding ke buffer sebelum menulis header jika membutuhkan jaminan lebih kuat.

## Kontrak lengkap response

Semua response yang ditulis handler account memakai `Content-Type: application/json`.

Success create:

```http
HTTP/1.1 201 Created
Content-Type: application/json
```

Success get:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

Invalid body:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "invalid request body"
  }
}
```

Request terlalu besar:

```json
{
  "error": {
    "code": "request_too_large",
    "message": "request body is too large"
  }
}
```

Content type salah:

```json
{
  "error": {
    "code": "unsupported_media_type",
    "message": "Content-Type must be application/json"
  }
}
```

Account tidak ditemukan:

```json
{
  "error": {
    "code": "account_not_found",
    "message": "account not found"
  }
}
```

Dependency tidak tersedia:

```json
{
  "error": {
    "code": "account_unavailable",
    "message": "account service unavailable"
  }
}
```

Error tak dikenal:

```json
{
  "error": {
    "code": "internal_error",
    "message": "internal server error"
  }
}
```

## Format dan verifikasi

Dari root server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w internal/router/account.go
go test ./internal/router
go test ./...
```

Pada Step 25 belum ada `_test.go` di package router, sehingga hasil compile package router dapat terlihat seperti:

```text
?    github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router    [no test files]
```

`[no test files]` bukan kegagalan. Perintah tetap mengompilasi package dan memvalidasi source Go, import, serta kecocokan interface. Compile saja belum memvalidasi string route. `http.ServeMux` baru mengurai dan memvalidasi pattern ketika `AccountHandler.Register` memanggil `HandleFunc`; pattern yang malformed atau saling konflik dapat panic pada saat registrasi. Test router pada Step 26 akan benar-benar memanggil `Register`, sehingga registrasi route ikut diuji. Verifikasi Step 25 menunjukkan compile package router dan seluruh `go test ./...` lulus.

Tidak perlu menjalankan:

- Docker;
- PostgreSQL;
- migration;
- server HTTP;
- `PRACTICE_RUN_POSTGRES_TESTS=1`.

Integration test PostgreSQL dari Step 20 tetap dilewati pada test biasa jika flag tersebut tidak disetel.

## File yang berubah dan tidak berubah

File yang dibuat pada step ini:

```text
srv-server/internal/router/account.go
learning-steps/01-foundation/07-http-router/step-25-account-http-handler.md
```

File yang tidak diubah:

```text
srv-server/main.go
srv-server/internal/app/account/usecase.go
srv-server/internal/domain/account/service.go
srv-server/internal/store/*
srv-server/db/queries/*
srv-server/db/migrations/*
srv-server/go.mod
srv-server/go.sum
```

Karena `main.go` belum melakukan wiring, endpoint account belum tersedia pada server yang dijalankan. Step ini baru membangun komponen handler dan memastikan komponen tersebut dapat dikompilasi.

## Ringkasan tanggung jawab

```text
AccountHandler
  - memahami HTTP, MIME, JSON, route, status code
  - membatasi request body
  - memetakan DTO transport
  - memetakan error ke kontrak HTTP

Application UseCase
  - menjadi batas workflow aplikasi
  - meneruskan pekerjaan ke domain service

Domain Service
  - normalisasi dan validasi aturan account
  - menerjemahkan kondisi persistence ke error domain

Store
  - menjalankan query PostgreSQL
```

## Step berikutnya

Step 26 membuat unit test router menggunakan `httptest` dan fake `AccountUseCase`. Test akan membuktikan kontrak POST/GET, strict JSON decoder, batas 1 MiB, status/error mapping, path value, context, DTO response, nil dependency, serta memastikan method fake yang salah tidak dipanggil. Test tersebut tetap tidak memerlukan server nyata, Docker, atau PostgreSQL.

Setelah handler dan test lulus, Step 27 akan melakukan wiring di `main.go`:

```text
pgx pool
  -> store.New(pool)
  -> account domain service
  -> account application use case
  -> account HTTP handler
  -> http.ServeMux
```

Barulah endpoint account tersedia saat practice server dijalankan.
