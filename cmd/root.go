package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/nicksherron/bashhub-server/internal"
	"github.com/spf13/cobra"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Flags().Parse(args)
		internal.Run()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize()
	rootCmd.PersistentFlags().StringVar(&internal.DbPath, "db", dbPath(), "DB location (sqlite or postgres)")

}
func dbPath() string {
	dbFile := "data.db"
	f := filepath.Join(appDir(), dbFile)
	return f
}

func appDir() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	ch := filepath.Join(cfgDir, ".bashhub-server")
	err = os.MkdirAll(ch, 0755)
	if err != nil {
		log.Fatal(err)
	}

	return ch
}
