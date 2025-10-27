-- Packages table
CREATE TABLE packages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    homepage_url VARCHAR(255),
    repository_url VARCHAR(255),
    license VARCHAR(50),
    author VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_packages_name ON packages(name);

-- Releases table
CREATE TABLE releases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    package_id INTEGER NOT NULL,
    version VARCHAR(20) NOT NULL,
    binary_url VARCHAR(255) NOT NULL,
    instructions TEXT NOT NULL,
    checksum_sha256 VARCHAR(64),
    file_size_bytes INTEGER,
    release_notes TEXT,
    is_prerelease BOOLEAN DEFAULT FALSE,
    is_deprecated BOOLEAN DEFAULT FALSE,
    released_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE,
    UNIQUE(package_id, version)
);

CREATE INDEX idx_releases_package_id ON releases(package_id);
CREATE INDEX idx_releases_version ON releases(package_id, version);
CREATE INDEX idx_releases_released_at ON releases(released_at DESC);

-- Platform compatibility table
CREATE TABLE platform_compatibility (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    release_id INTEGER NOT NULL,
    os VARCHAR(20) NOT NULL,
    arch VARCHAR(20) NOT NULL,
    binary_url VARCHAR(255),
    FOREIGN KEY (release_id) REFERENCES releases(id) ON DELETE CASCADE,
    UNIQUE(release_id, os, arch)
);

CREATE INDEX idx_platform_release ON platform_compatibility(release_id);

-- Dependencies table
CREATE TABLE dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    release_id INTEGER NOT NULL,
    depends_on_package_id INTEGER NOT NULL,
    version_constraint VARCHAR(50),
    dependency_type VARCHAR(20) DEFAULT 'runtime',
    FOREIGN KEY (release_id) REFERENCES releases(id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_package_id) REFERENCES packages(id) ON DELETE CASCADE
);

CREATE INDEX idx_dependencies_release ON dependencies(release_id);

-- Package tags
CREATE TABLE package_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    package_id INTEGER NOT NULL,
    tag VARCHAR(50) NOT NULL,
    FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE,
    UNIQUE(package_id, tag)
);

CREATE INDEX idx_tags_package ON package_tags(package_id);
CREATE INDEX idx_tags_tag ON package_tags(tag);

-- View for easy latest version queries, corrected for SQLite INSTR usage
CREATE VIEW latest_releases AS
SELECT 
    p.name,
    r.version,
    r.binary_url,
    r.instructions,
    r.checksum_sha256,
    r.released_at,
    r.is_prerelease,
    r.is_deprecated,
ROW_NUMBER() OVER (
  PARTITION BY p.id
  ORDER BY
    -- major
    CAST(SUBSTR(r.version, 1, INSTR(r.version, '.') - 1) AS INTEGER) DESC,
    -- minor
    CAST(SUBSTR(
        r.version,
        INSTR(r.version, '.') + 1,
        CASE
          WHEN INSTR(SUBSTR(r.version, INSTR(r.version, '.') + 1), '.') = 0 THEN LENGTH(r.version)
          ELSE INSTR(SUBSTR(r.version, INSTR(r.version, '.') + 1), '.') - 1
        END
    ) AS INTEGER) DESC,
    -- patch
    CAST(
      SUBSTR(
        r.version,
        INSTR(r.version, '.') + 1 + 
        CASE
          WHEN INSTR(SUBSTR(r.version, INSTR(r.version, '.') + 1), '.') = 0 THEN LENGTH(r.version)
          ELSE INSTR(r.version, '.') + INSTR(SUBSTR(r.version, INSTR(r.version, '.') + 1), '.')
        END
      ) AS INTEGER
    ) DESC,
    r.released_at DESC
) as rn
FROM packages p
JOIN releases r ON p.id = r.package_id
WHERE r.is_deprecated = FALSE;

-- View for getting latest stable (non-prerelease) versions
CREATE VIEW latest_stable_releases AS
SELECT * FROM latest_releases 
WHERE is_prerelease = FALSE AND rn = 1;
