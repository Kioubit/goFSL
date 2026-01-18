package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"

	_ "github.com/mattn/go-sqlite3"
)

var SystemDB *sql.DB

//go:embed schema.sql
var sqlSchema []byte

func InitDB(dataDir string) error {
	const filename = "state.db"
	var err error
	SystemDB, err = sql.Open("sqlite3", dataDir+filename+"?_busy_timeout=30000")
	if err != nil {
		return err
	}
	err = SystemDB.Ping()
	if err != nil {
		return err
	}
	_, err = SystemDB.Exec("pragma journal_mode = WAL")
	if err != nil {
		slog.Warn("failed to set WAL journal mode for the database", "error", err)
	}
	_, err = SystemDB.Exec(string(sqlSchema))
	if err != nil {
		return fmt.Errorf("failed to apply DB schema: %w", err)
	}
	return nil
}
