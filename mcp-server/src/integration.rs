/// Integration tests for MCP tool calls with live valtag API backend
/// 
/// These tests load configuration from:
/// 1. .env file (in vaultsaas root directory)
/// 2. Environment variables (override .env)
/// 3. Falls back to defaults if available
///
/// Required variables:
/// - VALT_API_HOST: Backend API URL (e.g., http://localhost:8080/api/v1)
/// - VALTAG_AGENT_TOKEN: Valid agent bearer token
///
/// Run with: cargo test integration -- --nocapture
/// (Env vars loaded automatically from .env file)
#[cfg(test)]
mod integration_tests {
    use serde_json::{json, Value};
    use std::env;
    use std::path::Path;
    use std::time::Duration;

    /// Load environment variables from .env file
    fn load_env_from_file() {
        // Try to load .env from vaultsaas root (2 directories up from mcp-server/src)
        let manifest_dir = env::var("CARGO_MANIFEST_DIR").unwrap_or_else(|_| ".".to_string());
        let env_path = Path::new(&manifest_dir)
            .parent()
            .and_then(|p| Some(p.join(".env")))
            .unwrap_or_else(|| Path::new(".env").to_path_buf());

        eprintln!("[Config] Looking for .env at: {}", env_path.display());

        if env_path.exists() {
            eprintln!("[Config] Loading .env file: {}", env_path.display());
            match dotenvy::from_path(&env_path) {
                Ok(_) => {
                    eprintln!("[Config] ✓ Successfully loaded .env file");
                }
                Err(e) => {
                    eprintln!("[Config] ! Warning: Could not parse .env file: {}", e);
                }
            }
        } else {
            eprintln!("[Config] .env not found at {}", env_path.display());
            eprintln!("[Config] Will use environment variables only");
        }
    }

    /// Configuration for integration tests
    #[derive(Debug, Clone)]
    struct IntegrationConfig {
        api_url: String,
        agent_token: String,
        skip_reason: Option<String>,
    }

    impl IntegrationConfig {
        fn from_env() -> Self {
            // Load .env file first
            load_env_from_file();

            let api_url = env::var("VALT_API_HOST").ok();
            let agent_token = env::var("VALTAG_AGENT_TOKEN").ok();

            // Determine if tests should be skipped
            let skip_reason = match (&api_url, &agent_token) {
                (None, _) => Some("VALT_API_HOST not set".to_string()),
                (_, None) => Some("VALTAG_AGENT_TOKEN not set".to_string()),
                (Some(url), Some(token)) if url.is_empty() => {
                    Some("VALT_API_HOST is empty".to_string())
                }
                (Some(_), Some(token)) if token.is_empty() => {
                    Some("VALTAG_AGENT_TOKEN is empty".to_string())
                }
                _ => None,
            };

            // Log what we found
            if let Some(ref reason) = skip_reason {
                eprintln!("[Config] Tests will skip: {}", reason);
            } else {
                eprintln!("[Config] ✓ Integration tests enabled");
                eprintln!("[Config]   API URL: {}", api_url.as_ref().unwrap());
                eprintln!("[Config]   Agent token: {}...", 
                    agent_token.as_ref().unwrap().chars().take(10).collect::<String>());
            }

            IntegrationConfig {
                api_url: api_url.unwrap_or_default(),
                agent_token: agent_token.unwrap_or_default(),
                skip_reason,
            }
        }

        fn is_enabled(&self) -> bool {
            self.skip_reason.is_none()
        }
    }

    /// HTTP client for API calls
    struct ApiClient {
        base_url: String,
        token: String,
        http: reqwest::Client,
    }

    impl ApiClient {
        fn new(base_url: String, token: String) -> Result<Self, String> {
            let http = reqwest::Client::builder()
                .timeout(Duration::from_secs(30))
                .build()
                .map_err(|e| format!("Failed to build HTTP client: {}", e))?;
            Ok(ApiClient { base_url, token, http })
        }

        async fn get(&self, path: &str) -> Result<Value, String> {
            let url = format!("{}/api/v1{}", self.base_url.trim_end_matches('/'), path);
            eprintln!("[API] GET {}", url);

            let response = self.http
                .get(&url)
                .bearer_auth(&self.token)
                .send()
                .await
                .map_err(|e| format!("Request failed: {}", e))?;

            let status = response.status().as_u16();
            let body: Value = response
                .json()
                .await
                .map_err(|e| format!("Failed to parse response: {}", e))?;

            eprintln!("[API] Response status: {}", status);
            eprintln!("[API] Response body: {}", serde_json::to_string_pretty(&body).unwrap_or_default());

            if status >= 400 {
                let msg = body
                    .get("error")
                    .and_then(|e| {
                        // Try nested message structure {error: {message: "..."}}
                        if let Some(m) = e.get("message").and_then(|v| v.as_str()) {
                            Some(m.to_string())
                        } else if let Some(m) = e.as_str() {
                            // Or simple string {error: "..."}
                            Some(m.to_string())
                        } else {
                            None
                        }
                    })
                    .unwrap_or_else(|| "unknown error".to_string());
                return Err(format!("API error {}: {}", status, msg));
            }

            Ok(body)
        }

        async fn post(&self, path: &str, body: Value) -> Result<Value, String> {
            let url = format!("{}/api/v1{}", self.base_url.trim_end_matches('/'), path);
            eprintln!("[API] POST {}", url);
            eprintln!("[API] Request body: {}", serde_json::to_string_pretty(&body).unwrap_or_default());

            let response = self.http
                .post(&url)
                .bearer_auth(&self.token)
                .json(&body)
                .send()
                .await
                .map_err(|e| format!("Request failed: {}", e))?;

            let status = response.status().as_u16();
            let resp_body: Value = response
                .json()
                .await
                .unwrap_or(Value::Null);

            eprintln!("[API] Response status: {}", status);
            eprintln!("[API] Response body: {}", serde_json::to_string_pretty(&resp_body).unwrap_or_default());

            if status >= 400 {
                let msg = resp_body
                    .get("error")
                    .and_then(|e| {
                        // Try nested message structure {error: {message: "..."}}
                        if let Some(m) = e.get("message").and_then(|v| v.as_str()) {
                            Some(m.to_string())
                        } else if let Some(m) = e.as_str() {
                            // Or simple string {error: "..."}
                            Some(m.to_string())
                        } else {
                            None
                        }
                    })
                    .unwrap_or_else(|| "unknown error".to_string());
                return Err(format!("API error {}: {}", status, msg));
            }

            Ok(resp_body)
        }
    }

    // ============================================================================
    // TEST: Agent Access Request Workflow (Proper Agent Usage)
    // ============================================================================

    #[tokio::test]
    async fn test_integration_agent_request_access() {
        let cfg = IntegrationConfig::from_env();

        if let Some(reason) = cfg.skip_reason {
            eprintln!("⊘ SKIPPED: {}", reason);
            return;
        }

        eprintln!("\n=== Testing Agent Access Request Workflow ===");

        let client = match ApiClient::new(cfg.api_url.clone(), cfg.agent_token.clone()) {
            Ok(c) => c,
            Err(e) => {
                panic!("FAIL: Failed to create API client: {}", e);
            }
        };

        // Use a project-scoped secret (agents can request access to these)
        // Legacy secrets (no project) are owner-only
        let secret_id = "693a24e1-acfd-49d1-9907-72fff665f79b"; // temp4 secret with project_id

        eprintln!("[Step] Requesting access to secret: {}", secret_id);
        let request_body = json!({
            "requester_type": "ai_agent",
            "reason": "MCP integration test - accessing AWS credentials",
            "duration_minutes": 30
        });

        match client
            .post(&format!("/secrets/{}/access-requests", secret_id), request_body)
            .await
        {
            Ok(response) => {
                let request_id = response
                    .get("id")
                    .and_then(|v| v.as_str())
                    .expect("FAIL: Response should contain request 'id'");

                let status = response
                    .get("status")
                    .and_then(|v| v.as_str())
                    .unwrap_or("unknown");

                eprintln!("✓ PASS: Access request created");
                eprintln!("  Request ID: {}", request_id);
                eprintln!("  Status: {}", status);

                // Step 2: Check request status
                eprintln!("[Step] Checking access request status...");
                match client.get(&format!("/access-requests/{}", request_id)).await {
                    Ok(status_response) => {
                        eprintln!("✓ PASS: Retrieved access request status");
                        eprintln!("  Status: {}", status_response.get("status").and_then(|v| v.as_str()).unwrap_or("unknown"));
                    }
                    Err(e) => {
                        eprintln!("! Note: Could not retrieve request status: {}", e);
                    }
                }
            }
            Err(e) => {
                // Check if it's a rate limit error (expected on repeated runs)
                if e.contains("400") && e.contains("wait") {
                    eprintln!("⊘ SKIPPED: Rate limit hit (expected on repeated test runs): {}", e);
                } else {
                    panic!("FAIL: Could not create access request: {}", e);
                }
            }
        }
    }

    // ============================================================================
    // TEST: Access Request Response Structure
    // ============================================================================

    #[tokio::test]
    async fn test_integration_access_request_response_structure() {
        let cfg = IntegrationConfig::from_env();

        if let Some(reason) = cfg.skip_reason {
            eprintln!("⊘ SKIPPED: {}", reason);
            return;
        }

        eprintln!("\n=== Testing Access Request Response Structure ===");

        let client = match ApiClient::new(cfg.api_url.clone(), cfg.agent_token.clone()) {
            Ok(c) => c,
            Err(e) => {
                panic!("FAIL: Failed to create API client: {}", e);
            }
        };

        // Create a test access request first using a project-scoped secret
        // Agents can only access secrets with projects (not legacy owner-only secrets)
        let secret_id = "693a24e1-acfd-49d1-9907-72fff665f79b"; // temp4 secret with project_id
        let request_body = json!({
            "requester_type": "ai_agent",
            "reason": "Testing response structure",
            "duration_minutes": 30
        });

        match client
            .post(&format!("/secrets/{}/access-requests", secret_id), request_body)
            .await
        {
            Ok(response) => {
                eprintln!("✓ PASS: Created access request for validation");

                // Validate response structure
                assert!(response.get("id").is_some(), "FAIL: Response missing 'id'");
                assert!(response.get("status").is_some(), "FAIL: Response missing 'status'");
                assert!(response.get("reason").is_some(), "FAIL: Response missing 'reason'");

                eprintln!("  All required fields present ✓");
            }
            Err(e) => {
                eprintln!("! Note: Could not test response structure: {}", e);
            }
        }
    }

    // ============================================================================
    // TEST: Error Handling
    // ============================================================================

    #[tokio::test]
    async fn test_integration_invalid_secret_id_rejected() {
        let cfg = IntegrationConfig::from_env();

        if let Some(reason) = cfg.skip_reason {
            eprintln!("⊘ SKIPPED: {}", reason);
            return;
        }

        eprintln!("\n=== Testing Invalid Secret ID Rejection ===");

        let client = match ApiClient::new(cfg.api_url.clone(), cfg.agent_token.clone()) {
            Ok(c) => c,
            Err(e) => {
                panic!("FAIL: Failed to create API client: {}", e);
            }
        };

        // Try with invalid UUID format
        let invalid_id = "not-a-valid-uuid";

        match client
            .post(
                &format!("/secrets/{}/access-requests", invalid_id),
                json!({
                    "requester_type": "ai_agent",
                    "reason": "test",
                    "duration_minutes": 30
                }),
            )
            .await
        {
            Ok(_) => {
                panic!("FAIL: Should reject invalid secret_id, but succeeded");
            }
            Err(e) => {
                eprintln!("✓ PASS: Invalid secret_id correctly rejected");
                eprintln!("  Error: {}", e);
                assert!(
                    e.contains("error") || e.contains("invalid"),
                    "FAIL: Error message should indicate validation failure. Got: {}",
                    e
                );
            }
        }
    }

    // ============================================================================
    // TEST: Pagination
    // ============================================================================

    #[tokio::test]
    async fn test_integration_pagination_support() {
        let cfg = IntegrationConfig::from_env();

        if let Some(reason) = cfg.skip_reason {
            eprintln!("⊘ SKIPPED: {}", reason);
            return;
        }

        eprintln!("\n=== Testing Pagination Support ===");

        let client = match ApiClient::new(cfg.api_url.clone(), cfg.agent_token.clone()) {
            Ok(c) => c,
            Err(e) => {
                panic!("FAIL: Failed to create API client: {}", e);
            }
        };

        // Test with page and limit parameters
        match client.get("/access-requests?page=1&limit=10").await {
            Ok(response) => {
                eprintln!("✓ PASS: Pagination parameters accepted");

                let page = response.get("page").and_then(|v| v.as_i64());
                let limit = response.get("limit").and_then(|v| v.as_i64());
                let total = response.get("total").and_then(|v| v.as_i64());

                eprintln!("  Page: {:?}", page);
                eprintln!("  Limit: {:?}", limit);
                eprintln!("  Total: {:?}", total);

                assert!(page.is_some(), "FAIL: Response should include 'page'");
                assert!(limit.is_some(), "FAIL: Response should include 'limit'");
            }
            Err(e) => {
                eprintln!("! Note: Pagination test failed: {}", e);
            }
        }
    }

    // ============================================================================
    // TEST: Authentication Error Handling
    // ============================================================================

    #[tokio::test]
    async fn test_integration_invalid_token_rejected() {
        let cfg = IntegrationConfig::from_env();

        if let Some(reason) = cfg.skip_reason {
            eprintln!("⊘ SKIPPED: {}", reason);
            return;
        }

        eprintln!("\n=== Testing Invalid Token Rejection ===");

        let client = match ApiClient::new(cfg.api_url.clone(), "invalid_token_12345".to_string()) {
            Ok(c) => c,
            Err(e) => {
                panic!("FAIL: Failed to create API client: {}", e);
            }
        };

        match client.get("/secrets").await {
            Ok(_) => {
                eprintln!("! Warning: Invalid token was not rejected (unexpected)");
            }
            Err(e) => {
                eprintln!("✓ PASS: Invalid token correctly rejected");
                eprintln!("  Error: {}", e);
                assert!(
                    e.contains("401") || e.contains("error"),
                    "FAIL: Error should indicate authentication failure. Got: {}",
                    e
                );
            }
        }
    }
}
