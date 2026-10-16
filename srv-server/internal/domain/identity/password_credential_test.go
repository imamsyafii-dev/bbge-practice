package identity

import (
	"context"
	"errors"
	"testing"
)

func TestNewEncodedPasswordHash(
	t *testing.T,
) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr error
	}{
		{
			name:  "valid encoded hash",
			value: "practice-encoded-password-hash",
			want:  "practice-encoded-password-hash",
		},
		{
			name:    "empty encoded hash",
			value:   "",
			wantErr: ErrPasswordHashInvalid,
		},
		{
			name:    "spaces only",
			value:   "   ",
			wantErr: ErrPasswordHashInvalid,
		},
		{
			name:    "whitespace only",
			value:   "\t\n",
			wantErr: ErrPasswordHashInvalid,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				hash, err :=
					newEncodedPasswordHash(
						test.value,
					)

				if test.wantErr != nil {
					if !errors.Is(err, test.wantErr) {
						t.Fatalf(
							"newEncodedPasswordHash() error = %v, want %v",
							err,
							test.wantErr,
						)
					}

					return
				}

				if err != nil {
					t.Fatalf(
						"newEncodedPasswordHash() error = %v, want nil",
						err,
					)
				}

				if hash.Encoded() != test.want {
					t.Errorf(
						"Encoded() = %q, want %q",
						hash.Encoded(),
						test.want,
					)
				}
			},
		)
	}
}

func TestPasswordCredentialUsesEncodedHash(
	t *testing.T,
) {
	hash, err := newEncodedPasswordHash(
		"practice-encoded-password-hash",
	)
	if err != nil {
		t.Fatalf(
			"newEncodedPasswordHash() error = %v",
			err,
		)
	}

	credential := PasswordCredential{
		UserID:                "practice-user-001",
		Username:              "practice_user",
		PasswordHash:          hash,
		RegistrationRequestID: "practice-request-0001",
	}

	if credential.UserID != "practice-user-001" {
		t.Errorf(
			"UserID = %q, want %q",
			credential.UserID,
			"practice-user-001",
		)
	}

	if credential.Username != "practice_user" {
		t.Errorf(
			"Username = %q, want %q",
			credential.Username,
			"practice_user",
		)
	}

	if credential.PasswordHash.Encoded() !=
		"practice-encoded-password-hash" {
		t.Errorf(
			"PasswordHash = %q, want %q",
			credential.PasswordHash.Encoded(),
			"practice-encoded-password-hash",
		)
	}

	if credential.RegistrationRequestID !=
		"practice-request-0001" {
		t.Errorf(
			"RegistrationRequestID = %q, want %q",
			credential.RegistrationRequestID,
			"practice-request-0001",
		)
	}
}

type fakePasswordHasher struct {
	result           EncodedPasswordHash
	err              error
	calls            int
	receivedContext  context.Context
	receivedPassword string
}

func (
	hasher *fakePasswordHasher,
) Hash(
	ctx context.Context,
	plaintext string,
) (EncodedPasswordHash, error) {
	hasher.calls++
	hasher.receivedContext = ctx
	hasher.receivedPassword = plaintext

	return hasher.result, hasher.err
}

var _ PasswordHasher = (*fakePasswordHasher)(nil)

type passwordHasherContextKey struct{}

func TestPasswordHasherBoundary(
	t *testing.T,
) {
	expectedHash, err := newEncodedPasswordHash(
		"practice-fake-encoded-hash",
	)
	if err != nil {
		t.Fatalf(
			"newEncodedPasswordHash() error = %v",
			err,
		)
	}

	ctx := context.WithValue(
		context.Background(),
		passwordHasherContextKey{},
		"practice-context",
	)
	plaintext := " practice-password "

	fake := &fakePasswordHasher{
		result: expectedHash,
	}

	var hasher PasswordHasher = fake

	result, err := hasher.Hash(
		ctx,
		plaintext,
	)
	if err != nil {
		t.Fatalf(
			"Hash() error = %v, want nil",
			err,
		)
	}

	if result.Encoded() != expectedHash.Encoded() {
		t.Errorf(
			"Hash() = %q, want %q",
			result.Encoded(),
			expectedHash.Encoded(),
		)
	}

	if fake.calls != 1 {
		t.Errorf(
			"Hash() calls = %d, want 1",
			fake.calls,
		)
	}

	if fake.receivedPassword != plaintext {
		t.Errorf(
			"received password = %q, want %q",
			fake.receivedPassword,
			plaintext,
		)
	}

	if fake.receivedContext.Value(
		passwordHasherContextKey{},
	) != "practice-context" {
		t.Errorf(
			"context value was not forwarded",
		)
	}
}
