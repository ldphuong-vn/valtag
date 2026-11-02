use chrono::Utc;
use serde_json::{json, Value};
use crate::client::ValtagClient;
use crate::error::{Result, ValtError};
use crate::protocol::ResourceDef;

pub fn list_resources() -> Vec<ResourceDef> {
    vec![
        ResourceDef {
            uri: "vault://secrets".into(),
            name: "My Secrets".into(),
            description: "List of all secrets you own".into(),
            mime_type: "application/json".into(),
        },
        ResourceDef {
            uri: "vault://requests/{id}".into(),
            name: "Access Request".into(),
            description: "Details of a specific access request including credential data if approved".into(),
            mime_type: "application/json".into(),
        },
        ResourceDef {
            uri: "vault://audit/today".into(),
            name: "Today's Audit Log".into(),
            description: "Audit log entries for the current day".into(),
            mime_type: "application/json".into(),
        },
    ]
}

pub async fn read_resource(uri: &str, client: &ValtagClient) -> Result<Value> {
    if uri == "vault://secrets" {
        let data = client.list_secrets().await?;
        return Ok(json!({ "contents": [{ "uri": uri, "mimeType": "application/json", "text": data.to_string() }] }));
    }
    if uri.starts_with("vault://requests/") {
        let request_id = uri.trim_start_matches("vault://requests/");
        if request_id.is_empty() {
            return Err(ValtError::Protocol("missing request ID in URI".into()));
        }
        let cred = client.get_credential(request_id).await?;
        return Ok(json!({ "contents": [{ "uri": uri, "mimeType": "application/json", "text": cred.to_string() }] }));
    }
    if uri == "vault://audit/today" {
        let today = Utc::now().format("%Y-%m-%d").to_string();
        let data = client.get_audit_logs(Some(&today)).await?;
        return Ok(json!({ "contents": [{ "uri": uri, "mimeType": "application/json", "text": data.to_string() }] }));
    }
    Err(ValtError::Protocol(format!("Unknown resource URI: {uri}")))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_list_resources_returns_three() {
        let resources = list_resources();
        assert_eq!(resources.len(), 3);
    }

    #[test]
    fn test_resource_uris() {
        let resources = list_resources();
        let uris: Vec<&str> = resources.iter().map(|r| r.uri.as_str()).collect();
        assert!(uris.contains(&"vault://secrets"));
        assert!(uris.contains(&"vault://audit/today"));
    }

    #[tokio::test]
    async fn test_unknown_uri_errors() {
        let config = crate::config::ValtConfig::default();
        if let Ok(client) = crate::client::ValtagClient::new(config.api_url, "tok".into()) {
            let err = read_resource("vault://unknown", &client).await;
            assert!(err.is_err());
        }
    }
}
