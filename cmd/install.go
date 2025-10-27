package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"jpm/db"
	"jpm/lib"
	"jpm/model"
	"jpm/parser"
	"jpm/version"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	forceInstall bool
	skipVerify   bool
	workingDir   string
)

var installCmd = &cobra.Command{
	Use:   "install <package-name>[@version]",
	Short: "Install a package from the remote repository",
	Long: `Install a package by downloading, extracting, and configuring it
according to the package's installation instructions.

Version Specifications:
  jpm install nodejs              # Latest version
  jpm install nodejs@latest       # Explicitly latest
  jpm install nodejs@1.2.3        # Exact version
  jpm install nodejs@^1.2.0       # Compatible with 1.x.x (>=1.2.0, <2.0.0)
  jpm install nodejs@~1.2.0       # Compatible with 1.2.x (>=1.2.0, <1.3.0)
  jpm install nodejs@>=1.2.0      # Greater than or equal to 1.2.0
  jpm install nodejs@1.2.x        # Any 1.2.x version

Flags:
  -f, --force                     # Force reinstall
  --skip-verify                   # Skip checksum verification
  --work-dir string               # Working directory (default "bin")`,
	Args: cobra.ExactArgs(1),
	Run:  install,
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().BoolVarP(&forceInstall, "force", "f", false, "Force reinstall even if already installed")
	installCmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "Skip checksum verification")
	installCmd.Flags().StringVar(&workingDir, "work-dir", "bin", "Working directory for downloads and extractions")
}

func install(cmd *cobra.Command, args []string) {
	packageSpec := args[0]

	// Parse package name and version
	var packageName, versionSpec string
	if strings.Contains(packageSpec, "@") {
		parts := strings.SplitN(packageSpec, "@", 2)
		packageName = parts[0]
		versionSpec = parts[1]

		// Validate version format if it's not a constraint
		if versionSpec != "" && versionSpec != "latest" {
			if !strings.ContainsAny(versionSpec, ">=<^~*x") {
				if _, err := version.Parse(versionSpec); err != nil {
					fmt.Printf("%sInvalid version format '%s': %v%s\n", lib.Red, versionSpec, err, lib.Reset)
					fmt.Println("\nValid version formats:")
					fmt.Println("  • 1.2.3 (exact version)")
					fmt.Println("  • ^1.2.0 (compatible with 1.x)")
					fmt.Println("  • ~1.2.0 (patch updates)")
					fmt.Println("  • >=1.2.0 (greater than or equal)")
					fmt.Println("  • 1.2.x (wildcard)")
					return
				}
			}
		}
	} else {
		packageName = packageSpec
		versionSpec = ""
	}

	if versionSpec == "" {
		fmt.Printf("%sInstalling package: %s (latest)%s\n", lib.Blue, packageName, lib.Reset)
	} else {
		fmt.Printf("%sInstalling package: %s@%s%s\n", lib.Blue, packageName, versionSpec, lib.Reset)
	}

	// Initialize databases
	rdb := db.NewRemoteDB()
	defer rdb.Close()

	ldb := db.NewLocalDB()
	defer ldb.Close()

	// Check if already installed
	if !forceInstall {
		existing, err := ldb.GetByName(packageName)
		if err == nil && existing != nil && existing.IsCompleted() {
			// Check if we need to update
			if versionSpec == "" || versionSpec == "latest" {
				release, err := rdb.GetRelease(packageName, "latest")
				if err == nil && release.Version == existing.Version {
					fmt.Printf("%sPackage '%s' is already at latest version (%s)%s\n",
						lib.Yellow, packageName, existing.Version, lib.Reset)
					fmt.Println("Use --force to reinstall")
					return
				}
			} else if versionSpec == existing.Version {
				fmt.Printf("%sPackage '%s' version %s is already installed%s\n",
					lib.Yellow, packageName, existing.Version, lib.Reset)
				fmt.Println("Use --force to reinstall")
				return
			}

			// Different version requested
			fmt.Printf("%s%s '%s' from %s to %s%s\n",
				lib.Yellow,
				getUpgradeDowngradeText(existing.Version, versionSpec),
				packageName, existing.Version, versionSpec, lib.Reset)
		}
	}

	// Fetch package info
	fmt.Println("Fetching package information...")
	pkg, err := rdb.GetPackageInfo(packageName)
	if err != nil {
		fmt.Printf("%sError: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	// Get release
	release, err := rdb.GetRelease(packageName, versionSpec)
	if err != nil {
		fmt.Printf("%sError: %v%s\n", lib.Red, err, lib.Reset)

		// Suggest available versions
		releases, vErr := rdb.GetAllReleasesByName(packageName)
		if vErr == nil && len(releases) > 0 {
			fmt.Printf("\nAvailable versions for '%s':\n", packageName)
			for i, r := range releases {
				if i < 10 {
					status := ""
					if r.IsPrerelease {
						status = " (pre-release)"
					} else if r.IsDeprecated {
						status = " (deprecated)"
					}
					fmt.Printf("  • %s%s\n", r.Version, status)
				}
			}
			if len(releases) > 10 {
				fmt.Printf("  ... and %d more\n", len(releases)-10)
			}
			fmt.Println("\nTip: Use 'jpm search " + packageName + " --all' to see all versions")
		}
		return
	}

	fmt.Printf("Found version: %s%s%s", lib.Green, release.Version, lib.Reset)
	if release.IsPrerelease {
		fmt.Printf(" %s(pre-release)%s", lib.Yellow, lib.Reset)
	}
	fmt.Println()

	// Check for deprecation
	if release.IsDeprecated {
		fmt.Printf("%sWarning: This version is deprecated%s\n", lib.Yellow, lib.Reset)
	}

	// Ensure working directory exists
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		fmt.Printf("%sError creating working directory: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	absWorkDir, err := filepath.Abs(workingDir)
	if err != nil {
		fmt.Printf("%sError resolving working directory: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	// Create installation context
	ctx := model.NewInstallationContext(packageName, release.Version, absWorkDir)
	ctx.Installation.InstalledFromURL = release.BinaryURL
	ctx.Installation.ChecksumSHA256 = release.ChecksumSHA256
	ctx.Installation.FileSizeBytes = release.FileSizeBytes
	ctx.Installation.Status = "in_progress"

	// Download the package
	fmt.Println("\nDownloading package...")
	downloadedFile, err := downloadPackage(release.BinaryURL, absWorkDir)
	if err != nil {
		fmt.Printf("%sDownload failed: %v%s\n", lib.Red, err, lib.Reset)
		ctx.MarkFailed(err)
		return
	}

	// Verify checksum if available
	if !skipVerify && release.ChecksumSHA256 != "" {
		fmt.Println("\nVerifying checksum...")
		if err := verifyChecksum(downloadedFile, release.ChecksumSHA256); err != nil {
			fmt.Printf("%sChecksum verification failed: %v%s\n", lib.Red, err, lib.Reset)
			fmt.Println("Use --skip-verify to bypass verification (not recommended)")
			cleanup(ctx, absWorkDir)
			return
		}
		fmt.Printf("%s✓ Checksum verified%s\n", lib.Green, lib.Reset)
	}

	// Parse installation instructions
	fmt.Println("\nParsing installation instructions...")
	p := parser.NewParser()
	instructions, err := p.Parse(release.Instructions)
	if err != nil {
		fmt.Printf("%sInvalid installation instructions: %v%s\n", lib.Red, err, lib.Reset)
		ctx.MarkFailed(err)
		cleanup(ctx, absWorkDir)
		return
	}

	fmt.Printf("Found %d installation steps\n", len(instructions))

	// Execute installation instructions
	fmt.Println("\nExecuting installation steps...")
	for i, instruction := range instructions {
		fmt.Printf("  [%d/%d] %s\n", i+1, len(instructions), instruction.RawLine)

		// Pass the context instead of just the installation
		if err := instruction.RunWithContext(ctx, absWorkDir); err != nil {
			fmt.Printf("%s✗ Step failed: %v%s\n", lib.Red, err, lib.Reset)
			ctx.MarkFailed(err)
			cleanup(ctx, absWorkDir)

			// Record failed installation in history
			_ = ldb.AddHistory(packageName, release.Version, "install", "", false, err.Error())
			return
		}

		fmt.Printf("%s  ✓ Success%s\n", lib.Green, lib.Reset)
	}

	// Mark installation as completed
	ctx.MarkCompleted()
	ctx.Installation.UpdatedAt = time.Now()

	// Check if package already exists and update or insert
	existing, _ := ldb.GetByName(packageName)
	packageExists := existing != nil

	// Save installation to database
	fmt.Println("\nSaving installation record...")
	if packageExists {
		if err := ldb.UpdateInstallation(ctx.Installation); err != nil {
			fmt.Printf("%sWarning: Failed to update installation record: %v%s\n",
				lib.Yellow, err, lib.Reset)
		}
	} else {
		if err := ldb.InsertInstallation(ctx.Installation); err != nil {
			fmt.Printf("%sWarning: Failed to save installation record: %v%s\n",
				lib.Yellow, err, lib.Reset)
		}
	}

	// Save environment modifications
	if len(ctx.EnvMods) > 0 && ctx.Installation.ID > 0 {
		for _, mod := range ctx.EnvMods {
			_ = ldb.AddEnvModification(ctx.Installation.ID, mod.ModificationType,
				mod.VariableName, mod.VariableValue, mod.OriginalValue)
		}
	}

	// Update metadata cache
	_ = ldb.UpdateCache(packageName, release.Version, pkg.Description, pkg.HomepageURL, 24*time.Hour)

	// Success message
	fmt.Printf("\n%s✓ Successfully installed %s (v%s)%s\n",
		lib.Green, packageName, release.Version, lib.Reset)

	if ctx.Installation.SysPath != "" {
		fmt.Printf("Added to PATH: %s\n", ctx.Installation.SysPath)
		fmt.Printf("%sNote: You may need to restart your terminal for PATH changes to take effect%s\n",
			lib.Yellow, lib.Reset)
	}

	if release.ReleaseNotes != "" {
		fmt.Printf("\n%sRelease Notes:%s\n%s\n", lib.Blue, lib.Reset, release.ReleaseNotes)
	}
}

func downloadPackage(url, destDir string) (string, error) {
	if err := lib.Download(url, destDir); err != nil {
		return "", err
	}

	// Return the downloaded file path
	filename := filepath.Base(url)
	return filepath.Join(destDir, filename), nil
}

func verifyChecksum(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

func getUpgradeDowngradeText(currentVersion, newVersionSpec string) string {
	if strings.ContainsAny(newVersionSpec, ">=<^~*x") {
		return "Changing"
	}

	current, err1 := version.Parse(currentVersion)
	new, err2 := version.Parse(newVersionSpec)

	if err1 != nil || err2 != nil {
		return "Changing"
	}

	if new.GreaterThan(current) {
		return "Upgrading"
	} else if new.LessThan(current) {
		return "Downgrading"
	}

	return "Reinstalling"
}

func cleanup(ctx *model.InstallationContext, workDir string) {
	fmt.Println("\nAttempting cleanup...")

	if ctx.Installation.SysPath != "" {
		fmt.Printf("Removing from PATH: %s\n", ctx.Installation.SysPath)
		if err := lib.RemoveFromPath(ctx.Installation.SysPath); err != nil {
			fmt.Printf("Warning: Failed to remove from PATH: %v\n", err)
		}
	}

	if ctx.Installation.Location != "" && ctx.Installation.Location != workDir {
		fmt.Printf("Removing extracted files: %s\n", ctx.Installation.Location)
		if err := lib.Delete(ctx.Installation.Location); err != nil {
			fmt.Printf("Warning: Failed to remove files: %v\n", err)
		}
	}

	// Clean up downloaded files
	for _, file := range ctx.Files {
		if err := lib.Delete(file); err != nil {
			fmt.Printf("Warning: Failed to delete %s: %v\n", file, err)
		}
	}
}
