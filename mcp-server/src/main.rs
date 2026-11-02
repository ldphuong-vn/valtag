mod config;
mod client;
mod crypto;
mod dynsecret_tools;
mod error;
mod http;
mod keychain;
mod protocol;
mod resources;
mod scanner;
mod scanner_tools;
mod tools;
mod tools_e2e;
mod integration;

use clap::Parser;
use serde_json::{json, Value};
use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader};
use tracing::info;

use client::ValtagClient;
use protocol::{JsonRpcRequest, error_response, success_response};

#[derive(Parser)]
#[command(name = "valtag-mcp-server", version = "0.1.0")]
struct Args {
    /// Transport mode: stdio (default) or http
    #[arg(long, default_value = "stdio")]
    transport: String,

    /// Port for HTTP transport (default: 3737)
    #[arg(long, default_value = "3737")]
    port: u16,
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt()
        .with_writer(std::io::stderr)
        .with_env_filter("valt_mcp_server=info")
        .init();

    let args = Args::parse();

    let cfg = match config::load_config() {
        Ok(c) => c,
        Err(e) => {
            eprintln!("Failed to load config: {e}");
            std::process::exit(1);
        }
    };

    let token = client::get_auth_token().unwrap_or_default();
    let client = ValtagClient::new(cfg.api_url.clone(), token)
        .expect("Failed to build HTTP client");

    match args.transport.as_str() {
        "http" => {
            info!("valtag MCP Server v0.1.0 starting (http mode, port {})", args.port);
            if let Err(e) = http::run_http_server(args.port, client).await {
                eprintln!("HTTP server error: {e}");
                std::process::exit(1);
            }
        }
        _ => {
            info!("valtag MCP Server v0.1.0 starting (stdio mode)");
            run_stdio_loop(client).await;
        }
    }
}

async fn run_stdio_loop(client: ValtagClient) {
    let stdin = tokio::io::stdin();
    let stdout = tokio::io::stdout();
    let mut reader = BufReader::new(stdin);
    let mut writer = tokio::io::BufWriter::new(stdout);
    let mut line = String::new();

    loop {
        line.clear();
        match reader.read_line(&mut line).await {
            Ok(0) => break, // EOF
            Ok(_) => {}
            Err(e) => {
                eprintln!("Read error: {e}");
                break;
            }
        }

        let trimmed = line.trim();
        if trimmed.is_empty() {
            continue;
        }

        let response = handle_request(trimmed, &client).await;
        // Skip writing for notifications (null response)
        if response.is_null() {
            continue;
        }
        let mut out = serde_json::to_string(&response).unwrap_or_else(|_| {
            r#"{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"serialize error"}}"#.to_string()
        });
        out.push('\n');
        let _ = writer.write_all(out.as_bytes()).await;
        let _ = writer.flush().await;
    }
}

pub async fn handle_request(raw: &str, client: &ValtagClient) -> Value {
    let req: JsonRpcRequest = match serde_json::from_str(raw) {
        Ok(r) => r,
        Err(e) => {
            let resp = error_response(None, -32700, format!("Parse error: {e}"));
            return serde_json::to_value(resp).unwrap_or(Value::Null);
        }
    };

    let id = req.id.clone();
    let resp = match req.method.as_str() {
        "initialize" => success_response(id, json!({
            "protocolVersion": "2024-11-05",
            "capabilities": {
                "tools": { "listChanged": false },
                "resources": { "subscribe": false, "listChanged": false }
            },
            "serverInfo": { "name": "valtag-mcp-server", "version": "0.1.0" }
        })),
        "notifications/initialized" => {
            // No response for notifications
            return Value::Null;
        }
        "tools/list" => success_response(id, json!({
            "tools": tools::list_tools()
        })),
        "tools/call" => {
            let params = req.params.as_ref().unwrap_or(&Value::Null);
            let name = params.get("name").and_then(|v| v.as_str()).unwrap_or("");
            let args = params.get("arguments").unwrap_or(&Value::Null);
            match tools::call_tool(name, args, client).await {
                Ok(result) => success_response(req.id, json!({
                    "content": [{ "type": "text", "text": result.to_string() }]
                })),
                Err(e) => error_response(req.id, -32603, e.to_string()),
            }
        }
        "resources/list" => success_response(id, json!({
            "resources": resources::list_resources()
        })),
        "resources/read" => {
            let params = req.params.as_ref().unwrap_or(&Value::Null);
            let uri = params.get("uri").and_then(|v| v.as_str()).unwrap_or("");
            match resources::read_resource(uri, client).await {
                Ok(result) => success_response(req.id, result),
                Err(e) => error_response(req.id, -32603, e.to_string()),
            }
        }
        method => error_response(id, -32601, format!("Method not found: {method}")),
    };

    serde_json::to_value(resp).unwrap_or(Value::Null)
}
