package cmd

import (
	"fmt"
	"jpm/db"
	"jpm/lib"
	"jpm/model"
	"strings"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <package-name>",
	Short: "Show detailed information about an installed package",
	Long: `Display comprehensive information about an installed package including:
  • Installation details
  • File locations
  • Environment modifications
  • Dependencies
  • Installation history

Examples:
  jpm info nodejs                # Show info for nodejs`,
	Args: cobra.ExactArgs(1),
	Run:  showInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func showInfo(cmd *cobra.Command, args []string) {
	packageName := args[0]

	ldb := db.NewLocalDB()
	defer ldb.Close()

	// Get installation info
	inst, err := ldb.GetByName(packageName)
	if err != nil {
		fmt.Printf("%sError: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	if inst == nil {
		fmt.Printf("%sPackage '%s' is not installed%s\n", lib.Yellow, packageName, lib.Reset)
		fmt.Println("\nTip: Use 'jpm search " + packageName + "' to find it in the repository")
		return
	}

	// Display header
	fmt.Printf("%s%s%s\n", lib.Blue, inst.Name, lib.Reset)
	fmt.Println(strings.Repeat("=", len(inst.Name)+10))
	fmt.Println()

	// Basic information
	fmt.Printf("Version:        %s\n", inst.Version)
	fmt.Printf("Status:         %s", inst.Status)
	if inst.Status == "completed" {
		fmt.Printf(" %s✓%s", lib.Green, lib.Reset)
	} else if inst.Status == "failed" {
		fmt.Printf(" %s✗%s", lib.Red, lib.Reset)
	}
	fmt.Println()

	fmt.Printf("Installed:      %s\n", inst.InstalledAt.Format("2006-01-02 15:04:05"))
	if inst.UpdatedAt.After(inst.InstalledAt) {
		fmt.Printf("Last Updated:   %s\n", inst.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	if inst.InstalledFromURL != "" {
		fmt.Printf("Downloaded From: %s\n", inst.InstalledFromURL)
	}

	if inst.FileSizeBytes > 0 {
		fmt.Printf("Size:           %s\n", formatBytes(inst.FileSizeBytes))
	}

	if inst.ChecksumSHA256 != "" {
		fmt.Printf("Checksum:       %s\n", inst.ChecksumSHA256[:16]+"...")
	}

	// Location information
	if inst.Location != "" || inst.SysPath != "" {
		fmt.Println("\n" + strings.Repeat("-", 50))
		fmt.Println("Location")
		fmt.Println(strings.Repeat("-", 50))

		if inst.Location != "" {
			fmt.Printf("Install Path:   %s\n", inst.Location)
		}
		if inst.SysPath != "" {
			fmt.Printf("PATH Entry:     %s\n", inst.SysPath)
		}
	}

	// Installed files
	files, err := ldb.GetInstalledFiles(inst.ID)
	if err == nil && len(files) > 0 {
		fmt.Println("\n" + strings.Repeat("-", 50))
		fmt.Println("Installed Files")
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("Total: %d file(s)\n\n", len(files))

		// Group by type
		filesByType := make(map[string][]model.InstalledFile)
		for _, f := range files {
			fileType := f.FileType
			if fileType == "" {
				fileType = "other"
			}
			filesByType[fileType] = append(filesByType[fileType], f)
		}

		for fileType, typeFiles := range filesByType {
			fmt.Printf("%s (%d):\n", strings.Title(fileType), len(typeFiles))
			for i, f := range typeFiles {
				if i < 5 { // Show first 5 of each type
					exec := ""
					if f.IsExecutable {
						exec = " [executable]"
					}
					fmt.Printf("  • %s%s\n", f.FilePath, exec)
				}
			}
			if len(typeFiles) > 5 {
				fmt.Printf("  ... and %d more\n", len(typeFiles)-5)
			}
			fmt.Println()
		}
	}

	// Environment modifications
	envMods, err := ldb.GetEnvModifications(inst.ID)
	if err == nil && len(envMods) > 0 {
		fmt.Println(strings.Repeat("-", 50))
		fmt.Println("Environment Modifications")
		fmt.Println(strings.Repeat("-", 50))

		for _, mod := range envMods {
			fmt.Printf("Type:     %s\n", mod.ModificationType)
			if mod.VariableName != "" {
				fmt.Printf("Variable: %s\n", mod.VariableName)
			}
			if mod.VariableValue != "" {
				fmt.Printf("Value:    %s\n", mod.VariableValue)
			}
			fmt.Println()
		}
	}

	// Dependencies
	deps, err := ldb.GetDependencies(inst.ID)
	if err == nil && len(deps) > 0 {
		fmt.Println(strings.Repeat("-", 50))
		fmt.Println("Dependencies")
		fmt.Println(strings.Repeat("-", 50))

		for _, dep := range deps {
			autoInstalled := ""
			if dep.IsAutoInstalled {
				autoInstalled = " (auto-installed)"
			}
			version := dep.DependencyVersion
			if version == "" {
				version = "any"
			}
			fmt.Printf("  • %s@%s%s\n", dep.DependencyName, version, autoInstalled)
		}
		fmt.Println()
	}

	// Installation history for this package
	history, err := ldb.GetHistory(packageName, 5)
	if err == nil && len(history) > 0 {
		fmt.Println(strings.Repeat("-", 50))
		fmt.Println("Recent History")
		fmt.Println(strings.Repeat("-", 50))

		for _, h := range history {
			status := "✓"
			statusColor := lib.Green
			if !h.Success {
				status = "✗"
				statusColor = lib.Red
			}

			date := h.PerformedAt.Format("2006-01-02 15:04")
			versionInfo := h.Version
			if h.PreviousVersion != "" {
				versionInfo = fmt.Sprintf("%s → %s", h.PreviousVersion, h.Version)
			}

			fmt.Printf("%s%s %s%-10s%s %s\n", status, date, statusColor, h.Action, lib.Reset, versionInfo)

			if !h.Success && h.ErrorMessage != "" {
				fmt.Printf("           Error: %s\n", h.ErrorMessage)
			}
		}
		fmt.Println()
	}

	// Check for updates
	rdb := db.NewRemoteDB()
	defer rdb.Close()

	cached, _ := ldb.GetCachedMetadata(packageName)
	if cached != nil && cached.LatestVersion != inst.Version {
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("%sUpdate Available%s\n", lib.Yellow, lib.Reset)
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("Current:  %s\n", inst.Version)
		fmt.Printf("Latest:   %s%s%s\n", lib.Green, cached.LatestVersion, lib.Reset)
		fmt.Printf("\nRun: jpm update %s\n", packageName)
	} else if inst.Status == "completed" {
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("%s✓ Package is up to date%s\n", lib.Green, lib.Reset)
	}

	// Error information if failed
	if inst.Status == "failed" && inst.ErrorMessage != "" {
		fmt.Println("\n" + strings.Repeat("-", 50))
		fmt.Printf("%sInstallation Error%s\n", lib.Red, lib.Reset)
		fmt.Println(strings.Repeat("-", 50))
		fmt.Println(inst.ErrorMessage)
	}
}
