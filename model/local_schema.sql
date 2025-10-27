-- Installed packages table with enhanced tracking
CREATE TABLE installed (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) UNIQUE NOT NULL,
    version VARCHAR(20) NOT NULL,
    location VARCHAR(255),
    sys_path VARCHAR(255),
    installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    installed_from_url VARCHAR(255), -- track where it was downloaded from
    checksum_sha256 VARCHAR(64) DEFAULT '', -- verify integrity
    file_size_bytes INTEGER,
    installation_status VARCHAR(20) DEFAULT 'completed', -- 'pending', 'in_progress', 'completed', 'failed'
    error_message TEXT DEFAULT '' -- store error if installation failed
);

CREATE INDEX idx_installed_name ON installed(name);
CREATE INDEX idx_installed_status ON installed(installation_status);
CREATE INDEX idx_installed_installed_at ON installed(installed_at DESC);

-- Installed files table: track individual files for easier uninstall
CREATE TABLE installed_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    installed_id INTEGER NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_type VARCHAR(20), -- 'binary', 'library', 'config', 'documentation'
    is_executable BOOLEAN DEFAULT FALSE,
    FOREIGN KEY (installed_id) REFERENCES installed(id) ON DELETE CASCADE,
    UNIQUE(installed_id, file_path)
);

CREATE INDEX idx_installed_files_package ON installed_files(installed_id);

-- Environment modifications table: track PATH and other env changes
CREATE TABLE environment_modifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    installed_id INTEGER NOT NULL,
    modification_type VARCHAR(20) NOT NULL, -- 'path_addition', 'env_variable'
    variable_name VARCHAR(100),
    variable_value TEXT,
    original_value TEXT, -- for rollback
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (installed_id) REFERENCES installed(id) ON DELETE CASCADE
);

CREATE INDEX idx_env_mods_package ON environment_modifications(installed_id);

-- Installation history: keep audit trail
CREATE TABLE installation_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    package_name VARCHAR(100) NOT NULL,
    version VARCHAR(20) NOT NULL,
    action VARCHAR(20) NOT NULL, -- 'install', 'update', 'remove', 'rollback'
    previous_version VARCHAR(20), -- for updates
    performed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    success BOOLEAN DEFAULT TRUE,
    error_message TEXT,
    user_comment TEXT
);

CREATE INDEX idx_history_package ON installation_history(package_name);
CREATE INDEX idx_history_performed_at ON installation_history(performed_at DESC);

-- Package dependencies (local tracking): what's installed with what
CREATE TABLE installed_dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_installed_id INTEGER NOT NULL,
    dependency_name VARCHAR(100) NOT NULL,
    dependency_version VARCHAR(20),
    is_auto_installed BOOLEAN DEFAULT FALSE, -- was it auto-installed as dependency?
    FOREIGN KEY (parent_installed_id) REFERENCES installed(id) ON DELETE CASCADE
);

CREATE INDEX idx_deps_parent ON installed_dependencies(parent_installed_id);
CREATE INDEX idx_deps_dependency ON installed_dependencies(dependency_name);

-- Configuration table: store package manager settings
CREATE TABLE config (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Package metadata cache: cache remote package info locally
CREATE TABLE metadata_cache (
    package_name VARCHAR(100) PRIMARY KEY,
    latest_version VARCHAR(20),
    description TEXT,
    homepage_url VARCHAR(255),
    cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP
);

CREATE INDEX idx_metadata_expires ON metadata_cache(expires_at);

-- View for easy package listing with update status
CREATE VIEW package_list AS
SELECT 
    i.name,
    i.version as installed_version,
    mc.latest_version,
    CASE 
        WHEN mc.latest_version IS NULL THEN 'unknown'
        WHEN mc.latest_version = i.version THEN 'up_to_date'
        ELSE 'update_available'
    END as update_status,
    i.installed_at,
    i.location,
    i.sys_path,
    (SELECT COUNT(*) FROM installed_dependencies 
     WHERE dependency_name = i.name) as used_by_count
FROM installed i
LEFT JOIN metadata_cache mc ON i.name = mc.package_name
WHERE i.installation_status = 'completed';

-- View for orphaned packages (auto-installed dependencies no longer needed)
CREATE VIEW orphaned_packages AS
SELECT 
    i.name,
    i.version,
    i.installed_at,
    id.is_auto_installed
FROM installed i
JOIN installed_dependencies id ON i.name = id.dependency_name
WHERE id.is_auto_installed = TRUE
AND NOT EXISTS (
    SELECT 1 FROM installed_dependencies id2
    JOIN installed i2 ON id2.parent_installed_id = i2.id
    WHERE id2.dependency_name = i.name
    AND i2.installation_status = 'completed'
);
