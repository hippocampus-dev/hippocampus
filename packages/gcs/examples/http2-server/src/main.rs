#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let address = "127.0.0.1:8080".parse::<std::net::SocketAddr>()?;
    serve(&address).await?;

    Ok(())
}

fn serve(address: &std::net::SocketAddr) -> impl std::future::Future<Output = hyper::Result<()>> {
    let service_fn =
        hyper::service::make_service_fn(|_socket: &hyper::server::conn::AddrStream| async {
            Ok::<_, std::convert::Infallible>(hyper::service::service_fn(
                |request: hyper::Request<hyper::Body>| async move {
                    let mut response = hyper::Response::new(hyper::Body::empty());
                    match (request.method(), request.uri().path()) {
                        (&hyper::Method::GET, "/") => {
                            *response.body_mut() = hyper::Body::from("OK");
                        }
                        _ => {
                            *response.status_mut() = hyper::StatusCode::NOT_FOUND;
                        }
                    }
                    Ok::<_, std::convert::Infallible>(response)
                },
            ))
        });
    hyper::Server::bind(address).serve(service_fn)
}
