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
