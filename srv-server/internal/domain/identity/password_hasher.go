package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordHashTime        uint32 = 2
	passwordHashMemory      uint32 = 19456
	passwordHashParallelism uint8  = 1
	passwordHashSaltLength         = 16
	passwordHashKeyLength          = 32

	passwordHashPrefix = "$argon2id$v=19$m=19456,t=2,p=1$"
)

var (
	ErrPasswordHasherConcurrencyInvalid = errors.New(
		"password hasher concurrency must be positive",
	)
	ErrPasswordHasherNotConfigured = errors.New(
		"password hasher is not configured",
	)
	ErrPasswordHasherBusy = errors.New(
		"password hasher is busy",
	)
)

type Argon2IDPasswordHasher struct {
	slots chan struct{}
}

func NewArgon2IDPasswordHasher(
	concurrency int,
) (*Argon2IDPasswordHasher, error) {
	if concurrency <= 0 {
		return nil,
			ErrPasswordHasherConcurrencyInvalid
	}

	return &Argon2IDPasswordHasher{
		slots: make(
			chan struct{},
			concurrency,
		),
	}, nil
}

var _ PasswordHasher = (*Argon2IDPasswordHasher)(nil)

func (
	hasher *Argon2IDPasswordHasher,
) Hash(
	ctx context.Context,
	plaintext string,
) (EncodedPasswordHash, error) {
	if err := hasher.ensureConfigured(); err != nil {
		return EncodedPasswordHash{}, err
	}

	if err := ctx.Err(); err != nil {
		return EncodedPasswordHash{}, err
	}

	salt := make(
		[]byte,
		passwordHashSaltLength,
	)
	defer clear(salt)

	if _, err := rand.Read(salt); err != nil {
		return EncodedPasswordHash{},
			fmt.Errorf(
				"generate password salt: %w",
				err,
			)
	}

	key, err := hasher.derive(
		ctx,
		plaintext,
		salt,
	)
	if err != nil {
		return EncodedPasswordHash{}, err
	}
	defer clear(key)

	encoded :=
		passwordHashPrefix +
			base64.RawStdEncoding.EncodeToString(
				salt,
			) +
			"$" +
			base64.RawStdEncoding.EncodeToString(
				key,
			)

	return newEncodedPasswordHash(encoded)
}

func (
	hasher *Argon2IDPasswordHasher,
) Verify(
	ctx context.Context,
	plaintext string,
	encoded EncodedPasswordHash,
) (bool, error) {
	if err := hasher.ensureConfigured(); err != nil {
		return false, err
	}

	salt, expected, valid :=
		parseEncodedPasswordHash(
			encoded.Encoded(),
		)
	defer clear(salt)
	defer clear(expected)

	actual, err := hasher.derive(
		ctx,
		plaintext,
		salt,
	)
	if err != nil {
		return false, err
	}
	defer clear(actual)

	matched :=
		subtle.ConstantTimeCompare(
			actual,
			expected,
		) == 1

	return valid && matched, nil
}

func (
	hasher *Argon2IDPasswordHasher,
) ensureConfigured() error {
	if hasher == nil || hasher.slots == nil {
		return ErrPasswordHasherNotConfigured
	}

	return nil
}

func (
	hasher *Argon2IDPasswordHasher,
) derive(
	ctx context.Context,
	plaintext string,
	salt []byte,
) ([]byte, error) {
	if err := hasher.ensureConfigured(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	select {
	case hasher.slots <- struct{}{}:
		defer func() {
			<-hasher.slots
		}()
	default:
		return nil, ErrPasswordHasherBusy
	}

	plaintextBytes := []byte(plaintext)
	defer clear(plaintextBytes)

	result := argon2.IDKey(
		plaintextBytes,
		salt,
		passwordHashTime,
		passwordHashMemory,
		passwordHashParallelism,
		passwordHashKeyLength,
	)

	if err := ctx.Err(); err != nil {
		clear(result)

		return nil, err
	}

	return result, nil
}

func parseEncodedPasswordHash(
	value string,
) (
	salt []byte,
	expected []byte,
	valid bool,
) {
	dummySalt := make(
		[]byte,
		passwordHashSaltLength,
	)
	dummyExpected := make(
		[]byte,
		passwordHashKeyLength,
	)

	expectedLength :=
		len(passwordHashPrefix) +
			base64.RawStdEncoding.EncodedLen(
				passwordHashSaltLength,
			) +
			1 +
			base64.RawStdEncoding.EncodedLen(
				passwordHashKeyLength,
			)

	if len(value) != expectedLength {
		return dummySalt, dummyExpected, false
	}

	parts := strings.Split(value, "$")

	if len(parts) != 6 ||
		parts[0] != "" ||
		parts[1] != "argon2id" ||
		parts[2] != "v=19" ||
		parts[3] != "m=19456,t=2,p=1" {
		return dummySalt, dummyExpected, false
	}

	parsedSalt, saltErr :=
		base64.RawStdEncoding.DecodeString(
			parts[4],
		)
	parsedExpected, expectedErr :=
		base64.RawStdEncoding.DecodeString(
			parts[5],
		)

	if saltErr != nil ||
		expectedErr != nil ||
		len(parsedSalt) !=
			passwordHashSaltLength ||
		len(parsedExpected) !=
			passwordHashKeyLength {
		clear(parsedSalt)
		clear(parsedExpected)

		return dummySalt, dummyExpected, false
	}

	clear(dummySalt)
	clear(dummyExpected)

	return parsedSalt, parsedExpected, true
}
