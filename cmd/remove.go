package cmd

import (
	"bufio"
	"fmt"
	"jpm/db"
	"jpm/lib"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	removeForce     bool
	removeAutoClean bool
)

var removeCmd = &cobra.Command{
	Use:   "remove <package-name>",
	Short: "Remove an installed package",
	Long: `Remove an installed package and clean up its files, PATH entries, and configurations.

Examples:
  jpm remove nodejs                    # Remove nodejs
  jpm remove nodejs --force            # Remove without confirmation
  jpm remove nodejs --auto-clean       # Also remove unused dependencies

Flags:
  -f, --force                          # Skip confirmation prompt
  --auto-clean                         # Remove unused auto-installed dependencies`,
	Args: cobra.ExactArgs(1),
	Run:  removePackage,
}

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Skip confirmation prompt")
	removeCmd.Flags().BoolVar(&removeAutoClean, "auto-clean", false, "Remove unused auto-installed dependencies")
}

func removePackage(cmd *cobra.Command, args []string) {
	packageName := args[0]

	ldb := db.NewLocalDB()
	defer ldb.Close()

	// Check if package is installed
	installation, err := ldb.GetByName(packageName)
	if err != nil {
		fmt.Printf("%sError: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	if installation == nil {
		fmt.Printf("%sPackage '%s' is not installed%s\n", lib.Yellow, packageName, lib.Reset)
		fmt.Println("\nTip: Use 'jpm list' to see installed packages")
		return
	}

	// Check if other packages depend on this
	deps, err := ldb.GetDependencies(installation.ID)
	if err == nil && len(deps) > 0 {
		fmt.Printf("%sWarning: The following packages depend on '%s':%s\n",
			lib.Yellow, packageName, lib.Reset)
		for _, dep := range deps {
			fmt.Printf("  • %s\n", dep.DependencyName)
		}
		fmt.Println("\nRemoving this package may break these dependencies.")

		if !removeForce {
			fmt.Print("\nDo you want to continue? [y/N]: ")
			if !confirmAction() {
				fmt.Println("Removal cancelled")
				return
			}
		}
	}

	// Show what will be removed
	fmt.Printf("\n%sPackage to remove:%s\n", lib.Blue, lib.Reset)
	fmt.Printf("  Name:     %s\n", installation.Name)
	fmt.Printf("  Version:  %s\n", installation.Version)
	if installation.Location != "" {
		fmt.Printf("  Location: %s\n", installation.Location)
	}
	if installation.SysPath != "" {
		fmt.Printf("  PATH:     %s\n", installation.SysPath)
	}

	// Get installed files
	files, err := ldb.GetInstalledFiles(installation.ID)
	if err == nil && len(files) > 0 {
		fmt.Printf("\n%d file(s) will be removed\n", len(files))
	}

	// Get environment modifications
	envMods, err := ldb.GetEnvModifications(installation.ID)
	if err == nil && len(envMods) > 0 {
		fmt.Printf("%d environment modification(s) will be reverted\n", len(envMods))
	}

	// Confirmation
	if !removeForce {
		fmt.Print("\nAre you sure you want to remove this package? [y/N]: ")
		if !confirmAction() {
			fmt.Println("Removal cancelled")
			return
		}
	}

	// Start removal process
	fmt.Printf("\n%sRemoving package...%s\n", lib.Blue, lib.Reset)

	// Revert environment modifications
	if len(envMods) > 0 {
		fmt.Println("\nReverting environment modifications...")
		for _, mod := range envMods {
			if mod.ModificationType == "path_addition" {
				if err := lib.RemoveFromPath(mod.VariableValue); err != nil {
					fmt.Printf("%sWarning: Failed to remove PATH entry: %v%s\n", lib.Yellow, err, lib.Reset)
				} else {
					fmt.Printf("  ✓ Removed from PATH: %s\n", mod.VariableValue)
				}
			}
		}
	}

	// Remove files
	if len(files) > 0 {
		fmt.Println("\nRemoving installed files...")
		failedFiles := 0
		for _, file := range files {
			if err := lib.Delete(file.FilePath); err != nil {
				failedFiles++
				if removeForce {
					// Only warn in force mode
					fmt.Printf("  ! Could not remove: %s\n", file.FilePath)
				}
			}
		}
		if failedFiles > 0 && !removeForce {
			fmt.Printf("%sWarning: Failed to remove %d file(s)%s\n", lib.Yellow, failedFiles, lib.Reset)
		}
	}

	// Remove installation location
	if installation.Location != "" {
		fmt.Println("\nRemoving installation directory...")
		if err := lib.Delete(installation.Location); err != nil {
			fmt.Printf("%sWarning: Failed to remove directory: %v%s\n", lib.Yellow, err, lib.Reset)
		} else {
			fmt.Printf("  ✓ Removed: %s\n", installation.Location)
		}
	}

	// Remove from database
	if err := ldb.DeleteInstallation(packageName); err != nil {
		fmt.Printf("%sError removing from database: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	// Success
	fmt.Printf("\n%s✓ Successfully removed %s (v%s)%s\n",
		lib.Green, packageName, installation.Version, lib.Reset)

	// Handle auto-clean if requested
	if removeAutoClean {
		cleanOrphanedPackages(ldb)
	}
}

func confirmAction() bool {
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func cleanOrphanedPackages(ldb db.LocalDB) {
	fmt.Printf("\n%sChecking for orphaned dependencies...%s\n", lib.Blue, lib.Reset)

	// Get all installed packages
	allInstalled, err := ldb.GetAll()
	if err != nil {
		fmt.Printf("%sError checking dependencies: %v%s\n", lib.Yellow, err, lib.Reset)
		return
	}

	// Find packages that were auto-installed but no longer needed
	var orphans []string
	for _, installed := range allInstalled {
		// Check if this package is a dependency of any installed package
		isNeeded := false
		for _, other := range allInstalled {
			if other.ID == installed.ID {
				continue
			}
			deps, err := ldb.GetDependencies(other.ID)
			if err != nil {
				continue
			}
			for _, dep := range deps {
				if dep.DependencyName == installed.Name && dep.IsAutoInstalled {
					isNeeded = true
					break
				}
			}
			if isNeeded {
				break
			}
		}

		// If not needed and was auto-installed, mark as orphan
		if !isNeeded {
			// Check if it was auto-installed by looking at its dependencies
			deps, err := ldb.GetDependencies(installed.ID)
			if err == nil {
				for _, dep := range deps {
					if dep.IsAutoInstalled {
						orphans = append(orphans, installed.Name)
						break
					}
				}
			}
		}
	}

	if len(orphans) == 0 {
		fmt.Println("No orphaned packages found")
		return
	}

	fmt.Printf("\nFound %d orphaned package(s):\n", len(orphans))
	for _, name := range orphans {
		fmt.Printf("  • %s\n", name)
	}

	fmt.Print("\nRemove these packages? [y/N]: ")
	if !confirmAction() {
		return
	}

	for _, name := range orphans {
		fmt.Printf("\nRemoving %s...\n", name)
		if err := ldb.DeleteInstallation(name); err != nil {
			fmt.Printf("%sError: %v%s\n", lib.Yellow, err, lib.Reset)
		} else {
			fmt.Printf("%s✓ Removed %s%s\n", lib.Green, name, lib.Reset)
		}
	}
}
