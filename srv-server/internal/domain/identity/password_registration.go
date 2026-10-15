package identity

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	minLoginUsernameLength = 3
	maxLoginUsernameLength = 32
	minLoginPasswordLength = 15
	maxLoginPasswordLength = 128
	minRequestIDLength     = 16
	maxRequestIDLength     = 64
)

var (
	ErrUsernameInput = errors.New(
		"username must contain 3 to 32 ASCII letters, digits or underscores",
	)
	ErrPasswordInput = errors.New(
		"password must contain 15 to 128 characters",
	)
	ErrRegistrationRequestIDInvalid = errors.New(
		"registration request id is invalid",
	)
)

type PasswordRegisterInput struct {
	Username     string
	Password     string
	Registration RegisterInput
	RequestID    string
}

func NormalizeLoginUsername(
	value string,
) (string, error) {
	if len(value) < minLoginUsernameLength ||
		len(value) > maxLoginUsernameLength {
		return "", ErrUsernameInput
	}

	for _, character := range value {
		isLowercaseLetter :=
			character >= 'a' && character <= 'z'
		isUppercaseLetter :=
			character >= 'A' && character <= 'Z'
		isNumber :=
			character >= '0' && character <= '9'
		isUnderscore := character == '_'

		if !isLowercaseLetter &&
			!isUppercaseLetter &&
			!isNumber &&
			!isUnderscore {
			return "", ErrUsernameInput
		}
	}

	return strings.ToLower(value), nil
}

func validateLoginPassword(
	value string,
) error {
	if !utf8.ValidString(value) {
		return ErrPasswordInput
	}

	length := utf8.RuneCountInString(value)

	if length < minLoginPasswordLength ||
		length > maxLoginPasswordLength {
		return ErrPasswordInput
	}

	return nil
}

func validatePasswordRegisterInput(
	input PasswordRegisterInput,
) (PasswordRegisterInput, error) {
	username, err :=
		NormalizeLoginUsername(input.Username)
	if err != nil {
		return PasswordRegisterInput{}, err
	}

	if err := validateLoginPassword(
		input.Password,
	); err != nil {
		return PasswordRegisterInput{}, err
	}

	if len(input.RequestID) < minRequestIDLength ||
		len(input.RequestID) > maxRequestIDLength ||
		strings.TrimSpace(input.RequestID) !=
			input.RequestID {
		return PasswordRegisterInput{},
			ErrRegistrationRequestIDInvalid
	}

	if err := validateRegisterInput(
		input.Registration,
	); err != nil {
		return PasswordRegisterInput{}, err
	}

	input.Username = username

	return input, nil
}
