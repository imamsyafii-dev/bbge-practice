package identity

import (
	"errors"
	"testing"
)

func TestValidateRegisterInput(
	t *testing.T,
) {
	tests := []struct {
		name    string
		change  func(*RegisterInput)
		wantErr error
	}{
		{
			name: "valid input",
		},
		{
			name: "password is empty",
			change: func(input *RegisterInput) {
				input.Password = "   "
			},
			wantErr: ErrPasswordRequired,
		},
		{
			name: "device ID is empty",
			change: func(input *RegisterInput) {
				input.DeviceID = "\t"
			},
			wantErr: ErrDeviceIDRequired,
		},
		{
			name: "X25519 identifier has invalid size",
			change: func(input *RegisterInput) {
				input.IdentifierX25519 = make(
					[]byte,
					x25519PublicKeySize-1,
				)
			},
			wantErr: ErrIdentityIdentifiersInvalid,
		},
		{
			name: "Ed25519 identifier has invalid size",
			change: func(input *RegisterInput) {
				input.IdentifierEd25519 = make(
					[]byte,
					ed25519PublicKeySizeForTest-1,
				)
			},
			wantErr: ErrIdentityIdentifiersInvalid,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				input := validRegisterInput()

				if test.change != nil {
					test.change(&input)
				}

				err := validateRegisterInput(input)

				if test.wantErr == nil {
					if err != nil {
						t.Fatalf(
							"validateRegisterInput() error = %v, want nil",
							err,
						)
					}

					return
				}

				if !errors.Is(err, test.wantErr) {
					t.Fatalf(
						"validateRegisterInput() error = %v, want %v",
						err,
						test.wantErr,
					)
				}
			},
		)
	}
}

const ed25519PublicKeySizeForTest = 32

func validRegisterInput() RegisterInput {
	return RegisterInput{
		Password:    "practice-password",
		DeviceID:    "practice-device-001",
		DeviceName:  "Practice Phone",
		DeviceModel: "Practice Model",
		DeviceFlag:  1,
		IdentifierX25519: make(
			[]byte,
			x25519PublicKeySize,
		),
		IdentifierEd25519: make(
			[]byte,
			ed25519PublicKeySizeForTest,
		),
	}
}
