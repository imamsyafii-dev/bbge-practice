package db

import "testing"

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
