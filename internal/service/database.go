package service

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Database interface {
	Ping(context.Context) error
	Close(context.Context) error
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
