package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ldphuong-vn/valtag/valt-cli/internal/config"
)

var runCmd = &cobra.Command{
	Use:                "run -- <command> [args...]",
	Short:              "Run a command with all accessible secrets injected as env vars",
	Args:               cobra.MinimumNArgs(1),
	RunE:               runRun,
	DisableFlagParsing: true, // pass all flags through to child command
}

func init() { rootCmd.AddCommand(runCmd) }

func runRun(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("command required: valtag run -- <command>")
	}
	// Strip leading "--" separator if present
	if args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return fmt.Errorf("command required after --")
	}

	cfg, _ := config.Load()
	client := MustClient()

	// Fetch all accessible secrets with active credentials
	path := "/credentials/active"
	if cfg.ProjectID != "" {
		path += "?project_id=" + cfg.ProjectID
	}

	var credsResp struct {
		Credentials []struct {
			SecretName string `json:"secret_name"`
			Value      string `json:"value"`
		} `json:"credentials"`
	}
	if err := client.Get(context.Background(), path, &credsResp); err != nil {
		return fmt.Errorf("fetching credentials: %w", err)
	}

	// Build env: start with current env, overlay secrets
	env := os.Environ()
	injected := 0
	for _, c := range credsResp.Credentials {
		if c.Value == "" {
			continue
		}
		envKey := strings.ToUpper(strings.ReplaceAll(c.SecretName, "-", "_"))
		env = append(env, envKey+"="+c.Value)
		injected++
	}

	fmt.Fprintf(os.Stderr, "valtag: injecting %d secret(s)\n", injected)

	child := exec.Command(args[0], args[1:]...)
	child.Env = env
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	if err := child.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}
