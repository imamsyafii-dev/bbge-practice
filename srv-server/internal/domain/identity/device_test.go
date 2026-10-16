package identity

import (
	"errors"
	"testing"
	"time"
)

func TestDeviceFlagValid(
	t *testing.T,
) {
	tests := []struct {
		name string
		flag DeviceFlag
		want bool
	}{
		{
			name: "app",
			flag: DeviceFlagApp,
			want: true,
		},
		{
			name: "web",
			flag: DeviceFlagWeb,
			want: true,
		},
		{
			name: "PC",
			flag: DeviceFlagPC,
			want: true,
		},
		{
			name: "system",
			flag: DeviceFlagSystem,
			want: true,
		},
		{
			name: "negative",
			flag: DeviceFlag(-1),
		},
		{
			name: "unknown sequential value",
			flag: DeviceFlag(3),
		},
		{
			name: "unknown large value",
			flag: DeviceFlag(100),
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				got := test.flag.valid()

				if got != test.want {
					t.Errorf(
						"DeviceFlag.valid() = %v, want %v",
						got,
						test.want,
					)
				}
			},
		)
	}
}

func TestDeviceSessionStatusDomainValues(
	t *testing.T,
) {
	tests := []struct {
		name   string
		status DeviceSessionStatus
		want   int16
	}{
		{
			name:   "unknown",
			status: DeviceSessionStatusUnknown,
			want:   0,
		},
		{
			name:   "active",
			status: DeviceSessionStatusActive,
			want:   1,
		},
		{
			name:   "invalid",
			status: DeviceSessionStatusInvalid,
			want:   2,
		},
		{
			name:   "logged out",
			status: DeviceSessionStatusLoggedOut,
			want:   3,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				got := int16(test.status)

				if got != test.want {
					t.Errorf(
						"status value = %d, want %d",
						got,
						test.want,
					)
				}
			},
		)
	}
}

func TestNewRefreshToken(
	t *testing.T,
) {
	token, err := newRefreshToken(
		"practice-refresh-token",
	)
	if err != nil {
		t.Fatalf(
			"newRefreshToken() error = %v, want nil",
			err,
		)
	}

	if token.Value() !=
		"practice-refresh-token" {
		t.Errorf(
			"RefreshToken.Value() = %q, want %q",
			token.Value(),
			"practice-refresh-token",
		)
	}

	invalidValues := []string{
		"",
		" ",
		" practice-refresh-token",
		"practice-refresh-token ",
		"\tpractice-refresh-token",
	}

	for _, value := range invalidValues {
		t.Run(
			value,
			func(t *testing.T) {
				_, err := newRefreshToken(value)

				if !errors.Is(
					err,
					ErrRefreshTokenInvalid,
				) {
					t.Fatalf(
						"newRefreshToken(%q) error = %v, want %v",
						value,
						err,
						ErrRefreshTokenInvalid,
					)
				}
			},
		)
	}
}

func TestNewActiveDevice(
	t *testing.T,
) {
	location := time.FixedZone(
		"practice-zone",
		7*60*60,
	)
	issuedAt := time.Date(
		2026,
		time.October,
		16,
		15,
		30,
		0,
		123456789,
		location,
	)
	expiredAt := issuedAt.Add(
		30 * 24 * time.Hour,
	)
	token := mustRefreshToken(
		t,
		"practice-refresh-token",
	)

	device, err := newActiveDevice(
		activeDeviceInput{
			UserID:       " practice-user-001 ",
			DeviceID:     " practice-device-001 ",
			DeviceName:   " Practice Phone ",
			DeviceModel:  " Practice Model ",
			DeviceFlag:   DeviceFlagWeb,
			RefreshToken: token,
			IssuedAt:     issuedAt,
			ExpiredAt:    expiredAt,
		},
	)
	if err != nil {
		t.Fatalf(
			"newActiveDevice() error = %v, want nil",
			err,
		)
	}

	if device.UserID != "practice-user-001" {
		t.Errorf(
			"Device.UserID = %q, want %q",
			device.UserID,
			"practice-user-001",
		)
	}

	if device.DeviceID !=
		"practice-device-001" {
		t.Errorf(
			"Device.DeviceID = %q, want %q",
			device.DeviceID,
			"practice-device-001",
		)
	}

	if device.DeviceName != "Practice Phone" {
		t.Errorf(
			"Device.DeviceName = %q, want %q",
			device.DeviceName,
			"Practice Phone",
		)
	}

	if device.DeviceModel != "Practice Model" {
		t.Errorf(
			"Device.DeviceModel = %q, want %q",
			device.DeviceModel,
			"Practice Model",
		)
	}

	if device.DeviceFlag != DeviceFlagWeb {
		t.Errorf(
			"Device.DeviceFlag = %d, want %d",
			device.DeviceFlag,
			DeviceFlagWeb,
		)
	}

	if device.RefreshToken.Value() !=
		token.Value() {
		t.Errorf(
			"Device.RefreshToken = %q, want %q",
			device.RefreshToken.Value(),
			token.Value(),
		)
	}

	if device.SessionStatus !=
		DeviceSessionStatusActive {
		t.Errorf(
			"Device.SessionStatus = %d, want %d",
			device.SessionStatus,
			DeviceSessionStatusActive,
		)
	}

	if !device.IssuedAt.Equal(
		issuedAt,
	) {
		t.Errorf(
			"Device.IssuedAt = %v, want instant %v",
			device.IssuedAt,
			issuedAt,
		)
	}

	if !device.ExpiredAt.Equal(
		expiredAt,
	) {
		t.Errorf(
			"Device.ExpiredAt = %v, want instant %v",
			device.ExpiredAt,
			expiredAt,
		)
	}

	if device.IssuedAt.Location() != time.UTC ||
		device.ExpiredAt.Location() != time.UTC {
		t.Errorf(
			"device times are not normalized to UTC",
		)
	}
}

func TestNewActiveDeviceRejectsInvalidInput(
	t *testing.T,
) {
	issuedAt := time.Date(
		2026,
		time.October,
		16,
		8,
		0,
		0,
		0,
		time.UTC,
	)

	validInput := activeDeviceInput{
		UserID:      "practice-user-001",
		DeviceID:    "practice-device-001",
		DeviceName:  "Practice Phone",
		DeviceModel: "Practice Model",
		DeviceFlag:  DeviceFlagApp,
		RefreshToken: mustRefreshToken(
			t,
			"practice-refresh-token",
		),
		IssuedAt: issuedAt,
		ExpiredAt: issuedAt.Add(
			30 * 24 * time.Hour,
		),
	}

	tests := []struct {
		name    string
		change  func(*activeDeviceInput)
		wantErr error
	}{
		{
			name: "owner is missing",
			change: func(
				input *activeDeviceInput,
			) {
				input.UserID = " "
			},
			wantErr: ErrDeviceOwnerRequired,
		},
		{
			name: "device ID is missing",
			change: func(
				input *activeDeviceInput,
			) {
				input.DeviceID = "\t"
			},
			wantErr: ErrDeviceIDRequired,
		},
		{
			name: "device flag is invalid",
			change: func(
				input *activeDeviceInput,
			) {
				input.DeviceFlag =
					DeviceFlag(3)
			},
			wantErr: ErrDeviceFlagInvalid,
		},
		{
			name: "refresh token is missing",
			change: func(
				input *activeDeviceInput,
			) {
				input.RefreshToken =
					RefreshToken{}
			},
			wantErr: ErrRefreshTokenInvalid,
		},
		{
			name: "issued at is missing",
			change: func(
				input *activeDeviceInput,
			) {
				input.IssuedAt = time.Time{}
			},
			wantErr: ErrDeviceSessionIssuedAtRequired,
		},
		{
			name: "expired at is missing",
			change: func(
				input *activeDeviceInput,
			) {
				input.ExpiredAt = time.Time{}
			},
			wantErr: ErrDeviceSessionExpirationInvalid,
		},
		{
			name: "expiration equals issuance",
			change: func(
				input *activeDeviceInput,
			) {
				input.ExpiredAt =
					input.IssuedAt
			},
			wantErr: ErrDeviceSessionExpirationInvalid,
		},
		{
			name: "expiration precedes issuance",
			change: func(
				input *activeDeviceInput,
			) {
				input.ExpiredAt =
					input.IssuedAt.Add(
						-time.Second,
					)
			},
			wantErr: ErrDeviceSessionExpirationInvalid,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				input := validInput
				test.change(&input)

				device, err :=
					newActiveDevice(input)

				if !errors.Is(
					err,
					test.wantErr,
				) {
					t.Fatalf(
						"newActiveDevice() error = %v, want %v",
						err,
						test.wantErr,
					)
				}

				if device != (Device{}) {
					t.Error(
						"newActiveDevice() returned non-zero device for invalid input",
					)
				}
			},
		)
	}
}

func TestDeviceSessionActiveAt(
	t *testing.T,
) {
	issuedAt := time.Date(
		2026,
		time.October,
		16,
		8,
		0,
		0,
		0,
		time.UTC,
	)
	expiredAt := issuedAt.Add(time.Hour)

	baseDevice := Device{
		SessionStatus: DeviceSessionStatusActive,
		IssuedAt:      issuedAt,
		ExpiredAt:     expiredAt,
	}

	tests := []struct {
		name   string
		status DeviceSessionStatus
		now    time.Time
		want   bool
	}{
		{
			name:   "before issuance",
			status: DeviceSessionStatusActive,
			now:    issuedAt.Add(-time.Nanosecond),
		},
		{
			name:   "exact issuance",
			status: DeviceSessionStatusActive,
			now:    issuedAt,
			want:   true,
		},
		{
			name:   "between issuance and expiry",
			status: DeviceSessionStatusActive,
			now:    issuedAt.Add(30 * time.Minute),
			want:   true,
		},
		{
			name:   "exact expiry",
			status: DeviceSessionStatusActive,
			now:    expiredAt,
		},
		{
			name:   "after expiry",
			status: DeviceSessionStatusActive,
			now:    expiredAt.Add(time.Nanosecond),
		},
		{
			name:   "unknown status",
			status: DeviceSessionStatusUnknown,
			now:    issuedAt,
		},
		{
			name:   "invalid status",
			status: DeviceSessionStatusInvalid,
			now:    issuedAt,
		},
		{
			name:   "logged out status",
			status: DeviceSessionStatusLoggedOut,
			now:    issuedAt,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				device := baseDevice
				device.SessionStatus =
					test.status

				got := device.SessionActiveAt(
					test.now,
				)

				if got != test.want {
					t.Errorf(
						"SessionActiveAt() = %v, want %v",
						got,
						test.want,
					)
				}
			},
		)
	}
}

func TestNewActiveDeviceAllowsDeviceIDAcrossOwners(
	t *testing.T,
) {
	issuedAt := time.Date(
		2026,
		time.October,
		16,
		8,
		0,
		0,
		0,
		time.UTC,
	)
	token := mustRefreshToken(
		t,
		"practice-refresh-token",
	)

	for _, userID := range []string{
		"practice-user-001",
		"practice-user-002",
	} {
		device, err := newActiveDevice(
			activeDeviceInput{
				UserID:       userID,
				DeviceID:     "shared-device-id",
				DeviceFlag:   DeviceFlagApp,
				RefreshToken: token,
				IssuedAt:     issuedAt,
				ExpiredAt: issuedAt.Add(
					time.Hour,
				),
			},
		)
		if err != nil {
			t.Fatalf(
				"newActiveDevice(%q) error = %v, want nil",
				userID,
				err,
			)
		}

		if device.UserID != userID ||
			device.DeviceID !=
				"shared-device-id" {
			t.Errorf(
				"device ownership = %q/%q, want %q/%q",
				device.UserID,
				device.DeviceID,
				userID,
				"shared-device-id",
			)
		}
	}
}

func mustRefreshToken(
	t *testing.T,
	value string,
) RefreshToken {
	t.Helper()

	token, err := newRefreshToken(value)
	if err != nil {
		t.Fatalf(
			"newRefreshToken(%q) error = %v",
			value,
			err,
		)
	}

	return token
}
