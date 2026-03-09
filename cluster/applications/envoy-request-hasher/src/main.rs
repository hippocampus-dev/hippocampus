use clap::Parser;
use opentelemetry_otlp::WithExportConfig;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt, Layer};

type Return = Result<(), Box<dyn std::error::Error + Send + Sync + 'static>>;

#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(long, default_value = "0.0.0.0:50051")]
    address: String,

    #[clap(long)]
    enable_body_hash: bool,

    #[clap(long)]
    enable_path_hash: bool,

    #[clap(long)]
    enable_url_hash: bool,
}

fn header_value(header: &envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValue) -> &str {
    if header.raw_value.is_empty() {
        &header.value
    } else {
        std::str::from_utf8(&header.raw_value).unwrap_or("")
    }
}

struct HashProcessor {
    enable_body_hash: bool,
    enable_path_hash: bool,
    enable_url_hash: bool,
}

#[tonic::async_trait]
impl envoy_grpc_ext_proc::envoy::service::ext_proc::v3::external_processor_server::ExternalProcessor
    for HashProcessor
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

        let enable_body_hash = self.enable_body_hash;
        let enable_path_hash = self.enable_path_hash;
        let enable_url_hash = self.enable_url_hash;

        let output = async_stream::try_stream! {
            let mut hasher = <sha2::Sha256 as sha2::Digest>::new();

            while let Some(request) = tokio_stream::StreamExt::next(&mut stream).await {
                let request = request?;

                match request.request {
                    Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_request::Request::RequestHeaders(headers)) => {
                        let mut set_headers = Vec::new();

                        if enable_path_hash || enable_url_hash {
                            if let Some(path_header) = headers.headers.as_ref().and_then(|h| {
                                h.headers.iter().find(|header| header.key == ":path")
                            }) {
                                let url = header_value(path_header);
                                let path = url.split('?').next().unwrap_or(url);

                                if enable_path_hash {
                                    let hash =
                                        hex::encode(<sha2::Sha256 as sha2::Digest>::digest(path));
                                    set_headers.push(envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValueOption {
                                        header: Some(
                                            envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValue {
                                                key: "x-path-hash".to_string(),
                                                raw_value: hash.into_bytes(),
                                                ..Default::default()
                                            },
                                        ),
                                        ..Default::default()
                                    });
                                }

                                if enable_url_hash {
                                    let hash =
                                        hex::encode(<sha2::Sha256 as sha2::Digest>::digest(url));
                                    set_headers.push(envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValueOption {
                                        header: Some(
                                            envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValue {
                                                key: "x-url-hash".to_string(),
                                                raw_value: hash.into_bytes(),
                                                ..Default::default()
                                            },
                                        ),
                                        ..Default::default()
                                    });
                                }
                            } else {
                                tracing::warn!(":path header not found");
                            }
                        }

                        let response = if set_headers.is_empty() {
                            envoy_grpc_ext_proc::envoy::service::ext_proc::v3::HeadersResponse::default()
                        } else {
                            envoy_grpc_ext_proc::envoy::service::ext_proc::v3::HeadersResponse {
                                response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::CommonResponse {
                                    header_mutation: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::HeaderMutation {
                                        set_headers,
                                        ..Default::default()
                                    }),
                                    ..Default::default()
                                }),
                            }
                        };

                        yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                            response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::RequestHeaders(response)),
                            ..Default::default()
                        };
                    }
                    Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_request::Request::RequestBody(body)) => {
                        if enable_body_hash {
                            sha2::Digest::update(&mut hasher, &body.body);
                        }

                        if body.end_of_stream {
                            let response = if enable_body_hash {
                                let hash = hex::encode(sha2::Digest::finalize_reset(&mut hasher));
                                envoy_grpc_ext_proc::envoy::service::ext_proc::v3::BodyResponse {
                                    response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::CommonResponse {
                                        header_mutation: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::HeaderMutation {
                                            set_headers: vec![envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValueOption {
                                                header: Some(
                                                    envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValue {
                                                        key: "x-body-hash".to_string(),
                                                        raw_value: hash.into_bytes(),
                                                        ..Default::default()
                                                    },
                                                ),
                                                ..Default::default()
                                            }],
                                            ..Default::default()
                                        }),
                                        ..Default::default()
                                    }),
                                }
                            } else {
                                envoy_grpc_ext_proc::envoy::service::ext_proc::v3::BodyResponse::default()
                            };

                            yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                                response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::RequestBody(response)),
                                ..Default::default()
                            };
                        } else {
                            yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                                response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::RequestBody(
                                    envoy_grpc_ext_proc::envoy::service::ext_proc::v3::BodyResponse::default(),
                                )),
                                ..Default::default()
                            };
                        }
                    }
                    Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_request::Request::ResponseHeaders(_)) => {
                        yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                            response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::ResponseHeaders(
                                envoy_grpc_ext_proc::envoy::service::ext_proc::v3::HeadersResponse::default(),
                            )),
                            ..Default::default()
                        };
                    }
                    Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_request::Request::ResponseBody(_)) => {
                        yield envoy_grpc_ext_proc::envoy::service::ext_proc::v3::ProcessingResponse {
                            response: Some(envoy_grpc_ext_proc::envoy::service::ext_proc::v3::processing_response::Response::ResponseBody(
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

    tracing::info!("Starting request-hasher server on {}", address);

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

    let processor = HashProcessor {
        enable_body_hash: args.enable_body_hash,
        enable_path_hash: args.enable_path_hash,
        enable_url_hash: args.enable_url_hash,
    };

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
