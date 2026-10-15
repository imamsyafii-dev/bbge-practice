package account

import (
	"context"
	"errors"

	accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
)

var ErrServiceNotConfigured = errors.New(
	"account service not configured",
)

type AccountService interface {
	Create(
		ctx context.Context,
		input accountdomain.CreateInput,
	) (store.Account, error)

	GetByUserID(
		ctx context.Context,
		userID string,
	) (store.Account, error)
}

var _ AccountService = (*accountdomain.Service)(nil)

type UseCase struct {
	service AccountService
}

type CreateInput struct {
	UserID string
	Name   string
}

func NewUseCase(
	accountService AccountService,
) *UseCase {
	return &UseCase{
		service: accountService,
	}
}

func (u *UseCase) Create(
	ctx context.Context,
	input CreateInput,
) (store.Account, error) {
	if u == nil || u.service == nil {
		return store.Account{},
			ErrServiceNotConfigured
	}

	return u.service.Create(
		ctx,
		accountdomain.CreateInput{
			UserID: input.UserID,
			Name:   input.Name,
		},
	)
}

func (u *UseCase) GetByUserID(
	ctx context.Context,
	userID string,
) (store.Account, error) {
	if u == nil || u.service == nil {
		return store.Account{},
			ErrServiceNotConfigured
	}

	return u.service.GetByUserID(
		ctx,
		userID,
	)
}
