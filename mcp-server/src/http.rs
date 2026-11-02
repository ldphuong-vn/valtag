use axum::{
    Router,
    routing::{get, post},
    extract::State,
    http::{HeaderMap, StatusCode},
    response::{IntoResponse, Response},
    response::sse::{Event, KeepAlive, Sse},
    Json,
};
use serde_json::Value;
use std::sync::Arc;
use std::time::Duration;
use futures_util::StreamExt;
use tokio_stream::wrappers::IntervalStream;

use crate::client::ValtClient;

pub struct AppState {
    pub client: Arc<ValtClient>,
}

fn verify_token(headers: &HeaderMap) -> bool {
    // Get expected token; if none configured, skip auth
    let expected = match crate::client::get_auth_token() {
        Some(t) if !t.is_empty() => t,
        _ => return true,
    };

    let auth = match headers.get("authorization").and_then(|v| v.to_str().ok()) {
        Some(v) => v,
        None => return false,
    };

    let provided = auth.strip_prefix("Bearer ").unwrap_or("").trim();

    // Constant-time comparison to prevent timing side-channel attacks
    let a = provided.as_bytes();
    let b = expected.as_bytes();
    if a.len() != b.len() {
        return false;
    }
    let diff: u8 = a.iter().zip(b.iter()).fold(0u8, |acc, (x, y)| acc | (x ^ y));
    diff == 0
}

/// POST /mcp — handle JSON-RPC request/response
pub async fn handle_mcp_post(
    State(state): State<Arc<AppState>>,
    headers: HeaderMap,
    Json(body): Json<Value>,
) -> Response {
    if !verify_token(&headers) {
        return (StatusCode::UNAUTHORIZED, "Unauthorized").into_response();
    }

    let raw = match serde_json::to_string(&body) {
        Ok(s) => s,
        Err(e) => {
            tracing::error!("Failed to serialize request body: {e}");
            return (StatusCode::BAD_REQUEST, "Bad request").into_response();
        }
    };

    // TODO(Phase 14): resolve $VALT_SECRET_<id> placeholders via regex before dispatch
    let response = crate::handle_request(&raw, &state.client).await;
    Json(response).into_response()
}

type SseStream = std::pin::Pin<
    Box<dyn futures_util::Stream<Item = Result<Event, std::convert::Infallible>> + Send>,
>;

/// GET /mcp/sse — Server-Sent Events stream
/// Sends a "connected" event on connect, then keep-alive pings every 30s.
/// Returns HTTP 401 (not 200) on auth failure.
pub async fn handle_mcp_sse(
    State(_state): State<Arc<AppState>>,
    headers: HeaderMap,
) -> Response {
    if !verify_token(&headers) {
        return (StatusCode::UNAUTHORIZED, "Unauthorized").into_response();
    }

    let interval = tokio::time::interval(Duration::from_secs(30));
    let stream: SseStream = Box::pin(
        IntervalStream::new(interval)
            .enumerate()
            .map(|(i, _)| {
                let event = if i == 0 {
                    Event::default().event("connected").data("valt-mcp-server ready")
                } else {
                    Event::default().event("ping").data("keep-alive")
                };
                Ok::<Event, std::convert::Infallible>(event)
            }),
    );

    Sse::new(stream).keep_alive(KeepAlive::default()).into_response()
}

pub async fn run_http_server(port: u16, client: ValtClient) -> anyhow::Result<()> {
    let state = Arc::new(AppState {
        client: Arc::new(client),
    });

    let app = Router::new()
        .route("/mcp", post(handle_mcp_post))
        .route("/mcp/sse", get(handle_mcp_sse))
        .with_state(state);

    // No token configured: verify_token lets everything through, so bind to
    // loopback only — never expose an unauthenticated MCP server to the
    // network (report §4.14). With a token set, remote binding is fine.
    let host = match crate::client::get_auth_token() {
        Some(t) if !t.is_empty() => "0.0.0.0",
        _ => {
            tracing::warn!(
                "No auth token configured (VALT_TOKEN): HTTP transport bound to 127.0.0.1 only. \
                 Set VALT_TOKEN to require Bearer auth and allow remote clients."
            );
            "127.0.0.1"
        }
    };
    let addr = format!("{host}:{port}");
    tracing::info!("Valt MCP HTTP server listening on {}", addr);

    let listener = tokio::net::TcpListener::bind(&addr).await?;
    axum::serve(listener, app).await?;
    Ok(())
}
