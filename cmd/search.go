/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"jpm/db"
	"jpm/lib"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	allVersions bool
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search [package-name]",
	Short: "Search for packages in the remote repository",
	Long: `Search for available packages and view their versions.

Examples:
  jpm search                    # List all available packages
  jpm search nodejs             # Search for packages matching "nodejs"
  jpm search nodejs --all       # Show all versions of nodejs`,
	Run: search,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().BoolVarP(&allVersions, "all", "a", false, "Show all versions of matched packages")
}

func search(cmd *cobra.Command, args []string) {
	rdb := db.NewRemoteDB()
	defer rdb.Close()

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
	versions, err := rdb.GetAllVersions(packageName)
	if err != nil {
		fmt.Printf("%sPackage '%s' not found%s\n", lib.Red, packageName, lib.Reset)
		fmt.Println("\nTip: Use 'jpm search' to list all available packages")
		return
	}

	fmt.Printf("%sPackage: %s%s\n", lib.Blue, packageName, lib.Reset)
	fmt.Println(strings.Repeat("=", 50))

	if allVersions {
		fmt.Println("\nAvailable versions:")
		for _, v := range versions {
			if v.IsLatest {
				fmt.Printf("  %s• %s%s (latest)%s\n", lib.Green, v.Version, lib.Yellow, lib.Reset)
			} else {
				fmt.Printf("  • %s\n", v.Version)
			}
		}
		fmt.Printf("\n%sTotal versions: %d%s\n", lib.Yellow, len(versions), lib.Reset)
	} else {
		fmt.Printf("\n%sLatest version: %s%s\n", lib.Green, versions[0].Version, lib.Reset)
		if len(versions) > 1 {
			fmt.Printf("%s%d other version(s) available%s\n", lib.Yellow, len(versions)-1, lib.Reset)
			fmt.Println("\nUse --all flag to see all versions")
		}
	}

	fmt.Println("\nInstallation:")
	fmt.Printf("  jpm install %s           # Latest version\n", packageName)
	fmt.Printf("  jpm install %s@%s   # Specific version\n", packageName, versions[0].Version)
	if len(versions) > 0 {
		fmt.Printf("  jpm install %s@^%s  # Compatible with %s.x.x\n",
			packageName, versions[0].Version, getFirstPart(versions[0].Version))
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tLATEST VERSION")
	fmt.Fprintln(w, "----\t--------------")

	for _, pkg := range packages {
		fmt.Fprintf(w, "%s\t%s\n", pkg.Name, pkg.Version)
	}

	w.Flush()

	fmt.Printf("\n%sTotal packages: %d%s\n", lib.Yellow, len(packages), lib.Reset)
	fmt.Println("\nTip: Use 'jpm search <package-name>' for more details")
}

func getFirstPart(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return version
}
