package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ldphuong-vn/valtag/valt-cli/internal/config"
)

var getCmd = &cobra.Command{
	Use:   "get <secret-name>",
	Short: "Get a secret value (prints to stdout, safe to pipe)",
	Args:  cobra.ExactArgs(1),
	RunE:  runGet,
}

func init() { rootCmd.AddCommand(getCmd) }

func runGet(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg, _ := config.Load()
	client := MustClient()

	// Find secret by name
	path := "/secrets?name=" + name
	if cfg.ProjectID != "" {
		path += "&project_id=" + cfg.ProjectID
	}

	var secretsResp struct {
		Secrets []struct {
			ID             string `json:"id"`
			CredentialType string `json:"credential_type"`
		} `json:"secrets"`
	}
	if err := client.Get(context.Background(), path, &secretsResp); err != nil {
		return fmt.Errorf("searching secrets: %w", err)
	}
	if len(secretsResp.Secrets) == 0 {
		fmt.Fprintf(os.Stderr, "Secret '%s' not found\n", name)
		os.Exit(1)
	}
	secretID := secretsResp.Secrets[0].ID

	// Try to get active credential first
	var cred struct {
		Value  string `json:"value"`
		Status string `json:"status"`
	}
	err := client.Get(context.Background(), "/credentials/by-secret/"+secretID, &cred)
	if err == nil && cred.Value != "" {
		fmt.Print(cred.Value) // no newline — safe for $(valtag get NAME)
		return nil
	}

	// No active credential — create access request
	fmt.Fprintf(os.Stderr, "No active credential for '%s'. Requesting access...\n", name)
	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := client.Post(context.Background(),
		"/secrets/"+secretID+"/access-requests",
		map[string]interface{}{
			"reason":           "valtag CLI get",
			"duration_minutes": 60,
		},
		&req,
	); err != nil {
		return fmt.Errorf("creating access request: %w", err)
	}

	if req.Status == "approved" {
		// Auto-approved (Tier 1)
		var c struct {
			Value string `json:"value"`
		}
		client.Get(context.Background(), "/credentials/"+req.ID, &c) //nolint:errcheck
		fmt.Print(c.Value)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Access request created (ID: %s). Run: valtag status %s\n", req.ID, req.ID)
	fmt.Fprintf(os.Stderr, "Waiting for approval. Check your email/Slack for the approval link.\n")
	return nil
}
