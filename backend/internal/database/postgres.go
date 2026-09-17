package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(
	ctx context.Context,
	databaseURL string,
) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf(
			"analizar configuracion PostgreSQL: %w",
			err,
		)
	}

	config.MaxConns = 5
	config.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf(
			"crear pool de conexiones PostgreSQL: %w",
			err,
		)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf(
			"ping a la base de datos PostgreSQL: %w",
			err,
		)
	}

	return pool, nil
}
