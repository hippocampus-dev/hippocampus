use tracing_opentelemetry::OpenTelemetrySpanExt;

pub mod hello_world {
    tonic::include_proto!("helloworld");
}

#[derive(Clone, Debug)]
pub struct MyGreeter {
    http_requests_total: opentelemetry::metrics::Counter<u64>,
}

impl MyGreeter {
    pub fn new(http_requests_total: opentelemetry::metrics::Counter<u64>) -> Self {
        Self {
            http_requests_total,
        }
    }
}

#[tonic::async_trait]
impl hello_world::greeter_server::Greeter for MyGreeter {
    async fn say_hello(
        &self,
        request: tonic::Request<hello_world::HelloRequest>,
    ) -> Result<tonic::Response<hello_world::HelloResponse>, tonic::Status> {
        self.http_requests_total.add(
            &tracing::Span::current().context(),
            1,
            &[opentelemetry::KeyValue::new("handler", "root")],
        );

        println!("Got a request from {:?}", request.remote_addr());

        let reply = hello_world::HelloResponse {
            message: format!("Hello {}!", request.into_inner().name),
        };
        Ok(tonic::Response::new(reply))
    }
}
