package database

import (
	"fmt"
	"log"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          string
	Down        string
}

// migrations contains all database migrations in order
var migrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema",
		Up: `
		CREATE TABLE IF NOT EXISTS releases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_name TEXT NOT NULL,
			env_name TEXT NOT NULL,
			namespace TEXT NOT NULL,
			workload_name TEXT NOT NULL,
			workload_type TEXT NOT NULL,
			container_name TEXT NOT NULL,
			image_repo TEXT NOT NULL,
			image_name TEXT NOT NULL,
			image_tag TEXT NOT NULL,
			first_seen DATETIME NOT NULL,
			last_seen DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(namespace, workload_name, container_name, client_name, env_name, image_repo, image_name, image_tag)
		);

		CREATE INDEX IF NOT EXISTS idx_releases_component ON releases(namespace, workload_name, container_name, client_name, env_name);
		CREATE INDEX IF NOT EXISTS idx_releases_last_seen ON releases(last_seen);
		CREATE INDEX IF NOT EXISTS idx_releases_namespace ON releases(namespace);

		CREATE TABLE IF NOT EXISTS pending_releases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_name TEXT NOT NULL,
			env_name TEXT NOT NULL,
			namespace TEXT NOT NULL,
			workload_name TEXT NOT NULL,
			workload_type TEXT NOT NULL,
			container_name TEXT NOT NULL,
			image_repo TEXT NOT NULL,
			image_name TEXT NOT NULL,
			image_tag TEXT NOT NULL,
			first_seen DATETIME NOT NULL,
			last_seen DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(namespace, workload_name, container_name, client_name, env_name, image_repo, image_name, image_tag)
		);

		CREATE INDEX IF NOT EXISTS idx_pending_releases_created_at ON pending_releases(created_at);

		CREATE TABLE IF NOT EXISTS slave_pings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_name TEXT NOT NULL,
			env_name TEXT NOT NULL,
			last_ping_time DATETIME NOT NULL,
			status TEXT NOT NULL DEFAULT 'online',
			slave_version TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(client_name, env_name)
		);

		CREATE INDEX IF NOT EXISTS idx_slave_pings_last_ping ON slave_pings(last_ping_time);
		CREATE INDEX IF NOT EXISTS idx_slave_pings_status ON slave_pings(status);
		`,
		Down: `
		DROP TABLE IF EXISTS releases;
		DROP TABLE IF EXISTS pending_releases;
		DROP TABLE IF EXISTS slave_pings;
		`,
	},
	{
		Version:     2,
		Description: "Add image_sha column and update unique constraints",
		Up: `
		-- Add image_sha column to releases table
		ALTER TABLE releases ADD COLUMN image_sha TEXT NOT NULL DEFAULT '';

		-- Add image_sha column to pending_releases table
		ALTER TABLE pending_releases ADD COLUMN image_sha TEXT NOT NULL DEFAULT '';

		-- Create new tables with updated schema
		CREATE TABLE releases_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_name TEXT NOT NULL,
			env_name TEXT NOT NULL,
			namespace TEXT NOT NULL,
			workload_name TEXT NOT NULL,
			workload_type TEXT NOT NULL,
			container_name TEXT NOT NULL,
			image_repo TEXT NOT NULL,
			image_name TEXT NOT NULL,
			image_tag TEXT NOT NULL,
			image_sha TEXT NOT NULL DEFAULT '',
			first_seen DATETIME NOT NULL,
			last_seen DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(namespace, workload_name, container_name, client_name, env_name, image_sha)
		);

		CREATE TABLE pending_releases_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_name TEXT NOT NULL,
			env_name TEXT NOT NULL,
			namespace TEXT NOT NULL,
			workload_name TEXT NOT NULL,
			workload_type TEXT NOT NULL,
			container_name TEXT NOT NULL,
			image_repo TEXT NOT NULL,
			image_name TEXT NOT NULL,
			image_tag TEXT NOT NULL,
			image_sha TEXT NOT NULL DEFAULT '',
			first_seen DATETIME NOT NULL,
			last_seen DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(namespace, workload_name, container_name, client_name, env_name, image_sha)
		);

		-- Copy data from old tables to new tables
		INSERT INTO releases_new (
			id, client_name, env_name, namespace, workload_name, workload_type,
			container_name, image_repo, image_name, image_tag, image_sha,
			first_seen, last_seen, created_at, updated_at
		)
		SELECT
			id, client_name, env_name, namespace, workload_name, workload_type,
			container_name, image_repo, image_name, image_tag, '',
			first_seen, last_seen, created_at, updated_at
		FROM releases;

		INSERT INTO pending_releases_new (
			id, client_name, env_name, namespace, workload_name, workload_type,
			container_name, image_repo, image_name, image_tag, image_sha,
			first_seen, last_seen, created_at, updated_at
		)
		SELECT
			id, client_name, env_name, namespace, workload_name, workload_type,
			container_name, image_repo, image_name, image_tag, '',
			first_seen, last_seen, created_at, updated_at
		FROM pending_releases;

		-- Drop old tables
		DROP TABLE releases;
		DROP TABLE pending_releases;

		-- Rename new tables
		ALTER TABLE releases_new RENAME TO releases;
		ALTER TABLE pending_releases_new RENAME TO pending_releases;

		-- Recreate indexes
		CREATE INDEX IF NOT EXISTS idx_releases_component ON releases(namespace, workload_name, container_name, client_name, env_name);
		CREATE INDEX IF NOT EXISTS idx_releases_last_seen ON releases(last_seen);
		CREATE INDEX IF NOT EXISTS idx_releases_namespace ON releases(namespace);
		CREATE INDEX IF NOT EXISTS idx_pending_releases_created_at ON pending_releases(created_at);
		`,
		Down: `
		-- This migration cannot be safely rolled back as it changes the unique constraint
		-- Manual intervention would be required
		`,
	},
	{
		Version:     3,
		Description: "Delete all releases without image SHA",
		Up: `
		DELETE FROM releases WHERE image_sha = '';
		DELETE FROM pending_releases WHERE image_sha = '';
		`,
		Down: `
		-- This migration cannot be safely rolled back as it deletes data
		-- Manual intervention would be required
		`,
	},
}

// createMigrationsTable creates the migrations tracking table
func (db *DB) createMigrationsTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		description TEXT NOT NULL,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.conn.Exec(query)
	return err
}

// getCurrentVersion returns the current database schema version
func (db *DB) getCurrentVersion() (int, error) {
	if err := db.createMigrationsTable(); err != nil {
		return 0, fmt.Errorf("failed to create migrations table: %w", err)
	}

	var version int
	err := db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}

	return version, nil
}

// runMigrations applies all pending migrations
func (db *DB) runMigrations() error {
	currentVersion, err := db.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	log.Printf("Current database schema version: %d", currentVersion)

	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue
		}

		log.Printf("Applying migration %d: %s", migration.Version, migration.Description)

		// Execute migration in a transaction
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
		}

		// Execute the migration SQL
		if _, err := tx.Exec(migration.Up); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %d: %w", migration.Version, err)
		}

		// Record the migration
		if _, err := tx.Exec("INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
			migration.Version, migration.Description); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}

		log.Printf("Successfully applied migration %d", migration.Version)
	}

	return nil
}
