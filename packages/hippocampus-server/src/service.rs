use hippocampus_core::indexer::Indexer;
use hippocampus_core::searcher::Searcher;
use tracing_opentelemetry::OpenTelemetrySpanExt;

pub mod hello_world {
    tonic::include_proto!("helloworld");
}

pub mod hippocampus {
    tonic::include_proto!("hippocampus");
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
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self, request)))]
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

pub struct HippocampusService<DocumentStorage, TokenStorage, Tokenizer>
where
    DocumentStorage: hippocampus_core::storage::DocumentStorage + Clone + Send + Sync + 'static,
    TokenStorage: hippocampus_core::storage::TokenStorage + Clone + Send + Sync + 'static,
    Tokenizer: hippocampus_core::tokenizer::Tokenizer + Clone + Send + Sync + 'static,
{
    document_storage: DocumentStorage,
    token_storage: TokenStorage,
    tokenizer: Tokenizer,
    schema: hippocampus_core::Schema,
    http_requests_total: opentelemetry::metrics::Counter<u64>,
}

impl<DocumentStorage, TokenStorage, Tokenizer>
    HippocampusService<DocumentStorage, TokenStorage, Tokenizer>
where
    DocumentStorage: hippocampus_core::storage::DocumentStorage + Clone + Send + Sync + 'static,
    TokenStorage: hippocampus_core::storage::TokenStorage + Clone + Send + Sync + 'static,
    Tokenizer: hippocampus_core::tokenizer::Tokenizer + Clone + Send + Sync + 'static,
{
    pub fn new(
        document_storage: DocumentStorage,
        token_storage: TokenStorage,
        tokenizer: Tokenizer,
        schema: hippocampus_core::Schema,
        http_requests_total: opentelemetry::metrics::Counter<u64>,
    ) -> Self {
        Self {
            document_storage,
            token_storage,
            tokenizer,
            schema,
            http_requests_total,
        }
    }
}

#[tonic::async_trait]
impl<DocumentStorage, TokenStorage, Tokenizer> hippocampus::hippocampus_server::Hippocampus
    for HippocampusService<DocumentStorage, TokenStorage, Tokenizer>
where
    DocumentStorage: hippocampus_core::storage::DocumentStorage + Clone + Send + Sync + 'static,
    TokenStorage: hippocampus_core::storage::TokenStorage + Clone + Send + Sync + 'static,
    Tokenizer: hippocampus_core::tokenizer::Tokenizer + Clone + Send + Sync + 'static,
{
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self, request)))]
    async fn index(
        &self,
        request: tonic::Request<hippocampus::IndexRequest>,
    ) -> Result<tonic::Response<hippocampus::IndexResponse>, tonic::Status> {
        self.http_requests_total.add(
            &tracing::Span::current().context(),
            1,
            &[opentelemetry::KeyValue::new("handler", "index")],
        );

        let inner = request.into_inner();
        let mut field_values = std::collections::BTreeMap::new();
        for (key, value) in inner.fields {
            field_values.insert(key, hippocampus_core::Value::String(value));
        }
        let document = hippocampus_core::Document(field_values);

        let indexer = hippocampus_core::indexer::DocumentIndexer::new(
            self.document_storage.clone(),
            self.token_storage.clone(),
            self.tokenizer.clone(),
            self.schema.clone(),
        );
        indexer.index(vec![document]).await.map_err(|error| {
            opentelemetry_tracing::error!("{}", error);
            tonic::Status::internal("failed to index document")
        })?;

        Ok(tonic::Response::new(hippocampus::IndexResponse {
            document_id: 0,
        }))
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self, request)))]
    async fn search(
        &self,
        request: tonic::Request<hippocampus::SearchRequest>,
    ) -> Result<tonic::Response<hippocampus::SearchResponse>, tonic::Status> {
        self.http_requests_total.add(
            &tracing::Span::current().context(),
            1,
            &[opentelemetry::KeyValue::new("handler", "search")],
        );

        let inner = request.into_inner();
        let (_, query) = hippocampusql::parse(&inner.query)
            .map_err(|error| tonic::Status::invalid_argument(format!("{error:?}")))?;

        let indexed_count = self.document_storage.count().await.map_err(|error| {
            opentelemetry_tracing::error!("{}", error);
            tonic::Status::internal("failed to count documents")
        })?;

        let scorer = hippocampus_core::scorer::tf_idf::TfIdf::new(indexed_count);
        let searcher = hippocampus_core::searcher::DocumentSearcher::new(
            self.document_storage.clone(),
            self.token_storage.clone(),
            self.tokenizer.clone(),
            scorer,
            self.schema.clone(),
        );

        let mut search_option = hippocampus_core::searcher::SearchOption::default();
        if inner.limit > 0 {
            search_option.page_size = inner.limit as usize;
        }

        let results = searcher
            .search(&query, search_option)
            .await
            .map_err(|error| {
                opentelemetry_tracing::error!("{}", error);
                tonic::Status::internal("failed to execute search")
            })?;

        let search_results: Vec<hippocampus::SearchResult> = results
            .into_iter()
            .map(|result| {
                let document: std::collections::HashMap<String, String> = result
                    .document
                    .iter()
                    .map(|(key, value)| (key.clone(), value.to_string()))
                    .collect();
                hippocampus::SearchResult {
                    document,
                    score: result.score as f64,
                }
            })
            .collect();

        Ok(tonic::Response::new(hippocampus::SearchResponse {
            results: search_results,
        }))
    }
}
