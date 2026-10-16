package identity

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"testing/iotest"
)

var errRefreshTokenRandomRead = errors.New(
	"practice refresh token random reader failed",
)

type refreshTokenReaderFunc func(
	buffer []byte,
) (int, error)

func (
	function refreshTokenReaderFunc,
) Read(
	buffer []byte,
) (int, error) {
	return function(buffer)
}

func TestNewRandomRefreshTokenGenerator(
	t *testing.T,
) {
	generator :=
		NewRandomRefreshTokenGenerator()

	if generator == nil {
		t.Fatal(
			"NewRandomRefreshTokenGenerator() returned nil",
		)
	}

	if generator.random == nil {
		t.Fatal(
			"NewRandomRefreshTokenGenerator() did not configure random reader",
		)
	}
}

func TestRandomRefreshTokenGeneratorGenerate(
	t *testing.T,
) {
	randomInput := make(
		[]byte,
		refreshTokenRandomByteLength,
	)
	for index := range randomInput {
		randomInput[index] = byte(index)
	}

	generator := &RandomRefreshTokenGenerator{
		random: iotest.OneByteReader(
			bytes.NewReader(randomInput),
		),
	}

	token, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"Generate() error = %v, want nil",
			err,
		)
	}

	const wantToken = "000102030405060708090a0b0c0d0e0f" +
		"101112131415161718191a1b1c1d1e1f"

	if token.Value() != wantToken {
		t.Errorf(
			"Generate() = %q, want %q",
			token.Value(),
			wantToken,
		)
	}
}

func TestRandomRefreshTokenGeneratorUsesFreshRandomness(
	t *testing.T,
) {
	randomInput := append(
		bytes.Repeat(
			[]byte{0x11},
			refreshTokenRandomByteLength,
		),
		bytes.Repeat(
			[]byte{0x22},
			refreshTokenRandomByteLength,
		)...,
	)

	generator := &RandomRefreshTokenGenerator{
		random: bytes.NewReader(randomInput),
	}

	firstToken, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"first Generate() error = %v, want nil",
			err,
		)
	}

	secondToken, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"second Generate() error = %v, want nil",
			err,
		)
	}

	wantFirstToken := strings.Repeat(
		"11",
		refreshTokenRandomByteLength,
	)
	wantSecondToken := strings.Repeat(
		"22",
		refreshTokenRandomByteLength,
	)

	if firstToken.Value() != wantFirstToken {
		t.Errorf(
			"first Generate() = %q, want %q",
			firstToken.Value(),
			wantFirstToken,
		)
	}

	if secondToken.Value() != wantSecondToken {
		t.Errorf(
			"second Generate() = %q, want %q",
			secondToken.Value(),
			wantSecondToken,
		)
	}

	if firstToken.Value() ==
		secondToken.Value() {
		t.Fatal(
			"successive Generate() calls returned the same token",
		)
	}
}

func TestRandomRefreshTokenGeneratorRejectsMissingConfiguration(
	t *testing.T,
) {
	tests := []struct {
		name      string
		generator *RandomRefreshTokenGenerator
	}{
		{
			name:      "nil receiver",
			generator: nil,
		},
		{
			name:      "nil random reader",
			generator: &RandomRefreshTokenGenerator{},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				token, err :=
					test.generator.Generate(
						context.Background(),
					)

				if token.Value() != "" {
					t.Errorf(
						"Generate() token = %q, want empty",
						token.Value(),
					)
				}

				if !errors.Is(
					err,
					ErrRefreshTokenGeneratorNotConfigured,
				) {
					t.Fatalf(
						"Generate() error = %v, want %v",
						err,
						ErrRefreshTokenGeneratorNotConfigured,
					)
				}
			},
		)
	}
}

func TestRandomRefreshTokenGeneratorForwardsReaderError(
	t *testing.T,
) {
	generator := &RandomRefreshTokenGenerator{
		random: refreshTokenReaderFunc(
			func(
				_ []byte,
			) (int, error) {
				return 0,
					errRefreshTokenRandomRead
			},
		),
	}

	token, err := generator.Generate(
		context.Background(),
	)

	if token.Value() != "" {
		t.Errorf(
			"Generate() token = %q, want empty",
			token.Value(),
		)
	}

	if !errors.Is(
		err,
		errRefreshTokenRandomRead,
	) {
		t.Fatalf(
			"Generate() error = %v, want wrapped %v",
			err,
			errRefreshTokenRandomRead,
		)
	}
}

func TestRandomRefreshTokenGeneratorHonorsCanceledContext(
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
			generator :=
				&RandomRefreshTokenGenerator{
					random: refreshTokenReaderFunc(
						func(
							buffer []byte,
						) (int, error) {
							readerCalled = true

							return len(buffer), nil
						},
					),
				}

			token, err :=
				generator.Generate(ctx)

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

			if token.Value() != "" {
				t.Errorf(
					"Generate() token = %q, want empty",
					token.Value(),
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

			generator :=
				&RandomRefreshTokenGenerator{
					random: refreshTokenReaderFunc(
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

			token, err :=
				generator.Generate(ctx)

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

			if token.Value() != "" {
				t.Errorf(
					"Generate() token = %q, want empty",
					token.Value(),
				)
			}
		},
	)
}
