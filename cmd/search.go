package cmd

import (
	"fmt"
	"jpm/db"
	"jpm/lib"
	"jpm/model"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	allVersions  bool
	searchByTag  string
	searchDetail bool
)

var searchCmd = &cobra.Command{
	Use:   "search [package-name]",
	Short: "Search for packages in the remote repository",
	Long: `Search for available packages and view their versions.

Examples:
  jpm search                       # List all available packages
  jpm search nodejs                # Search for packages matching "nodejs"
  jpm search nodejs --all          # Show all versions of nodejs
  jpm search nodejs --detail       # Show detailed information
  jpm search --tag database        # Search packages by tag

Flags:
  -a, --all                        # Show all versions
  -d, --detail                     # Show detailed information
  --tag string                     # Search by tag`,
	Run: search,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().BoolVarP(&allVersions, "all", "a", false, "Show all versions of matched packages")
	searchCmd.Flags().BoolVarP(&searchDetail, "detail", "d", false, "Show detailed package information")
	searchCmd.Flags().StringVar(&searchByTag, "tag", "", "Search packages by tag")
}

func search(cmd *cobra.Command, args []string) {
	rdb := db.NewRemoteDB()
	defer rdb.Close()

	// Search by tag
	if searchByTag != "" {
		searchPackagesByTag(rdb, searchByTag)
		return
	}

	// If specific package name provided
	if len(args) > 0 {
		packageName := args[0]
		searchSpecificPackage(rdb, packageName)
		return
	}

	// Otherwise list all packages
	listAllPackages(rdb)
}

func searchSpecificPackage(rdb db.RemoteDB, packageName string) {
	// Try to get package info
	pkg, err := rdb.GetPackageInfo(packageName)
	if err != nil {
		// Try fuzzy search
		fmt.Printf("%sPackage '%s' not found. Searching similar packages...%s\n\n",
			lib.Yellow, packageName, lib.Reset)
		packages, err := rdb.SearchPackages(packageName)
		if err != nil || len(packages) == 0 {
			fmt.Printf("%sNo packages found matching '%s'%s\n", lib.Red, packageName, lib.Reset)
			fmt.Println("\nTip: Use 'jpm search' to list all available packages")
			return
		}

		fmt.Printf("Did you mean one of these?\n\n")
		displayPackageSummaries(packages)
		return
	}

	// Display package details
	fmt.Printf("%s%s%s\n", lib.Blue, pkg.Name, lib.Reset)
	fmt.Println(strings.Repeat("=", len(pkg.Name)+10))
	fmt.Println()

	if pkg.Description != "" {
		fmt.Printf("%s\n\n", pkg.Description)
	}

	// Package metadata
	if searchDetail {
		if pkg.HomepageURL != "" {
			fmt.Printf("Homepage:   %s\n", pkg.HomepageURL)
		}
		if pkg.RepositoryURL != "" {
			fmt.Printf("Repository: %s\n", pkg.RepositoryURL)
		}
		if pkg.License != "" {
			fmt.Printf("License:    %s\n", pkg.License)
		}
		if pkg.Author != "" {
			fmt.Printf("Author:     %s\n", pkg.Author)
		}

		// Get tags
		tags, err := rdb.GetPackageTags(pkg.ID)
		if err == nil && len(tags) > 0 {
			fmt.Printf("Tags:       %s\n", strings.Join(tags, ", "))
		}
		fmt.Println()
	}

	// Get releases
	releases, err := rdb.GetAllReleases(pkg.ID)
	if err != nil {
		fmt.Printf("%sError fetching releases: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	if len(releases) == 0 {
		fmt.Println("No releases available")
		return
	}

	if allVersions {
		fmt.Println("Available versions:")
		for _, r := range releases {
			status := ""
			if r.IsPrerelease {
				status = fmt.Sprintf(" %s(pre-release)%s", lib.Yellow, lib.Reset)
			} else if r.IsDeprecated {
				status = fmt.Sprintf(" %s(deprecated)%s", lib.Red, lib.Reset)
			}

			releasedDate := r.ReleasedAt.Format("2006-01-02")
			fmt.Printf("  • %s%s - %s%s\n", lib.Green, r.Version, releasedDate, lib.Reset)

			if status != "" {
				fmt.Printf("    %s\n", status)
			}

			if searchDetail && r.ReleaseNotes != "" {
				notes := r.ReleaseNotes
				if len(notes) > 80 {
					notes = notes[:77] + "..."
				}
				fmt.Printf("    %s\n", notes)
			}
		}
		fmt.Printf("\n%sTotal versions: %d%s\n", lib.Yellow, len(releases), lib.Reset)
	} else {
		// Show only latest
		latest := releases[0]
		fmt.Printf("Latest version: %s%s%s", lib.Green, latest.Version, lib.Reset)
		if latest.IsPrerelease {
			fmt.Printf(" %s(pre-release)%s", lib.Yellow, lib.Reset)
		}
		fmt.Println()
		fmt.Printf("Released:       %s\n", latest.ReleasedAt.Format("2006-01-02"))

		if searchDetail && latest.ReleaseNotes != "" {
			fmt.Printf("\nRelease Notes:\n%s\n", latest.ReleaseNotes)
		}

		if len(releases) > 1 {
			stableCount := 0
			for _, r := range releases {
				if !r.IsPrerelease && !r.IsDeprecated {
					stableCount++
				}
			}
			fmt.Printf("\n%s%d other version(s) available (%d stable)%s\n",
				lib.Yellow, len(releases)-1, stableCount-1, lib.Reset)
			fmt.Println("\nUse --all flag to see all versions")
		}
	}

	// Get dependencies if detailed
	if searchDetail && len(releases) > 0 {
		deps, err := rdb.GetDependencies(releases[0].ID)
		if err == nil && len(deps) > 0 {
			fmt.Println("\nDependencies:")
			for _, dep := range deps {
				constraint := dep.VersionConstraint
				if constraint == "" {
					constraint = "any"
				}
				depType := ""
				if dep.DependencyType != "runtime" {
					depType = fmt.Sprintf(" (%s)", dep.DependencyType)
				}
				fmt.Printf("  • %s %s%s\n", dep.PackageName, constraint, depType)
			}
		}
	}

	// Installation instructions
	fmt.Println("\n" + strings.Repeat("-", 50))
	fmt.Println("Installation:")
	fmt.Printf("  jpm install %s              # Latest version\n", pkg.Name)
	if len(releases) > 0 {
		fmt.Printf("  jpm install %s@%s      # Specific version\n", pkg.Name, releases[0].Version)
		if !releases[0].IsPrerelease {
			major := strings.Split(releases[0].Version, ".")[0]
			fmt.Printf("  jpm install %s@^%s         # Compatible with %s.x.x\n",
				pkg.Name, releases[0].Version, major)
		}
	}

	// Check if already installed
	ldb := db.NewLocalDB()
	defer ldb.Close()

	installed, err := ldb.GetByName(pkg.Name)
	if err == nil && installed != nil {
		fmt.Println()
		fmt.Printf("%s✓ Already installed: v%s%s\n", lib.Green, installed.Version, lib.Reset)
		if len(releases) > 0 && releases[0].Version != installed.Version {
			fmt.Printf("  Update available: v%s → v%s\n", installed.Version, releases[0].Version)
			fmt.Printf("  Run: jpm install %s@latest\n", pkg.Name)
		}
	}
}

func listAllPackages(rdb db.RemoteDB) {
	packages, err := rdb.ListAllPackages()
	if err != nil {
		fmt.Printf("%sError fetching packages: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	if len(packages) == 0 {
		fmt.Println("No packages available in the repository")
		return
	}

	fmt.Printf("%sAvailable Packages%s\n", lib.Blue, lib.Reset)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	displayPackageSummaries(packages)

	fmt.Printf("\n%sTotal packages: %d%s\n", lib.Yellow, len(packages), lib.Reset)
	fmt.Println("\nTip: Use 'jpm search <package-name>' for more details")
	fmt.Println("     Use 'jpm search <package-name> --detail' for full information")
}

func displayPackageSummaries(packages []model.PackageSummary) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tLATEST VERSION\tDESCRIPTION")
	fmt.Fprintln(w, "----\t--------------\t-----------")

	for _, pkg := range packages {
		desc := pkg.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		if desc == "" {
			desc = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", pkg.Name, pkg.LatestVersion, desc)
	}

	w.Flush()
}

func searchPackagesByTag(rdb db.RemoteDB, tag string) {
	packages, err := rdb.GetPackagesByTag(tag)
	if err != nil {
		fmt.Printf("%sError searching by tag: %v%s\n", lib.Red, err, lib.Reset)
		return
	}

	if len(packages) == 0 {
		fmt.Printf("%sNo packages found with tag '%s'%s\n", lib.Yellow, tag, lib.Reset)
		return
	}

	fmt.Printf("%sPackages tagged with '%s':%s\n\n", lib.Blue, tag, lib.Reset)
	displayPackageSummaries(packages)
	fmt.Printf("\n%sFound %d package(s)%s\n", lib.Yellow, len(packages), lib.Reset)
}
