package postgres

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	_ "github.com/lib/pq"
	"github.com/w-h-a/workflow/internal/migration/clients/migrator"
)

type postgresMigrator struct {
	options migrator.Options
	conn    *sql.DB
}

func (m *postgresMigrator) Migrate(opts ...migrator.MigrateOption) error {
	schema, err := os.ReadFile("internal/migration/clients/migrator/postgres/migrations/schema.sql")
	if err != nil {
		return err
	}

	if _, err := m.conn.Exec(string(schema)); err != nil {
		return err
	}

	return nil
}

func NewMigrator(opts ...migrator.Option) migrator.Migrator {
	options := migrator.NewOptions(opts...)

	m := &postgresMigrator{
		options: options,
	}

	// postgres://user:password@host:port/db?sslmode=disable
	conn, err := sql.Open("postgres", m.options.Location)
	if err != nil {
		detail := "failed to connect with postgres migrator"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(detail)
	}

	if err := conn.Ping(); err != nil {
		detail := "failed to ping with postgres migrator"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(detail)
	}

	m.conn = conn

	return m
}
