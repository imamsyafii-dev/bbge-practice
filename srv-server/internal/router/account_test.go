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
