package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func AutoMigrate(
	pool *pgxpool.Pool,
) error {
	if pool == nil {
		return fmt.Errorf("postgres pool is nil")
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	return Migrate(sqlDB)
}

func Migrate(
	database *sql.DB,
) error {
	if database == nil {
		return fmt.Errorf("sql database is nil")
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf(
			"set goose dialect: %w",
			err,
		)
	}

	goose.SetBaseFS(migrations)

	if err := goose.Up(
		database,
		"migrations",
	); err != nil {
		return fmt.Errorf(
			"run migrations: %w",
			err,
		)
	}

	return nil
}
