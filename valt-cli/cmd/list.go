package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ldphuong-vn/valtag/valt-cli/internal/config"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List accessible secrets in your project",
	RunE:  runList,
}

func init() { rootCmd.AddCommand(listCmd) }

func runList(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()
	client := MustClient()

	path := "/secrets"
	if cfg.ProjectID != "" {
		path += "?project_id=" + cfg.ProjectID
	}

	var resp struct {
		Secrets []struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			CredentialType string `json:"credential_type"`
			CreatedAt      string `json:"created_at"`
		} `json:"secrets"`
		Total int `json:"total"`
	}
	if err := client.Get(context.Background(), path, &resp); err != nil {
		return fmt.Errorf("fetching secrets: %w", err)
	}

	if len(resp.Secrets) == 0 {
		fmt.Println("No secrets found.")
		return nil
	}

	fmt.Printf("%-30s %-20s %s\n", "NAME", "TYPE", "CREATED")
	fmt.Printf("%-30s %-20s %s\n", "----", "----", "-------")
	for _, s := range resp.Secrets {
		created := ""
		if len(s.CreatedAt) >= 10 {
			created = s.CreatedAt[:10]
		}
		fmt.Printf("%-30s %-20s %s\n", s.Name, s.CredentialType, created)
	}
	fmt.Printf("\nTotal: %d\n", resp.Total)
	return nil
}
