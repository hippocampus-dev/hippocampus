use proxy_wasm::traits::Context;

#[derive(Debug)]
enum Error {
    Base64DecodeError(base64::DecodeError),
    ProtobufDecodeError(prost::DecodeError),
}

impl std::error::Error for Error {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Error::Base64DecodeError(e) => Some(e),
            Error::ProtobufDecodeError(e) => Some(e),
        }
    }
}
impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::Base64DecodeError(e) => write!(f, "base64 decode error: {}", e),
            Error::ProtobufDecodeError(e) => write!(f, "protobuf decode error: {}", e),
        }
    }
}

#[cfg(not(test))]
#[unsafe(no_mangle)]
pub fn _start() {
    proxy_wasm::set_log_level(MyFrom::from(default_log_level().as_ref()));
    proxy_wasm::set_root_context(|_| -> Box<dyn proxy_wasm::traits::RootContext> {
        Box::new(EnvoyXAuthRequestRoot)
    });
}

trait MyFrom<T> {
    fn from(value: T) -> Self;
}

impl MyFrom<&str> for proxy_wasm::types::LogLevel {
    fn from(value: &str) -> Self {
        match value {
            "trace" => proxy_wasm::types::LogLevel::Trace,
            "debug" => proxy_wasm::types::LogLevel::Debug,
            "info" => proxy_wasm::types::LogLevel::Info,
            "warn" => proxy_wasm::types::LogLevel::Warn,
            "error" => proxy_wasm::types::LogLevel::Error,
            "critical" => proxy_wasm::types::LogLevel::Critical,
            _ => proxy_wasm::types::LogLevel::Info,
        }
    }
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct Configuration {
    #[serde(default = "default_log_level")]
    log_level: String,
}

fn default_log_level() -> String {
    "info".to_string()
}

#[derive(Clone, Debug)]
struct PeerMetadata {
    name: String,
    namespace: String,
}

impl std::fmt::Display for PeerMetadata {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}/{}", self.namespace, self.name)
    }
}

struct EnvoyXAuthRequestRoot;

impl proxy_wasm::traits::Context for EnvoyXAuthRequestRoot {}

impl proxy_wasm::traits::RootContext for EnvoyXAuthRequestRoot {
    fn on_configure(&mut self, _plugin_configuration_size: usize) -> bool {
        if let Some(configuration) = self.get_vm_configuration() {
            match serde_json::from_slice::<Configuration>(&configuration) {
                Ok(configuration) => {
                    proxy_wasm::set_log_level(MyFrom::from(configuration.log_level.as_ref()));
                }
                Err(e) => log::error!("{:?}", e),
            }
        }

        true
    }

    fn create_http_context(
        &self,
        context_id: u32,
    ) -> Option<Box<dyn proxy_wasm::traits::HttpContext>> {
        Some(Box::new(EnvoyXAuthRequest { context_id }))
    }

    fn get_type(&self) -> Option<proxy_wasm::types::ContextType> {
        Some(proxy_wasm::types::ContextType::HttpContext)
    }
}

struct EnvoyXAuthRequest {
    context_id: u32,
}

impl proxy_wasm::traits::Context for EnvoyXAuthRequest {}

impl proxy_wasm::traits::HttpContext for EnvoyXAuthRequest {
    fn on_http_request_headers(&mut self, _: usize, _: bool) -> proxy_wasm::types::Action {
        if let Some(value) = self.get_http_request_header("x-envoy-peer-metadata") {
            // https://github.com/istio/proxy/blob/1.21.2/source/extensions/filters/http/peer_metadata/filter.cc#L143
            if let Ok(peer_metadata) = base64::decode_config(value, base64::STANDARD)
                .map_err(Error::Base64DecodeError)
                .and_then(|decoded| {
                    <prost_types::Struct as prost::Message>::decode(decoded.as_slice())
                        .map_err(Error::ProtobufDecodeError)
                })
                .and_then(|metadata| {
                    log::trace!("peer metadata: {:?}", metadata.fields);

                    let name = metadata.fields.get("NAME").and_then(|v| v.string_value());
                    // let workload_name = metadata
                    //     .fields
                    //     .get("WORKLOAD_NAME")
                    //     .and_then(|v| v.string_value());
                    let namespace = metadata
                        .fields
                        .get("NAMESPACE")
                        .and_then(|v| v.string_value());

                    match (name, namespace) {
                        (Some(name), Some(namespace)) => Ok(PeerMetadata { name, namespace }),
                        _ => Err(Error::ProtobufDecodeError(prost::DecodeError::new(
                            "missing fields",
                        ))),
                    }
                })
            {
                let key = self.context_id.to_string();
                let mut data = if let (Some(bytes), _) = self.get_shared_data(&key) {
                    serde_json::from_slice(&bytes).unwrap_or_else(|_| serde_json::json!({}))
                } else {
                    serde_json::json!({})
                };

                data["x-auth-request-user"] = serde_json::Value::String(peer_metadata.to_string());

                self.set_shared_data(&key, Some(data.to_string().as_bytes()), None)
                    .unwrap();
            }
        }
        proxy_wasm::types::Action::Continue
    }
}

trait StringValue {
    fn string_value(&self) -> Option<String>;
}

impl StringValue for prost_types::Value {
    fn string_value(&self) -> Option<String> {
        match self.kind {
            Some(prost_types::value::Kind::NullValue(_)) => Some("".to_string()),
            Some(prost_types::value::Kind::NumberValue(n)) => Some(n.to_string()),
            Some(prost_types::value::Kind::StringValue(ref s)) => Some(s.clone()),
            Some(prost_types::value::Kind::BoolValue(b)) => Some(b.to_string()),
            Some(prost_types::value::Kind::StructValue(ref s)) => {
                let mut result = "{".to_string();
                for (k, v) in &s.fields {
                    result.push_str(&format!(
                        "{}:{},",
                        k,
                        v.string_value().unwrap_or("".to_string())
                    ));
                }
                result.push('}');
                Some(result)
            }
            Some(prost_types::value::Kind::ListValue(ref l)) => {
                let mut result = "[".to_string();
                for v in &l.values {
                    result.push_str(&format!("{},", v.string_value().unwrap_or("".to_string())));
                }
                result.push(']');
                Some(result)
            }
            None => None,
        }
    }
}
