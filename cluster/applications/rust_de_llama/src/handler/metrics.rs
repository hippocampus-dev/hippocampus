use prometheus::Encoder;

pub async fn metrics(
    axum::extract::State(crate::MonitorState { exporter, .. }): axum::extract::State<
        crate::MonitorState,
    >,
) -> impl axum::response::IntoResponse {
    match handle(exporter).await {
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

async fn handle(
    exporter: opentelemetry_prometheus::PrometheusExporter,
) -> Result<Vec<u8>, Box<dyn std::error::Error + Send + Sync>> {
    let metric_families = exporter.registry().gather();
    let encoder = prometheus::TextEncoder::new();
    let mut result = Vec::new();
    encoder.encode(&metric_families, &mut result)?;
    Ok(result)
}
