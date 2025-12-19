package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type PostgresStorage struct {
	DB *pgxpool.Pool
}

func NewPostgresStorage(db *pgxpool.Pool) *PostgresStorage {
	return &PostgresStorage{DB: db}
}

func (p *PostgresConfig) GetDSN() string {
	return "postgres://" + p.User + ":" + p.Password + "@" + p.Host + ":" + p.Port + "/" + p.DBName + "?sslmode=" + p.SSLMode
}

func NewPostgresConfig(host, port, user, password, dbname, sslmode string) *PostgresConfig {
	return &PostgresConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbname,
		SSLMode:  sslmode,
	}
}

func (p *PostgresStorage) Close() {
	p.DB.Close()
}

func NewPostgresPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	const op = "repositories.postgres.NewPostgresPool"
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create connection pool: %w", op, err)
	}
	const maxAttempts = 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err = pool.Ping(ctx); err == nil {
			return pool, nil
		}
		fmt.Printf("[%s] Attempt %d: failed to connect to database: %v. Retrying in 5 seconds...\n", op, attempt, err)
		time.Sleep(5 * time.Second)
	}
	return nil, fmt.Errorf("%s: failed to connect to database after %d attempts: %w", op, maxAttempts, err)
}

func (p *PostgresStorage) Ping(ctx context.Context) error {
	return p.DB.Ping(ctx)
}
