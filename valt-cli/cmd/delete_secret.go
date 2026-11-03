package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ldphuong-vn/valtag/valt-cli/internal/config"
)

var deleteSecretCmd = &cobra.Command{
	Use:   "delete-secret <name-or-id>",
	Short: "Delete a secret by name or ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeleteSecret,
}

var deleteSecretForce bool

func init() {
	deleteSecretCmd.Flags().BoolVar(&deleteSecretForce, "force", false, "Skip confirmation prompt")
	rootCmd.AddCommand(deleteSecretCmd)
}

func runDeleteSecret(cmd *cobra.Command, args []string) error {
	nameOrID := args[0]
	cfg, _ := config.Load()
	client := MustClient()

	// Resolve name to ID if needed (UUIDs contain hyphens with specific length)
	secretID := nameOrID
	if !looksLikeUUID(nameOrID) {
		path := "/secrets?name=" + nameOrID
		if cfg.ProjectID != "" {
			path += "&project_id=" + cfg.ProjectID
		}
		var resp struct {
			Secrets []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"secrets"`
		}
		if err := client.Get(context.Background(), path, &resp); err != nil {
			return fmt.Errorf("looking up secret: %w", err)
		}
		if len(resp.Secrets) == 0 {
			return fmt.Errorf("secret '%s' not found", nameOrID)
		}
		secretID = resp.Secrets[0].ID
		nameOrID = resp.Secrets[0].Name
	}

	if !deleteSecretForce {
		fmt.Printf("Delete secret '%s' (ID: %s)? [y/N] ", nameOrID, secretID)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := client.Delete(context.Background(), "/secrets/"+secretID); err != nil {
		return fmt.Errorf("deleting secret: %w", err)
	}

	fmt.Printf("Deleted secret: %s\n", nameOrID)
	return nil
}

// isHexDigit reports whether c is an ASCII hexadecimal digit.
func isHexDigit(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// looksLikeUUID returns true if s matches the UUID v4 pattern (8-4-4-4-12 hex chars).
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !isHexDigit(c) {
			return false
		}
	}
	return true
}
