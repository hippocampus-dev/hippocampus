use tracing::Instrument;
use tracing_opentelemetry::OpenTelemetrySpanExt;

#[derive(Clone, Debug)]
struct HeaderMapWrapper<'a>(pub &'a axum::http::HeaderMap);

impl<'a> opentelemetry::propagation::Extractor for HeaderMapWrapper<'a> {
    fn get(&self, key: &str) -> Option<&str> {
        self.0.get(key).and_then(|value| value.to_str().ok())
    }

    fn keys(&self) -> Vec<&str> {
        self.0
            .keys()
            .map(|value| value.as_str())
            .collect::<Vec<_>>()
    }
}

pub async fn propagator<B>(
    mut request: axum::http::Request<B>,
    next: axum::middleware::Next<B>,
) -> Result<impl axum::response::IntoResponse, axum::response::Response> {
    let context = opentelemetry::global::get_text_map_propagator(|propagator| {
        propagator.extract(&HeaderMapWrapper(request.headers_mut()))
    });
    let span = tracing::info_span!("handler");
    span.set_parent(context.clone());

    Ok(next.run(request).instrument(span).await)
}
