package identity

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestNewArgon2IDPasswordHasher(
	t *testing.T,
) {
	hasher, err :=
		NewArgon2IDPasswordHasher(1)
	if err != nil {
		t.Fatalf(
			"NewArgon2IDPasswordHasher() error = %v, want nil",
			err,
		)
	}

	if hasher == nil {
		t.Fatal(
			"NewArgon2IDPasswordHasher() returned nil",
		)
	}

	for _, concurrency := range []int{0, -1} {
		t.Run(
			"invalid concurrency",
			func(t *testing.T) {
				_, err :=
					NewArgon2IDPasswordHasher(
						concurrency,
					)

				if !errors.Is(
					err,
					ErrPasswordHasherConcurrencyInvalid,
				) {
					t.Fatalf(
						"NewArgon2IDPasswordHasher() error = %v, want %v",
						err,
						ErrPasswordHasherConcurrencyInvalid,
					)
				}
			},
		)
	}
}

func TestArgon2IDPasswordHasherHashAndVerify(
	t *testing.T,
) {
	hasher := mustNewArgon2IDPasswordHasher(
		t,
		1,
	)
	ctx := context.Background()
	password := " practice-password "

	firstHash, err := hasher.Hash(
		ctx,
		password,
	)
	if err != nil {
		t.Fatalf(
			"Hash() error = %v, want nil",
			err,
		)
	}

	if firstHash.Encoded() == password {
		t.Fatal(
			"Hash() returned plaintext password",
		)
	}

	if !strings.HasPrefix(
		firstHash.Encoded(),
		passwordHashPrefix,
	) {
		t.Fatalf(
			"Hash() = %q, want prefix %q",
			firstHash.Encoded(),
			passwordHashPrefix,
		)
	}

	secondHash, err := hasher.Hash(
		ctx,
		password,
	)
	if err != nil {
		t.Fatalf(
			"second Hash() error = %v, want nil",
			err,
		)
	}

	if firstHash.Encoded() ==
		secondHash.Encoded() {
		t.Fatal(
			"same password produced identical hashes",
		)
	}

	tests := []struct {
		name      string
		plaintext string
		want      bool
	}{
		{
			name:      "correct password",
			plaintext: password,
			want:      true,
		},
		{
			name: "trimmed password",
			plaintext: strings.TrimSpace(
				password,
			),
			want: false,
		},
		{
			name:      "wrong password",
			plaintext: "wrong-practice-password",
			want:      false,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				verified, err := hasher.Verify(
					ctx,
					test.plaintext,
					firstHash,
				)
				if err != nil {
					t.Fatalf(
						"Verify() error = %v, want nil",
						err,
					)
				}

				if verified != test.want {
					t.Errorf(
						"Verify() = %v, want %v",
						verified,
						test.want,
					)
				}
			},
		)
	}
}

func TestParseEncodedPasswordHash(
	t *testing.T,
) {
	hasher := mustNewArgon2IDPasswordHasher(
		t,
		1,
	)

	validHash, err := hasher.Hash(
		context.Background(),
		"practice-password",
	)
	if err != nil {
		t.Fatalf(
			"Hash() error = %v",
			err,
		)
	}

	invalidBase64 :=
		passwordHashPrefix +
			strings.Repeat(
				"!",
				base64.RawStdEncoding.EncodedLen(
					passwordHashSaltLength,
				),
			) +
			"$" +
			strings.Repeat(
				"!",
				base64.RawStdEncoding.EncodedLen(
					passwordHashKeyLength,
				),
			)

	tests := []struct {
		name      string
		value     string
		wantValid bool
	}{
		{
			name:      "valid encoded hash",
			value:     validHash.Encoded(),
			wantValid: true,
		},
		{
			name: "wrong algorithm",
			value: strings.Replace(
				validHash.Encoded(),
				"argon2id",
				"argon2zz",
				1,
			),
		},
		{
			name: "wrong version",
			value: strings.Replace(
				validHash.Encoded(),
				"v=19",
				"v=18",
				1,
			),
		},
		{
			name: "wrong fixed parameters",
			value: strings.Replace(
				validHash.Encoded(),
				"m=19456,t=2,p=1",
				"m=19457,t=2,p=1",
				1,
			),
		},
		{
			name: "hostile parameters",
			value: strings.Replace(
				validHash.Encoded(),
				"m=19456,t=2,p=1",
				"m=999999999,t=99,p=255",
				1,
			),
		},
		{
			name:  "invalid base64",
			value: invalidBase64,
		},
		{
			name: "salt has wrong size",
			value: passwordHashPrefix +
				base64.RawStdEncoding.EncodeToString(
					make([]byte, 15),
				) +
				"$" +
				base64.RawStdEncoding.EncodeToString(
					make([]byte, 32),
				),
		},
		{
			name: "key has wrong size",
			value: passwordHashPrefix +
				base64.RawStdEncoding.EncodeToString(
					make([]byte, 16),
				) +
				"$" +
				base64.RawStdEncoding.EncodeToString(
					make([]byte, 31),
				),
		},
		{
			name: "padded base64",
			value: validHash.Encoded() +
				"=",
		},
		{
			name: "extra field",
			value: validHash.Encoded() +
				"$extra",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				salt, expected, valid :=
					parseEncodedPasswordHash(
						test.value,
					)
				defer clear(salt)
				defer clear(expected)

				if valid != test.wantValid {
					t.Errorf(
						"parseEncodedPasswordHash() valid = %v, want %v",
						valid,
						test.wantValid,
					)
				}

				if len(salt) !=
					passwordHashSaltLength {
					t.Errorf(
						"salt length = %d, want %d",
						len(salt),
						passwordHashSaltLength,
					)
				}

				if len(expected) !=
					passwordHashKeyLength {
					t.Errorf(
						"key length = %d, want %d",
						len(expected),
						passwordHashKeyLength,
					)
				}
			},
		)
	}
}

func TestArgon2IDPasswordHasherMalformedHashUsesDummyWork(
	t *testing.T,
) {
	hasher := mustNewArgon2IDPasswordHasher(
		t,
		1,
	)

	verified, err := hasher.Verify(
		context.Background(),
		"practice-password",
		EncodedPasswordHash{},
	)
	if err != nil {
		t.Fatalf(
			"Verify() error = %v, want nil",
			err,
		)
	}

	if verified {
		t.Fatal(
			"Verify() accepted zero-value hash",
		)
	}

	malformed, err := newEncodedPasswordHash(
		"$argon2id$v=19$m=999999999,t=99,p=255$x$x",
	)
	if err != nil {
		t.Fatalf(
			"newEncodedPasswordHash() error = %v",
			err,
		)
	}

	hasher.slots <- struct{}{}

	_, busyErr := hasher.Verify(
		context.Background(),
		"practice-password",
		malformed,
	)

	<-hasher.slots

	if !errors.Is(
		busyErr,
		ErrPasswordHasherBusy,
	) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			busyErr,
			ErrPasswordHasherBusy,
		)
	}
}

func TestArgon2IDPasswordHasherCapacityAndCancellation(
	t *testing.T,
) {
	hasher := mustNewArgon2IDPasswordHasher(
		t,
		1,
	)

	hasher.slots <- struct{}{}

	_, busyErr := hasher.Hash(
		context.Background(),
		"practice-password",
	)

	<-hasher.slots

	if !errors.Is(
		busyErr,
		ErrPasswordHasherBusy,
	) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			busyErr,
			ErrPasswordHasherBusy,
		)
	}

	ctx, cancel :=
		context.WithCancel(
			context.Background(),
		)
	cancel()

	_, err := hasher.Hash(
		ctx,
		"practice-password",
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			err,
			context.Canceled,
		)
	}

	_, err = hasher.Verify(
		ctx,
		"practice-password",
		EncodedPasswordHash{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			context.Canceled,
		)
	}
}

func TestArgon2IDPasswordHasherNilReceiver(
	t *testing.T,
) {
	var hasher *Argon2IDPasswordHasher

	_, err := hasher.Hash(
		context.Background(),
		"practice-password",
	)
	if !errors.Is(
		err,
		ErrPasswordHasherNotConfigured,
	) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			err,
			ErrPasswordHasherNotConfigured,
		)
	}

	_, err = hasher.Verify(
		context.Background(),
		"practice-password",
		EncodedPasswordHash{},
	)
	if !errors.Is(
		err,
		ErrPasswordHasherNotConfigured,
	) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			ErrPasswordHasherNotConfigured,
		)
	}
}

func mustNewArgon2IDPasswordHasher(
	t *testing.T,
	concurrency int,
) *Argon2IDPasswordHasher {
	t.Helper()

	hasher, err :=
		NewArgon2IDPasswordHasher(
			concurrency,
		)
	if err != nil {
		t.Fatalf(
			"NewArgon2IDPasswordHasher() error = %v",
			err,
		)
	}

	return hasher
}
