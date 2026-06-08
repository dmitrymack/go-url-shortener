package storage

import (
	"context"
	"errors"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextKeys"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrDuplicateOriginalURL = errors.New("duplicate original url")

type Database interface {
	Ping(context.Context) error
	Close(context.Context) error
}

type URLRecord struct {
	ID        string `json:"short_url"`
	OriginURL string `json:"original_url"`
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

func (p *Postgres) Set(ctx context.Context, key string, value string) (string, error) {
	var pgErr *pgconn.PgError

	userID := ctx.Value(contextKeys.UserIDContextKey)

	_, err := p.Conn.Exec(ctx,
		"INSERT INTO short_urls(short_url_id, original_url, user_id) VALUES($1, $2, $3)",
		key, value, userID,
	)

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

func (p *Postgres) SetBatch(ctx context.Context, batchItems []URLRecord) error {
	tx, err := p.Conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	userID := ctx.Value(contextKeys.UserIDContextKey)
	for _, item := range batchItems {
		_, err = tx.Exec(ctx,
			"INSERT INTO short_urls(short_url_id, original_url, user_id) VALUES($1, $2, $3)",
			item.ID, item.OriginURL, userID,
		)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	}

	return tx.Commit(ctx)
}

func (p *Postgres) GetUrlsByUser(userID string) ([]URLRecord, error) {
	rows, err := p.Conn.Query(
		context.Background(),
		"SELECT short_url_id, original_url FROM short_urls WHERE user_id = $1",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]URLRecord, 0)

	for rows.Next() {
		var item URLRecord

		err := rows.Scan(
			&item.ID,
			&item.OriginURL,
		)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
