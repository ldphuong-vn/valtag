use serde_json::{json, Value};
use crate::client::ValtagClient;
use crate::error::Result;
use crate::scanner;
use crate::tools;

pub async fn tool_scan_secrets(args: &Value, client: &ValtagClient) -> Result<Value> {
    let path = args["path"].as_str()
        .ok_or_else(|| crate::error::ValtError::Protocol("path required".into()))?;
    // Validate path safety — reject absolute paths and traversal sequences
    if path.starts_with('/') || path.starts_with('\\') {
        return Err(crate::error::ValtError::Protocol(
            "path must be relative (absolute paths not allowed)".into()
        ));
    }
    // Reject Windows drive letters (e.g. C:)
    if path.len() >= 2 && path.as_bytes()[1] == b':' {
        return Err(crate::error::ValtError::Protocol(
            "path must be relative (drive paths not allowed)".into()
        ));
    }
    // Reject path traversal
    if path.contains("..") {
        return Err(crate::error::ValtError::Protocol(
            "path cannot contain '..'".into()
        ));
    }
    // Reject excessively long paths
    if path.len() > 500 {
        return Err(crate::error::ValtError::Protocol("path too long".into()));
    }
    let recursive = args.get("recursive").and_then(|v| v.as_bool()).unwrap_or(true);
    let findings = scanner::scan_directory(path, recursive);
    let count = findings.len();
    let findings_json: Vec<Value> = findings.iter().map(|f| json!({
        "file_path": f.file_path,
        "line_number": f.line_number,
        "pattern_name": f.pattern_name,
        "credential_type": f.credential_type,
        "redacted_preview": f.redacted_preview,
    })).collect();
    // If project_id provided, persist to backend
    if let Some(pid) = args.get("project_id").and_then(|v| v.as_str()) {
        let _ = client.create_scan_result(pid, path, &findings).await;
        // Ignore errors (scan result persistence is best-effort)
    }
    Ok(json!({
        "findings": findings_json,
        "count": count,
        "scanned_path": path
    }))
}

pub async fn tool_store_secret(args: &Value, client: &ValtagClient) -> Result<Value> {
    let name = args["name"].as_str()
        .ok_or_else(|| crate::error::ValtError::Protocol("name required".into()))?;
    let credential_type = args["credential_type"].as_str()
        .ok_or_else(|| crate::error::ValtError::Protocol("credential_type required".into()))?;
    let value = args["value"].as_str()
        .ok_or_else(|| crate::error::ValtError::Protocol("value required".into()))?;
    let source = args.get("source").and_then(|v| v.as_str()).unwrap_or("scanner");
    let result = client.create_secret(name, credential_type, value, source).await?;
    Ok(json!({
        "secret_id": result.get("id"),
        "name": name,
        "message": "Secret stored successfully"
    }))
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_list_tools_returns_nine() {
        let tool_list = tools::list_tools();
        assert_eq!(tool_list.len(), 9);
        let names: Vec<&str> = tool_list.iter().map(|t| t.name.as_str()).collect();
        assert!(names.contains(&"request_secret_access"));
        assert!(names.contains(&"list_my_secrets"));
        assert!(names.contains(&"authenticate_agent"));
        assert!(names.contains(&"scan_secrets"));
        assert!(names.contains(&"store_secret"));
        assert!(names.contains(&"request_dynamic_secret"));
    }

    #[test]
    fn test_tool_schemas_have_required_fields() {
        for tool in tools::list_tools() {
            assert!(!tool.name.is_empty());
            assert!(!tool.description.is_empty());
            assert_eq!(tool.input_schema.get("type").and_then(|v| v.as_str()), Some("object"));
        }
    }

    #[tokio::test]
    async fn test_call_unknown_tool_errors() {
        use crate::config::ValtConfig;
        let config = ValtConfig::default();
        let result = crate::client::ValtagClient::new(config.api_url, "test_token".into());
        if let Ok(client) = result {
            let err = tools::call_tool("nonexistent_tool", &json!({}), &client).await;
            assert!(err.is_err());
            let msg = err.unwrap_err().to_string();
            assert!(msg.contains("Unknown tool"));
        }
    }
}
