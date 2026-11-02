package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <request-id>",
	Short: "Check approval status of an access request",
	Args:  cobra.ExactArgs(1),
	RunE:  runStatus,
}

func init() { rootCmd.AddCommand(statusCmd) }

func runStatus(cmd *cobra.Command, args []string) error {
	requestID := args[0]
	client := MustClient()

	var req struct {
		ID         string  `json:"id"`
		SecretName string  `json:"secret_name"`
		Status     string  `json:"status"`
		Reason     *string `json:"reason"`
		CreatedAt  string  `json:"created_at"`
		ExpiresAt  *string `json:"expires_at"`
	}
	if err := client.Get(context.Background(), "/access-requests/"+requestID, &req); err != nil {
		return fmt.Errorf("fetching request: %w", err)
	}

	fmt.Printf("Request:  %s\n", req.ID)
	fmt.Printf("Secret:   %s\n", req.SecretName)
	fmt.Printf("Status:   %s\n", req.Status)
	if req.Reason != nil {
		fmt.Printf("Reason:   %s\n", *req.Reason)
	}
	if len(req.CreatedAt) >= 19 {
		fmt.Printf("Created:  %s\n", req.CreatedAt[:19])
	}
	if req.ExpiresAt != nil && len(*req.ExpiresAt) >= 19 {
		fmt.Printf("Expires:  %s\n", (*req.ExpiresAt)[:19])
	}

	switch req.Status {
	case "approved":
		fmt.Printf("\n✓ Approved. Retrieve:  valtag get %s\n", req.SecretName)
	case "pending":
		fmt.Println("\n⏳ Pending approval. Approver notified via email/Slack.")
	case "rejected":
		fmt.Println("\n✗ Rejected. Create a new request with a more detailed reason.")
	case "expired":
		fmt.Println("\n⚠ Expired. Create a new request.")
	}
	return nil
}
