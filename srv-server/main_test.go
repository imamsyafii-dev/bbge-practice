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
