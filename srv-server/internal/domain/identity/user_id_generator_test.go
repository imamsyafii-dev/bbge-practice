package identity

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

var errUserIDRandomRead = errors.New(
	"practice random reader failed",
)

type userIDReaderFunc func(
	buffer []byte,
) (int, error)

func (
	function userIDReaderFunc,
) Read(
	buffer []byte,
) (int, error) {
	return function(buffer)
}

func TestNewRandomUserIDGenerator(
	t *testing.T,
) {
	generator := NewRandomUserIDGenerator()

	if generator == nil {
		t.Fatal(
			"NewRandomUserIDGenerator() returned nil",
		)
	}

	if generator.random == nil {
		t.Fatal(
			"NewRandomUserIDGenerator() did not configure random reader",
		)
	}
}

func TestRandomUserIDGeneratorGenerate(
	t *testing.T,
) {
	randomInput := []byte{
		0x00, 0x01, 0x02, 0x03,
		0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b,
		0x0c, 0x0d, 0x0e, 0x0f,
	}

	generator := &RandomUserIDGenerator{
		random: bytes.NewReader(randomInput),
	}

	userID, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"Generate() error = %v, want nil",
			err,
		)
	}

	const wantUserID = "000102030405060708090a0b0c0d0e0f"

	if userID != wantUserID {
		t.Errorf(
			"Generate() = %q, want %q",
			userID,
			wantUserID,
		)
	}
}

func TestRandomUserIDGeneratorGenerateUsesFreshRandomness(
	t *testing.T,
) {
	randomInput := append(
		bytes.Repeat(
			[]byte{0x11},
			userIDRandomByteLength,
		),
		bytes.Repeat(
			[]byte{0x22},
			userIDRandomByteLength,
		)...,
	)

	generator := &RandomUserIDGenerator{
		random: bytes.NewReader(randomInput),
	}

	firstUserID, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"first Generate() error = %v, want nil",
			err,
		)
	}

	secondUserID, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"second Generate() error = %v, want nil",
			err,
		)
	}

	const wantFirstUserID = "11111111111111111111111111111111"
	const wantSecondUserID = "22222222222222222222222222222222"

	if firstUserID != wantFirstUserID {
		t.Errorf(
			"first Generate() = %q, want %q",
			firstUserID,
			wantFirstUserID,
		)
	}

	if secondUserID != wantSecondUserID {
		t.Errorf(
			"second Generate() = %q, want %q",
			secondUserID,
			wantSecondUserID,
		)
	}

	if firstUserID == secondUserID {
		t.Fatal(
			"successive Generate() calls returned the same user ID",
		)
	}
}

func TestRandomUserIDGeneratorRejectsMissingConfiguration(
	t *testing.T,
) {
	tests := []struct {
		name      string
		generator *RandomUserIDGenerator
	}{
		{
			name:      "nil receiver",
			generator: nil,
		},
		{
			name:      "nil random reader",
			generator: &RandomUserIDGenerator{},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := test.generator.Generate(
					context.Background(),
				)

				if !errors.Is(
					err,
					ErrUserIDGeneratorNotConfigured,
				) {
					t.Fatalf(
						"Generate() error = %v, want %v",
						err,
						ErrUserIDGeneratorNotConfigured,
					)
				}
			},
		)
	}
}

func TestRandomUserIDGeneratorForwardsReaderError(
	t *testing.T,
) {
	generator := &RandomUserIDGenerator{
		random: userIDReaderFunc(
			func(
				_ []byte,
			) (int, error) {
				return 0, errUserIDRandomRead
			},
		),
	}

	userID, err := generator.Generate(
		context.Background(),
	)

	if userID != "" {
		t.Errorf(
			"Generate() user ID = %q, want empty",
			userID,
		)
	}

	if !errors.Is(
		err,
		errUserIDRandomRead,
	) {
		t.Fatalf(
			"Generate() error = %v, want wrapped %v",
			err,
			errUserIDRandomRead,
		)
	}
}

func TestRandomUserIDGeneratorHonorsCanceledContext(
	t *testing.T,
) {
	t.Run(
		"before random read",
		func(t *testing.T) {
			ctx, cancel :=
				context.WithCancel(
					context.Background(),
				)
			cancel()

			readerCalled := false
			generator := &RandomUserIDGenerator{
				random: userIDReaderFunc(
					func(
						buffer []byte,
					) (int, error) {
						readerCalled = true

						return len(buffer), nil
					},
				),
			}

			userID, err := generator.Generate(ctx)

			if !errors.Is(
				err,
				context.Canceled,
			) {
				t.Fatalf(
					"Generate() error = %v, want %v",
					err,
					context.Canceled,
				)
			}

			if userID != "" {
				t.Errorf(
					"Generate() user ID = %q, want empty",
					userID,
				)
			}

			if readerCalled {
				t.Fatal(
					"Generate() read randomness after context cancellation",
				)
			}
		},
	)

	t.Run(
		"during random read",
		func(t *testing.T) {
			ctx, cancel :=
				context.WithCancel(
					context.Background(),
				)

			generator := &RandomUserIDGenerator{
				random: userIDReaderFunc(
					func(
						buffer []byte,
					) (int, error) {
						for index := range buffer {
							buffer[index] =
								byte(index)
						}

						cancel()

						return len(buffer), nil
					},
				),
			}

			userID, err := generator.Generate(ctx)

			if !errors.Is(
				err,
				context.Canceled,
			) {
				t.Fatalf(
					"Generate() error = %v, want %v",
					err,
					context.Canceled,
				)
			}

			if userID != "" {
				t.Errorf(
					"Generate() user ID = %q, want empty",
					userID,
				)
			}
		},
	)
}

func TestRegistrationInputsDoNotExposeUserID(
	t *testing.T,
) {
	tests := []struct {
		name  string
		input any
	}{
		{
			name:  "register input",
			input: RegisterInput{},
		},
		{
			name:  "password register input",
			input: PasswordRegisterInput{},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				inputType := reflect.TypeOf(
					test.input,
				)

				if _, found :=
					inputType.FieldByName(
						"UserID",
					); found {
					t.Fatalf(
						"%s exposes client-controlled UserID",
						inputType.Name(),
					)
				}
			},
		)
	}
}
