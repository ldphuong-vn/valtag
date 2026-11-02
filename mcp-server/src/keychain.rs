use crate::error::{Result, ValtError};

/// Unified keychain service name shared between CLI and MCP server.
const SERVICE: &str = "valtag";

/// Legacy MCP service name kept for backward-compat migration.
const LEGACY_SERVICE: &str = "valtag-mcp-server";

const USER: &str = "auth-token";

const AGENT_TOKEN_USER: &str = "valtag-agent";
const AGENT_TOKEN_KEY: &str = "valtag-agent-token";

/// get_token returns the stored auth token, checking env var first,
/// then the unified "valtag" keychain entry, then the legacy "valtag-mcp-server" entry.
pub fn get_token() -> Option<String> {
    // Check env var first (useful for CI/headless).
    if let Ok(token) = std::env::var("VALTAG_AUTH_TOKEN") {
        if !token.is_empty() {
            return Some(token);
        }
    }
    // Try unified service name (written by CLI after valtag setup).
    if let Some(token) = keyring::Entry::new(SERVICE, USER)
        .ok()
        .and_then(|e| e.get_password().ok())
        .filter(|t| !t.is_empty())
    {
        return Some(token);
    }
    // Fallback: try legacy MCP service name for backward compatibility.
    keyring::Entry::new(LEGACY_SERVICE, USER)
        .ok()
        .and_then(|e| e.get_password().ok())
        .filter(|t| !t.is_empty())
}

#[allow(dead_code)]
pub fn store_token(token: &str) -> Result<()> {
    let entry = keyring::Entry::new(SERVICE, USER)
        .map_err(|e| ValtError::Keychain(e.to_string()))?;
    entry.set_password(token)
        .map_err(|e| ValtError::Keychain(e.to_string()))
}

#[allow(dead_code)]
pub fn delete_token() -> Result<()> {
    // Delete from unified service name (ignore not-found).
    let _ = keyring::Entry::new(SERVICE, USER)
        .ok()
        .and_then(|e| e.delete_credential().ok());
    // Delete from legacy service name (ignore not-found).
    let _ = keyring::Entry::new(LEGACY_SERVICE, USER)
        .ok()
        .and_then(|e| e.delete_credential().ok());
    Ok(())
}

pub fn get_agent_token() -> Result<Option<String>> {
    // Check env var first (useful for CI/headless).
    if let Ok(token) = std::env::var("VALTAG_AGENT_TOKEN") {
        if !token.is_empty() {
            return Ok(Some(token));
        }
    }
    // Try OS keychain; not-found is not an error.
    let token = keyring::Entry::new(AGENT_TOKEN_USER, AGENT_TOKEN_KEY)
        .ok()
        .and_then(|e| e.get_password().ok())
        .filter(|t| !t.is_empty());
    Ok(token)
}

pub fn set_agent_token(token: &str) -> Result<()> {
    let entry = keyring::Entry::new(AGENT_TOKEN_USER, AGENT_TOKEN_KEY)
        .map_err(|e| ValtError::Keychain(e.to_string()))?;
    entry.set_password(token)
        .map_err(|e| ValtError::Keychain(e.to_string()))
}

#[allow(dead_code)]
pub fn delete_agent_token() -> Result<()> {
    let entry = keyring::Entry::new(AGENT_TOKEN_USER, AGENT_TOKEN_KEY)
        .map_err(|e| ValtError::Keychain(e.to_string()))?;
    entry.delete_credential()
        .map_err(|e| ValtError::Keychain(e.to_string()))
}
