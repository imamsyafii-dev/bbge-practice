package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const userIDRandomByteLength = 16

var ErrUserIDGeneratorNotConfigured = errors.New(
	"user ID generator is not configured",
)

type RandomUserIDGenerator struct {
	random io.Reader
}

func NewRandomUserIDGenerator() *RandomUserIDGenerator {
	return &RandomUserIDGenerator{
		random: rand.Reader,
	}
}

func (
	generator *RandomUserIDGenerator,
) Generate(
	ctx context.Context,
) (string, error) {
	if generator == nil ||
		generator.random == nil {
		return "",
			ErrUserIDGeneratorNotConfigured
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	var randomBytes [userIDRandomByteLength]byte

	if _, err := io.ReadFull(
		generator.random,
		randomBytes[:],
	); err != nil {
		return "",
			fmt.Errorf(
				"generate user ID: %w",
				err,
			)
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	return hex.EncodeToString(
		randomBytes[:],
	), nil
}
