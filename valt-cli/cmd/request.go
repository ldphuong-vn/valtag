package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ldphuong-vn/valtag/valt-cli/internal/config"
)

var requestCmd = &cobra.Command{
	Use:   "request <secret-name>",
	Short: "Create an access request for a secret",
	Args:  cobra.ExactArgs(1),
	RunE:  runRequest,
}

var (
	requestReason   string
	requestDuration string
)

func init() {
	requestCmd.Flags().StringVar(&requestReason, "reason", "", "Reason for access (required for Tier 2+)")
	requestCmd.Flags().StringVar(&requestDuration, "duration", "1h", "Access duration (e.g. 30m, 2h, 8h)")
	rootCmd.AddCommand(requestCmd)
}

func runRequest(cmd *cobra.Command, args []string) error {
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
			ID string `json:"id"`
		} `json:"secrets"`
	}
	if err := client.Get(context.Background(), path, &secretsResp); err != nil {
		return fmt.Errorf("searching for secret: %w", err)
	}
	if len(secretsResp.Secrets) == 0 {
		return fmt.Errorf("secret '%s' not found", name)
	}

	durationMin := parseDuration(requestDuration)

	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := client.Post(context.Background(),
		"/secrets/"+secretsResp.Secrets[0].ID+"/access-requests",
		map[string]interface{}{
			"reason":           requestReason,
			"duration_minutes": durationMin,
		},
		&req,
	); err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if jsonFlag {
		// AX: the create returns the poll handle immediately.
		return printJSON(struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Secret string `json:"secret"`
		}{req.ID, req.Status, name})
	}

	fmt.Printf("Request ID: %s\nStatus:     %s\n", req.ID, req.Status)
	switch req.Status {
	case "pending":
		fmt.Printf("\nCheck status:  valtag status %s\n", req.ID)
		fmt.Println("Approvers have been notified via email/Slack.")
	case "approved":
		fmt.Printf("\nAuto-approved. Get value:  valtag get %s\n", name)
	}
	return nil
}

// parseDuration converts "2h", "30m", "1h30m" to minutes.
func parseDuration(s string) int {
	total := 0
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else if c == 'h' {
			total += n * 60
			n = 0
		} else if c == 'm' {
			total += n
			n = 0
		}
	}
	if total == 0 {
		total = 60 // default 1h
	}
	return total
}
