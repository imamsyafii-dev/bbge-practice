package identity

import (
	"context"
	"errors"
	"strings"
)

var ErrPasswordHashInvalid = errors.New(
	"encoded password hash is invalid",
)

type EncodedPasswordHash struct {
	value string
}

func newEncodedPasswordHash(
	value string,
) (EncodedPasswordHash, error) {
	if strings.TrimSpace(value) == "" {
		return EncodedPasswordHash{},
			ErrPasswordHashInvalid
	}

	return EncodedPasswordHash{
		value: value,
	}, nil
}

func (hash EncodedPasswordHash) Encoded() string {
	return hash.value
}

type PasswordCredential struct {
	UserID                string
	Username              string
	PasswordHash          EncodedPasswordHash
	RegistrationRequestID string
}

type PasswordHasher interface {
	Hash(
		ctx context.Context,
		plaintext string,
	) (EncodedPasswordHash, error)
}
