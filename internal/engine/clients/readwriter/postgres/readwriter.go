package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	_ "github.com/lib/pq"
	"github.com/w-h-a/workflow/internal/engine/clients/reader"
	"github.com/w-h-a/workflow/internal/engine/clients/readwriter"
	"github.com/w-h-a/workflow/internal/engine/clients/writer"
	"go.nhat.io/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

var DRIVER string

func init() {
	driver, err := otelsql.Register(
		"postgres",
		otelsql.TraceQueryWithoutArgs(),
		otelsql.TraceRowsClose(),
		otelsql.TraceRowsAffected(),
		otelsql.WithSystem(semconv.DBSystemPostgreSQL),
	)
	if err != nil {
		detail := "failed to register postgres readwriter with otel"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(detail)
	}

	DRIVER = driver
}

type postgresReadWriter struct {
	options readwriter.Options
	conn    *sql.DB
}

func (rw *postgresReadWriter) Read(ctx context.Context, opts ...reader.ReadOption) (*reader.Page, error) {
	options := reader.NewReadOptions(opts...)

	offset := (options.Page - 1) * options.Size

	rows, err := rw.conn.QueryContext(
		ctx,
		`SELECT count(*) OVER(), id, value FROM tasks ORDER BY created_at DESC OFFSET $1 LIMIT $2;`,
		offset,
		options.Size,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	totalRecords := 0
	results := [][]byte{}

	for rows.Next() {
		record := &reader.Record{}

		if err := rows.Scan(&totalRecords, &record.Id, &record.Value); err != nil {
			return nil, err
		}

		results = append(results, append([]byte{}, record.Value...))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	totalPages := (totalRecords + options.Size - 1) / options.Size

	return &reader.Page{
		Items:      results,
		Size:       len(results),
		Number:     options.Page,
		TotalPages: totalPages,
		TotalItems: totalRecords,
	}, nil
}

func (rw *postgresReadWriter) ReadById(ctx context.Context, id string, opts ...reader.ReadByIdOption) ([]byte, error) {
	row := rw.conn.QueryRowContext(
		ctx,
		`SELECT id, value FROM tasks WHERE id = $1;`,
		id,
	)

	record := &reader.Record{}

	if err := row.Scan(&record.Id, &record.Value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, reader.ErrRecordNotFound
		}
		return nil, err
	}

	return record.Value, nil
}

func (rw *postgresReadWriter) CheckHealth(ctx context.Context) error {
	return rw.conn.PingContext(ctx)
}

func (rw *postgresReadWriter) Write(ctx context.Context, id string, data []byte, opts ...writer.WriteOption) error {
	if _, err := rw.conn.ExecContext(
		ctx,
		`INSERT INTO tasks (id, value) VALUES ($1, $2::bytea) ON CONFLICT (id) DO UPDATE SET value = EXCLUDED.value;`,
		id,
		data,
	); err != nil {
		return err
	}

	return nil
}

func NewReadWriter(opts ...readwriter.Option) readwriter.ReadWriter {
	options := readwriter.NewOptions(opts...)

	// TODO: validate options

	rw := &postgresReadWriter{
		options: options,
	}

	// postgres://user:password@host:port/db?sslmode=disable
	conn, err := sql.Open(DRIVER, rw.options.Location)
	if err != nil {
		detail := "failed to connect with postgres readwriter"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(detail)
	}

	if err := conn.Ping(); err != nil {
		detail := "failed to ping with postgres readwriter"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(detail)
	}

	if err := otelsql.RecordStats(conn); err != nil {
		detail := "failed to initialize postgres instrumentation for postgres readwriter"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(detail)
	}

	rw.conn = conn

	return rw
}
