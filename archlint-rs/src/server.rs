use axum::{
    extract::Query,
    http::StatusCode,
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

use crate::{analyzer, costlint, perflint, promptlint, seclint};

// --- Request / Response types ---

#[derive(Deserialize)]
struct DirRequest {
    dir: String,
}

#[derive(Deserialize)]
struct ModelQuery {
    model: Option<String>,
}

#[derive(Serialize)]
struct HealthResponse {
    status: String,
}

#[derive(Serialize)]
struct CostResponse {
    model: String,
    tokens: usize,
    cost_usd: f64,
}

#[derive(Serialize)]
struct ErrorResponse {
    error: String,
}

// --- Handlers ---

async fn health() -> Json<HealthResponse> {
    Json(HealthResponse {
        status: "ok".to_string(),
    })
}

async fn scan_handler(
    Json(body): Json<DirRequest>,
) -> Result<Json<serde_json::Value>, (StatusCode, Json<ErrorResponse>)> {
    let path = PathBuf::from(&body.dir);
    match analyzer::analyze(&path) {
        Ok(graph) => {
            let val = serde_json::to_value(&graph).unwrap_or_default();
            Ok(Json(val))
        }
        Err(e) => Err((
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse { error: e }),
        )),
    }
}

async fn analyze_handler(body: String) -> Json<promptlint::PromptAnalysis> {
    Json(promptlint::analyze(&body))
}

async fn rate_handler(body: String) -> Json<seclint::ContentRating> {
    Json(seclint::classify(&body))
}

async fn cost_handler(
    Query(params): Query<ModelQuery>,
    body: String,
) -> Json<CostResponse> {
    let model = params.model.unwrap_or_else(|| "sonnet".to_string());
    let tokens = costlint::count_tokens(&body);
    let cost = costlint::estimate(&model, tokens, tokens);
    Json(CostResponse {
        model,
        tokens: tokens * 2,
        cost_usd: cost,
    })
}

async fn perf_handler(
    Json(body): Json<DirRequest>,
) -> Json<perflint::PerfReport> {
    let path = PathBuf::from(&body.dir);
    Json(perflint::analyze(&path))
}

// --- Server entry point ---

pub async fn run(port: u16) {
    let app = Router::new()
        .route("/health", get(health))
        .route("/scan", post(scan_handler))
        .route("/analyze", post(analyze_handler))
        .route("/rate", post(rate_handler))
        .route("/cost", post(cost_handler))
        .route("/perf", post(perf_handler));

    let addr = format!("0.0.0.0:{}", port);
    eprintln!("archlint server listening on {}", addr);

    let listener = tokio::net::TcpListener::bind(&addr)
        .await
        .expect("failed to bind");

    axum::serve(listener, app)
        .await
        .expect("server error");
}
