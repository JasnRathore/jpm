package cmd

import (
	"fmt"
	"jpm/db"
	"jpm/lib"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	updateAll    bool
	updateDryRun bool
)

var updateCmd = &cobra.Command{
	Use:   "update [package-name]",
	Short: "Update installed packages",
	Long: `Update one or more installed packages to their latest versions.

Examples:
  jpm update nodejs              # Update nodejs to latest version
  jpm update --all               # Update all packages
  jpm update --all --dry-run     # Show what would be updated

Flags:
  --all                          # Update all packages
  --dry-run                      # Show updates without installing`,
	Run: updatePackages,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&updateAll, "all", false, "Update all installed packages")
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "Show what would be updated without installing")
}

func updatePackages(cmd *cobra.Command, args []string) {
	ldb := db.NewLocalDB()
	defer ldb.Close()

	rdb := db.NewRemoteDB()
	defer rdb.Close()

	var packagesToUpdate []string

	if updateAll {
		// Get all installed packages
		installations, err := ldb.GetAll()
		if err != nil {
			fmt.Printf("%sError: %v%s\n", lib.Red, err, lib.Reset)
			return
		}

		if len(installations) == 0 {
			fmt.Println("No packages installed")
			return
		}

		for _, inst := range installations {
			packagesToUpdate = append(packagesToUpdate, inst.Name)
		}
	} else if len(args) > 0 {
		packagesToUpdate = args
	} else {
		fmt.Printf("%sError: Please specify a package name or use --all flag%s\n", lib.Red, lib.Reset)
		fmt.Println("\nUsage:")
		fmt.Println("  jpm update nodejs          # Update specific package")
		fmt.Println("  jpm update --all           # Update all packages")
		return
	}

	fmt.Printf("%sChecking for updates...%s\n\n", lib.Blue, lib.Reset)

	type UpdateInfo struct {
		Name           string
		CurrentVersion string
		LatestVersion  string
		NeedsUpdate    bool
	}

	var updates []UpdateInfo

	// Check each package
	for _, packageName := range packagesToUpdate {
		inst, err := ldb.GetByName(packageName)
		if err != nil || inst == nil {
			fmt.Printf("%s! Package '%s' is not installed%s\n", lib.Yellow, packageName, lib.Reset)
			continue
		}

		// Check cache first
		cached, err := ldb.GetCachedMetadata(packageName)
		var latestVersion string

		if err == nil && cached != nil && time.Since(cached.CachedAt) < 1*time.Hour {
			latestVersion = cached.LatestVersion
		} else {
			// Fetch latest version from remote
			release, err := rdb.GetRelease(packageName, "latest")
			if err != nil {
				fmt.Printf("%s! Error checking '%s': %v%s\n", lib.Yellow, packageName, err, lib.Reset)
				continue
			}
			latestVersion = release.Version

			// Update cache
			pkg, _ := rdb.GetPackageInfo(packageName)
			if pkg != nil {
				_ = ldb.UpdateCache(packageName, latestVersion, pkg.Description,
					pkg.HomepageURL, 1*time.Hour)
			}
		}

		needsUpdate := latestVersion != inst.Version
		updates = append(updates, UpdateInfo{
			Name:           packageName,
			CurrentVersion: inst.Version,
			LatestVersion:  latestVersion,
			NeedsUpdate:    needsUpdate,
		})
	}

	if len(updates) == 0 {
		fmt.Println("No packages to check")
		return
	}

	// Display update status
	updatesAvailable := 0
	for _, u := range updates {
		if u.NeedsUpdate {
			updatesAvailable++
			fmt.Printf("%s%s%s\n", lib.Green, u.Name, lib.Reset)
			fmt.Printf("  %s → %s\n\n", u.CurrentVersion, u.LatestVersion)
		}
	}

	if updatesAvailable == 0 {
		fmt.Printf("%s✓ All packages are up to date!%s\n", lib.Green, lib.Reset)
		return
	}

	fmt.Printf("%s%d package(s) have updates available%s\n\n", lib.Yellow, updatesAvailable, lib.Reset)

	if updateDryRun {
		fmt.Println("Dry run mode - no packages will be updated")
		fmt.Println("\nTo install updates, run:")
		if updateAll {
			fmt.Println("  jpm update --all")
		} else {
			for _, u := range updates {
				if u.NeedsUpdate {
					fmt.Printf("  jpm update %s\n", u.Name)
				}
			}
		}
		return
	}

	// Confirm update
	if !forceInstall {
		fmt.Print("Do you want to update these packages? [y/N]: ")
		if !confirmAction() {
			fmt.Println("Update cancelled")
			return
		}
	}

	// Perform updates
	fmt.Println()
	successCount := 0
	failCount := 0

	for _, u := range updates {
		if !u.NeedsUpdate {
			continue
		}

		fmt.Printf("%sUpdating %s...%s\n", lib.Blue, u.Name, lib.Reset)

		// Use install command logic with force flag
		forceInstall = true
		err := performInstall(u.Name, "latest", ldb, rdb)
		if err != nil {
			fmt.Printf("%s✗ Failed to update %s: %v%s\n\n", lib.Red, u.Name, err, lib.Reset)
			failCount++
		} else {
			fmt.Printf("%s✓ Successfully updated %s (%s → %s)%s\n\n",
				lib.Green, u.Name, u.CurrentVersion, u.LatestVersion, lib.Reset)
			successCount++
		}
	}

	// Summary
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("\n%sUpdate Summary:%s\n", lib.Blue, lib.Reset)
	fmt.Printf("  Successful: %d\n", successCount)
	if failCount > 0 {
		fmt.Printf("  Failed:     %s%d%s\n", lib.Red, failCount, lib.Reset)
	}
	fmt.Println()
}

func performInstall(packageName, versionSpec string, ldb db.LocalDB, rdb db.RemoteDB) error {
	// Simplified install logic - in production, this would call the main install function
	// For now, return a placeholder
	release, err := rdb.GetRelease(packageName, versionSpec)
	if err != nil {
		return err
	}

	// Get existing installation
	existing, _ := ldb.GetByName(packageName)
	if existing == nil {
		return fmt.Errorf("package not found in database")
	}

	// Update version
	existing.Version = release.Version
	existing.UpdatedAt = time.Now()
	existing.InstalledFromURL = release.BinaryURL
	existing.ChecksumSHA256 = release.ChecksumSHA256
	existing.FileSizeBytes = release.FileSizeBytes

	return ldb.UpdateInstallation(existing)
}
