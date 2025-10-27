package db

import (
	"database/sql"
	"fmt"
	"jpm/model"
	"time"

	_ "github.com/tursodatabase/turso-go"
)

type LocalDB struct {
	Connection *sql.DB
}

func NewLocalDB() LocalDB {
	conn, _ := sql.Open("turso", "jpm.db")
	return LocalDB{
		Connection: conn,
	}
}

func (ldb *LocalDB) InitSchema() error {
	schema := `
		-- Main installed packages table
		CREATE TABLE IF NOT EXISTS installed (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR(100) UNIQUE NOT NULL,
			version VARCHAR(20) NOT NULL,
			location VARCHAR(255),
			sys_path VARCHAR(255),
			installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			installed_from_url VARCHAR(255),
			checksum_sha256 VARCHAR(64)  DEFAULT '',
			file_size_bytes INTEGER,
			installation_status VARCHAR(20) DEFAULT 'completed',
			error_message TEXT DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_installed_name ON installed(name);
		CREATE INDEX IF NOT EXISTS idx_installed_status ON installed(installation_status);
		CREATE INDEX IF NOT EXISTS idx_installed_installed_at ON installed(installed_at DESC);

		-- Track individual files
		CREATE TABLE IF NOT EXISTS installed_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			installed_id INTEGER NOT NULL,
			file_path VARCHAR(500) NOT NULL,
			file_type VARCHAR(20),
			is_executable BOOLEAN DEFAULT FALSE,
			FOREIGN KEY (installed_id) REFERENCES installed(id) ON DELETE CASCADE,
			UNIQUE(installed_id, file_path)
		);

		CREATE INDEX IF NOT EXISTS idx_installed_files_package ON installed_files(installed_id);

		-- Track environment modifications
		CREATE TABLE IF NOT EXISTS environment_modifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			installed_id INTEGER NOT NULL,
			modification_type VARCHAR(20) NOT NULL,
			variable_name VARCHAR(100),
			variable_value TEXT,
			original_value TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (installed_id) REFERENCES installed(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_env_mods_package ON environment_modifications(installed_id);

		-- Installation history
		CREATE TABLE IF NOT EXISTS installation_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			package_name VARCHAR(100) NOT NULL,
			version VARCHAR(20) NOT NULL,
			action VARCHAR(20) NOT NULL,
			previous_version VARCHAR(20),
			performed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			success BOOLEAN DEFAULT TRUE,
			error_message TEXT,
			user_comment TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_history_package ON installation_history(package_name);
		CREATE INDEX IF NOT EXISTS idx_history_performed_at ON installation_history(performed_at DESC);

		-- Dependencies tracking
		CREATE TABLE IF NOT EXISTS installed_dependencies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			parent_installed_id INTEGER NOT NULL,
			dependency_name VARCHAR(100) NOT NULL,
			dependency_version VARCHAR(20),
			is_auto_installed BOOLEAN DEFAULT FALSE,
			FOREIGN KEY (parent_installed_id) REFERENCES installed(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_deps_parent ON installed_dependencies(parent_installed_id);
		CREATE INDEX IF NOT EXISTS idx_deps_dependency ON installed_dependencies(dependency_name);

		-- Configuration
		CREATE TABLE IF NOT EXISTS config (
			key VARCHAR(100) PRIMARY KEY,
			value TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		-- Metadata cache
		CREATE TABLE IF NOT EXISTS metadata_cache (
			package_name VARCHAR(100) PRIMARY KEY,
			latest_version VARCHAR(20),
			description TEXT,
			homepage_url VARCHAR(255),
			cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_metadata_expires ON metadata_cache(expires_at);
	`

	_, err := ldb.Connection.Exec(schema)
	return err
}

// Package operations
func (ldb *LocalDB) InsertInstallation(ins *model.Installation) error {
	result, err := ldb.Connection.Exec(`
		INSERT INTO installed (
			name, version, location, sys_path, installed_from_url, 
			checksum_sha256, file_size_bytes, installation_status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ins.Name, ins.Version, ins.Location, ins.SysPath,
		ins.InstalledFromURL, ins.ChecksumSHA256, ins.FileSizeBytes, ins.Status,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	ins.ID = int(id)

	// Record in history
	return ldb.AddHistory(ins.Name, ins.Version, "install", "", true, "")
}

func (ldb *LocalDB) UpdateInstallation(ins *model.Installation) error {
	// Get previous version for history
	existing, err := ldb.GetByName(ins.Name)
	if err != nil {
		return err
	}

	_, err = ldb.Connection.Exec(`
		UPDATE installed 
		SET version = ?, location = ?, sys_path = ?, updated_at = ?,
		    installed_from_url = ?, checksum_sha256 = ?, file_size_bytes = ?,
		    installation_status = ?
		WHERE name = ?`,
		ins.Version, ins.Location, ins.SysPath, time.Now(),
		ins.InstalledFromURL, ins.ChecksumSHA256, ins.FileSizeBytes,
		ins.Status, ins.Name,
	)
	if err != nil {
		return err
	}

	// Record in history
	prevVersion := ""
	if existing != nil {
		prevVersion = existing.Version
	}
	return ldb.AddHistory(ins.Name, ins.Version, "update", prevVersion, true, "")
}

func (ldb *LocalDB) DeleteInstallation(name string) error {
	existing, err := ldb.GetByName(name)
	if err != nil {
		return err
	}

	_, err = ldb.Connection.Exec("DELETE FROM installed WHERE name = ?", name)
	if err != nil {
		return err
	}

	// Record in history
	if existing != nil {
		return ldb.AddHistory(name, existing.Version, "remove", "", true, "")
	}
	return nil
}

func (ldb *LocalDB) GetByName(name string) (*model.Installation, error) {
	stmt, err := ldb.Connection.Prepare(`
		SELECT id, name, version, location, sys_path, installed_at, updated_at,
		       installed_from_url, checksum_sha256, file_size_bytes, installation_status, error_message
		FROM installed 
		WHERE name = ? 
		LIMIT 1
	`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var ins model.Installation
	err = stmt.QueryRow(name).Scan(
		&ins.ID, &ins.Name, &ins.Version, &ins.Location, &ins.SysPath,
		&ins.InstalledAt, &ins.UpdatedAt, &ins.InstalledFromURL,
		&ins.ChecksumSHA256, &ins.FileSizeBytes, &ins.Status, &ins.ErrorMessage,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ins, nil
}

func (ldb *LocalDB) GetAll() ([]model.Installation, error) {
	rows, err := ldb.Connection.Query(`
		SELECT id, name, version, location, sys_path, installed_at, updated_at,
		       installed_from_url, checksum_sha256, file_size_bytes, installation_status, error_message
		FROM installed 
		WHERE installation_status = 'completed'
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var installations []model.Installation
	for rows.Next() {
		var ins model.Installation
		err := rows.Scan(
			&ins.ID, &ins.Name, &ins.Version, &ins.Location, &ins.SysPath,
			&ins.InstalledAt, &ins.UpdatedAt, &ins.InstalledFromURL,
			&ins.ChecksumSHA256, &ins.FileSizeBytes, &ins.Status, &ins.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		installations = append(installations, ins)
	}
	return installations, nil
}

func (ldb *LocalDB) GetCount() int {
	var count int
	_ = ldb.Connection.QueryRow(`
		SELECT COUNT(*) FROM installed 
		WHERE installation_status = 'completed'
	`).Scan(&count)
	return count
}

// File tracking
func (ldb *LocalDB) AddInstalledFile(installedID int, filePath, fileType string, isExecutable bool) error {
	_, err := ldb.Connection.Exec(`
		INSERT INTO installed_files (installed_id, file_path, file_type, is_executable)
		VALUES (?, ?, ?, ?)`,
		installedID, filePath, fileType, isExecutable,
	)
	return err
}

func (ldb *LocalDB) GetInstalledFiles(installedID int) ([]model.InstalledFile, error) {
	rows, err := ldb.Connection.Query(`
		SELECT id, file_path, file_type, is_executable
		FROM installed_files
		WHERE installed_id = ?
		ORDER BY file_path`,
		installedID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []model.InstalledFile
	for rows.Next() {
		var f model.InstalledFile
		err := rows.Scan(&f.ID, &f.FilePath, &f.FileType, &f.IsExecutable)
		if err != nil {
			return nil, err
		}
		f.InstalledID = installedID
		files = append(files, f)
	}
	return files, nil
}

// Environment modifications
func (ldb *LocalDB) AddEnvModification(installedID int, modType, varName, varValue, originalValue string) error {
	_, err := ldb.Connection.Exec(`
		INSERT INTO environment_modifications 
		(installed_id, modification_type, variable_name, variable_value, original_value)
		VALUES (?, ?, ?, ?, ?)`,
		installedID, modType, varName, varValue, originalValue,
	)
	return err
}

func (ldb *LocalDB) GetEnvModifications(installedID int) ([]model.EnvModification, error) {
	rows, err := ldb.Connection.Query(`
		SELECT id, modification_type, variable_name, variable_value, original_value, created_at
		FROM environment_modifications
		WHERE installed_id = ?
		ORDER BY created_at`,
		installedID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mods []model.EnvModification
	for rows.Next() {
		var m model.EnvModification
		err := rows.Scan(&m.ID, &m.ModificationType, &m.VariableName,
			&m.VariableValue, &m.OriginalValue, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		m.InstalledID = installedID
		mods = append(mods, m)
	}
	return mods, nil
}

// History
func (ldb *LocalDB) AddHistory(packageName, version, action, prevVersion string, success bool, errorMsg string) error {
	_, err := ldb.Connection.Exec(`
		INSERT INTO installation_history 
		(package_name, version, action, previous_version, success, error_message)
		VALUES (?, ?, ?, ?, ?, ?)`,
		packageName, version, action, prevVersion, success, errorMsg,
	)
	return err
}

func (ldb *LocalDB) GetHistory(packageName string, limit int) ([]model.HistoryEntry, error) {
	query := `
		SELECT id, package_name, version, action, previous_version, 
		       performed_at, success, error_message, user_comment
		FROM installation_history`

	if packageName != "" {
		query += " WHERE package_name = ?"
	}

	query += " ORDER BY performed_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	var rows *sql.Rows
	var err error
	if packageName != "" {
		rows, err = ldb.Connection.Query(query, packageName)
	} else {
		rows, err = ldb.Connection.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.HistoryEntry
	for rows.Next() {
		var h model.HistoryEntry
		err := rows.Scan(&h.ID, &h.PackageName, &h.Version, &h.Action,
			&h.PreviousVersion, &h.PerformedAt, &h.Success, &h.ErrorMessage, &h.UserComment)
		if err != nil {
			return nil, err
		}
		entries = append(entries, h)
	}
	return entries, nil
}

// Dependencies
func (ldb *LocalDB) AddDependency(parentID int, depName, depVersion string, isAuto bool) error {
	_, err := ldb.Connection.Exec(`
		INSERT INTO installed_dependencies 
		(parent_installed_id, dependency_name, dependency_version, is_auto_installed)
		VALUES (?, ?, ?, ?)`,
		parentID, depName, depVersion, isAuto,
	)
	return err
}

func (ldb *LocalDB) GetDependencies(installedID int) ([]model.Dependency, error) {
	rows, err := ldb.Connection.Query(`
		SELECT id, dependency_name, dependency_version, is_auto_installed
		FROM installed_dependencies
		WHERE parent_installed_id = ?`,
		installedID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []model.Dependency
	for rows.Next() {
		var d model.Dependency
		err := rows.Scan(&d.ID, &d.DependencyName, &d.DependencyVersion, &d.IsAutoInstalled)
		if err != nil {
			return nil, err
		}
		d.ParentInstalledID = installedID
		deps = append(deps, d)
	}
	return deps, nil
}

// Metadata cache
func (ldb *LocalDB) UpdateCache(packageName, latestVersion, description, homepage string, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)
	_, err := ldb.Connection.Exec(`
		INSERT OR REPLACE INTO metadata_cache 
		(package_name, latest_version, description, homepage_url, cached_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		packageName, latestVersion, description, homepage, time.Now(), expiresAt,
	)
	return err
}

func (ldb *LocalDB) GetCachedMetadata(packageName string) (*model.CachedMetadata, error) {
	var cache model.CachedMetadata
	err := ldb.Connection.QueryRow(`
		SELECT package_name, latest_version, description, homepage_url, cached_at, expires_at
		FROM metadata_cache
		WHERE package_name = ? AND expires_at > ?`,
		packageName, time.Now(),
	).Scan(&cache.PackageName, &cache.LatestVersion, &cache.Description,
		&cache.HomepageURL, &cache.CachedAt, &cache.ExpiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

// Config
func (ldb *LocalDB) SetConfig(key, value string) error {
	_, err := ldb.Connection.Exec(`
		INSERT OR REPLACE INTO config (key, value, updated_at)
		VALUES (?, ?, ?)`,
		key, value, time.Now(),
	)
	return err
}

func (ldb *LocalDB) GetConfig(key string) (string, error) {
	var value string
	err := ldb.Connection.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (ldb *LocalDB) Close() {
	ldb.Connection.Close()
}
