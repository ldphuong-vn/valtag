package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ldphuong-vn/valtag/valt-cli/internal/keychain"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored authentication token",
	RunE:  runLogout,
}

func init() { rootCmd.AddCommand(logoutCmd) }

func runLogout(cmd *cobra.Command, args []string) error {
	if err := keychain.DeleteToken(); err != nil {
		return fmt.Errorf("removing token: %w", err)
	}
	fmt.Println("Logged out successfully.")
	return nil
}
