use prometheus::Encoder;

pub fn serve(
    address: std::net::SocketAddr,
    registry: prometheus::Registry,
    mut signal: tokio::signal::unix::Signal,
) -> impl std::future::Future<Output = hyper::Result<()>> {
    let registry = std::sync::Arc::new(registry);
    let service_fn =
        hyper::service::make_service_fn(move |_socket: &hyper::server::conn::AddrStream| {
            let cloned_registry = std::sync::Arc::clone(&registry);
            async move {
                Ok::<_, std::convert::Infallible>(hyper::service::service_fn(
                    move |request: hyper::Request<hyper::Body>| {
                        let cloned_registry = std::sync::Arc::clone(&cloned_registry);
                        async move {
                            let mut response = hyper::Response::new(hyper::Body::empty());
                            match (request.method(), request.uri().path()) {
                                (&hyper::Method::GET, "/metrics") => {
                                    let metric_families = cloned_registry.gather();
                                    let encoder = prometheus::TextEncoder::new();
                                    let mut result = Vec::new();
                                    if let Ok(()) = encoder.encode(&metric_families, &mut result) {
                                        *response.body_mut() = hyper::Body::from(result);
                                    } else {
                                        let status = hyper::StatusCode::INTERNAL_SERVER_ERROR;
                                        *response.status_mut() = status;
                                        *response.body_mut() = hyper::Body::from(
                                            status.canonical_reason().unwrap_or_default(),
                                        );
                                    }
                                }
                                _ => {
                                    let status = hyper::StatusCode::NOT_FOUND;
                                    *response.status_mut() = status;
                                    *response.body_mut() = hyper::Body::from(
                                        status.canonical_reason().unwrap_or_default(),
                                    );
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
