package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const refreshTokenRandomByteLength = 32

var ErrRefreshTokenGeneratorNotConfigured = errors.New(
	"refresh token generator is not configured",
)

type RandomRefreshTokenGenerator struct {
	random io.Reader
}

func NewRandomRefreshTokenGenerator() *RandomRefreshTokenGenerator {
	return &RandomRefreshTokenGenerator{
		random: rand.Reader,
	}
}

func (
	generator *RandomRefreshTokenGenerator,
) Generate(
	ctx context.Context,
) (RefreshToken, error) {
	if generator == nil ||
		generator.random == nil {
		return RefreshToken{},
			ErrRefreshTokenGeneratorNotConfigured
	}

	if err := ctx.Err(); err != nil {
		return RefreshToken{}, err
	}

	var randomBytes [refreshTokenRandomByteLength]byte
	defer clear(randomBytes[:])

	if _, err := io.ReadFull(
		generator.random,
		randomBytes[:],
	); err != nil {
		return RefreshToken{},
			fmt.Errorf(
				"generate refresh token: %w",
				err,
			)
	}

	if err := ctx.Err(); err != nil {
		return RefreshToken{}, err
	}

	encoded := hex.EncodeToString(
		randomBytes[:],
	)

	return newRefreshToken(encoded)
}
