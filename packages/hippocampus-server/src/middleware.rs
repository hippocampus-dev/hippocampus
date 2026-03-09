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

#[derive(Clone, Debug, Default)]
pub struct TracingLayer;

impl<S> tower::layer::Layer<S> for TracingLayer {
    type Service = TracingService<S>;

    fn layer(&self, service: S) -> Self::Service {
        TracingService { inner: service }
    }
}

#[derive(Clone, Debug)]
pub struct TracingService<S> {
    inner: S,
}

impl<S, Body> tower::Service<axum::http::Request<Body>> for TracingService<S>
where
    S: tower::Service<axum::http::Request<Body>>,
    S::Future: std::future::Future + Send + 'static,
{
    type Response = S::Response;
    type Error = S::Error;
    type Future = std::pin::Pin<
        Box<dyn std::future::Future<Output = Result<Self::Response, Self::Error>> + Send>,
    >;

    fn poll_ready(
        &mut self,
        cx: &mut std::task::Context<'_>,
    ) -> std::task::Poll<Result<(), Self::Error>> {
        self.inner.poll_ready(cx)
    }

    fn call(&mut self, request: axum::http::Request<Body>) -> Self::Future {
        let context = opentelemetry::global::get_text_map_propagator(|propagator| {
            propagator.extract(&HeaderMapWrapper(request.headers()))
        });
        let span = tracing::info_span!("handler");
        span.set_parent(context);

        let future = self.inner.call(request);

        Box::pin(async move { future.instrument(span).await })
    }
}
