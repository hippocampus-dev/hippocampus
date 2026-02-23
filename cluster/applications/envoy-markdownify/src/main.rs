use clap::Parser;
use opentelemetry_otlp::WithExportConfig;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt, Layer};

type Return = Result<(), Box<dyn std::error::Error + Send + Sync + 'static>>;

#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(long, default_value = "0.0.0.0:50051")]
    address: String,
}

fn header_value(header: &envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValue) -> &str {
    if header.raw_value.is_empty() {
        &header.value
    } else {
        std::str::from_utf8(&header.raw_value).unwrap_or("")
    }
}

fn accepts_markdown(accept_value: &str) -> bool {
    accept_value.split(',').any(|media_range| {
        let mut params = media_range.split(';');
        let media_type = params.next().unwrap_or("").trim();
        if media_type != "text/markdown" {
            return false;
        }
        for param in params {
            if let Some((key, value)) = param.split_once('=')
                && key.trim() == "q"
                && let Ok(q) = value.trim().parse::<f32>()
            {
                return q > 0.0;
            }
        }
        true
    })
}

struct MarkdownifyProcessor;

#[tonic::async_trait]
impl envoy_grpc_ext_proc::envoy::service::ext_proc::v3::external_processor_server::ExternalProcessor
    for MarkdownifyProcessor
{
    type ProcessStream = std::pin::Pin<
        Box<
            dyn tokio_stream::Stream<
                    Item = Result<
                        envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse,
                        tonic::Status,
                    >,
                > + Send
                + 'static,
        >,
    >;

    async fn process(
        &self,
        request: tonic::Request<
            tonic::Streaming<envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingRequest>,
        >,
    ) -> Result<tonic::Response<Self::ProcessStream>, tonic::Status> {
        let mut stream = request.into_inner();

        let output = async_stream::try_stream! {
            let mut should_convert = false;
            let mut response_body_buffer = Vec::new();

            while let Some(request) = tokio_stream::StreamExt::next(&mut stream).await {
                let request = request?;

                match request.request {
                    Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_request::Request::RequestHeaders(headers)) => {
                        if let Some(accept) = headers.headers.as_ref().and_then(|h| {
                            h.headers.iter().find(|header| header.key == "accept")
                        }) {
                            if accepts_markdown(header_value(accept)) {
                                should_convert = true;
                            }
                        }

                        yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                            response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::RequestHeaders(
                                envoy_grpc_ext_proc::envoy::service::ext_proc::v3::HeadersResponse::default(),
                            )),
                            ..Default::default()
                        };
                    }
                    Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_request::Request::ResponseHeaders(headers)) => {
                        if should_convert {
                            let is_html = headers.headers.as_ref()
                                .and_then(|h| h.headers.iter().find(|header| header.key == "content-type"))
                                .map(|h| header_value(h).contains("text/html"))
                                .unwrap_or(false);

                            let has_encoding = headers.headers.as_ref()
                                .and_then(|h| h.headers.iter().find(|header| header.key == "content-encoding"))
                                .is_some();

                            if !is_html || has_encoding {
                                should_convert = false;
                            }
                        }

                        yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                            response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::ResponseHeaders(
                                envoy_grpc_ext_proc::envoy::service::ext_proc::v3::HeadersResponse::default(),
                            )),
                            ..Default::default()
                        };
                    }
                    Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_request::Request::ResponseBody(body)) => {
                        if !should_convert {
                            yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                                response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::ResponseBody(
                                    envoy_grpc_ext_proc::envoy::service::ext_proc::v3::BodyResponse::default(),
                                )),
                                ..Default::default()
                            };
                            continue;
                        }

                        response_body_buffer.extend_from_slice(&body.body);

                        if !body.end_of_stream {
                            continue;
                        }

                        let html = match std::str::from_utf8(&response_body_buffer) {
                            Ok(html) => html,
                            Err(_) => {
                                yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                                    response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::ResponseBody(
                                        envoy_grpc_ext_proc::envoy::service::ext_proc::v3::BodyResponse::default(),
                                    )),
                                    ..Default::default()
                                };
                                continue;
                            }
                        };

                        match html_to_markdown_rs::convert(html, None) {
                            Ok(markdown) => {
                                yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                                    response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::ResponseBody(
                                        envoy_grpc_ext_proc::envoy::service::ext_proc::v3::BodyResponse {
                                            response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::CommonResponse {
                                                body_mutation: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::BodyMutation {
                                                    mutation: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::body_mutation::Mutation::Body(
                                                        markdown.into_bytes(),
                                                    )),
                                                }),
                                                header_mutation: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::HeaderMutation {
                                                    set_headers: vec![
                                                        envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValueOption {
                                                            header: Some(
                                                                envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValue {
                                                                    key: "content-type".to_string(),
                                                                    raw_value: b"text/markdown; charset=utf-8".to_vec(),
                                                                    ..Default::default()
                                                                },
                                                            ),
                                                            ..Default::default()
                                                        },
                                                        envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValueOption {
                                                            header: Some(
                                                                envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValue {
                                                                    key: "vary".to_string(),
                                                                    raw_value: b"Accept".to_vec(),
                                                                    ..Default::default()
                                                                },
                                                            ),
                                                            ..Default::default()
                                                        },
                                                    ],
                                                    remove_headers: vec![
                                                        "content-length".to_string(),
                                                    ],
                                                    ..Default::default()
                                                }),
                                                ..Default::default()
                                            }),
                                        },
                                    )),
                                    ..Default::default()
                                };
                            }
                            Err(e) => {
                                tracing::warn!("markdown conversion failed: {:?}, passing through", e);
                                yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                                    response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::ResponseBody(
                                        envoy_grpc_ext_proc::envoy::service::ext_proc::v3::BodyResponse::default(),
                                    )),
                                    ..Default::default()
                                };
                            }
                        }
                    }
                    Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_request::Request::RequestBody(_)) => {
                        yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                            response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::RequestBody(
                                envoy_grpc_ext_proc::envoy::service::ext_proc::v3::BodyResponse::default(),
                            )),
                            ..Default::default()
                        };
                    }
                    Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_request::Request::RequestTrailers(_)) => {
                        yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                            response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::RequestTrailers(
                                envoy_grpc_ext_proc::envoy::service::ext_proc::v3::TrailersResponse::default(),
                            )),
                            ..Default::default()
                        };
                    }
                    Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_request::Request::ResponseTrailers(_)) => {
                        yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                            response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::ResponseTrailers(
                                envoy_grpc_ext_proc::envoy::service::ext_proc::v3::TrailersResponse::default(),
                            )),
                            ..Default::default()
                        };
                    }
                    None => {}
                }
            }
        };

        Ok(tonic::Response::new(Box::pin(output)))
    }
}

async fn process(args: Args) -> Return {
    let address = args.address.parse::<std::net::SocketAddr>()?;

    tracing::info!("Starting markdownify server on {}", address);

    let (tx, mut rx) = tokio::sync::broadcast::channel(1);

    let mut sigterm = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())?;
    tokio::spawn(async move {
        tokio::select! {
            _ = sigterm.recv() => {
                tracing::info!("Received SIGTERM, shutting down");
                let _ = tx.send(());
            }
        }
    });

    let processor = MarkdownifyProcessor;

    tonic::transport::Server::builder()
        .add_service(
            envoy_grpc_ext_proc::envoy::service::ext_proc::v3::external_processor_server::ExternalProcessorServer::new(processor),
        )
        .serve_with_shutdown(address, async move {
            let _ = rx.recv().await;
        })
        .await?;

    Ok(())
}

fn main() -> Return {
    let args: Args = Args::parse();

    tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()?
        .block_on(async {
            opentelemetry::global::set_text_map_propagator(
                opentelemetry::sdk::propagation::TraceContextPropagator::new(),
            );

            if cfg!(feature = "tracing") {
                let tracer = opentelemetry_otlp::new_pipeline()
                    .tracing()
                    .with_exporter(opentelemetry_otlp::new_exporter().tonic().with_env())
                    .with_trace_config(opentelemetry::sdk::trace::config().with_resource(
                        opentelemetry::sdk::Resource::from_detectors(
                            std::time::Duration::from_secs(0),
                            vec![
                                Box::new(opentelemetry::sdk::resource::EnvResourceDetector::new()),
                                Box::new(opentelemetry::sdk::resource::SdkProvidedResourceDetector),
                            ],
                        ),
                    ))
                    .install_batch(opentelemetry::runtime::Tokio)?;
                let telemetry = tracing_opentelemetry::layer().with_tracer(tracer);

                let formatter = tracing_subscriber::fmt::format().with_source_location(true);
                #[cfg(not(debug_assertions))]
                let formatter = formatter
                    .json()
                    .with_current_span(false)
                    .with_span_list(false)
                    .flatten_event(true);
                let fmt = tracing_subscriber::fmt::layer()
                    .event_format(formatter)
                    .with_filter(tracing_subscriber::EnvFilter::from_default_env());

                tracing_subscriber::registry()
                    .with(fmt)
                    .with(telemetry)
                    .init();
            }

            process(args).await
        })?;

    if cfg!(feature = "tracing") {
        opentelemetry::global::shutdown_tracer_provider();
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_accepts_markdown() {
        assert!(accepts_markdown("text/markdown"));
        assert!(accepts_markdown("text/markdown;q=1"));
        assert!(accepts_markdown("text/markdown;q=0.5"));
        assert!(accepts_markdown("text/html, text/markdown"));
        assert!(accepts_markdown("text/html, text/markdown;q=0.9"));
        assert!(accepts_markdown("text/markdown; q = 1"));
        assert!(accepts_markdown("text/markdown; q = 0.5"));
    }

    #[test]
    fn test_rejects_markdown_with_q_zero() {
        assert!(!accepts_markdown("text/markdown;q=0"));
        assert!(!accepts_markdown("text/markdown;q=0.0"));
        assert!(!accepts_markdown("text/markdown;q=0.000"));
        assert!(!accepts_markdown("text/html, text/markdown;q=0"));
        assert!(!accepts_markdown("text/markdown; q = 0"));
        assert!(!accepts_markdown("text/markdown; q = 0.0"));
    }

    #[test]
    fn test_rejects_non_markdown() {
        assert!(!accepts_markdown("text/html"));
        assert!(!accepts_markdown("*/*"));
        assert!(!accepts_markdown("text/*"));
        assert!(!accepts_markdown("application/json"));
        assert!(!accepts_markdown("text/markdown-foo"));
    }
}
