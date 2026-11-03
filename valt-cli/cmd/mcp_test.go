package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func valtagBlock() []byte {
	return []byte(`{"mcpServers":{"valtag":{"args":[],"command":"C:\\tools\\valtag-mcp-server.exe"}}}`)
}

func str(m map[string]interface{}, path ...string) string {
	var cur interface{} = m
	for _, p := range path {
		next, ok := cur.(map[string]interface{})
		if !ok {
			return ""
		}
		cur = next[p]
	}
	s, _ := cur.(string)
	return s
}

// TestMergeMCPConfig_PreservesOtherServers: a user with another MCP server
// configured must not lose it when valtag installs itself (found by the
// Windows dogfood run — the old code overwrote the whole mcpServers map).
func TestMergeMCPConfig_PreservesOtherServers(t *testing.T) {
	existing := []byte(`{
		"mcpServers": {"other-tool": {"command": "C:\\tools\\other.exe"}},
		"globalShortcut": "Ctrl+V"
	}`)
	merged, err := mergeMCPConfig(existing, valtagBlock())
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if got := str(merged, "mcpServers", "other-tool", "command"); got != `C:\tools\other.exe` {
		t.Fatalf("other-tool lost or altered: %q", got)
	}
	if got := str(merged, "mcpServers", "valtag", "command"); got != `C:\tools\valtag-mcp-server.exe` {
		t.Fatalf("valtag entry missing: %q", got)
	}
	if got := str(merged, "globalShortcut"); got != "Ctrl+V" {
		t.Fatalf("top-level key lost: %q", got)
	}
}

// TestMergeMCPConfig_FreshConfig: empty file yields a config with just the
// valtag entry.
func TestMergeMCPConfig_FreshConfig(t *testing.T) {
	merged, err := mergeMCPConfig(nil, valtagBlock())
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if got := str(merged, "mcpServers", "valtag", "command"); got == "" {
		t.Fatalf("valtag entry missing: %v", merged)
	}
	serversMap, hasMap := merged["mcpServers"].(map[string]interface{})
	if !hasMap {
		t.Fatalf("mcpServers missing or wrong type: %v", merged)
	}
	if _, has := serversMap["other-tool"]; has {
		t.Fatal("fresh config should not contain other servers")
	}
}

// TestMergeMCPConfig_MalformedExistingFailsClosed: an invalid JSON config
// must error out, never be silently overwritten (that would destroy the
// user's file).
func TestMergeMCPConfig_MalformedExistingFailsClosed(t *testing.T) {
	_, err := mergeMCPConfig([]byte("{not-json"), valtagBlock())
	if err == nil {
		t.Fatal("expected error for malformed existing config")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("error should explain the refusal: %v", err)
	}
}

// TestMergeMCPConfig_ReinstallUpdatesValtagEntry: re-running install keeps
// other servers and refreshes only the valtag entry.
func TestMergeMCPConfig_ReinstallUpdatesValtagEntry(t *testing.T) {
	first, err := mergeMCPConfig(nil, valtagBlock())
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}
	raw, _ := json.Marshal(first)

	newer := []byte(`{"mcpServers":{"valtag":{"args":["--port","3737"],"command":"C:\\new\\valtag-mcp-server.exe"}}}`)
	merged, err := mergeMCPConfig(raw, newer)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if got := str(merged, "mcpServers", "valtag", "command"); got != `C:\new\valtag-mcp-server.exe` {
		t.Fatalf("valtag entry not updated: %q", got)
	}
	servers, serversOk := merged["mcpServers"].(map[string]interface{})
	if !serversOk {
		t.Fatalf("mcpServers missing: %v", merged)
	}
	valtagEntry, valtagOk := servers["valtag"].(map[string]interface{})
	if !valtagOk {
		t.Fatalf("valtag entry missing: %v", merged)
	}
	if args, ok := valtagEntry["args"].([]interface{}); !ok || len(args) == 0 {
		t.Fatalf("updated args missing: %v", valtagEntry)
	}
}
