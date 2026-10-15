package account

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
	"github.com/jackc/pgx/v5"
)

type testContextKey struct{}

type fakeStore struct {
	createAccountFunc func(
		context.Context,
		store.CreateAccountParams,
	) (store.Account, error)

	getAccountByUserIDFunc func(
		context.Context,
		string,
	) (store.Account, error)

	createAccountCalls      int
	getAccountByUserIDCalls int
}

var _ Store = (*fakeStore)(nil)

func (f *fakeStore) CreateAccount(
	ctx context.Context,
	arg store.CreateAccountParams,
) (store.Account, error) {
	f.createAccountCalls++

	if f.createAccountFunc == nil {
		panic("unexpected CreateAccount call")
	}

	return f.createAccountFunc(ctx, arg)
}

func (f *fakeStore) GetAccountByUserID(
	ctx context.Context,
	userID string,
) (store.Account, error) {
	f.getAccountByUserIDCalls++

	if f.getAccountByUserIDFunc == nil {
		panic("unexpected GetAccountByUserID call")
	}

	return f.getAccountByUserIDFunc(
		ctx,
		userID,
	)
}

func TestServiceRequiresConfiguredStore(
	t *testing.T,
) {
	tests := []struct {
		name    string
		service *Service
	}{
		{
			name:    "nil receiver",
			service: nil,
		},
		{
			name:    "nil dependency",
			service: NewService(nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.service.Create(
				context.Background(),
				CreateInput{
					UserID: "practice-user-022",
					Name:   "Practice User",
				},
			)
			if !errors.Is(
				err,
				ErrStoreNotConfigured,
			) {
				t.Fatalf(
					"Create error = %v, want %v",
					err,
					ErrStoreNotConfigured,
				)
			}

			_, err = test.service.GetByUserID(
				context.Background(),
				"practice-user-022",
			)
			if !errors.Is(
				err,
				ErrStoreNotConfigured,
			) {
				t.Fatalf(
					"GetByUserID error = %v, want %v",
					err,
					ErrStoreNotConfigured,
				)
			}
		})
	}
}

func TestServiceCreateRejectsInvalidInputWithoutStoreCall(
	t *testing.T,
) {
	tests := []struct {
		name    string
		input   CreateInput
		wantErr error
	}{
		{
			name: "user id is required",
			input: CreateInput{
				UserID: " \t ",
				Name:   "Practice User",
			},
			wantErr: ErrUserIDRequired,
		},
		{
			name: "name is required",
			input: CreateInput{
				UserID: "practice-user-022",
				Name:   " \n ",
			},
			wantErr: ErrNameRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeStore{}
			service := NewService(fake)

			_, err := service.Create(
				context.Background(),
				test.input,
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"Create error = %v, want %v",
					err,
					test.wantErr,
				)
			}

			if fake.createAccountCalls != 0 {
				t.Fatalf(
					"CreateAccount calls = %d, want 0",
					fake.createAccountCalls,
				)
			}
		})
	}
}

func TestServiceCreateNormalizesAndDelegates(
	t *testing.T,
) {
	ctx := context.WithValue(
		context.Background(),
		testContextKey{},
		"create",
	)

	expectedParams := store.CreateAccountParams{
		UserID: "practice-user-022",
		Name:   "Practice User",
	}

	expectedAccount := store.Account{
		ID:      22,
		UserID:  expectedParams.UserID,
		Name:    expectedParams.Name,
		Status:  1,
		Version: 0,
	}

	fake := &fakeStore{
		createAccountFunc: func(
			gotCtx context.Context,
			arg store.CreateAccountParams,
		) (store.Account, error) {
			if got := gotCtx.Value(
				testContextKey{},
			); got != "create" {
				t.Fatalf(
					"context value = %v, want create",
					got,
				)
			}

			if !reflect.DeepEqual(
				arg,
				expectedParams,
			) {
				t.Fatalf(
					"CreateAccount params = %+v, want %+v",
					arg,
					expectedParams,
				)
			}

			return expectedAccount, nil
		},
	}

	service := NewService(fake)

	account, err := service.Create(
		ctx,
		CreateInput{
			UserID: "  practice-user-022  ",
			Name:   "  Practice User  ",
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

	if !reflect.DeepEqual(
		account,
		expectedAccount,
	) {
		t.Fatalf(
			"Create account = %+v, want %+v",
			account,
			expectedAccount,
		)
	}
}

func TestServiceCreateWrapsStoreError(
	t *testing.T,
) {
	storeErr := errors.New("insert failed")

	fake := &fakeStore{
		createAccountFunc: func(
			context.Context,
			store.CreateAccountParams,
		) (store.Account, error) {
			return store.Account{}, storeErr
		},
	}

	service := NewService(fake)

	account, err := service.Create(
		context.Background(),
		CreateInput{
			UserID: "practice-user-022",
			Name:   "Practice User",
		},
	)

	if !errors.Is(err, storeErr) {
		t.Fatalf(
			"Create error = %v, want wrapped %v",
			err,
			storeErr,
		)
	}

	if err == nil ||
		!strings.HasPrefix(
			err.Error(),
			"create account: ",
		) {
		t.Fatalf(
			"Create error = %v, want create account context",
			err,
		)
	}

	if !reflect.DeepEqual(
		account,
		store.Account{},
	) {
		t.Fatalf(
			"Create account = %+v, want zero value",
			account,
		)
	}
}

func TestServiceGetByUserIDRejectsBlankUserIDWithoutStoreCall(
	t *testing.T,
) {
	fake := &fakeStore{}
	service := NewService(fake)

	_, err := service.GetByUserID(
		context.Background(),
		" \t ",
	)
	if !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf(
			"GetByUserID error = %v, want %v",
			err,
			ErrUserIDRequired,
		)
	}

	if fake.getAccountByUserIDCalls != 0 {
		t.Fatalf(
			"GetAccountByUserID calls = %d, want 0",
			fake.getAccountByUserIDCalls,
		)
	}
}

func TestServiceGetByUserIDNormalizesAndDelegates(
	t *testing.T,
) {
	ctx := context.WithValue(
		context.Background(),
		testContextKey{},
		"get",
	)

	expectedAccount := store.Account{
		ID:      22,
		UserID:  "practice-user-022",
		Name:    "Practice User",
		Status:  1,
		Version: 0,
	}

	fake := &fakeStore{
		getAccountByUserIDFunc: func(
			gotCtx context.Context,
			userID string,
		) (store.Account, error) {
			if got := gotCtx.Value(
				testContextKey{},
			); got != "get" {
				t.Fatalf(
					"context value = %v, want get",
					got,
				)
			}

			if userID != expectedAccount.UserID {
				t.Fatalf(
					"userID = %q, want %q",
					userID,
					expectedAccount.UserID,
				)
			}

			return expectedAccount, nil
		},
	}

	service := NewService(fake)

	account, err := service.GetByUserID(
		ctx,
		"  practice-user-022  ",
	)
	if err != nil {
		t.Fatalf(
			"GetByUserID returned error: %v",
			err,
		)
	}

	if fake.getAccountByUserIDCalls != 1 {
		t.Fatalf(
			"GetAccountByUserID calls = %d, want 1",
			fake.getAccountByUserIDCalls,
		)
	}

	if !reflect.DeepEqual(
		account,
		expectedAccount,
	) {
		t.Fatalf(
			"GetByUserID account = %+v, want %+v",
			account,
			expectedAccount,
		)
	}
}

func TestServiceGetByUserIDMapsNoRowsToAccountNotFound(
	t *testing.T,
) {
	fake := &fakeStore{
		getAccountByUserIDFunc: func(
			context.Context,
			string,
		) (store.Account, error) {
			return store.Account{}, fmt.Errorf(
				"query account: %w",
				pgx.ErrNoRows,
			)
		},
	}

	service := NewService(fake)

	account, err := service.GetByUserID(
		context.Background(),
		"practice-user-missing",
	)

	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf(
			"GetByUserID error = %v, want %v",
			err,
			ErrAccountNotFound,
		)
	}

	if !reflect.DeepEqual(
		account,
		store.Account{},
	) {
		t.Fatalf(
			"GetByUserID account = %+v, want zero value",
			account,
		)
	}
}

func TestServiceGetByUserIDWrapsStoreError(
	t *testing.T,
) {
	storeErr := errors.New("query failed")

	fake := &fakeStore{
		getAccountByUserIDFunc: func(
			context.Context,
			string,
		) (store.Account, error) {
			return store.Account{}, storeErr
		},
	}

	service := NewService(fake)

	account, err := service.GetByUserID(
		context.Background(),
		"practice-user-022",
	)

	if !errors.Is(err, storeErr) {
		t.Fatalf(
			"GetByUserID error = %v, want wrapped %v",
			err,
			storeErr,
		)
	}

	if err == nil ||
		!strings.HasPrefix(
			err.Error(),
			"get account by user id: ",
		) {
		t.Fatalf(
			"GetByUserID error = %v, want operation context",
			err,
		)
	}

	if !reflect.DeepEqual(
		account,
		store.Account{},
	) {
		t.Fatalf(
			"GetByUserID account = %+v, want zero value",
			account,
		)
	}
}
