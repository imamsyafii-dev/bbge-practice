package identity

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeLoginUsername(
	t *testing.T,
) {
	username, err :=
		NormalizeLoginUsername("Practice_User123")
	if err != nil {
		t.Fatalf(
			"NormalizeLoginUsername() error = %v, want nil",
			err,
		)
	}

	if username != "practice_user123" {
		t.Fatalf(
			"NormalizeLoginUsername() = %q, want %q",
			username,
			"practice_user123",
		)
	}

	invalidUsernames := []string{
		"ab",
		strings.Repeat("a", 33),
		" practice_user",
		"practice-user",
		"pengguna界",
	}

	for _, value := range invalidUsernames {
		_, err := NormalizeLoginUsername(value)

		if !errors.Is(err, ErrUsernameInput) {
			t.Errorf(
				"NormalizeLoginUsername(%q) error = %v, want %v",
				value,
				err,
				ErrUsernameInput,
			)
		}
	}
}

func TestValidateLoginPassword(
	t *testing.T,
) {
	validPasswords := []string{
		strings.Repeat("a", 15),
		strings.Repeat("界", 15),
		strings.Repeat(" ", 15),
		strings.Repeat("a", 128),
	}

	for _, password := range validPasswords {
		if err := validateLoginPassword(
			password,
		); err != nil {
			t.Errorf(
				"validateLoginPassword() error = %v, want nil",
				err,
			)
		}
	}

	invalidPasswords := []string{
		strings.Repeat("a", 14),
		strings.Repeat("界", 129),
		string([]byte{0xff}),
	}

	for _, password := range invalidPasswords {
		err := validateLoginPassword(password)

		if !errors.Is(err, ErrPasswordInput) {
			t.Errorf(
				"validateLoginPassword() error = %v, want %v",
				err,
				ErrPasswordInput,
			)
		}
	}
}

func TestValidatePasswordRegisterInput(
	t *testing.T,
) {
	validInput := func() PasswordRegisterInput {
		return PasswordRegisterInput{
			Username:     "Practice_User",
			Password:     " practice-password ",
			Registration: validRegisterInput(),
			RequestID:    "practice-request-0001",
		}
	}

	tests := []struct {
		name    string
		change  func(*PasswordRegisterInput)
		wantErr error
	}{
		{
			name: "valid input",
		},
		{
			name: "minimum request ID length",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.RequestID =
					strings.Repeat("r", 16)
			},
		},
		{
			name: "maximum request ID length",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.RequestID =
					strings.Repeat("r", 64)
			},
		},
		{
			name: "invalid username",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.Username = "practice-user"
			},
			wantErr: ErrUsernameInput,
		},
		{
			name: "invalid password",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.Password =
					strings.Repeat("a", 14)
			},
			wantErr: ErrPasswordInput,
		},
		{
			name: "request ID is too short",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.RequestID =
					strings.Repeat("r", 15)
			},
			wantErr: ErrRegistrationRequestIDInvalid,
		},
		{
			name: "request ID is too long",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.RequestID =
					strings.Repeat("r", 65)
			},
			wantErr: ErrRegistrationRequestIDInvalid,
		},
		{
			name: "request ID has surrounding whitespace",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.RequestID =
					" " + strings.Repeat("r", 16)
			},
			wantErr: ErrRegistrationRequestIDInvalid,
		},
		{
			name: "nested registration is invalid",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.Registration.DeviceID = " "
			},
			wantErr: ErrDeviceIDRequired,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				input := validInput()

				if test.change != nil {
					test.change(&input)
				}

				originalUsername := input.Username
				originalPassword := input.Password

				result, err :=
					validatePasswordRegisterInput(
						input,
					)

				if test.wantErr != nil {
					if !errors.Is(err, test.wantErr) {
						t.Fatalf(
							"validatePasswordRegisterInput() error = %v, want %v",
							err,
							test.wantErr,
						)
					}

					return
				}

				if err != nil {
					t.Fatalf(
						"validatePasswordRegisterInput() error = %v, want nil",
						err,
					)
				}

				if result.Username !=
					strings.ToLower(originalUsername) {
					t.Errorf(
						"result.Username = %q, want %q",
						result.Username,
						strings.ToLower(
							originalUsername,
						),
					)
				}

				if result.Password != originalPassword {
					t.Errorf(
						"password was modified",
					)
				}

				if input.Username != originalUsername {
					t.Errorf(
						"original input was modified",
					)
				}
			},
		)
	}
}
