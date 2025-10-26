package db

import (
	"database/sql"
	"fmt"
	"jpm/version"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

const (
	url   = "libsql://jpm-jasnrathore.aws-ap-south-1.turso.io"
	token = "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJhIjoicm8iLCJpYXQiOjE3NjEzNzM5OTcsImlkIjoiYWM1N2NmY2ItNTFmMi00MzU5LThjYTktODQzODFmOGY4MTdhIiwicmlkIjoiMzZjZGUxZjEtNjQyZS00ZGFkLTk5YWQtZGU0ZWYyZTkzNmIwIn0.5Wah3NkS6u1vz0yiZjuYLScGYGwGCZTxi2WLNhlIyy7P4wJLWuUDXl3gv3ja7jJ-bW_AZDdnuYSlt5tUS6WdCA"
)

type Metadata struct {
	Version      string
	Url          string
	Instructions string
}

type PackageVersion struct {
	Name         string
	Version      string
	BinaryUrl    string
	Instructions string
	IsLatest     bool
}

type RemoteDB struct {
	Connection *sql.DB
}

func NewRemoteDB() RemoteDB {
	newUrl := fmt.Sprintf("%s?authToken=%s", url, token)
	conn, _ := sql.Open("libsql", newUrl)
	return RemoteDB{
		Connection: conn,
	}
}

func (rdb *RemoteDB) GetAll() {
	stmt, _ := rdb.Connection.Prepare("SELECT * FROM releases")
	defer stmt.Close()

	rows, _ := stmt.Query()
	for rows.Next() {
		var name string
		var binary_url string
		var version string
		var instructions string
		_ = rows.Scan(&name, &version, &binary_url, &instructions)
		fmt.Printf("%s, %s, %s\n", name, version, binary_url)
	}
}

// GetOne fetches the latest version of a package by default
func (rdb *RemoteDB) GetOne(name string) (*Metadata, error) {
	return rdb.GetVersion(name, "")
}

// GetVersion fetches a specific version of a package
// If versionConstraint is empty, returns the latest version
func (rdb *RemoteDB) GetVersion(name, versionConstraint string) (*Metadata, error) {
	if versionConstraint == "" || versionConstraint == "latest" {
		return rdb.getLatestVersion(name)
	}

	// Check if it's an exact version
	if _, err := version.Parse(versionConstraint); err == nil {
		return rdb.getExactVersion(name, versionConstraint)
	}

	// Otherwise treat it as a constraint (e.g., ">=1.2.0", "^1.0.0")
	return rdb.getVersionByConstraint(name, versionConstraint)
}

// getLatestVersion gets the latest version of a package
func (rdb *RemoteDB) getLatestVersion(name string) (*Metadata, error) {
	stmt, err := rdb.Connection.Prepare(`
		SELECT version, binary_url, instructions 
		FROM releases 
		WHERE name = ? 
		ORDER BY version DESC 
		LIMIT 1
	`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var metadata Metadata
	err = stmt.QueryRow(name).Scan(&metadata.Version, &metadata.Url, &metadata.Instructions)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("package '%s' not found", name)
	}
	if err != nil {
		return nil, err
	}

	return &metadata, nil
}

// getExactVersion gets a specific version of a package
func (rdb *RemoteDB) getExactVersion(name, versionStr string) (*Metadata, error) {
	// Normalize version (remove 'v' prefix if present)
	v, err := version.Parse(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %s", versionStr)
	}
	normalizedVersion := v.String()

	stmt, err := rdb.Connection.Prepare(`
		SELECT version, binary_url, instructions 
		FROM releases 
		WHERE name = ? AND version = ? 
		LIMIT 1
	`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var metadata Metadata
	err = stmt.QueryRow(name, normalizedVersion).Scan(&metadata.Version, &metadata.Url, &metadata.Instructions)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("package '%s' version '%s' not found", name, versionStr)
	}
	if err != nil {
		return nil, err
	}

	return &metadata, nil
}

// getVersionByConstraint finds the best matching version based on a constraint
func (rdb *RemoteDB) getVersionByConstraint(name, constraint string) (*Metadata, error) {
	// Get all versions of the package
	versions, err := rdb.GetAllVersions(name)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("package '%s' not found", name)
	}

	// Find the highest version that satisfies the constraint
	var bestMatch *PackageVersion
	var bestVersion *version.Version

	for i := range versions {
		v, err := version.Parse(versions[i].Version)
		if err != nil {
			continue // Skip invalid versions
		}

		compatible, err := v.IsCompatible(constraint)
		if err != nil {
			return nil, fmt.Errorf("invalid constraint '%s': %w", constraint, err)
		}

		if compatible {
			if bestVersion == nil || v.GreaterThan(bestVersion) {
				bestVersion = v
				bestMatch = &versions[i]
			}
		}
	}

	if bestMatch == nil {
		return nil, fmt.Errorf("no version of '%s' satisfies constraint '%s'", name, constraint)
	}

	return &Metadata{
		Version:      bestMatch.Version,
		Url:          bestMatch.BinaryUrl,
		Instructions: bestMatch.Instructions,
	}, nil
}

// GetAllVersions returns all versions of a package
func (rdb *RemoteDB) GetAllVersions(name string) ([]PackageVersion, error) {
	stmt, err := rdb.Connection.Prepare(`
		SELECT name, version, binary_url, instructions 
		FROM releases 
		WHERE name = ? 
		ORDER BY version DESC
	`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []PackageVersion
	for rows.Next() {
		var pv PackageVersion
		err := rows.Scan(&pv.Name, &pv.Version, &pv.BinaryUrl, &pv.Instructions)
		if err != nil {
			return nil, err
		}
		versions = append(versions, pv)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("package '%s' not found", name)
	}

	// Mark the latest version
	if len(versions) > 0 {
		versions[0].IsLatest = true
	}

	return versions, nil
}

// ListAllPackages returns all unique package names with their latest version
func (rdb *RemoteDB) ListAllPackages() ([]PackageVersion, error) {
	stmt, err := rdb.Connection.Prepare(`
		SELECT DISTINCT name, 
		       (SELECT version FROM releases r2 WHERE r2.name = r1.name ORDER BY version DESC LIMIT 1) as latest_version
		FROM releases r1
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []PackageVersion
	for rows.Next() {
		var pv PackageVersion
		err := rows.Scan(&pv.Name, &pv.Version)
		if err != nil {
			return nil, err
		}
		pv.IsLatest = true
		packages = append(packages, pv)
	}

	return packages, nil
}

func (rdb *RemoteDB) Close() {
	rdb.Connection.Close()
}
