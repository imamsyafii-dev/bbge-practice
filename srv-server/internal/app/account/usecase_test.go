package account

import (
	"context"
	"errors"
	"reflect"
	"testing"

	accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
)

type useCaseTestContextKey struct{}

type fakeAccountService struct {
	createFunc func(
		context.Context,
		accountdomain.CreateInput,
	) (store.Account, error)

	getByUserIDFunc func(
		context.Context,
		string,
	) (store.Account, error)

	createCalls      int
	getByUserIDCalls int
}

var _ AccountService = (*fakeAccountService)(nil)

func (f *fakeAccountService) Create(
	ctx context.Context,
	input accountdomain.CreateInput,
) (store.Account, error) {
	f.createCalls++

	if f.createFunc == nil {
		panic("unexpected Create call")
	}

	return f.createFunc(ctx, input)
}

func (f *fakeAccountService) GetByUserID(
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

func TestUseCaseRequiresConfiguredService(
	t *testing.T,
) {
	tests := []struct {
		name    string
		useCase *UseCase
	}{
		{
			name:    "nil receiver",
			useCase: nil,
		},
		{
			name:    "nil dependency",
			useCase: NewUseCase(nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account, err := test.useCase.Create(
				context.Background(),
				CreateInput{
					UserID: "practice-user-024",
					Name:   "Practice User",
				},
			)
			if err != ErrServiceNotConfigured {
				t.Fatalf(
					"Create error = %v, want %v",
					err,
					ErrServiceNotConfigured,
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

			account, err = test.useCase.GetByUserID(
				context.Background(),
				"practice-user-024",
			)
			if err != ErrServiceNotConfigured {
				t.Fatalf(
					"GetByUserID error = %v, want %v",
					err,
					ErrServiceNotConfigured,
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
		})
	}
}

func TestUseCaseCreateForwardsInputAndResult(
	t *testing.T,
) {
	ctx := context.WithValue(
		context.Background(),
		useCaseTestContextKey{},
		"create",
	)

	input := CreateInput{
		UserID: "  practice-user-024  ",
		Name:   "  Practice User  ",
	}

	expectedDomainInput := accountdomain.CreateInput{
		UserID: input.UserID,
		Name:   input.Name,
	}

	expectedResult := store.Account{
		ID:      24,
		UserID:  "practice-user-024",
		Name:    "Practice User",
		Status:  1,
		Version: 0,
	}

	fake := &fakeAccountService{
		createFunc: func(
			gotCtx context.Context,
			gotInput accountdomain.CreateInput,
		) (store.Account, error) {
			if gotCtx != ctx {
				t.Fatal(
					"Create context was not forwarded unchanged",
				)
			}

			if gotInput != expectedDomainInput {
				t.Fatalf(
					"Create input = %+v, want %+v",
					gotInput,
					expectedDomainInput,
				)
			}

			return expectedResult, nil
		},
	}

	useCase := NewUseCase(fake)

	result, err := useCase.Create(
		ctx,
		input,
	)
	if err != nil {
		t.Fatalf(
			"Create returned error: %v",
			err,
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

	if !reflect.DeepEqual(
		result,
		expectedResult,
	) {
		t.Fatalf(
			"Create result = %+v, want %+v",
			result,
			expectedResult,
		)
	}
}

func TestUseCaseCreateForwardsExactError(
	t *testing.T,
) {
	serviceErr := errors.New("create failed")

	expectedResult := store.Account{
		ID:     24,
		UserID: "practice-user-024",
		Name:   "Practice User",
	}

	fake := &fakeAccountService{
		createFunc: func(
			context.Context,
			accountdomain.CreateInput,
		) (store.Account, error) {
			return expectedResult, serviceErr
		},
	}

	useCase := NewUseCase(fake)

	result, err := useCase.Create(
		context.Background(),
		CreateInput{
			UserID: "practice-user-024",
			Name:   "Practice User",
		},
	)

	if err != serviceErr {
		t.Fatalf(
			"Create error = %v, want exact error %v",
			err,
			serviceErr,
		)
	}

	if fake.createCalls != 1 {
		t.Fatalf(
			"Create calls = %d, want 1",
			fake.createCalls,
		)
	}

	if !reflect.DeepEqual(
		result,
		expectedResult,
	) {
		t.Fatalf(
			"Create result = %+v, want %+v",
			result,
			expectedResult,
		)
	}
}

func TestUseCaseGetByUserIDForwardsInputAndResult(
	t *testing.T,
) {
	ctx := context.WithValue(
		context.Background(),
		useCaseTestContextKey{},
		"get",
	)

	userID := "  practice-user-024  "

	expectedResult := store.Account{
		ID:      24,
		UserID:  "practice-user-024",
		Name:    "Practice User",
		Status:  1,
		Version: 0,
	}

	fake := &fakeAccountService{
		getByUserIDFunc: func(
			gotCtx context.Context,
			gotUserID string,
		) (store.Account, error) {
			if gotCtx != ctx {
				t.Fatal(
					"GetByUserID context was not forwarded unchanged",
				)
			}

			if gotUserID != userID {
				t.Fatalf(
					"GetByUserID userID = %q, want %q",
					gotUserID,
					userID,
				)
			}

			return expectedResult, nil
		},
	}

	useCase := NewUseCase(fake)

	result, err := useCase.GetByUserID(
		ctx,
		userID,
	)
	if err != nil {
		t.Fatalf(
			"GetByUserID returned error: %v",
			err,
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

	if !reflect.DeepEqual(
		result,
		expectedResult,
	) {
		t.Fatalf(
			"GetByUserID result = %+v, want %+v",
			result,
			expectedResult,
		)
	}
}

func TestUseCaseGetByUserIDForwardsExactError(
	t *testing.T,
) {
	serviceErr := errors.New("get failed")

	expectedResult := store.Account{
		ID:     24,
		UserID: "practice-user-024",
		Name:   "Practice User",
	}

	fake := &fakeAccountService{
		getByUserIDFunc: func(
			context.Context,
			string,
		) (store.Account, error) {
			return expectedResult, serviceErr
		},
	}

	useCase := NewUseCase(fake)

	result, err := useCase.GetByUserID(
		context.Background(),
		"practice-user-024",
	)

	if err != serviceErr {
		t.Fatalf(
			"GetByUserID error = %v, want exact error %v",
			err,
			serviceErr,
		)
	}

	if fake.getByUserIDCalls != 1 {
		t.Fatalf(
			"GetByUserID calls = %d, want 1",
			fake.getByUserIDCalls,
		)
	}

	if !reflect.DeepEqual(
		result,
		expectedResult,
	) {
		t.Fatalf(
			"GetByUserID result = %+v, want %+v",
			result,
			expectedResult,
		)
	}
}
