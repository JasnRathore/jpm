package db

import (
	"database/sql"
	"fmt"
	"jpm/config"
	"jpm/model"
	"jpm/version"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

type RemoteDB struct {
	Connection *sql.DB
}

func NewRemoteDB() RemoteDB {
	newUrl := fmt.Sprintf("%s?authToken=%s", config.GetEnvVar("URL"), config.GetEnvVar("TOKEN"))
	conn, _ := sql.Open("libsql", newUrl)
	return RemoteDB{
		Connection: conn,
	}
}

// GetPackageInfo retrieves full package information
func (rdb *RemoteDB) GetPackageInfo(name string) (*model.Package, error) {
	var pkg model.Package
	err := rdb.Connection.QueryRow(`
		SELECT id, name, description, homepage_url, repository_url, license, author, created_at, updated_at
		FROM packages
		WHERE name = ?`,
		name,
	).Scan(&pkg.ID, &pkg.Name, &pkg.Description, &pkg.HomepageURL,
		&pkg.RepositoryURL, &pkg.License, &pkg.Author, &pkg.CreatedAt, &pkg.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("package '%s' not found", name)
	}
	if err != nil {
		return nil, err
	}
	return &pkg, nil
}

// GetRelease fetches a specific release
func (rdb *RemoteDB) GetRelease(packageName, versionConstraint string) (*model.Release, error) {
	// Get package first
	pkg, err := rdb.GetPackageInfo(packageName)
	if err != nil {
		return nil, err
	}

	if versionConstraint == "" || versionConstraint == "latest" {
		return rdb.getLatestRelease(pkg.ID)
	}

	// Check if it's an exact version
	if _, err := version.Parse(versionConstraint); err == nil {
		return rdb.getExactRelease(pkg.ID, versionConstraint)
	}

	// Otherwise treat it as a constraint
	return rdb.getReleaseByConstraint(pkg.ID, versionConstraint)
}

func (rdb *RemoteDB) getLatestRelease(packageID int) (*model.Release, error) {
	var release model.Release
	err := rdb.Connection.QueryRow(`
		SELECT id, package_id, version, binary_url, instructions, 
		       checksum_sha256, file_size_bytes, release_notes, 
		       is_prerelease, is_deprecated, released_at
		FROM releases
		WHERE package_id = ? AND is_deprecated = FALSE
		ORDER BY released_at DESC
		LIMIT 1`,
		packageID,
	).Scan(&release.ID, &release.PackageID, &release.Version, &release.BinaryURL,
		&release.Instructions, &release.ChecksumSHA256, &release.FileSizeBytes,
		&release.ReleaseNotes, &release.IsPrerelease, &release.IsDeprecated, &release.ReleasedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no releases found for package")
	}
	if err != nil {
		return nil, err
	}
	return &release, nil
}

func (rdb *RemoteDB) getExactRelease(packageID int, versionStr string) (*model.Release, error) {
	v, err := version.Parse(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %s", versionStr)
	}
	normalizedVersion := v.String()

	var release model.Release
	err = rdb.Connection.QueryRow(`
		SELECT id, package_id, version, binary_url, instructions,
		       checksum_sha256, file_size_bytes, release_notes,
		       is_prerelease, is_deprecated, released_at
		FROM releases
		WHERE package_id = ? AND version = ?
		LIMIT 1`,
		packageID, normalizedVersion,
	).Scan(&release.ID, &release.PackageID, &release.Version, &release.BinaryURL,
		&release.Instructions, &release.ChecksumSHA256, &release.FileSizeBytes,
		&release.ReleaseNotes, &release.IsPrerelease, &release.IsDeprecated, &release.ReleasedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("version '%s' not found", versionStr)
	}
	if err != nil {
		return nil, err
	}
	return &release, nil
}

func (rdb *RemoteDB) getReleaseByConstraint(packageID int, constraint string) (*model.Release, error) {
	releases, err := rdb.GetAllReleases(packageID)
	if err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	var bestMatch *model.Release
	var bestVersion *version.Version

	for i := range releases {
		if releases[i].IsDeprecated {
			continue
		}

		v, err := version.Parse(releases[i].Version)
		if err != nil {
			continue
		}

		compatible, err := v.IsCompatible(constraint)
		if err != nil {
			return nil, fmt.Errorf("invalid constraint '%s': %w", constraint, err)
		}

		if compatible {
			if bestVersion == nil || v.GreaterThan(bestVersion) {
				bestVersion = v
				bestMatch = &releases[i]
			}
		}
	}

	if bestMatch == nil {
		return nil, fmt.Errorf("no version satisfies constraint '%s'", constraint)
	}

	return bestMatch, nil
}

// GetAllReleases returns all releases for a package
func (rdb *RemoteDB) GetAllReleases(packageID int) ([]model.Release, error) {
	rows, err := rdb.Connection.Query(`
		SELECT id, package_id, version, binary_url, instructions,
		       checksum_sha256, file_size_bytes, release_notes,
		       is_prerelease, is_deprecated, released_at
		FROM releases
		WHERE package_id = ?
		ORDER BY released_at DESC`,
		packageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []model.Release
	for rows.Next() {
		var r model.Release
		err := rows.Scan(&r.ID, &r.PackageID, &r.Version, &r.BinaryURL,
			&r.Instructions, &r.ChecksumSHA256, &r.FileSizeBytes,
			&r.ReleaseNotes, &r.IsPrerelease, &r.IsDeprecated, &r.ReleasedAt)
		if err != nil {
			return nil, err
		}
		releases = append(releases, r)
	}
	return releases, nil
}

// GetAllReleasesByName returns all releases for a package by name
func (rdb *RemoteDB) GetAllReleasesByName(packageName string) ([]model.Release, error) {
	pkg, err := rdb.GetPackageInfo(packageName)
	if err != nil {
		return nil, err
	}
	return rdb.GetAllReleases(pkg.ID)
}

// ListAllPackages returns all packages with their latest version
func (rdb *RemoteDB) ListAllPackages() ([]model.PackageSummary, error) {
	rows, err := rdb.Connection.Query(`
		SELECT p.id, p.name, p.description,
		       (SELECT r.version FROM releases r 
		        WHERE r.package_id = p.id AND r.is_deprecated = FALSE 
		        ORDER BY r.released_at DESC LIMIT 1) as latest_version
		FROM packages p
		ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []model.PackageSummary
	for rows.Next() {
		var ps model.PackageSummary
		err := rows.Scan(&ps.ID, &ps.Name, &ps.Description, &ps.LatestVersion)
		if err != nil {
			return nil, err
		}
		packages = append(packages, ps)
	}
	return packages, nil
}

// SearchPackages searches for packages by name or description
func (rdb *RemoteDB) SearchPackages(query string) ([]model.PackageSummary, error) {
	rows, err := rdb.Connection.Query(`
		SELECT p.id, p.name, p.description,
		       (SELECT r.version FROM releases r 
		        WHERE r.package_id = p.id AND r.is_deprecated = FALSE 
		        ORDER BY r.released_at DESC LIMIT 1) as latest_version
		FROM packages p
		WHERE p.name LIKE ? OR p.description LIKE ?
		ORDER BY p.name`,
		"%"+query+"%", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []model.PackageSummary
	for rows.Next() {
		var ps model.PackageSummary
		err := rows.Scan(&ps.ID, &ps.Name, &ps.Description, &ps.LatestVersion)
		if err != nil {
			return nil, err
		}
		packages = append(packages, ps)
	}
	return packages, nil
}

// GetDependencies returns dependencies for a release
func (rdb *RemoteDB) GetDependencies(releaseID int) ([]model.ReleaseDependency, error) {
	rows, err := rdb.Connection.Query(`
		SELECT d.id, d.release_id, p.name, d.version_constraint, d.dependency_type
		FROM dependencies d
		JOIN packages p ON d.depends_on_package_id = p.id
		WHERE d.release_id = ?`,
		releaseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []model.ReleaseDependency
	for rows.Next() {
		var d model.ReleaseDependency
		err := rows.Scan(&d.ID, &d.ReleaseID, &d.PackageName, &d.VersionConstraint, &d.DependencyType)
		if err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	return deps, nil
}

// GetPlatformCompatibility returns platform info for a release
func (rdb *RemoteDB) GetPlatformCompatibility(releaseID int) ([]model.PlatformCompat, error) {
	rows, err := rdb.Connection.Query(`
		SELECT id, release_id, os, arch, binary_url
		FROM platform_compatibility
		WHERE release_id = ?`,
		releaseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var platforms []model.PlatformCompat
	for rows.Next() {
		var p model.PlatformCompat
		err := rows.Scan(&p.ID, &p.ReleaseID, &p.OS, &p.Arch, &p.BinaryURL)
		if err != nil {
			return nil, err
		}
		platforms = append(platforms, p)
	}
	return platforms, nil
}

// GetPackageTags returns tags for a package
func (rdb *RemoteDB) GetPackageTags(packageID int) ([]string, error) {
	rows, err := rdb.Connection.Query(`
		SELECT tag FROM package_tags WHERE package_id = ? ORDER BY tag`,
		packageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// GetPackagesByTag returns packages with a specific tag
func (rdb *RemoteDB) GetPackagesByTag(tag string) ([]model.PackageSummary, error) {
	rows, err := rdb.Connection.Query(`
		SELECT p.id, p.name, p.description,
		       (SELECT r.version FROM releases r 
		        WHERE r.package_id = p.id AND r.is_deprecated = FALSE 
		        ORDER BY r.released_at DESC LIMIT 1) as latest_version
		FROM packages p
		JOIN package_tags pt ON p.id = pt.package_id
		WHERE pt.tag = ?
		ORDER BY p.name`,
		tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []model.PackageSummary
	for rows.Next() {
		var ps model.PackageSummary
		err := rows.Scan(&ps.ID, &ps.Name, &ps.Description, &ps.LatestVersion)
		if err != nil {
			return nil, err
		}
		packages = append(packages, ps)
	}
	return packages, nil
}

func (rdb *RemoteDB) Close() {
	rdb.Connection.Close()
}
