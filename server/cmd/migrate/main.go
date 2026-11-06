package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/internal/database"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: valtag-migrate <up|down>")
		os.Exit(1)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// golang-migrate pgx/v5 driver expects "pgx5://" scheme
	migrateURL := strings.Replace(dbURL, "postgres://", "pgx5://", 1)

	source, err := iofs.New(database.MigrationsFS, "migrations")
	if err != nil {
		log.Fatalf("Failed to create migration source: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, migrateURL)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}
	defer m.Close()

	cmd := os.Args[1]
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Migration up failed: %v", err)
		}
		log.Println("Migrations applied successfully")
		ensurePartitions(dbURL)
	case "down":
		if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Migration down failed: %v", err)
		}
		log.Println("Rolled back one migration")
	default:
		fmt.Printf("Unknown command: %s\nUsage: valtag-migrate <up|down>\n", cmd)
		os.Exit(1)
	}
}

// ensurePartitions creates the audit_logs monthly partitions (current + next 2
// months) right after migrate-up. A fresh database has no partitions until the
// server starts its partition manager, and any audit write before that fails
// (deploy bug found 2026-11-04: fresh install rejected audit inserts). The
// operation is idempotent, so the server's StartPartitionManager will re-run
// it harmlessly. Uses the original postgres:// URL — pgxpool parses it directly.
func ensurePartitions(dbURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect for partition ensure: %v", err)
	}
	defer pool.Close()
	database.EnsurePartitions(ctx, pool)
	log.Println("audit_logs partitions ensured")
}
