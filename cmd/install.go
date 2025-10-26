package cmd

import (
	"fmt"
	"jpm/db"
	"jpm/lib"
	"jpm/model"
	"jpm/parser"
	"jpm/version"
	"os"
	"path/filepath"
	"strings"

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
  --skip-verify                   # Skip verification
  --work-dir string               # Working directory (default "bin")`,
	Args: cobra.ExactArgs(1),
	Run:  install,
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().BoolVarP(&forceInstall, "force", "f", false, "Force reinstall even if already installed")
	installCmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "Skip verification steps")
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
			// Check if it's a valid version or constraint
			if !strings.ContainsAny(versionSpec, ">=<^~*x") {
				// It should be a valid version
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
		versionSpec = "" // Will get latest
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
		existingPkgs := ldb.GetAll()
		for _, pkg := range existingPkgs {
			if pkg.Name == packageName {
				// Check if same version is requested
				if versionSpec == "" || versionSpec == "latest" {
					// Check if latest is already installed
					metadata, err := rdb.GetVersion(packageName, "latest")
					if err == nil && metadata.Version == pkg.Version {
						fmt.Printf("%sPackage '%s' is already at latest version (%s)%s\n",
							lib.Yellow, packageName, pkg.Version, lib.Reset)
						fmt.Println("Use --force to reinstall")
						return
					}
				} else if versionSpec == pkg.Version {
					fmt.Printf("%sPackage '%s' version %s is already installed%s\n",
						lib.Yellow, packageName, pkg.Version, lib.Reset)
					fmt.Println("Use --force to reinstall")
					return
				}

				// Different version requested - show upgrade/downgrade message
				fmt.Printf("%s%s '%s' from %s to %s%s\n",
					lib.Yellow,
					getUpgradeDowngradeText(pkg.Version, versionSpec),
					packageName, pkg.Version, versionSpec, lib.Reset)
			}
		}
	}

	// Fetch package metadata
	fmt.Println("Fetching package metadata...")
	metadata, err := rdb.GetVersion(packageName, versionSpec)
	if err != nil {
		fmt.Printf("%sError: %v%s\n", lib.Red, err, lib.Reset)

		// Suggest available versions
		versions, vErr := rdb.GetAllVersions(packageName)
		if vErr == nil && len(versions) > 0 {
			fmt.Printf("\nAvailable versions for '%s':\n", packageName)
			for i, v := range versions {
				if i < 10 { // Show first 10
					if v.IsLatest {
						fmt.Printf("  %s• %s (latest)%s\n", lib.Green, v.Version, lib.Reset)
					} else {
						fmt.Printf("  • %s\n", v.Version)
					}
				}
			}
			if len(versions) > 10 {
				fmt.Printf("  ... and %d more\n", len(versions)-10)
			}
			fmt.Println("\nTip: Use 'jpm search " + packageName + " --all' to see all versions")
		}
		return
	}

	fmt.Printf("Found version: %s%s%s\n", lib.Green, metadata.Version, lib.Reset)

	// Ensure working directory exists
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		fmt.Printf("%sError creating working directory: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	// Get absolute path for working directory
	absWorkDir, err := filepath.Abs(workingDir)
	if err != nil {
		fmt.Printf("%sError resolving working directory: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	// Download the package
	fmt.Println("\nDownloading package...")
	if err := lib.Download(metadata.Url, absWorkDir); err != nil {
		fmt.Printf("%sDownload failed: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	// Parse installation instructions
	fmt.Println("\nParsing installation instructions...")
	p := parser.NewParser()
	instructions, err := p.Parse(metadata.Instructions)
	if err != nil {
		fmt.Printf("%sInvalid installation instructions: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	fmt.Printf("Found %d installation steps\n", len(instructions))

	// Create installation record
	installation := &model.Installed{
		Name:     packageName,
		Version:  metadata.Version,
		Location: "",
		SysPath:  "",
	}

	// Execute installation instructions
	fmt.Println("\nExecuting installation steps...")
	for i, instruction := range instructions {
		fmt.Printf("  [%d/%d] %s\n", i+1, len(instructions), instruction.RawLine)

		if err := instruction.Run(installation, absWorkDir); err != nil {
			fmt.Printf("%s✗ Step failed: %v%s\n", lib.Red, err, lib.Reset)

			// Attempt cleanup
			fmt.Println("\nAttempting cleanup...")
			cleanup(installation, absWorkDir)
			return
		}

		fmt.Printf("%s  ✓ Success%s\n", lib.Green, lib.Reset)
	}

	// Check if package already exists in DB and update or insert
	existingPkgs := ldb.GetAll()
	packageExists := false
	for _, pkg := range existingPkgs {
		if pkg.Name == packageName {
			packageExists = true
			break
		}
	}

	// Save installation to database
	fmt.Println("\nSaving installation record...")
	if packageExists {
		if err := ldb.UpdateInstallation(installation); err != nil {
			fmt.Printf("%sWarning: Failed to update installation record: %v%s\n",
				lib.Yellow, err, lib.Reset)
		}
	} else {
		if err := ldb.InsertInstallation(installation); err != nil {
			fmt.Printf("%sWarning: Failed to save installation record: %v%s\n",
				lib.Yellow, err, lib.Reset)
			fmt.Println("Package installed but may not appear in 'jpm list'")
		}
	}

	// Success message
	fmt.Printf("\n%s✓ Successfully installed %s (v%s)%s\n",
		lib.Green, packageName, metadata.Version, lib.Reset)

	if installation.SysPath != "" {
		fmt.Printf("Added to PATH: %s\n", installation.SysPath)
		fmt.Printf("%sNote: You may need to restart your terminal for PATH changes to take effect%s\n",
			lib.Yellow, lib.Reset)
	}
}

// getUpgradeDowngradeText determines if this is an upgrade or downgrade
func getUpgradeDowngradeText(currentVersion, newVersionSpec string) string {
	// If it's a constraint, we can't determine without resolving
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

// cleanup attempts to clean up after a failed installation
func cleanup(installation *model.Installed, workDir string) {
	if installation.SysPath != "" {
		fmt.Printf("Removing from PATH: %s\n", installation.SysPath)
		if err := lib.RemoveFromPath(installation.SysPath); err != nil {
			fmt.Printf("Warning: Failed to remove from PATH: %v\n", err)
		}
	}

	if installation.Location != "" && installation.Location != workDir {
		fmt.Printf("Removing extracted files: %s\n", installation.Location)
		if err := lib.Delete(installation.Location); err != nil {
			fmt.Printf("Warning: Failed to remove files: %v\n", err)
		}
	}
}
