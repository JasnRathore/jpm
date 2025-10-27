package model

import (
	"fmt"
	"time"
)

// Installation represents an installed package
type Installation struct {
	ID               int
	Name             string
	Version          string
	Location         string
	SysPath          string
	InstalledAt      time.Time
	UpdatedAt        time.Time
	InstalledFromURL string
	ChecksumSHA256   string
	FileSizeBytes    int64
	Status           string // 'pending', 'in_progress', 'completed', 'failed'
	ErrorMessage     string
}

// InstalledFile represents a file installed by a package
type InstalledFile struct {
	ID           int
	InstalledID  int
	FilePath     string
	FileType     string // 'binary', 'library', 'config', 'documentation'
	IsExecutable bool
}

// EnvModification represents an environment modification
type EnvModification struct {
	ID               int
	InstalledID      int
	ModificationType string // 'path_addition', 'env_variable'
	VariableName     string
	VariableValue    string
	OriginalValue    string
	CreatedAt        time.Time
}

// HistoryEntry represents an installation history record
type HistoryEntry struct {
	ID              int
	PackageName     string
	Version         string
	Action          string // 'install', 'update', 'remove', 'rollback'
	PreviousVersion string
	PerformedAt     time.Time
	Success         bool
	ErrorMessage    string
	UserComment     string
}

// Dependency represents a package dependency
type Dependency struct {
	ID                int
	ParentInstalledID int
	DependencyName    string
	DependencyVersion string
	IsAutoInstalled   bool
}

// CachedMetadata represents cached package metadata
type CachedMetadata struct {
	PackageName   string
	LatestVersion string
	Description   string
	HomepageURL   string
	CachedAt      time.Time
	ExpiresAt     time.Time
}

// Package represents a package in the remote repository
type Package struct {
	ID            int
	Name          string
	Description   string
	HomepageURL   string
	RepositoryURL string
	License       string
	Author        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Release represents a package release
type Release struct {
	ID             int
	PackageID      int
	Version        string
	BinaryURL      string
	Instructions   string
	ChecksumSHA256 string
	FileSizeBytes  int64
	ReleaseNotes   string
	IsPrerelease   bool
	IsDeprecated   bool
	ReleasedAt     time.Time
}

// PackageSummary is a lightweight package representation
type PackageSummary struct {
	ID            int
	Name          string
	Description   string
	LatestVersion string
}

// ReleaseDependency represents a dependency in a release
type ReleaseDependency struct {
	ID                int
	ReleaseID         int
	PackageName       string
	VersionConstraint string
	DependencyType    string // 'runtime', 'development', 'optional'
}

// PlatformCompat represents platform compatibility info
type PlatformCompat struct {
	ID        int
	ReleaseID int
	OS        string // 'windows', 'linux', 'darwin', 'all'
	Arch      string // 'amd64', 'arm64', '386', 'all'
	BinaryURL string
}

// Display methods for better output
func (ins *Installation) Display() {
	fmt.Printf("Name: %s\n", ins.Name)
	fmt.Printf("Version: %s\n", ins.Version)
	fmt.Printf("Location: %s\n", ins.Location)
	fmt.Printf("SysPath: %s\n", ins.SysPath)
	fmt.Printf("Installed: %s\n", ins.InstalledAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Status: %s\n", ins.Status)
}

func (pkg *Package) Display() {
	fmt.Printf("Name: %s\n", pkg.Name)
	if pkg.Description != "" {
		fmt.Printf("Description: %s\n", pkg.Description)
	}
	if pkg.HomepageURL != "" {
		fmt.Printf("Homepage: %s\n", pkg.HomepageURL)
	}
	if pkg.License != "" {
		fmt.Printf("License: %s\n", pkg.License)
	}
	if pkg.Author != "" {
		fmt.Printf("Author: %s\n", pkg.Author)
	}
}

func (r *Release) Display() {
	fmt.Printf("Version: %s\n", r.Version)
	fmt.Printf("Released: %s\n", r.ReleasedAt.Format("2006-01-02"))
	if r.IsPrerelease {
		fmt.Printf("Type: Pre-release\n")
	}
	if r.ReleaseNotes != "" {
		fmt.Printf("Release Notes:\n%s\n", r.ReleaseNotes)
	}
}

// Helper methods
func (ins *Installation) IsCompleted() bool {
	return ins.Status == "completed"
}

func (ins *Installation) IsFailed() bool {
	return ins.Status == "failed"
}

func (r *Release) IsStable() bool {
	return !r.IsPrerelease && !r.IsDeprecated
}

// InstallationContext holds context during installation
type InstallationContext struct {
	Installation  *Installation
	WorkDir       string
	ExtractedPath string
	Files         []string
	EnvMods       []EnvModification
}

func NewInstallationContext(name, version, workDir string) *InstallationContext {
	return &InstallationContext{
		Installation: &Installation{
			Name:    name,
			Version: version,
			Status:  "pending",
		},
		WorkDir: workDir,
		Files:   make([]string, 0),
		EnvMods: make([]EnvModification, 0),
	}
}

func (ctx *InstallationContext) AddFile(path, fileType string, isExec bool) {
	ctx.Files = append(ctx.Files, path)
}

func (ctx *InstallationContext) AddEnvMod(modType, varName, varValue, original string) {
	ctx.EnvMods = append(ctx.EnvMods, EnvModification{
		ModificationType: modType,
		VariableName:     varName,
		VariableValue:    varValue,
		OriginalValue:    original,
	})
}

func (ctx *InstallationContext) MarkCompleted() {
	ctx.Installation.Status = "completed"
}

func (ctx *InstallationContext) MarkFailed(err error) {
	ctx.Installation.Status = "failed"
	ctx.Installation.ErrorMessage = err.Error()
}
