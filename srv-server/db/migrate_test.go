package db

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestAutoMigrateRejectsNilPool(
	t *testing.T,
) {
	err := AutoMigrate(nil)
	if err == nil {
		t.Fatal(
			"AutoMigrate(nil) error = nil, want non-nil",
		)
	}

	const want = "postgres pool is nil"

	if got := err.Error(); got != want {
		t.Fatalf(
			"AutoMigrate(nil) error = %q, want %q",
			got,
			want,
		)
	}
}

func TestMigrateRejectsNilDatabase(
	t *testing.T,
) {
	err := Migrate(nil)
	if err == nil {
		t.Fatal(
			"Migrate(nil) error = nil, want non-nil",
		)
	}

	const want = "sql database is nil"

	if got := err.Error(); got != want {
		t.Fatalf(
			"Migrate(nil) error = %q, want %q",
			got,
			want,
		)
	}
}

func TestRunMigrationsWrapsUpError(
	t *testing.T,
) {
	database := &sql.DB{}

	upErr := errors.New(
		"practice migration failure",
	)

	called := false

	fakeUp := func(
		gotDatabase *sql.DB,
		gotDirectory string,
		options ...goose.OptionsFunc,
	) error {
		called = true

		if gotDatabase != database {
			t.Errorf(
				"database = %p, want %p",
				gotDatabase,
				database,
			)
		}

		const wantDirectory = "migrations"

		if gotDirectory != wantDirectory {
			t.Errorf(
				"directory = %q, want %q",
				gotDirectory,
				wantDirectory,
			)
		}

		if len(options) != 0 {
			t.Errorf(
				"options count = %d, want 0",
				len(options),
			)
		}

		return upErr
	}

	err := runMigrations(
		database,
		fakeUp,
	)

	if !called {
		t.Fatal("up was not called")
	}

	if err == nil {
		t.Fatal(
			"runMigrations() error = nil, want non-nil",
		)
	}

	if !errors.Is(err, upErr) {
		t.Fatalf(
			"errors.Is(error, upErr) = false; error = %v",
			err,
		)
	}

	const wantPrefix = "run migrations: "

	if got := err.Error(); !strings.HasPrefix(
		got,
		wantPrefix,
	) {
		t.Fatalf(
			"error = %q, want prefix %q",
			got,
			wantPrefix,
		)
	}
}
