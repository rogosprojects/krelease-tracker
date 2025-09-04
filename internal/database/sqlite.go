package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and runs migrations
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.runMigrations(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// UpsertRelease inserts or updates a release record
func (db *DB) UpsertRelease(release *Release) error {
	// parse time like "2006-01-02 15:04:05+00:00"
	now := time.Now().Format(time.RFC3339)

	query := `
	INSERT INTO releases (
		namespace, workload_name, workload_type, container_name,
		image_repo, image_name, image_tag, image_sha, client_name, env_name,
		first_seen, last_seen, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(namespace, workload_name, container_name, client_name, env_name, image_sha)
	DO UPDATE SET
		last_seen = ?,
		updated_at = ?
	`

	_, err := db.conn.Exec(query,
		release.Namespace, release.WorkloadName, release.WorkloadType, release.ContainerName,
		release.ImageRepo, release.ImageName, release.ImageTag, release.ImageSHA, release.ClientName, release.EnvName,
		release.FirstSeen.Format(time.RFC3339), release.LastSeen.Format(time.RFC3339), now, now,
		release.LastSeen.Format(time.RFC3339), now,
	)

	return err
}

// GetCurrentReleases returns all current deployed images grouped by namespace/workload/container
func (db *DB) GetCurrentReleases() ([]CurrentRelease, error) {
	// Check if connection is still valid
	if err := db.conn.Ping(); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
	}

	query := `
	SELECT DISTINCT
		namespace, workload_name, workload_type, container_name,
		image_repo, image_name, image_tag, image_sha, client_name, env_name, last_seen
	FROM releases r1
	WHERE last_seen = (
		SELECT MAX(last_seen)
		FROM releases r2
		WHERE r2.namespace = r1.namespace
		AND r2.workload_name = r1.workload_name
		AND r2.container_name = r1.container_name
		AND r2.client_name = r1.client_name
		AND r2.env_name = r1.env_name
		AND length(r2.image_sha) > 0
	)
	ORDER BY namespace, workload_name, container_name
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query current releases: %w", err)
	}
	defer rows.Close()

	var releases []CurrentRelease
	for rows.Next() {
		var r CurrentRelease
		err := rows.Scan(
			&r.Namespace, &r.WorkloadName, &r.WorkloadType, &r.ContainerName,
			&r.ImageRepo, &r.ImageName, &r.ImageTag, &r.ImageSHA, &r.ClientName, &r.EnvName, &r.LastSeen,
		)
		if err != nil {
			return nil, err
		}
		releases = append(releases, r)
	}

	return releases, rows.Err()
}

// GetCurrentReleasesFiltered returns current deployed images filtered by client and environment
func (db *DB) GetCurrentReleasesFiltered(clientName, envName string) ([]CurrentRelease, error) {
	// Check if connection is still valid
	if err := db.conn.Ping(); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
	}

	query := `
	SELECT DISTINCT
		namespace, workload_name, workload_type, container_name,
		image_repo, image_name, image_tag, image_sha, client_name, env_name, last_seen
	FROM releases r1
	WHERE last_seen = (
		SELECT MAX(last_seen)
		FROM releases r2
		WHERE r2.namespace = r1.namespace
		AND r2.workload_name = r1.workload_name
		AND r2.container_name = r1.container_name
		AND r2.client_name = r1.client_name
		AND r2.env_name = r1.env_name
	)`

	var args []interface{}
	if clientName != "" {
		query += " AND client_name = ?"
		args = append(args, clientName)
	}
	if envName != "" {
		query += " AND env_name = ?"
		args = append(args, envName)
	}

	query += " ORDER BY namespace, workload_name, container_name"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query current releases: %w", err)
	}
	defer rows.Close()

	var releases []CurrentRelease
	for rows.Next() {
		var r CurrentRelease
		err := rows.Scan(
			&r.Namespace, &r.WorkloadName, &r.WorkloadType, &r.ContainerName,
			&r.ImageRepo, &r.ImageName, &r.ImageTag, &r.ImageSHA, &r.ClientName, &r.EnvName, &r.LastSeen,
		)
		if err != nil {
			return nil, err
		}
		releases = append(releases, r)
	}

	return releases, rows.Err()
}

// GetAvailableClientsAndEnvironments returns all unique client/environment combinations
func (db *DB) GetAvailableClientsAndEnvironments() (map[string][]string, error) {
	query := `
	SELECT DISTINCT client_name, env_name
	FROM releases
	ORDER BY client_name, env_name
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query clients and environments: %w", err)
	}
	defer rows.Close()

	clientEnvs := make(map[string][]string)
	for rows.Next() {
		var clientName, envName string
		err := rows.Scan(&clientName, &envName)
		if err != nil {
			return nil, err
		}

		if _, exists := clientEnvs[clientName]; !exists {
			clientEnvs[clientName] = []string{}
		}

		// Add environment if not already present
		found := false
		for _, env := range clientEnvs[clientName] {
			if env == envName {
				found = true
				break
			}
		}
		if !found {
			clientEnvs[clientName] = append(clientEnvs[clientName], envName)
		}
	}

	return clientEnvs, rows.Err()
}

// GetCurrentReleaseByWorkload returns the current release for a specific workload and container
// across all namespaces. Returns an error if multiple matches are found in different namespaces.
func (db *DB) GetCurrentReleaseByWorkload(workloadType, workloadName, containerName, clientName, envName string) (*CurrentRelease, error) {
	// Check if connection is still valid
	if err := db.conn.Ping(); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
	}

	query := `
	SELECT DISTINCT
		namespace, workload_name, workload_type, container_name,
		image_repo, image_name, image_tag, image_sha, client_name, env_name, last_seen
	FROM releases r1
	WHERE workload_type = ? AND workload_name = ? AND container_name = ?
	AND client_name = ? AND env_name = ?
	AND last_seen = (
		SELECT MAX(last_seen)
		FROM releases r2
		WHERE r2.namespace = r1.namespace
		AND r2.workload_name = r1.workload_name
		AND r2.container_name = r1.container_name
		AND r2.client_name = r1.client_name
		AND r2.env_name = r1.env_name
	)
	ORDER BY namespace, workload_name, container_name
	`

	rows, err := db.conn.Query(query, workloadType, workloadName, containerName, clientName, envName)
	if err != nil {
		return nil, fmt.Errorf("failed to query current release: %w", err)
	}
	defer rows.Close()

	var releases []CurrentRelease
	for rows.Next() {
		var r CurrentRelease
		err := rows.Scan(
			&r.Namespace, &r.WorkloadName, &r.WorkloadType, &r.ContainerName,
			&r.ImageRepo, &r.ImageName, &r.ImageTag, &r.ImageSHA, &r.ClientName, &r.EnvName, &r.LastSeen,
		)
		if err != nil {
			return nil, err
		}
		releases = append(releases, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return nil, nil // No release found
	}

	if len(releases) > 1 {
		// Multiple releases found in different namespaces
		namespaces := make([]string, len(releases))
		for i, r := range releases {
			namespaces[i] = r.Namespace
		}
		return nil, fmt.Errorf("multiple releases found for %s/%s/%s in namespaces: %v",
			workloadType, workloadName, containerName, namespaces)
	}

	return &releases[0], nil
}

// GetReleaseHistory returns the release history for a specific component
func (db *DB) GetReleaseHistory(namespace, workloadName, containerName, clientName, envName string) (*ReleaseHistory, error) {
	query := `
	SELECT id, namespace, workload_name, workload_type, container_name,
		   image_repo, image_name, image_tag, image_sha, client_name, env_name,
		   first_seen, last_seen, created_at, updated_at
	FROM releases
	WHERE namespace = ? AND workload_name = ? AND container_name = ? AND client_name = ? AND env_name = ?
	ORDER BY last_seen DESC
	LIMIT 10
	`

	rows, err := db.conn.Query(query, namespace, workloadName, containerName, clientName, envName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []Release
	for rows.Next() {
		var r Release
		err := rows.Scan(
			&r.ID, &r.Namespace, &r.WorkloadName, &r.WorkloadType, &r.ContainerName,
			&r.ImageRepo, &r.ImageName, &r.ImageTag, &r.ImageSHA, &r.ClientName, &r.EnvName,
			&r.FirstSeen, &r.LastSeen, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		releases = append(releases, r)
	}

	return &ReleaseHistory{
		Releases: releases,
		Total:    len(releases),
	}, rows.Err()
}

// CleanupOldReleases removes old releases, keeping only the 10 most recent per component
func (db *DB) CleanupOldReleases() error {
	query := `
	DELETE FROM releases
	WHERE id NOT IN (
		SELECT id FROM (
			SELECT id,
				ROW_NUMBER() OVER (
					PARTITION BY namespace, workload_name, container_name
					ORDER BY last_seen DESC
				) as rn
			FROM releases
		) ranked
		WHERE rn <= 10
	)
	`

	result, err := db.conn.Exec(query)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Cleaned up %d old release records", rowsAffected)

	return nil
}

// UpsertPendingRelease inserts or updates a pending release record (used in slave mode)
func (db *DB) UpsertPendingRelease(release *PendingRelease) error {
	now := time.Now().Format(time.RFC3339)

	query := `
	INSERT INTO pending_releases (
		namespace, workload_name, workload_type, container_name,
		image_repo, image_name, image_tag, image_sha, client_name, env_name,
		first_seen, last_seen, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(namespace, workload_name, container_name, client_name, env_name, image_sha)
	DO UPDATE SET
		last_seen = ?,
		updated_at = ?
	`

	_, err := db.conn.Exec(query,
		release.Namespace, release.WorkloadName, release.WorkloadType, release.ContainerName,
		release.ImageRepo, release.ImageName, release.ImageTag, release.ImageSHA, release.ClientName, release.EnvName,
		release.FirstSeen.Format(time.RFC3339), release.LastSeen.Format(time.RFC3339), now, now,
		release.LastSeen.Format(time.RFC3339), now,
	)

	return err
}

// GetPendingReleases returns all pending releases (used in slave mode)
func (db *DB) GetPendingReleases() ([]PendingRelease, error) {
	query := `
	SELECT id, namespace, workload_name, workload_type, container_name,
		   image_repo, image_name, image_tag, image_sha, client_name, env_name,
		   first_seen, last_seen, created_at, updated_at
	FROM pending_releases
	WHERE length(image_sha) > 0
	ORDER BY created_at ASC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []PendingRelease
	for rows.Next() {
		var r PendingRelease
		err := rows.Scan(
			&r.ID, &r.Namespace, &r.WorkloadName, &r.WorkloadType, &r.ContainerName,
			&r.ImageRepo, &r.ImageName, &r.ImageTag, &r.ImageSHA, &r.ClientName, &r.EnvName,
			&r.FirstSeen, &r.LastSeen, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		releases = append(releases, r)
	}

	return releases, rows.Err()
}

// DeletePendingRelease removes a pending release by ID (used in slave mode after successful sync)
func (db *DB) DeletePendingRelease(id int) error {
	query := `DELETE FROM pending_releases WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	return err
}

// UpsertSlavePing inserts or updates a slave ping record
func (db *DB) UpsertSlavePing(clientName, envName, slaveVersion string) error {
	now := time.Now().Format(time.RFC3339)

	query := `
	INSERT INTO slave_pings (
		client_name, env_name, last_ping_time, status, slave_version, created_at, updated_at
	) VALUES (?, ?, ?, 'online', ?, ?, ?)
	ON CONFLICT(client_name, env_name)
	DO UPDATE SET
		last_ping_time = ?,
		status = 'online',
		slave_version = ?,
		updated_at = ?
	`

	_, err := db.conn.Exec(query,
		clientName, envName, now, slaveVersion, now, now,
		now, slaveVersion, now,
	)

	return err
}

// GetSlavePings returns all slave ping records with calculated status
func (db *DB) GetSlavePings() ([]SlavePing, error) {
	query := `
	SELECT id, client_name, env_name, last_ping_time, status, slave_version, created_at, updated_at
	FROM slave_pings
	ORDER BY client_name, env_name
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query slave pings: %w", err)
	}
	defer rows.Close()

	var pings []SlavePing
	now := time.Now()

	for rows.Next() {
		var ping SlavePing
		err := rows.Scan(
			&ping.ID, &ping.ClientName, &ping.EnvName, &ping.LastPingTime,
			&ping.Status, &ping.SlaveVersion, &ping.CreatedAt, &ping.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Calculate current status based on last ping time
		timeSinceLastPing := now.Sub(ping.LastPingTime)
		if timeSinceLastPing <= 10*time.Minute {
			ping.Status = "online"
		} else if timeSinceLastPing <= 15*time.Minute {
			ping.Status = "warning"
		} else {
			ping.Status = "offline"
		}

		pings = append(pings, ping)
	}

	return pings, rows.Err()
}

// GetSlavePingStatus returns the status for a specific client/environment
func (db *DB) GetSlavePingStatus(clientName, envName string) (string, time.Time, error) {
	query := `
	SELECT last_ping_time
	FROM slave_pings
	WHERE client_name = ? AND env_name = ?
	`

	var lastPingTime time.Time
	err := db.conn.QueryRow(query, clientName, envName).Scan(&lastPingTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return "never", time.Time{}, nil
		}
		return "", time.Time{}, fmt.Errorf("failed to query slave ping status: %w", err)
	}

	// Calculate status based on last ping time
	timeSinceLastPing := time.Now().Sub(lastPingTime)
	var status string
	if timeSinceLastPing <= 10*time.Minute {
		status = "online"
	} else if timeSinceLastPing <= 15*time.Minute {
		status = "warning"
	} else {
		status = "offline"
	}

	return status, lastPingTime, nil
}

// GetLastClientEnvUpdate returns the last update time for a specific client/environment
func (db *DB) GetLastClientEnvUpdate(clientName, envName string) (time.Time, error) {
	query := `
	SELECT MAX(updated_at) AS last_update
	FROM releases
	WHERE client_name = ? AND env_name = ?
	`
	var lastUpdateStr sql.NullString
	err := db.conn.QueryRow(query, clientName, envName).Scan(&lastUpdateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to query last update for %s/%s: %w", clientName, envName, err)
	}

	if !lastUpdateStr.Valid || lastUpdateStr.String == "" {
		log.Printf("No last update found for %s/%s", clientName, envName)
		return time.Time{}, nil
	}

	// Parse the string as time
	lastUpdate, err := time.Parse(time.RFC3339, lastUpdateStr.String)
	if err != nil {
		// Try alternative format if RFC3339 fails
		lastUpdate, err = time.Parse("2006-01-02 15:04:05+00:00", lastUpdateStr.String)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse time string '%s': %w", lastUpdateStr.String, err)
		}
	}

	return lastUpdate, nil
}
