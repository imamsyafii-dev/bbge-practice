package identity

import (
	"crypto/ed25519"
	"errors"
	"strings"
)

const x25519PublicKeySize = 32

var (
	ErrPasswordRequired = errors.New(
		"password is required",
	)
	ErrDeviceIDRequired = errors.New(
		"device id is required",
	)
	ErrIdentityIdentifiersInvalid = errors.New(
		"identity identifiers are invalid",
	)
)

type RegisterInput struct {
	Password          string
	DeviceID          string
	DeviceName        string
	DeviceModel       string
	DeviceFlag        int32
	IdentifierX25519  []byte
	IdentifierEd25519 []byte
}

func validateRegisterInput(
	input RegisterInput,
) error {
	if strings.TrimSpace(input.Password) == "" {
		return ErrPasswordRequired
	}

	if strings.TrimSpace(input.DeviceID) == "" {
		return ErrDeviceIDRequired
	}

	if len(input.IdentifierX25519) !=
		x25519PublicKeySize ||
		len(input.IdentifierEd25519) !=
			ed25519.PublicKeySize {
		return ErrIdentityIdentifiersInvalid
	}

	return nil
}
