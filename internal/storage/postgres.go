package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDuplicateOriginalURL is returned when a short link already exists for
// the original URL being saved. The identifier of the existing link is
// returned alongside the error.
var ErrDuplicateOriginalURL = errors.New("duplicate original url")

// Database is the minimal DB connection interface needed by the health
// check handler (see handler.Handler.PingDatabase).
type Database interface {
	Ping(context.Context) error
	Close(context.Context) error
}

// URLRecord is a user's short link: an identifier (or a full short URL,
// depending on context) and the original URL.
//
// generate:reset
type URLRecord struct {
	ID        string `json:"short_url"`
	OriginURL string `json:"original_url"`
}

// Postgres is a short link store backed by PostgreSQL. It deduplicates
// links by original URL using a unique index in the database.
type Postgres struct {
	Pool *pgxpool.Pool
}

// Ping checks database availability.
func (p *Postgres) Ping(ctx context.Context) error {
	return p.Pool.Ping(ctx)
}

// Close closes the database connection pool.
func (p *Postgres) Close(ctx context.Context) error {
	p.Pool.Close()
	return nil
}

// NewPostgres opens a PostgreSQL connection pool using connString.
func NewPostgres(ctx context.Context, connString string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}

	return &Postgres{
		Pool: pool,
	}, nil
}

// Get returns the original URL stored under the short identifier key. If
// the link is marked deleted, it returns ErrDeleted.
func (p *Postgres) Get(key string) (string, error) {
	var originalURL string
	var isDeleted bool
	err := p.Pool.QueryRow(
		context.Background(),
		"SELECT original_url, is_deleted FROM short_urls WHERE short_url_id = $1",
		key,
	).Scan(&originalURL, &isDeleted)

	if isDeleted {
		return "", ErrDeleted
	}
	if err != nil {
		return "", err
	}

	return originalURL, nil
}

// Set stores value under key. If value was already stored before (unique
// index violation on original_url), it returns the existing short
// identifier and ErrDuplicateOriginalURL.
func (p *Postgres) Set(ctx context.Context, key string, value string, userID string) (string, error) {
	var pgErr *pgconn.PgError

	_, err := p.Pool.Exec(ctx,
		"INSERT INTO short_urls(short_url_id, original_url, user_id) VALUES($1, $2, $3)",
		key, value, userID,
	)

	if errors.As(err, &pgErr) {
		if pgErr.Code == pgerrcode.UniqueViolation {
			duplicateShortURL := ""
			err = p.Pool.QueryRow(
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

// SetBatch stores all of batchItems in a single transaction.
func (p *Postgres) SetBatch(ctx context.Context, batchItems []URLRecord, userID string) error {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

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

// GetUrlsByUser returns all non-deleted links belonging to userID.
func (p *Postgres) GetUrlsByUser(userID string) ([]URLRecord, error) {
	rows, err := p.Pool.Query(
		context.Background(),
		"SELECT short_url_id, original_url FROM short_urls WHERE user_id = $1 AND is_deleted = false",
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

// SetDeletedBatch marks links keys, owned by userID, as deleted.
func (p *Postgres) SetDeletedBatch(ctx context.Context, keys []string, userID string) error {
	_, err := p.Pool.Exec(
		ctx,
		"UPDATE short_urls SET is_deleted = true WHERE user_id = $1 AND short_url_id = ANY($2)",
		userID, keys,
	)

	return err
}
