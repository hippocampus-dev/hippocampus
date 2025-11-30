use prometheus::Encoder;

pub fn serve(
    address: std::net::SocketAddr,
    exporter: opentelemetry_prometheus::PrometheusExporter,
    mut signal: tokio::signal::unix::Signal,
) -> impl std::future::Future<Output = hyper::Result<()>> {
    let exporter = std::sync::Arc::new(exporter);
    let service_fn =
        hyper::service::make_service_fn(move |_socket: &hyper::server::conn::AddrStream| {
            let cloned_exporter = std::sync::Arc::clone(&exporter);
            async move {
                Ok::<_, std::convert::Infallible>(hyper::service::service_fn(
                    move |request: hyper::Request<hyper::Body>| {
                        let cloned_exporter = std::sync::Arc::clone(&cloned_exporter);
                        async move {
                            let mut response = hyper::Response::new(hyper::Body::empty());
                            match (request.method(), request.uri().path()) {
                                (&hyper::Method::GET, "/metrics") => {
                                    let metric_families = cloned_exporter.registry().gather();
                                    let encoder = prometheus::TextEncoder::new();
                                    let mut result = Vec::new();
                                    if let Ok(()) = encoder.encode(&metric_families, &mut result) {
                                        *response.body_mut() = hyper::Body::from(result);
                                    } else {
                                        *response.status_mut() =
                                            hyper::StatusCode::INTERNAL_SERVER_ERROR;
                                    }
                                }
                                _ => {
                                    *response.status_mut() = hyper::StatusCode::NOT_FOUND;
                                }
                            }
                            Ok::<_, std::convert::Infallible>(response)
                        }
                    },
                ))
            }
        });

    hyper::Server::bind(&address)
        .serve(service_fn)
        .with_graceful_shutdown(async move {
            signal.recv().await;
        })
}
