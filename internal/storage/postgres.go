package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
)

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
		"SELECT original_url FROM short_urls WHERE url = $1",
		key,
	).Scan(&originalURL)

	if err != nil {
		return "", false
	}

	return originalURL, true
}

func (p *Postgres) Set(key string, value string) error {
	_, err := p.Conn.Exec(context.Background(), "INSERT INTO short_urls(url, original_url) VALUES($1, $2)", key, value)
	if err != nil {
		return err
	}

	return nil
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
