use reqwest::Client;
use serde_json::Value;
use crate::error::{Result, ValtError};
use crate::keychain;
use keyring::Entry as KeyringEntry;

/// Returns the best available auth token in priority order:
/// 1. VALTAG_AGENT_TOKEN env var
/// 2. Keychain agent token (OS keychain only, no env re-check)
/// 3. VALT_TOKEN env var (legacy user JWT)
/// 4. Keychain user token
pub fn get_auth_token() -> Option<String> {
    // 1. Agent token env var
    if let Ok(token) = std::env::var("VALTAG_AGENT_TOKEN") {
        if !token.is_empty() {
            return Some(token);
        }
    }
    // 2. Keychain agent token (OS keychain only)
    if let Ok(entry) = KeyringEntry::new("valtag-agent", "valtag-agent-token") {
        if let Ok(t) = entry.get_password() {
            if !t.is_empty() {
                return Some(t);
            }
        }
    }
    // 3. VALT_TOKEN env var (legacy user JWT)
    if let Ok(token) = std::env::var("VALT_TOKEN") {
        if !token.is_empty() {
            return Some(token);
        }
    }
    // 4. Keychain user token (also checks VALTAG_AUTH_TOKEN env via get_token)
    keychain::get_token()
}

pub struct ValtagClient {
    http: Client,
    base_url: String,
    token: String,
}

impl ValtagClient {
    pub fn new(base_url: String, token: String) -> Result<Self> {
        let http = Client::builder()
            .timeout(std::time::Duration::from_secs(30))
            .danger_accept_invalid_certs(false)
            .build()
            .map_err(ValtError::Http)?;
        Ok(Self { http, base_url, token })
    }

    async fn get(&self, path: &str) -> Result<Value> {
        let url = format!("{}/{}", self.base_url.trim_end_matches('/'), path.trim_start_matches('/'));
        let res = self.http.get(&url)
            .bearer_auth(&self.token)
            .send()
            .await?;
        let status = res.status().as_u16();
        let body: Value = res.json().await?;
        if status >= 400 {
            if status == 401 {
                eprintln!(
                    "[valtag-mcp-server] WARN unauthorized API call to {url}. token invalid/expired OR wrong token type for this endpoint"
                );
            }
            // Extract error message from API response
            // Format: {"error": {"code": "...", "message": "..."}}
            let msg = body.get("error")
                .and_then(|err| {
                    // Try to get message from error object
                    err.get("message")
                        .and_then(|v| v.as_str())
                        .or_else(|| {
                            // Fallback: try error as string directly
                            err.as_str()
                        })
                })
                .unwrap_or("unknown error")
                .to_string();
            return Err(ValtError::Api { status, message: msg });
        }
        Ok(body)
    }

    async fn post(&self, path: &str, body: Value) -> Result<Value> {
        let url = format!("{}/{}", self.base_url.trim_end_matches('/'), path.trim_start_matches('/'));
        let res = self.http.post(&url)
            .bearer_auth(&self.token)
            .json(&body)
            .send()
            .await?;
        let status = res.status().as_u16();
        let resp: Value = res.json().await.unwrap_or(Value::Null);
        if status >= 400 {
            if status == 401 {
                eprintln!(
                    "[valtag-mcp-server] WARN unauthorized API call to {url}. token invalid/expired OR wrong token type for this endpoint"
                );
            }
            // Extract error message from API response
            // Format: {"error": {"code": "...", "message": "..."}}
            let msg = resp.get("error")
                .and_then(|err| {
                    // Try to get message from error object
                    err.get("message")
                        .and_then(|v| v.as_str())
                        .or_else(|| {
                            // Fallback: try error as string directly
                            err.as_str()
                        })
                })
                .unwrap_or("unknown error")
                .to_string();
            return Err(ValtError::Api { status, message: msg });
        }
        Ok(resp)
    }

    pub async fn list_secrets(&self) -> Result<Value> {
        self.get("/secrets").await
    }

    pub async fn create_access_request(&self, secret_id: &str, reason: &str, duration_minutes: u32) -> Result<Value> {
        self.post(
            &format!("/secrets/{secret_id}/access-requests"),
            serde_json::json!({
                "requester_type": "ai_agent",
                "reason": reason,
                "duration_minutes": duration_minutes
            }),
        ).await
    }

    pub async fn get_access_requests(&self, status: Option<&str>) -> Result<Value> {
        let path = if let Some(s) = status {
            format!("/access-requests?status={s}")
        } else {
            "/access-requests".to_string()
        };
        self.get(&path).await
    }

    /// Fetch a single access request by ID (H4 fix: avoids full-list scan).
    pub async fn get_access_request(&self, request_id: &str) -> Result<Value> {
        self.get(&format!("/access-requests/{request_id}")).await
    }

    pub async fn get_credential(&self, request_id: &str) -> Result<Value> {
        self.get(&format!("/credentials/{request_id}")).await
    }

    pub async fn revoke_credential(&self, request_id: &str) -> Result<Value> {
        self.post(&format!("/credentials/{request_id}/revoke"), Value::Null).await
    }

    pub async fn get_audit_logs(&self, date: Option<&str>) -> Result<Value> {
        let path = if let Some(d) = date {
            format!("/audit/logs?date={d}")
        } else {
            "/audit/logs".to_string()
        };
        self.get(&path).await
    }

    pub async fn create_secret(&self, name: &str, credential_type: &str, value: &str, source: &str) -> Result<Value> {
        self.post("/secrets", serde_json::json!({
            "name": name,
            "credential_type": credential_type,
            "value": value,
            "source": source
        })).await
    }

    pub async fn create_dynamic_lease(&self, provider_id: &str, ttl_seconds: u32, agent_id: &str) -> Result<Value> {
        self.post(
            &format!("/providers/{provider_id}/leases"),
            serde_json::json!({
                "ttl_seconds": ttl_seconds,
                "agent_id": agent_id
            }),
        ).await
    }

    pub async fn create_scan_result(&self, project_id: &str, scan_path: &str, findings: &[crate::scanner::ScanFinding]) -> Result<Value> {
        let findings_json: Vec<serde_json::Value> = findings.iter().map(|f| serde_json::json!({
            "file_path": f.file_path,
            "line_number": f.line_number,
            "pattern_name": f.pattern_name,
            "credential_type": f.credential_type,
            "redacted_preview": f.redacted_preview
        })).collect();
        self.post(&format!("/projects/{project_id}/scans"), serde_json::json!({
            "scan_path": scan_path,
            "findings_count": findings.len(),
            "findings": findings_json
        })).await
    }
}
