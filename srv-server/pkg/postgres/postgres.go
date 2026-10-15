package postgres

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(
	ctx context.Context,
	cfg config.PostgresConfig,
) (*pgxpool.Pool, error) {
	connectionURL := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			cfg.User,
			cfg.Password,
		),
		Host: net.JoinHostPort(
			cfg.Host,
			cfg.Port,
		),
		Path: cfg.Database,
	}

	query := connectionURL.Query()
	query.Set("sslmode", cfg.SSLMode)
	connectionURL.RawQuery = query.Encode()

	pool, err := pgxpool.New(
		ctx,
		connectionURL.String(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create PostgreSQL pool: %w",
			err,
		)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf(
			"ping PostgreSQL: %w",
			err,
		)
	}

	return pool, nil
}
