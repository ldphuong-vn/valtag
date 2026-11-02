package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{Use: "mcp", Short: "MCP server management"}
var mcpInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Output or write MCP config for your IDE",
	RunE:  runMCPInstall,
}

var ideFlag string

func init() {
	mcpInstallCmd.Flags().StringVar(&ideFlag, "ide", "", "Target IDE: claude, cursor, vscode")
	mcpCmd.AddCommand(mcpInstallCmd)
	rootCmd.AddCommand(mcpCmd)
}

// mcpServerConfig is the JSON block to inject into IDE config.
func mcpServerConfig(binaryPath string) map[string]interface{} {
	return map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"valtag": map[string]interface{}{
				"command": binaryPath,
				"args":    []string{},
			},
		},
	}
}

func runMCPInstall(cmd *cobra.Command, args []string) error {
	// Find valtag-mcp-server binary path
	mcpBinary, err := findMCPBinary()
	if err != nil {
		fmt.Println("valtag-mcp-server binary not found in PATH.")
		fmt.Println("Download from: https://github.com/ldphuong-vn/valtag/releases")
		return err
	}

	cfg := mcpServerConfig(mcpBinary)
	block, _ := json.MarshalIndent(cfg, "", "  ")

	switch ideFlag {
	case "claude":
		return writeIDEConfig(claudeConfigPath(), block, "Claude Desktop")
	case "cursor":
		return writeIDEConfig(cursorConfigPath(), block, "Cursor")
	default:
		fmt.Println("Add this to your IDE's MCP config:")
		fmt.Println()
		fmt.Println(string(block))
	}
	return nil
}

func writeIDEConfig(path string, block []byte, ideName string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// Read existing config, merge mcpServers key
	existing := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &existing) //nolint:errcheck
	}
	var newCfg map[string]interface{}
	json.Unmarshal(block, &newCfg) //nolint:errcheck
	if servers, ok := newCfg["mcpServers"]; ok {
		existing["mcpServers"] = servers
	}
	out, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("writing %s config: %w", ideName, err)
	}
	fmt.Printf("✓ %s config updated: %s\n", ideName, path)
	fmt.Println("Restart your IDE to load the MCP server.")
	return nil
}

func findMCPBinary() (string, error) {
	binaryName := "valtag-mcp-server"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	// Check PATH for valtag-mcp-server
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		p := filepath.Join(dir, binaryName)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("valtag-mcp-server not in PATH")
}

func claudeConfigPath() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "Claude", "claude_desktop_config.json")
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	}
	return filepath.Join(home, ".config", "claude", "claude_desktop_config.json")
}

func cursorConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cursor", "mcp.json")
}
