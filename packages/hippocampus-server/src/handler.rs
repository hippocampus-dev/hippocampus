use tracing_opentelemetry::OpenTelemetrySpanExt;

pub mod metrics;
pub use metrics::metrics;
pub mod debug;

#[tracing::instrument(skip_all)]
pub async fn root(
    axum::extract::State(crate::AppState {
        http_requests_total,
        ..
    }): axum::extract::State<crate::AppState>,
) -> impl axum::response::IntoResponse {
    http_requests_total.add(
        &tracing::Span::current().context(),
        1,
        &[opentelemetry::KeyValue::new("handler", "root")],
    );

    foo().await;

    (axum::http::StatusCode::OK, "Hello, World!")
}

#[tracing::instrument]
async fn foo() {
    tracing::info!("aaa");
    bar().await;
}

#[tracing::instrument]
async fn bar() {
    tracing::info!("bbb");

    let mut hm = std::collections::HashMap::new();
    opentelemetry::global::get_text_map_propagator(|propagator| {
        propagator.inject_context(&tracing::Span::current().context(), &mut hm)
    });
    dbg!(&hm);
}
