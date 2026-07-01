package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return db, nil
}

func Migrate(db *sql.DB) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		content, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := execMigration(db, string(content)); err != nil {
			return fmt.Errorf("migrate %s: %w", name, err)
		}
	}
	return nil
}

func execMigration(db *sql.DB, content string) error {
	statements := strings.Split(content, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			if isIgnorableMigrationErr(err) {
				continue
			}
			return fmt.Errorf("%w\nstmt: %s", err, stmt)
		}
	}
	return nil
}

func isIgnorableMigrationErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "check that column/key exists") ||
		strings.Contains(msg, "Can't DROP")
}
