package identity

import (
	"errors"
	"strings"
	"time"
)

type DeviceFlag int32

const (
	DeviceFlagApp DeviceFlag = iota
	DeviceFlagWeb
	DeviceFlagPC

	DeviceFlagSystem DeviceFlag = 99
)

type DeviceSessionStatus int16

const (
	DeviceSessionStatusUnknown DeviceSessionStatus = iota
	DeviceSessionStatusActive
	DeviceSessionStatusInvalid
	DeviceSessionStatusLoggedOut
)

var (
	ErrDeviceOwnerRequired = errors.New(
		"device owner is required",
	)
	ErrDeviceFlagInvalid = errors.New(
		"device flag is invalid",
	)
	ErrRefreshTokenInvalid = errors.New(
		"refresh token is invalid",
	)
	ErrDeviceSessionIssuedAtRequired = errors.New(
		"device session issued at is required",
	)
	ErrDeviceSessionExpirationInvalid = errors.New(
		"device session expiration is invalid",
	)
)

type RefreshToken struct {
	value string
}

func newRefreshToken(
	value string,
) (RefreshToken, error) {
	if value == "" ||
		strings.TrimSpace(value) != value {
		return RefreshToken{},
			ErrRefreshTokenInvalid
	}

	return RefreshToken{
		value: value,
	}, nil
}

func (token RefreshToken) Value() string {
	return token.value
}

type Device struct {
	UserID        string
	DeviceID      string
	DeviceName    string
	DeviceModel   string
	DeviceFlag    DeviceFlag
	RefreshToken  RefreshToken
	SessionStatus DeviceSessionStatus
	IssuedAt      time.Time
	ExpiredAt     time.Time
}

type activeDeviceInput struct {
	UserID       string
	DeviceID     string
	DeviceName   string
	DeviceModel  string
	DeviceFlag   DeviceFlag
	RefreshToken RefreshToken
	IssuedAt     time.Time
	ExpiredAt    time.Time
}

func newActiveDevice(
	input activeDeviceInput,
) (Device, error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return Device{},
			ErrDeviceOwnerRequired
	}

	deviceID := strings.TrimSpace(
		input.DeviceID,
	)
	if deviceID == "" {
		return Device{},
			ErrDeviceIDRequired
	}

	if !input.DeviceFlag.valid() {
		return Device{},
			ErrDeviceFlagInvalid
	}

	if input.RefreshToken.Value() == "" {
		return Device{},
			ErrRefreshTokenInvalid
	}

	if input.IssuedAt.IsZero() {
		return Device{},
			ErrDeviceSessionIssuedAtRequired
	}

	issuedAt := input.IssuedAt.
		Round(0).
		UTC()
	expiredAt := input.ExpiredAt.
		Round(0).
		UTC()

	if !expiredAt.After(issuedAt) {
		return Device{},
			ErrDeviceSessionExpirationInvalid
	}

	return Device{
		UserID:   userID,
		DeviceID: deviceID,
		DeviceName: strings.TrimSpace(
			input.DeviceName,
		),
		DeviceModel: strings.TrimSpace(
			input.DeviceModel,
		),
		DeviceFlag:    input.DeviceFlag,
		RefreshToken:  input.RefreshToken,
		SessionStatus: DeviceSessionStatusActive,
		IssuedAt:      issuedAt,
		ExpiredAt:     expiredAt,
	}, nil
}

func (
	device Device,
) SessionActiveAt(
	now time.Time,
) bool {
	if device.SessionStatus !=
		DeviceSessionStatusActive ||
		device.IssuedAt.IsZero() ||
		device.ExpiredAt.IsZero() {
		return false
	}

	now = now.
		Round(0).
		UTC()

	return !now.Before(device.IssuedAt) &&
		now.Before(device.ExpiredAt)
}

func (
	flag DeviceFlag,
) valid() bool {
	switch flag {
	case DeviceFlagApp,
		DeviceFlagWeb,
		DeviceFlagPC,
		DeviceFlagSystem:
		return true
	default:
		return false
	}
}
