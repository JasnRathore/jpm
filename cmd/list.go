package cmd

import (
	"fmt"
	"jpm/db"
	"jpm/lib"
	"jpm/model"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	listVerbose  bool
	listOutdated bool
	listHistory  bool
	historyLimit int
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packages",
	Long: `List all installed packages with their versions and status.

Examples:
  jpm list                     # List all installed packages
  jpm list -v                  # Show verbose information
  jpm list --outdated          # Show only packages with updates available
  jpm list --history           # Show installation history

Flags:
  -v, --verbose                # Show detailed information
  --outdated                   # Only show packages with updates available
  --history                    # Show installation history`,
	Run: listPackages,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVarP(&listVerbose, "verbose", "v", false, "Show detailed information")
	listCmd.Flags().BoolVar(&listOutdated, "outdated", false, "Only show packages with updates available")
	listCmd.Flags().BoolVar(&listHistory, "history", false, "Show installation history")
	listCmd.Flags().IntVar(&historyLimit, "limit", 20, "Limit history entries (used with --history)")
}

func listPackages(cmd *cobra.Command, args []string) {
	ldb := db.NewLocalDB()
	defer ldb.Close()

	if listHistory {
		showHistory(ldb)
		return
	}

	installations, err := ldb.GetAll()
	if err != nil {
		fmt.Printf("%sError: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	if len(installations) == 0 {
		fmt.Println("No packages installed")
		fmt.Println("\nTip: Use 'jpm search' to find available packages")
		return
	}

	rdb := db.NewRemoteDB()
	defer rdb.Close()

	// Check for updates if needed
	updates := make(map[string]string)
	if listOutdated || listVerbose {
		fmt.Println("Checking for updates...")
		for _, inst := range installations {
			// Check cache first
			cached, err := ldb.GetCachedMetadata(inst.Name)
			if err == nil && cached != nil && time.Since(cached.CachedAt) < 6*time.Hour {
				if cached.LatestVersion != inst.Version {
					updates[inst.Name] = cached.LatestVersion
				}
				continue
			}

			// Fetch from remote
			release, err := rdb.GetRelease(inst.Name, "latest")
			if err == nil && release.Version != inst.Version {
				updates[inst.Name] = release.Version
				// Update cache
				pkg, _ := rdb.GetPackageInfo(inst.Name)
				if pkg != nil {
					_ = ldb.UpdateCache(inst.Name, release.Version, pkg.Description,
						pkg.HomepageURL, 6*time.Hour)
				}
			}
		}
		fmt.Println()
	}

	// Filter outdated if requested
	if listOutdated {
		var outdated []string
		for _, inst := range installations {
			if _, hasUpdate := updates[inst.Name]; hasUpdate {
				outdated = append(outdated, inst.Name)
			}
		}

		if len(outdated) == 0 {
			fmt.Println("All packages are up to date!")
			return
		}

		fmt.Printf("%sPackages with updates available:%s\n\n", lib.Blue, lib.Reset)
	}

	// Display packages
	if listVerbose {
		displayVerboseList(installations, updates)
	} else {
		displayCompactList(installations, updates)
	}

	// Summary
	fmt.Println()
	count := len(installations)
	updateCount := len(updates)

	if count == 1 {
		fmt.Printf("%s%d package installed%s", lib.Yellow, count, lib.Reset)
	} else {
		fmt.Printf("%s%d packages installed%s", lib.Yellow, count, lib.Reset)
	}

	if updateCount > 0 {
		fmt.Printf(" | %s%d update(s) available%s", lib.Green, updateCount, lib.Reset)
		fmt.Println("\n\nTip: Use 'jpm list --outdated' to see packages with updates")
	}
	fmt.Println()
}

func displayCompactList(installations []model.Installation, updates map[string]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	fmt.Fprintln(w, "NAME\tVERSION\tINSTALLED\tSTATUS")
	fmt.Fprintln(w, "----\t-------\t---------\t------")

	for _, inst := range installations {
		installed := inst.InstalledAt.Format("2006-01-02")
		status := ""

		if newVer, hasUpdate := updates[inst.Name]; hasUpdate {
			status = fmt.Sprintf("%s→ %s%s", lib.Green, newVer, lib.Reset)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", inst.Name, inst.Version, installed, status)
	}

	w.Flush()
}

func displayVerboseList(installations []model.Installation, updates map[string]string) {
	for i, inst := range installations {
		if i > 0 {
			fmt.Println()
		}

		fmt.Printf("%s%s%s\n", lib.Blue, inst.Name, lib.Reset)
		fmt.Printf("  Version:     %s", inst.Version)

		if newVer, hasUpdate := updates[inst.Name]; hasUpdate {
			fmt.Printf(" %s→ %s available%s", lib.Green, newVer, lib.Reset)
		}
		fmt.Println()

		fmt.Printf("  Installed:   %s\n", inst.InstalledAt.Format("2006-01-02 15:04:05"))

		if inst.UpdatedAt.After(inst.InstalledAt) {
			fmt.Printf("  Updated:     %s\n", inst.UpdatedAt.Format("2006-01-02 15:04:05"))
		}

		if inst.Location != "" {
			fmt.Printf("  Location:    %s\n", inst.Location)
		}

		if inst.SysPath != "" {
			fmt.Printf("  In PATH:     %s\n", inst.SysPath)
		}

		if inst.FileSizeBytes > 0 {
			fmt.Printf("  Size:        %s\n", formatBytes(inst.FileSizeBytes))
		}

		// Show cached description if available
		ldb := db.NewLocalDB()
		cached, err := ldb.GetCachedMetadata(inst.Name)
		ldb.Close()

		if err == nil && cached != nil && cached.Description != "" {
			fmt.Printf("  Description: %s\n", cached.Description)
		}
	}
}

func showHistory(ldb db.LocalDB) {
	entries, err := ldb.GetHistory("", historyLimit)
	if err != nil {
		fmt.Printf("%sError: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	if len(entries) == 0 {
		fmt.Println("No installation history found")
		return
	}

	fmt.Printf("%sInstallation History (last %d entries):%s\n\n", lib.Blue, len(entries), lib.Reset)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "DATE\tACTION\tPACKAGE\tVERSION\tSTATUS")
	fmt.Fprintln(w, "----\t------\t-------\t-------\t------")

	for _, entry := range entries {
		date := entry.PerformedAt.Format("2006-01-02 15:04")
		status := "✓"
		statusColor := lib.Green

		if !entry.Success {
			status = "✗"
			statusColor = lib.Red
		}

		versionInfo := entry.Version
		if entry.PreviousVersion != "" {
			versionInfo = fmt.Sprintf("%s → %s", entry.PreviousVersion, entry.Version)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s%s%s\n",
			date, entry.Action, entry.PackageName, versionInfo,
			statusColor, status, lib.Reset)
	}

	w.Flush()

	if len(entries) == historyLimit {
		fmt.Printf("\n%sShowing latest %d entries. Adjust with --limit flag%s\n",
			lib.Yellow, historyLimit, lib.Reset)
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
