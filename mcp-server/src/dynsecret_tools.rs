use serde_json::{json, Value};
use crate::client::ValtagClient;
use crate::error::Result;

pub async fn tool_request_dynamic_secret(args: &Value, client: &ValtagClient) -> Result<Value> {
    let provider_id = args["provider_id"].as_str()
        .ok_or_else(|| crate::error::ValtError::Protocol("provider_id required".into()))?;
    let ttl = args.get("ttl_seconds").and_then(|v| v.as_u64()).unwrap_or(300) as u32;
    let agent_id = args.get("agent_id").and_then(|v| v.as_str()).unwrap_or("");
    let result = client.create_dynamic_lease(provider_id, ttl, agent_id).await?;
    Ok(json!({
        "lease_id": result.get("id"),
        "credentials": result.get("credentials"),
        "expires_at": result.get("expires_at"),
        "message": "Temporary credentials issued. They will auto-expire."
    }))
}
