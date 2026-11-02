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

var createSecretCmd = &cobra.Command{
	Use:   "create-secret",
	Short: "Create a new secret in your project",
	RunE:  runCreateSecret,
}

var (
	createSecretName        string
	createSecretType        string
	createSecretValue       string
	createSecretDescription string
	createSecretSource      string
)

func init() {
	createSecretCmd.Flags().StringVar(&createSecretName, "name", "", "Secret name (required)")
	createSecretCmd.Flags().StringVar(&createSecretType, "type", "", "Credential type: api_key, oauth_token, ssh_key, db_credential, cloud_credential, personal_session (required)")
	createSecretCmd.Flags().StringVar(&createSecretValue, "value", "", "Secret value (required)")
	createSecretCmd.Flags().StringVar(&createSecretDescription, "description", "", "Optional description")
	createSecretCmd.Flags().StringVar(&createSecretSource, "source", "", "Optional source identifier")
	rootCmd.AddCommand(createSecretCmd)
}

func runCreateSecret(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()
	client := MustClient()

	reader := bufio.NewReader(os.Stdin)

	// Interactive prompts when required flags are missing
	if createSecretName == "" {
		fmt.Print("Secret name: ")
		input, _ := reader.ReadString('\n')
		createSecretName = strings.TrimSpace(input)
	}
	if createSecretName == "" {
		return fmt.Errorf("--name is required")
	}

	if createSecretType == "" {
		fmt.Print("Credential type (api_key/oauth_token/ssh_key/db_credential/cloud_credential/personal_session): ")
		input, _ := reader.ReadString('\n')
		createSecretType = strings.TrimSpace(input)
	}
	if createSecretType == "" {
		return fmt.Errorf("--type is required")
	}

	if createSecretValue == "" {
		fmt.Print("Value: ")
		input, _ := reader.ReadString('\n')
		createSecretValue = strings.TrimSpace(input)
	}
	if createSecretValue == "" {
		return fmt.Errorf("--value is required")
	}

	body := map[string]interface{}{
		"name":            createSecretName,
		"credential_type": createSecretType,
		"value":           createSecretValue,
	}
	if createSecretDescription != "" {
		body["description"] = createSecretDescription
	}
	if createSecretSource != "" {
		body["source"] = createSecretSource
	}
	if cfg.ProjectID != "" {
		body["project_id"] = cfg.ProjectID
	}

	var resp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := client.Post(context.Background(), "/secrets", body, &resp); err != nil {
		return fmt.Errorf("creating secret: %w", err)
	}

	fmt.Printf("Created secret: %s (ID: %s)\n", resp.Name, resp.ID)
	return nil
}
