// migrate runs database migrations using golang-migrate.
// Usage:
//
//	migrate up          — apply all pending migrations
//	migrate down        — roll back the last migration
//	migrate down N      — roll back N migrations
//	migrate version     — print current migration version
//	migrate force N     — force-set version without running migration (recovery)
//
// Environment variables:
//
//	DATABASE_URL        — Postgres connection string (required)
//	MIGRATIONS_PATH     — path to .sql files (default: /migrations)
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate <up|down|version|force> [N]")
		os.Exit(1)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "/migrations"
	}

	m, err := migrate.New("file://"+migrationsPath, dbURL)
	if err != nil {
		slog.Error("creating migrate instance", "err", err)
		os.Exit(1)
	}
	defer m.Close()

	cmd := os.Args[1]
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			slog.Error("migrate up failed", "err", err)
			os.Exit(1)
		}
		version, dirty, _ := m.Version()
		slog.Info("migrate up complete", "version", version, "dirty", dirty)

	case "down":
		n := 1
		if len(os.Args) >= 3 {
			n, err = strconv.Atoi(os.Args[2])
			if err != nil {
				slog.Error("invalid step count", "arg", os.Args[2])
				os.Exit(1)
			}
		}
		if err := m.Steps(-n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			slog.Error("migrate down failed", "err", err)
			os.Exit(1)
		}
		version, dirty, _ := m.Version()
		slog.Info("migrate down complete", "version", version, "dirty", dirty)

	case "version":
		version, dirty, err := m.Version()
		if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
			slog.Error("getting version", "err", err)
			os.Exit(1)
		}
		fmt.Printf("version=%d dirty=%v\n", version, dirty)

	case "force":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: migrate force <version>")
			os.Exit(1)
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			slog.Error("invalid version", "arg", os.Args[2])
			os.Exit(1)
		}
		if err := m.Force(v); err != nil {
			slog.Error("migrate force failed", "err", err)
			os.Exit(1)
		}
		slog.Info("forced version", "version", v)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(1)
	}
}
