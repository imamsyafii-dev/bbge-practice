package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/config"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

func TestAccountQueriesAgainstPostgres(t *testing.T) {
	if os.Getenv("PRACTICE_RUN_POSTGRES_TESTS") != "1" {
		t.Skip(
			"set PRACTICE_RUN_POSTGRES_TESTS=1 to run PostgreSQL integration tests",
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	cfg := config.Load()

	pool, err := postgres.Open(
		ctx,
		cfg.Postgres,
	)
	if err != nil {
		t.Fatalf(
			"open PostgreSQL: %v",
			err,
		)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf(
			"begin transaction: %v",
			err,
		)
	}

	defer func() {
		rollbackCtx, cancelRollback := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancelRollback()

		rollbackErr := tx.Rollback(rollbackCtx)
		if rollbackErr != nil &&
			!errors.Is(rollbackErr, pgx.ErrTxClosed) {
			t.Errorf(
				"rollback transaction during cleanup: %v",
				rollbackErr,
			)
		}
	}()

	queries := store.New(tx)

	input := store.CreateAccountParams{
		UserID: fmt.Sprintf(
			"practice-store-test-%d",
			time.Now().UnixNano(),
		),
		Name: "Practice Store Test",
	}

	created, err := queries.CreateAccount(
		ctx,
		input,
	)
	if err != nil {
		t.Fatalf(
			"create account: %v",
			err,
		)
	}

	if created.ID <= 0 {
		t.Fatalf(
			"created ID = %d, want positive value",
			created.ID,
		)
	}

	if created.UserID != input.UserID {
		t.Fatalf(
			"created user_id = %q, want %q",
			created.UserID,
			input.UserID,
		)
	}

	if created.Name != input.Name {
		t.Fatalf(
			"created name = %q, want %q",
			created.Name,
			input.Name,
		)
	}

	if created.Status != 1 {
		t.Fatalf(
			"created status = %d, want 1",
			created.Status,
		)
	}

	if created.Version != 0 {
		t.Fatalf(
			"created version = %d, want 0",
			created.Version,
		)
	}

	if !created.CreatedAt.Valid ||
		created.CreatedAt.Time.IsZero() {
		t.Fatal("created_at is not valid")
	}

	if !created.UpdatedAt.Valid ||
		created.UpdatedAt.Time.IsZero() {
		t.Fatal("updated_at is not valid")
	}

	found, err := queries.GetAccountByUserID(
		ctx,
		input.UserID,
	)
	if err != nil {
		t.Fatalf(
			"get account: %v",
			err,
		)
	}

	if found.ID != created.ID ||
		found.UserID != created.UserID ||
		found.Name != created.Name ||
		found.Status != created.Status ||
		found.Version != created.Version ||
		!found.CreatedAt.Time.Equal(created.CreatedAt.Time) ||
		!found.UpdatedAt.Time.Equal(created.UpdatedAt.Time) {
		t.Fatalf(
			"found account = %+v, want %+v",
			found,
			created,
		)
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf(
			"rollback transaction: %v",
			err,
		)
	}

	_, err = store.New(pool).GetAccountByUserID(
		ctx,
		input.UserID,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"get account after rollback error = %v, want pgx.ErrNoRows",
			err,
		)
	}
}
