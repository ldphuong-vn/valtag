package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ldphuong-vn/valtag/valt-cli/internal/api"
	"github.com/ldphuong-vn/valtag/valt-cli/internal/config"
	"github.com/ldphuong-vn/valtag/valt-cli/internal/keychain"
)

var rootCmd = &cobra.Command{
	Use:     "valtag",
	Short:   "valtag — secret vault CLI",
	Version: "0.1.0",
}

// jsonFlag (global --json): machine-readable output for scripts and agents.
// Commands that support it print a single JSON document on stdout and keep
// human chatter on stderr.
var jsonFlag bool

// printJSON marshals v as the command's whole stdout payload.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// MustClient returns a configured API client, exits if not authenticated.
func MustClient() *api.Client {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading config:", err)
		os.Exit(1)
	}
	token, err := keychain.GetToken()
	if err != nil || token == "" {
		fmt.Fprintln(os.Stderr, "Not logged in. Run: valtag setup")
		os.Exit(1)
	}
	return api.New(cfg.APIURL, token)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false,
		"machine-readable JSON output (for scripts and agents)")
}
