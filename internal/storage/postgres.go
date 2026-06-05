package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrDuplicateOriginalURL = errors.New("duplicate original url")

type Database interface {
	Ping(context.Context) error
	Close(context.Context) error
}

type BatchItem struct {
	ID        string
	OriginURL string
}

type Postgres struct {
	Conn *pgx.Conn
}

func (p *Postgres) Ping(ctx context.Context) error {
	return p.Conn.Ping(ctx)
}

func (p *Postgres) Close(ctx context.Context) error {
	return p.Conn.Close(ctx)
}

func NewPostgres(ctx context.Context, connString string) (*Postgres, error) {
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return nil, err
	}

	return &Postgres{
		Conn: conn,
	}, nil
}

func (p *Postgres) Get(key string) (string, bool) {
	var originalURL string
	err := p.Conn.QueryRow(
		context.Background(),
		"SELECT original_url FROM short_urls WHERE short_url_id = $1",
		key,
	).Scan(&originalURL)

	if err != nil {
		return "", false
	}

	return originalURL, true
}

func (p *Postgres) Set(key string, value string) (string, error) {
	var pgErr *pgconn.PgError

	_, err := p.Conn.Exec(context.Background(), "INSERT INTO short_urls(short_url_id, original_url) VALUES($1, $2)", key, value)

	if errors.As(err, &pgErr) {
		if pgErr.Code == pgerrcode.UniqueViolation {
			duplicateShortURL := ""
			err = p.Conn.QueryRow(
				context.Background(),
				"SELECT short_url_id FROM short_urls WHERE original_url = $1",
				value,
			).Scan(&duplicateShortURL)

			if err != nil {
				return "", err
			}

			return duplicateShortURL, ErrDuplicateOriginalURL
		}
	}

	if err != nil {
		return "", err
	}

	return key, nil
}

func (p *Postgres) SetBatch(ctx context.Context, batchItems []BatchItem) error {
	tx, err := p.Conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	for _, item := range batchItems {
		_, err = p.Conn.Exec(ctx,
			"INSERT INTO short_urls(url, original_url) VALUES($1, $2)",
			item.ID, item.OriginURL,
		)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	}

	return tx.Commit(ctx)
}
