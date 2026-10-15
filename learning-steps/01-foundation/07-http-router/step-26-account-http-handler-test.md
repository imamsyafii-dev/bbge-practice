# Step 26 — Test HTTP account handler dengan `httptest`

Pada Step 25 kita sudah membuat HTTP handler dan route untuk account. Pada step ini kita menguji kontrak HTTP tersebut tanpa menjalankan server sungguhan dan tanpa mengakses PostgreSQL.

## Tujuan

Setelah menyelesaikan step ini, kita dapat memastikan bahwa:

- request HTTP diteruskan ke use case yang benar;
- context dan input diteruskan tanpa berubah;
- response sukses mempunyai status, header, dan JSON yang tepat;
- request dengan `Content-Type` atau JSON yang tidak valid ditolak;
- body lebih dari 1 MiB ditolak dengan status `413`;
- error domain dan application dipetakan ke response HTTP yang tepat;
- pesan error internal tidak bocor ke client;
- dependency `nil`, method yang salah, dan route yang tidak dikenal ditangani dengan aman;
- perilaku khusus `HEAD` pada `http.ServeMux` dipahami.

Kontrak account latihan ini hanya memakai `user_id` dan `name`. Field `email` tidak menjadi bagian dari request account.

## Prasyarat

Sebelum memulai, Step 25 harus sudah selesai. File berikut harus sudah ada:

```text
srv-server/internal/router/account.go
srv-server/internal/app/account/usecase.go
srv-server/internal/domain/account/service.go
```

## File yang dibuat

```text
srv-server/internal/router/account_test.go
```

## Isi lengkap `internal/router/account_test.go`

```go
package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	accountapp "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account"
	accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
)

const expectedAccountResponseBody = `{"account":{"id":26,"user_id":"practice-user-026","name":"Practice User","status":1,"version":0,"created_at":"2026-10-15T10:00:00Z","updated_at":"2026-10-15T10:05:00Z"}}` + "\n"

type routerTestContextKey struct{}

type fakeAccountUseCase struct {
	createFunc func(
		context.Context,
		accountapp.CreateInput,
	) (store.Account, error)

	getByUserIDFunc func(
		context.Context,
		string,
	) (store.Account, error)

	createCalls      int
	getByUserIDCalls int
}

var _ AccountUseCase = (*fakeAccountUseCase)(nil)

func (f *fakeAccountUseCase) Create(
	ctx context.Context,
	input accountapp.CreateInput,
) (store.Account, error) {
	f.createCalls++

	if f.createFunc == nil {
		panic("unexpected Create call")
	}

	return f.createFunc(
		ctx,
		input,
	)
}

func (f *fakeAccountUseCase) GetByUserID(
	ctx context.Context,
	userID string,
) (store.Account, error) {
	f.getByUserIDCalls++

	if f.getByUserIDFunc == nil {
		panic("unexpected GetByUserID call")
	}

	return f.getByUserIDFunc(
		ctx,
		userID,
	)
}

func newAccountTestMux(
	t *testing.T,
	useCase AccountUseCase,
) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()

	NewAccountHandler(
		useCase,
	).Register(mux)

	return mux
}

func serveAccountRequest(
	t *testing.T,
	mux *http.ServeMux,
	method string,
	target string,
	body string,
	contentType string,
	ctx context.Context,
) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()

	request := httptest.NewRequest(
		method,
		target,
		strings.NewReader(body),
	)

	if contentType != "" {
		request.Header.Set(
			"Content-Type",
			contentType,
		)
	}

	if ctx != nil {
		request = request.WithContext(ctx)
	}

	recorder := httptest.NewRecorder()

	mux.ServeHTTP(
		recorder,
		request,
	)

	return request, recorder
}

func assertAccountJSONResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantBody string,
) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			wantStatus,
			recorder.Body.String(),
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

	if body := recorder.Body.String(); body != wantBody {
		t.Fatalf(
			"body = %q, want %q",
			body,
			wantBody,
		)
	}
}

func accountTestResult() store.Account {
	return store.Account{
		ID:      26,
		UserID:  "practice-user-026",
		Name:    "Practice User",
		Status:  1,
		Version: 0,
		CreatedAt: pgtype.Timestamp{
			Time: time.Date(
				2026,
				time.October,
				15,
				10,
				0,
				0,
				0,
				time.UTC,
			),
			Valid: true,
		},
		UpdatedAt: pgtype.Timestamp{
			Time: time.Date(
				2026,
				time.October,
				15,
				10,
				5,
				0,
				0,
				time.UTC,
			),
			Valid: true,
		},
	}
}

func TestAccountHandlerCreateSuccess(
	t *testing.T,
) {
	ctx := context.WithValue(
		context.Background(),
		routerTestContextKey{},
		"create",
	)

	expectedInput := accountapp.CreateInput{
		UserID: "  practice-user-026  ",
		Name:   "  Practice User  ",
	}

	fake := &fakeAccountUseCase{
		createFunc: func(
			gotCtx context.Context,
			input accountapp.CreateInput,
		) (store.Account, error) {
			if gotCtx != ctx {
				t.Fatal(
					"Create context was not forwarded unchanged",
				)
			}

			if input != expectedInput {
				t.Fatalf(
					"Create input = %+v, want %+v",
					input,
					expectedInput,
				)
			}

			return accountTestResult(), nil
		},
	}

	mux := newAccountTestMux(
		t,
		fake,
	)

	request, recorder := serveAccountRequest(
		t,
		mux,
		http.MethodPost,
		"/accounts",
		`{"user_id":"  practice-user-026  ","name":"  Practice User  "}`,
		"application/json; charset=utf-8",
		ctx,
	)

	if request.Pattern != "POST /accounts" {
		t.Fatalf(
			"matched pattern = %q, want %q",
			request.Pattern,
			"POST /accounts",
		)
	}

	if fake.createCalls != 1 {
		t.Fatalf(
			"Create calls = %d, want 1",
			fake.createCalls,
		)
	}

	if fake.getByUserIDCalls != 0 {
		t.Fatalf(
			"GetByUserID calls = %d, want 0",
			fake.getByUserIDCalls,
		)
	}

	assertAccountJSONResponse(
		t,
		recorder,
		http.StatusCreated,
		expectedAccountResponseBody,
	)
}

func TestAccountHandlerGetByUserIDSuccess(
	t *testing.T,
) {
	ctx := context.WithValue(
		context.Background(),
		routerTestContextKey{},
		"get",
	)

	fake := &fakeAccountUseCase{
		getByUserIDFunc: func(
			gotCtx context.Context,
			userID string,
		) (store.Account, error) {
			if gotCtx != ctx {
				t.Fatal(
					"GetByUserID context was not forwarded unchanged",
				)
			}

			if userID != "practice-user-026" {
				t.Fatalf(
					"userID = %q, want practice-user-026",
					userID,
				)
			}

			return accountTestResult(), nil
		},
	}

	mux := newAccountTestMux(
		t,
		fake,
	)

	request, recorder := serveAccountRequest(
		t,
		mux,
		http.MethodGet,
		"/accounts/practice-user-026",
		"",
		"",
		ctx,
	)

	if request.Pattern != "GET /accounts/{userID}" {
		t.Fatalf(
			"matched pattern = %q, want %q",
			request.Pattern,
			"GET /accounts/{userID}",
		)
	}

	if fake.getByUserIDCalls != 1 {
		t.Fatalf(
			"GetByUserID calls = %d, want 1",
			fake.getByUserIDCalls,
		)
	}

	if fake.createCalls != 0 {
		t.Fatalf(
			"Create calls = %d, want 0",
			fake.createCalls,
		)
	}

	assertAccountJSONResponse(
		t,
		recorder,
		http.StatusOK,
		expectedAccountResponseBody,
	)
}

func TestAccountHandlerRejectsInvalidCreateRequests(
	t *testing.T,
) {
	unsupportedMediaTypeBody := `{"error":{"code":"unsupported_media_type","message":"Content-Type must be application/json"}}` + "\n"
	invalidRequestBody := `{"error":{"code":"invalid_request","message":"invalid request body"}}` + "\n"

	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
		wantBody    string
	}{
		{
			name:        "missing content type",
			body:        `{"user_id":"practice-user-026","name":"Practice User"}`,
			contentType: "",
			wantStatus:  http.StatusUnsupportedMediaType,
			wantBody:    unsupportedMediaTypeBody,
		},
		{
			name:        "wrong content type",
			body:        `{"user_id":"practice-user-026","name":"Practice User"}`,
			contentType: "text/plain",
			wantStatus:  http.StatusUnsupportedMediaType,
			wantBody:    unsupportedMediaTypeBody,
		},
		{
			name:        "malformed content type",
			body:        `{"user_id":"practice-user-026","name":"Practice User"}`,
			contentType: "application/json; charset",
			wantStatus:  http.StatusUnsupportedMediaType,
			wantBody:    unsupportedMediaTypeBody,
		},
		{
			name:        "empty body",
			body:        "",
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantBody:    invalidRequestBody,
		},
		{
			name:        "malformed JSON",
			body:        `{"user_id":`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantBody:    invalidRequestBody,
		},
		{
			name:        "JSON array",
			body:        `[]`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantBody:    invalidRequestBody,
		},
		{
			name:        "unknown field",
			body:        `{"user_id":"practice-user-026","name":"Practice User","email":"practice@example.com"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantBody:    invalidRequestBody,
		},
		{
			name:        "multiple JSON values",
			body:        `{"user_id":"practice-user-026","name":"Practice User"} {"user_id":"practice-user-027","name":"Other User"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantBody:    invalidRequestBody,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeAccountUseCase{}
			mux := newAccountTestMux(
				t,
				fake,
			)

			_, recorder := serveAccountRequest(
				t,
				mux,
				http.MethodPost,
				"/accounts",
				test.body,
				test.contentType,
				nil,
			)

			if fake.createCalls != 0 {
				t.Fatalf(
					"Create calls = %d, want 0",
					fake.createCalls,
				)
			}

			if fake.getByUserIDCalls != 0 {
				t.Fatalf(
					"GetByUserID calls = %d, want 0",
					fake.getByUserIDCalls,
				)
			}

			assertAccountJSONResponse(
				t,
				recorder,
				test.wantStatus,
				test.wantBody,
			)
		})
	}
}

func TestAccountHandlerRejectsOversizedBody(
	t *testing.T,
) {
	fake := &fakeAccountUseCase{}
	mux := newAccountTestMux(
		t,
		fake,
	)

	body := `{"user_id":"` +
		strings.Repeat(
			"x",
			maxRequestBodyBytes,
		) +
		`","name":"Practice User"}`

	_, recorder := serveAccountRequest(
		t,
		mux,
		http.MethodPost,
		"/accounts",
		body,
		"application/json",
		nil,
	)

	if fake.createCalls != 0 {
		t.Fatalf(
			"Create calls = %d, want 0",
			fake.createCalls,
		)
	}

	assertAccountJSONResponse(
		t,
		recorder,
		http.StatusRequestEntityTooLarge,
		`{"error":{"code":"request_too_large","message":"request body is too large"}}`+"\n",
	)
}

func TestAccountHandlerMapsErrors(
	t *testing.T,
) {
	internalErr := errors.New(
		"database password leaked",
	)

	tests := []struct {
		name          string
		method        string
		target        string
		body          string
		contentType   string
		useCaseErr    error
		wantStatus    int
		wantBody      string
		wantCreate    int
		wantGet       int
		hiddenMessage string
	}{
		{
			name:        "user id required",
			method:      http.MethodPost,
			target:      "/accounts",
			body:        `{"user_id":" ","name":"Practice User"}`,
			contentType: "application/json",
			useCaseErr:  accountdomain.ErrUserIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantBody: `{"error":{"code":"invalid_request","message":"user id is required"}}` +
				"\n",
			wantCreate: 1,
		},
		{
			name:        "name required",
			method:      http.MethodPost,
			target:      "/accounts",
			body:        `{"user_id":"practice-user-026","name":" "}`,
			contentType: "application/json",
			useCaseErr:  accountdomain.ErrNameRequired,
			wantStatus:  http.StatusBadRequest,
			wantBody: `{"error":{"code":"invalid_request","message":"name is required"}}` +
				"\n",
			wantCreate: 1,
		},
		{
			name:       "account not found",
			method:     http.MethodGet,
			target:     "/accounts/practice-user-missing",
			useCaseErr: accountdomain.ErrAccountNotFound,
			wantStatus: http.StatusNotFound,
			wantBody: `{"error":{"code":"account_not_found","message":"account not found"}}` +
				"\n",
			wantGet: 1,
		},
		{
			name:        "application service unavailable",
			method:      http.MethodPost,
			target:      "/accounts",
			body:        `{"user_id":"practice-user-026","name":"Practice User"}`,
			contentType: "application/json",
			useCaseErr:  accountapp.ErrServiceNotConfigured,
			wantStatus:  http.StatusServiceUnavailable,
			wantBody: `{"error":{"code":"account_unavailable","message":"account service unavailable"}}` +
				"\n",
			wantCreate: 1,
		},
		{
			name:       "domain store unavailable",
			method:     http.MethodGet,
			target:     "/accounts/practice-user-026",
			useCaseErr: accountdomain.ErrStoreNotConfigured,
			wantStatus: http.StatusServiceUnavailable,
			wantBody: `{"error":{"code":"account_unavailable","message":"account service unavailable"}}` +
				"\n",
			wantGet: 1,
		},
		{
			name:          "internal error is hidden",
			method:        http.MethodGet,
			target:        "/accounts/practice-user-026",
			useCaseErr:    internalErr,
			wantStatus:    http.StatusInternalServerError,
			wantBody:      `{"error":{"code":"internal_error","message":"internal server error"}}` + "\n",
			wantGet:       1,
			hiddenMessage: internalErr.Error(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeAccountUseCase{
				createFunc: func(
					context.Context,
					accountapp.CreateInput,
				) (store.Account, error) {
					return store.Account{}, fmt.Errorf(
						"create test error: %w",
						test.useCaseErr,
					)
				},
				getByUserIDFunc: func(
					context.Context,
					string,
				) (store.Account, error) {
					return store.Account{}, fmt.Errorf(
						"get test error: %w",
						test.useCaseErr,
					)
				},
			}

			mux := newAccountTestMux(
				t,
				fake,
			)

			_, recorder := serveAccountRequest(
				t,
				mux,
				test.method,
				test.target,
				test.body,
				test.contentType,
				nil,
			)

			if fake.createCalls != test.wantCreate {
				t.Fatalf(
					"Create calls = %d, want %d",
					fake.createCalls,
					test.wantCreate,
				)
			}

			if fake.getByUserIDCalls != test.wantGet {
				t.Fatalf(
					"GetByUserID calls = %d, want %d",
					fake.getByUserIDCalls,
					test.wantGet,
				)
			}

			assertAccountJSONResponse(
				t,
				recorder,
				test.wantStatus,
				test.wantBody,
			)

			if test.hiddenMessage != "" &&
				strings.Contains(
					recorder.Body.String(),
					test.hiddenMessage,
				) {
				t.Fatalf(
					"response leaked internal message %q",
					test.hiddenMessage,
				)
			}
		})
	}
}

func TestAccountHandlerUnavailable(
	t *testing.T,
) {
	tests := []struct {
		name     string
		register func(*http.ServeMux)
	}{
		{
			name: "nil dependency",
			register: func(mux *http.ServeMux) {
				NewAccountHandler(nil).Register(mux)
			},
		},
		{
			name: "nil handler receiver",
			register: func(mux *http.ServeMux) {
				var handler *AccountHandler

				handler.Register(mux)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			test.register(mux)

			_, recorder := serveAccountRequest(
				t,
				mux,
				http.MethodGet,
				"/accounts/practice-user-026",
				"",
				"",
				nil,
			)

			assertAccountJSONResponse(
				t,
				recorder,
				http.StatusServiceUnavailable,
				`{"error":{"code":"account_unavailable","message":"account service unavailable"}}`+"\n",
			)
		})
	}
}

func TestAccountHandlerRouting(
	t *testing.T,
) {
	t.Run("HEAD uses GET route", func(t *testing.T) {
		fake := &fakeAccountUseCase{
			getByUserIDFunc: func(
				_ context.Context,
				userID string,
			) (store.Account, error) {
				if userID != "practice-user-026" {
					t.Fatalf(
						"userID = %q, want practice-user-026",
						userID,
					)
				}

				return accountTestResult(), nil
			},
		}

		mux := newAccountTestMux(
			t,
			fake,
		)

		request, recorder := serveAccountRequest(
			t,
			mux,
			http.MethodHead,
			"/accounts/practice-user-026",
			"",
			"",
			nil,
		)

		if request.Pattern != "GET /accounts/{userID}" {
			t.Fatalf(
				"matched pattern = %q, want GET pattern",
				request.Pattern,
			)
		}

		if fake.getByUserIDCalls != 1 {
			t.Fatalf(
				"GetByUserID calls = %d, want 1",
				fake.getByUserIDCalls,
			)
		}

		// ResponseRecorder records the body written by the handler.
		// A real net/http server suppresses the body on the wire for HEAD.
		assertAccountJSONResponse(
			t,
			recorder,
			http.StatusOK,
			expectedAccountResponseBody,
		)
	})

	t.Run("PUT accounts returns method not allowed", func(t *testing.T) {
		fake := &fakeAccountUseCase{}
		mux := newAccountTestMux(
			t,
			fake,
		)

		_, recorder := serveAccountRequest(
			t,
			mux,
			http.MethodPut,
			"/accounts",
			"",
			"",
			nil,
		)

		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf(
				"status = %d, want %d",
				recorder.Code,
				http.StatusMethodNotAllowed,
			)
		}

		if allow := recorder.Header().Get(
			"Allow",
		); allow != http.MethodPost {
			t.Fatalf(
				"Allow = %q, want %q",
				allow,
				http.MethodPost,
			)
		}

		if fake.createCalls != 0 ||
			fake.getByUserIDCalls != 0 {
			t.Fatal(
				"use case should not be called",
			)
		}
	})

	t.Run("POST account detail returns method not allowed", func(t *testing.T) {
		fake := &fakeAccountUseCase{}
		mux := newAccountTestMux(
			t,
			fake,
		)

		_, recorder := serveAccountRequest(
			t,
			mux,
			http.MethodPost,
			"/accounts/practice-user-026",
			"",
			"",
			nil,
		)

		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf(
				"status = %d, want %d",
				recorder.Code,
				http.StatusMethodNotAllowed,
			)
		}

		if allow := recorder.Header().Get(
			"Allow",
		); allow != "GET, HEAD" {
			t.Fatalf(
				"Allow = %q, want %q",
				allow,
				"GET, HEAD",
			)
		}
	})

	t.Run("unknown route returns not found", func(t *testing.T) {
		fake := &fakeAccountUseCase{}
		mux := newAccountTestMux(
			t,
			fake,
		)

		_, recorder := serveAccountRequest(
			t,
			mux,
			http.MethodGet,
			"/missing",
			"",
			"",
			nil,
		)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf(
				"status = %d, want %d",
				recorder.Code,
				http.StatusNotFound,
			)
		}

		if fake.createCalls != 0 ||
			fake.getByUserIDCalls != 0 {
			t.Fatal(
				"use case should not be called",
			)
		}
	})
}
```

## Cara kerja fake dan alur pemanggilan

`fakeAccountUseCase` adalah pengganti use case asli khusus untuk unit test. Fake ini hanya mengimplementasikan dua method yang dibutuhkan handler, yaitu `Create` dan `GetByUserID`.

```text
HTTP request
    ↓
http.ServeMux memilih route
    ↓
AccountHandler memvalidasi dan menerjemahkan request
    ↓
fakeAccountUseCase menerima context dan input
    ↓
fake mengembalikan store.Account atau error yang ditentukan test
    ↓
AccountHandler menerjemahkan hasil menjadi HTTP response
```

Baris berikut merupakan compile-time interface assertion:

```go
var _ AccountUseCase = (*fakeAccountUseCase)(nil)
```

Artinya compiler memastikan `*fakeAccountUseCase` mempunyai semua method pada `AccountUseCase`. Nilai `nil` tidak dijalankan; ia hanya dipakai untuk pemeriksaan tipe saat kompilasi. Jika signature fake tidak cocok, test bahkan tidak akan berhasil dikompilasi.

Field `createFunc` dan `getByUserIDFunc` membuat perilaku fake dapat diatur berbeda untuk setiap test. Counter `createCalls` dan `getByUserIDCalls` membuktikan method yang tepat dipanggil sejumlah yang diharapkan. Jika handler memanggil method yang tidak disiapkan, fake melakukan `panic("unexpected ... call")`, sehingga kesalahan alur segera terlihat.

## Helper test

Ada empat bagian pendukung utama:

- `newAccountTestMux` membuat `http.ServeMux`, membuat handler, lalu mendaftarkan route yang sesungguhnya. Karena test melewati mux, test juga memeriksa proses pencocokan route, bukan hanya memanggil method handler secara langsung.
- `serveAccountRequest` membuat request melalui `httptest.NewRequest`, memasang `Content-Type` dan context bila ada, lalu merekam response dengan `httptest.NewRecorder`.
- `assertAccountJSONResponse` membandingkan status, `Content-Type: application/json`, dan body secara tepat.
- `accountTestResult` menyediakan nilai account stabil, termasuk timestamp UTC, agar JSON hasil test selalu deterministik.

Pemanggilan `t.Helper()` membuat lokasi kegagalan menunjuk ke baris test yang memakai helper, bukan ke isi helper.

## Test response sukses

### `TestAccountHandlerCreateSuccess`

Test ini mengirim `POST /accounts` dengan `Content-Type: application/json; charset=utf-8`. Parameter MIME seperti `charset` tetap sah karena media type dasarnya adalah `application/json`.

Test membuktikan bahwa:

- route yang cocok adalah `POST /accounts`;
- context diteruskan sebagai object yang sama;
- spasi pada input belum diubah oleh layer HTTP dan diteruskan ke application use case;
- `Create` dipanggil tepat satu kali;
- `GetByUserID` tidak dipanggil;
- status response adalah `201 Created`;
- JSON account sama persis dengan kontrak.

Normalisasi seperti `strings.TrimSpace` merupakan tanggung jawab domain service, bukan HTTP handler.

### `TestAccountHandlerGetByUserIDSuccess`

Test ini mengirim `GET /accounts/practice-user-026`. Nilai path `practice-user-026` harus diteruskan ke `GetByUserID`, context tidak boleh diganti, dan hasilnya harus menjadi `200 OK` dengan JSON account yang tepat.

Setelah `ServeMux` menemukan route, Go mengisi `request.Pattern`. Karena itu test dapat memeriksa bahwa pattern yang dipilih adalah:

```text
GET /accounts/{userID}
```

## Validasi `Content-Type` dan JSON ketat

`TestAccountHandlerRejectsInvalidCreateRequests` memakai table-driven test agar banyak variasi dapat diuji dengan struktur yang sama.

Kasus `Content-Type` yang ditolak:

- header tidak ada;
- `text/plain`;
- parameter MIME tidak valid, yaitu `application/json; charset`.

Ketiganya menghasilkan `415 Unsupported Media Type`.

Kasus body yang ditolak:

- body kosong;
- JSON terpotong;
- top-level JSON berupa array;
- field tidak dikenal, termasuk `email`;
- dua nilai JSON dikirim berurutan.

Kasus tersebut menghasilkan `400 Bad Request`. Field `email` sengaja diuji sebagai unknown field karena kontrak account practice sekarang hanya menerima `user_id` dan `name`. Validasi ketat mencegah typo atau field lama diterima diam-diam.

Pada semua request yang tidak valid, fake tidak boleh dipanggil. Artinya validasi di boundary HTTP berhenti sebelum masuk ke application layer.

## Batas body 1 MiB dan status `413`

Nilai `maxRequestBodyBytes` pada handler adalah `1 << 20`, atau 1.048.576 byte (1 MiB). Handler memakai `http.MaxBytesReader` agar request yang terlalu besar tidak terus dibaca ke memory.

`TestAccountHandlerRejectsOversizedBody` membentuk JSON dengan `user_id` sepanjang 1 MiB. Setelah ditambah struktur JSON lainnya, total body melewati batas. Hasil yang diharapkan:

```text
413 Request Entity Too Large
{"error":{"code":"request_too_large","message":"request body is too large"}}
```

Use case tidak boleh dipanggil untuk request ini.

## Pemetaan error dan privasi response

`TestAccountHandlerMapsErrors` mengembalikan error dari fake melalui `fmt.Errorf("...: %w", err)`. `%w` membungkus error sambil mempertahankan identitas penyebabnya. Handler memakai `errors.Is`, sehingga sentinel error tetap dapat dikenali walaupun sudah dibungkus.

Pemetaan yang diperiksa adalah:

| Error | Status | Code response |
|---|---:|---|
| `ErrUserIDRequired` | 400 | `invalid_request` |
| `ErrNameRequired` | 400 | `invalid_request` |
| `ErrAccountNotFound` | 404 | `account_not_found` |
| `ErrServiceNotConfigured` | 503 | `account_unavailable` |
| `ErrStoreNotConfigured` | 503 | `account_unavailable` |
| error tidak dikenal | 500 | `internal_error` |

Untuk error internal, fake memakai pesan `database password leaked`. Test bukan hanya memastikan status `500`, tetapi juga memeriksa bahwa teks sensitif tersebut tidak muncul di response. Client hanya menerima pesan umum `internal server error`.

Ini penting karena error database, query, credential, atau detail implementasi tidak boleh dibocorkan melalui API.

## Dependency `nil` dan receiver `nil`

`TestAccountHandlerUnavailable` menguji dua kondisi:

- handler dibuat dengan use case `nil`;
- method `Register` dipanggil pada pointer `*AccountHandler` yang bernilai `nil`.

Keduanya tetap mendaftarkan route aman yang menjawab `503 Service Unavailable`, bukan menyebabkan panic. Ini menjaga server tetap mempunyai response yang jelas ketika wiring dependency belum benar.

## Registrasi pattern pada `ServeMux`

Pattern route didaftarkan ketika `Register(mux)` memanggil `mux.HandleFunc`. `http.ServeMux` memvalidasi pattern pada waktu registrasi tersebut. Pattern yang tidak valid atau konflik dapat menyebabkan panic saat route didaftarkan, bukan ketika source code dikompilasi.

Karena `newAccountTestMux` selalu memanggil `Register`, setiap test juga melewati proses registrasi pattern sebenarnya.

## Routing, `HEAD`, `405`, dan `404`

`TestAccountHandlerRouting` memeriksa perilaku bawaan `http.ServeMux` bersama pattern method-aware.

### `HEAD` memakai route `GET`

Jika tidak ada pattern `HEAD` yang lebih khusus, request `HEAD /accounts/{userID}` dapat cocok dengan pattern `GET /accounts/{userID}`. Karena itu `GetByUserID` tetap dipanggil dan `request.Pattern` berisi pattern `GET`.

Ada perbedaan penting dalam pengujian:

- `httptest.ResponseRecorder` mencatat body yang ditulis handler;
- server `net/http` sungguhan menahan body tersebut agar tidak dikirim melalui jaringan untuk response `HEAD`.

Oleh sebab itu test dengan recorder tetap mengharapkan body JSON penuh. Ini tidak berarti client `HEAD` pada server sungguhan akan menerima body.

### Method salah menghasilkan `405`

- `PUT /accounts` menghasilkan `405 Method Not Allowed` dengan `Allow: POST`.
- `POST /accounts/practice-user-026` menghasilkan `405 Method Not Allowed` dengan `Allow: GET, HEAD`.

Use case tidak boleh dipanggil saat method tidak cocok.

### Route tidak dikenal menghasilkan `404`

`GET /missing` tidak cocok dengan route mana pun, sehingga `ServeMux` menghasilkan `404 Not Found`. Fake juga tidak boleh dipanggil.

## Mengapa tidak memerlukan Docker atau PostgreSQL?

Test ini berhenti pada batas interface `AccountUseCase`. Dependency nyata diganti dengan `fakeAccountUseCase`, sehingga alurnya adalah:

```text
HTTP test → router/handler → fake use case
```

Tidak ada koneksi ke store nyata atau database. Karena itu Docker dan PostgreSQL tidak perlu dijalankan. Test menjadi cepat, deterministik, dan fokus pada tanggung jawab HTTP layer.

Test database tetap diperlukan pada layer store, tetapi itu merupakan jenis integration test dengan tujuan berbeda.

## Menjalankan test

Dari root `practice/srv-server`, jalankan:

```bash
gofmt -w internal/router/account_test.go
go test ./internal/router -v
go test ./...
```

Hasil focused router suite yang sudah diamati pada Step 26 (subtest juga tampil sebagai `PASS` pada output `-v`):

```text
--- PASS: TestAccountHandlerCreateSuccess (0.00s)
--- PASS: TestAccountHandlerGetByUserIDSuccess (0.00s)
--- PASS: TestAccountHandlerRejectsInvalidCreateRequests (0.00s)
--- PASS: TestAccountHandlerRejectsOversizedBody (0.04s)
--- PASS: TestAccountHandlerMapsErrors (0.00s)
--- PASS: TestAccountHandlerUnavailable (0.00s)
--- PASS: TestAccountHandlerRouting (0.00s)
PASS
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router  (cached)
```

Seluruh package juga lulus. Output yang diamati:

```text
?   github.com/imamsyafii-dev/bbge-practice/srv-server                         [no test files]
?   github.com/imamsyafii-dev/bbge-practice/srv-server/config                  [no test files]
?   github.com/imamsyafii-dev/bbge-practice/srv-server/db                      [no test files]
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account    (cached)
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account (cached)
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router         (cached)
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store          (cached)
?   github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres             [no test files]
```

Semua package selesai tanpa kegagalan. Tanda `(cached)` berarti Go memakai hasil test yang masih valid dari cache, bukan berarti test dilewati secara salah. Package tanpa test ditampilkan sebagai `[no test files]`.

## Ringkasan konsep yang dipelajari

- `httptest.NewRequest` membuat request tanpa membuka port server.
- `httptest.NewRecorder` menangkap status, header, dan body response.
- Fake berdasarkan interface mengisolasi HTTP layer dari database dan use case nyata.
- Compile-time interface assertion mendeteksi ketidakcocokan method sejak kompilasi.
- Table-driven test mengurangi pengulangan untuk banyak variasi input dan error.
- Call counter memastikan dependency dipanggil atau tidak dipanggil sesuai alur.
- `request.Pattern` menunjukkan pattern `ServeMux` yang benar-benar cocok.
- `errors.Is` tetap mengenali sentinel error yang dibungkus dengan `%w`.
- Response `500` harus menyembunyikan detail internal.
- `HEAD` dapat memakai handler `GET`, tetapi server nyata tidak mengirim body `HEAD` melalui jaringan.
- `405` berarti path dikenali tetapi method tidak didukung; `404` berarti route tidak ditemukan.

## Perubahan pada Step 26

File baru:

```text
srv-server/internal/router/account_test.go
learning-steps/01-foundation/07-http-router/step-26-account-http-handler-test.md
```

Tidak ada perubahan pada schema database, migration, query SQL, atau kontrak account.

## Preview Step 27

Pada Step 27 kita akan melakukan wiring dependency account di `main.go`:

```text
store.Queries
    ↓
domain account Service
    ↓
application account UseCase
    ↓
HTTP AccountHandler
    ↓
http.ServeMux
```

Secara garis besar, `main.go` akan membuat `store.New(database)`, memasukkannya ke domain service, memasukkan service ke application use case, lalu mendaftarkan account handler pada mux yang sama dengan health route. Setelah itu API dapat diuji secara langsung terhadap PostgreSQL practice.
