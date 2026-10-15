package account

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
	"github.com/jackc/pgx/v5"
)

var (
	ErrStoreNotConfigured = errors.New(
		"account store not configured",
	)
	ErrUserIDRequired = errors.New(
		"user id is required",
	)
	ErrNameRequired = errors.New(
		"name is required",
	)
	ErrAccountNotFound = errors.New(
		"account not found",
	)
)

type Store interface {
	CreateAccount(
		ctx context.Context,
		arg store.CreateAccountParams,
	) (store.Account, error)

	GetAccountByUserID(
		ctx context.Context,
		userID string,
	) (store.Account, error)
}

var _ Store = (*store.Queries)(nil)

type Service struct {
	store Store
}

type CreateInput struct {
	UserID string
	Name   string
}

func NewService(
	accountStore Store,
) *Service {
	return &Service{
		store: accountStore,
	}
}

func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (store.Account, error) {
	if s == nil || s.store == nil {
		return store.Account{},
			ErrStoreNotConfigured
	}

	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return store.Account{},
			ErrUserIDRequired
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return store.Account{},
			ErrNameRequired
	}

	account, err := s.store.CreateAccount(
		ctx,
		store.CreateAccountParams{
			UserID: userID,
			Name:   name,
		},
	)
	if err != nil {
		return store.Account{}, fmt.Errorf(
			"create account: %w",
			err,
		)
	}

	return account, nil
}

func (s *Service) GetByUserID(
	ctx context.Context,
	userID string,
) (store.Account, error) {
	if s == nil || s.store == nil {
		return store.Account{},
			ErrStoreNotConfigured
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return store.Account{},
			ErrUserIDRequired
	}

	account, err := s.store.GetAccountByUserID(
		ctx,
		userID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Account{},
				ErrAccountNotFound
		}

		return store.Account{}, fmt.Errorf(
			"get account by user id: %w",
			err,
		)
	}

	return account, nil
}
