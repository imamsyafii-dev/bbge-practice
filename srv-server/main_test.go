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
