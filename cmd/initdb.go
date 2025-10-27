package cmd

import (
	"fmt"
	"jpm/db"
	"jpm/lib"

	"github.com/spf13/cobra"
)

var initdbCmd = &cobra.Command{
	Use:   "initdb",
	Short: "Initialize the local database schema",
	Long: `Initialize the local database with all required tables and indexes.

This command creates:
  • installed packages table
  • installed files tracking
  • environment modifications tracking
  • installation history
  • dependencies tracking
  • configuration storage
  • metadata cache

The database file is created at: jpm.db

Note: This command is safe to run multiple times. Existing data will not be lost.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%sInitializing local database...%s\n\n", lib.Blue, lib.Reset)

		ldb := db.NewLocalDB()
		defer ldb.Close()

		// Initialize schema
		fmt.Println("Creating tables and indexes...")
		err := ldb.InitSchema()
		if err != nil {
			fmt.Printf("%s✗ Error: %v%s\n", lib.Red, err, lib.Reset)
			return
		}

		fmt.Printf("%s✓ Database schema initialized successfully%s\n", lib.Green, lib.Reset)
		fmt.Println("\nDatabase location: jpm.db")

		// Check if there are existing installations
		count := ldb.GetCount()
		if count > 0 {
			fmt.Printf("\nFound %d existing installation(s)\n", count)
		}

		fmt.Println("\nYou can now use jpm to install packages!")
		fmt.Println("\nNext steps:")
		fmt.Println("  • jpm search           - Browse available packages")
		fmt.Println("  • jpm install <pkg>    - Install a package")
		fmt.Println("  • jpm list             - List installed packages")
	},
}

func init() {
	rootCmd.AddCommand(initdbCmd)
}
