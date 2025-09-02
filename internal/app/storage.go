package app

import (
	"context"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3" // register sqlite3 driver
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// NewContainer mengembalikan sqlstore.Container untuk SQLite/Postgres
func NewContainer(dsn string) (*sqlstore.Container, error) {
	// DSN:
	// SQLite : "sqlite3://file:session.db?_foreign_keys=on"
	// Postgres: "postgres://user:pass@host:5432/dbname?sslmode=disable"
	parts := strings.SplitN(dsn, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("dsn tidak valid, contoh: sqlite3://file:session.db?_foreign_keys=on")
	}
	dialect := parts[0]
	container, err := sqlstore.New(context.Background(), dialect, parts[1], waLog.Noop)
	if err != nil {
		return nil, err
	}
	return container, nil
}
