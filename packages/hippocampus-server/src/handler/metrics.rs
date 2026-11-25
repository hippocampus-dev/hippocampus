use prometheus::Encoder;

#[cfg_attr(feature = "tracing", tracing::instrument(skip(registry)))]
pub async fn metrics(
    axum::extract::State(crate::MonitorState { registry, .. }): axum::extract::State<
        crate::MonitorState,
    >,
) -> impl axum::response::IntoResponse {
    match handle(registry).await {
        Ok(result) => (
            axum::http::StatusCode::OK,
            axum::response::Response::new(axum::body::Body::from(result)),
        ),
        Err(e) => {
            opentelemetry_tracing::error!("{}", e);
            (
                axum::http::StatusCode::INTERNAL_SERVER_ERROR,
                axum::response::Response::new(axum::body::Body::from("internal server error")),
            )
        }
    }
}

async fn handle(registry: prometheus::Registry) -> Result<Vec<u8>, error::Error> {
    let metric_families = registry.gather();
    let encoder = prometheus::TextEncoder::new();
    let mut result = Vec::new();
    encoder.encode(&metric_families, &mut result)?;
    Ok(result)
}
