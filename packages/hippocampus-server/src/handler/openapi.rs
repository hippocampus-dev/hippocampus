const OPENAPI_SPEC: &str = include_str!("../../openapi/api.swagger.json");

#[cfg_attr(feature = "tracing", tracing::instrument)]
pub async fn spec() -> impl axum::response::IntoResponse {
    (
        axum::http::StatusCode::OK,
        [(axum::http::header::CONTENT_TYPE, "application/json")],
        OPENAPI_SPEC,
    )
}
