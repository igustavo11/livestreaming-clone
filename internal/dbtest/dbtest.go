package dbtest

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/dbmigrate"
)

func Setup(ctx context.Context) (*pgxpool.Pool, *db.Queries, func(), error) {
	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("dbtest: start postgres: %w", err)
	}

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, nil, nil, fmt.Errorf("dbtest: connection string: %w", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, nil, nil, fmt.Errorf("dbtest: pool: %w", err)
	}

	if err := waitReady(ctx, pool); err != nil {
		pool.Close()
		_ = ctr.Terminate(ctx)
		return nil, nil, nil, fmt.Errorf("dbtest: wait ready: %w", err)
	}

	if err := dbmigrate.Up(dsn); err != nil {
		pool.Close()
		_ = ctr.Terminate(ctx)
		return nil, nil, nil, fmt.Errorf("dbtest: migrate: %w", err)
	}

	cleanup := func() {
		pool.Close()
		_ = ctr.Terminate(context.Background())
	}

	return pool, db.New(pool), cleanup, nil
}

func waitReady(ctx context.Context, pool *pgxpool.Pool) error {
	var lastErr error
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return lastErr
}
