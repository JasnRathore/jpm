# JPM — Jasn Package Manager

> A lightweight, extensible CLI package manager written in Go, backed by a remote Turso/libSQL repository and a local SQLite database for tracking installations, history, and environment state.

---

## What is JPM?

JPM is a developer-facing package manager that lets you **search, install, update, and remove packages** from a centralized remote repository — all from your terminal. Think of it as a slimmed-down `apt` or `brew`, but one you own entirely.

Under the hood, JPM:
- Fetches package metadata and release binaries from a **remote libSQL/Turso database**
- Tracks every installation, file, and environment modification in a **local SQLite database**
- Parses and executes a lightweight **declarative instruction language** to handle extraction, file moves, PATH updates, and more
- Resolves **semantic versioning constraints** (`^`, `~`, `>=`, `x` wildcards) to pick the right release

---

## Project Requirements

- **Go 1.24.4+**
- A [Turso](https://turso.tech) account with a remote database URL and auth token (for the package registry)
- A `config/.env` file with the following variables:
```env
URL=libsql://your-database.turso.io
TOKEN=your-turso-auth-token
```

> The config is embedded at build time using Go's `embed` package, so the `.env` must exist before building.

---

## Dependencies

JPM relies on these Go modules:

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/tursodatabase/libsql-client-go` | Remote Turso/libSQL client |
| `github.com/tursodatabase/turso-go` | Local libSQL (SQLite-compatible) driver |
| `github.com/dustin/go-humanize` | Human-readable file sizes in download progress |
| `github.com/joho/godotenv` | `.env` parsing for embedded config |

Install all dependencies with:
```bash
go mod download
```

---

## Getting Started

### 1. Configure your environment

Create `config/.env` with your Turso credentials:
```bash
echo "URL=libsql://your-db.turso.io" > config/.env
echo "TOKEN=your-token" >> config/.env
```

### 2. Build the binary
```bash
go build -o jpm .
```

Or cross-compile for a target platform:
```bash
GOOS=linux GOARCH=amd64 go build -o jpm-linux .
```

### 3. Initialize the local database

JPM uses a local `jpm.db` SQLite file to track everything it does. Before using any commands, bootstrap it:
```bash
./jpm initdb
```

Output:
```
Initializing local database...

Creating tables and indexes...
✓ Database schema initialized successfully

Database location: jpm.db
```

This is safe to run multiple times — it won't overwrite existing data.

---

## How to Run the Application
```bash
./jpm [command] [flags]
```

### Available Commands

| Command | Description |
|---|---|
| `initdb` | Initialize the local SQLite schema |
| `search [name]` | Browse or search packages in the remote registry |
| `install <name>[@version]` | Download and install a package |
| `list` | Show installed packages |
| `update [name]` | Update one or all packages |
| `remove <name>` | Uninstall a package and clean up |
| `info <name>` | Show detailed info about an installed package |

---

## Relevant Examples

### Searching for packages
```bash
# List everything in the registry
./jpm search

# Find a specific package
./jpm search nodejs

# Show all available versions
./jpm search nodejs --all

# Filter by tag
./jpm search --tag database

# Show full metadata
./jpm search nodejs --detail
```

### Installing packages

JPM supports the full range of semver constraint syntax:
```bash
./jpm install nodejs               # Latest stable
./jpm install nodejs@1.2.3        # Exact version
./jpm install nodejs@^1.2.0       # Compatible with 1.x.x (>=1.2.0, <2.0.0)
./jpm install nodejs@~1.2.0       # Patch-range: 1.2.x
./jpm install nodejs@>=1.2.0      # Any version >= 1.2.0
./jpm install nodejs@1.2.x        # Wildcard patch
./jpm install nodejs --force       # Reinstall even if already present
```

### Listing installed packages
```bash
./jpm list                        # Compact table view
./jpm list -v                     # Verbose, with descriptions and paths
./jpm list --outdated             # Only packages with updates available
./jpm list --history              # Full installation audit log
./jpm list --history --limit 50   # Last 50 history entries
```

### Viewing package details
```bash
./jpm info nodejs
```

This prints version, install path, PATH entries, tracked files, environment modifications, dependency tree, and recent history for that package.

### Updating packages
```bash
./jpm update nodejs               # Update one package
./jpm update --all                # Update everything
./jpm update --all --dry-run      # Preview what would change
```

### Removing packages
```bash
./jpm remove nodejs               # Interactive prompt
./jpm remove nodejs --force       # Skip confirmation
./jpm remove nodejs --auto-clean  # Also remove orphaned auto-dependencies
```

---

## How Installation Works

When you run `jpm install`, here's what happens behind the scenes:

1. **Fetch** — JPM queries the remote database for the package and resolves the version constraint
2. **Download** — The binary/archive is downloaded to the working directory (default: `bin/`) with live progress output
3. **Verify** — SHA-256 checksum is validated (skip with `--skip-verify`)
4. **Parse** — The release's `instructions` field is parsed into a sequence of steps
5. **Execute** — Steps are run in order using JPM's instruction language (see below)
6. **Record** — The installation, files, and environment changes are saved to `jpm.db`

### The Instruction Language

Each package release includes an `instructions` field stored in the remote database. JPM parses this into executable steps:
```
# Example instruction set for a CLI tool
EXTRACT app-v1.2.3.zip
CHMOD app/bin/mytool
SET_LOCATION app/
ADD_TO_PATH app/bin
DELETE app-v1.2.3.zip
```

Supported instructions:

| Instruction | Arguments | Description |
|---|---|---|
| `EXTRACT` | `<src> [dest]` | Extract a `.zip` archive |
| `EXTRACT_TAR` | `<src> [dest]` | Extract a `.tar` archive |
| `EXTRACT_TARGZ` | `<src> [dest]` | Extract a `.tar.gz` archive |
| `MOVE` | `<src> <dst>` | Move a file or directory |
| `COPY` | `<src> <dst>` | Copy a file or directory |
| `RENAME` | `<src> <dst>` | Alias for `MOVE` |
| `DELETE` | `<path>` | Delete a file or directory |
| `CHMOD` | `<path>` | Make a file executable (`chmod +x`) |
| `ADD_TO_PATH` | `<dir>` | Append a directory to the system `PATH` |
| `SET_LOCATION` | `<path>` | Record the install location in the database |

Paths with spaces are supported using single or double quotes:
```
MOVE "Program Files/app" bin/app
```

---

## Running the Tests

JPM has unit test coverage for the parser and version resolver:
```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Run benchmarks
go test ./version -bench=.
go test ./parser -bench=.
```

The test suite covers version parsing, semver constraint evaluation, instruction parsing, argument quoting, and validation edge cases.

---

## Database Schema Overview

JPM maintains two databases:

**Local (`jpm.db`)** — tracks your machine's state:
- `installed` — one row per installed package
- `installed_files` — individual files placed on disk
- `environment_modifications` — PATH and env var changes
- `installation_history` — full audit log of every action
- `installed_dependencies` — dependency graph
- `metadata_cache` — cached remote metadata with TTL

**Remote (Turso/libSQL)** — the package registry:
- `packages` — package names, descriptions, metadata
- `releases` — versioned binaries with instructions and checksums
- `dependencies` — inter-package dependency declarations
- `platform_compatibility` — per-OS/arch binary URLs
- `package_tags` — searchable tags

---

## Conclusion

JPM is a solid foundation for a self-hosted package manager. It handles the hardest parts — semver resolution, declarative installation steps, environment tracking, and rollback-friendly record-keeping — while staying small enough to understand and extend.

Whether you're using it as-is or as a starting point for your own distribution system, the codebase is organized cleanly across `cmd/`, `db/`, `parser/`, `lib/`, and `model/` packages with clear separation of concerns.

Contributions, issues, and ideas are welcome. Pull requests that add new instruction types, platform-aware binary selection, or rollback support would be a great place to start.
