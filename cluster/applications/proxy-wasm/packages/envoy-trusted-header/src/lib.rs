#[cfg(not(test))]
#[unsafe(no_mangle)]
pub fn _start() {
    proxy_wasm::set_log_level(MyFrom::from(default_log_level().as_ref()));
    proxy_wasm::set_root_context(|_| -> Box<dyn proxy_wasm::traits::RootContext> {
        Box::new(EnvoyTrustedHeaderRoot::new())
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

/// https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/headers#x-forwarded-client-cert
#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct XForwardedClientCert {
    by: Option<String>,
    hash: Option<String>,
    cert: Option<String>,
    chain: Option<String>,
    subject: Option<String>,
    uri: Option<String>,
    dns: Option<String>,
}

impl From<&str> for XForwardedClientCert {
    fn from(value: &str) -> Self {
        let parts: Vec<&str> = value.split(';').collect();
        let mut x_forwarded_client_cert = XForwardedClientCert {
            by: None,
            hash: None,
            cert: None,
            chain: None,
            subject: None,
            uri: None,
            dns: None,
        };
        for part in parts {
            let mut pair = part.split('=');
            if let (Some(k), Some(v)) = (pair.next(), pair.next()) {
                match k {
                    "By" => x_forwarded_client_cert.by = Some(v.to_string()),
                    "Hash" => x_forwarded_client_cert.hash = Some(v.to_string()),
                    "Cert" => x_forwarded_client_cert.cert = Some(v.to_string()),
                    "Chain" => x_forwarded_client_cert.chain = Some(v.to_string()),
                    "Subject" => x_forwarded_client_cert.subject = Some(v.to_string()),
                    "URI" => x_forwarded_client_cert.uri = Some(v.to_string()),
                    "DNS" => x_forwarded_client_cert.dns = Some(v.to_string()),
                    _ => (),
                }
            }
        }
        x_forwarded_client_cert
    }
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct Mapping {
    header_name: String,
    trusted_certs: Vec<XForwardedClientCert>,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct Configuration {
    #[serde(default = "default_log_level")]
    log_level: String,
    mappings: Vec<Mapping>,
}

fn default_log_level() -> String {
    "info".to_string()
}

struct EnvoyTrustedHeaderRoot {
    configuration: Option<Configuration>,
}

impl EnvoyTrustedHeaderRoot {
    fn new() -> Self {
        Self {
            configuration: None,
        }
    }

    fn set_configuration(&mut self, configuration: Configuration) {
        self.configuration = Some(configuration);
    }
}

impl proxy_wasm::traits::Context for EnvoyTrustedHeaderRoot {}

impl proxy_wasm::traits::RootContext for EnvoyTrustedHeaderRoot {
    fn on_configure(&mut self, _plugin_configuration_size: usize) -> bool {
        if let Some(configuration) = self.get_vm_configuration() {
            match serde_json::from_slice::<Configuration>(&configuration) {
                Ok(configuration) => {
                    proxy_wasm::set_log_level(MyFrom::from(configuration.log_level.as_ref()));

                    self.set_configuration(configuration);
                }
                Err(e) => log::error!("{:?}", e),
            }
        }
        true
    }

    fn create_http_context(
        &self,
        _context_id: u32,
    ) -> Option<Box<dyn proxy_wasm::traits::HttpContext>> {
        let mappings = self
            .configuration
            .clone()
            .map(|c| c.mappings)
            .unwrap_or_default();
        Some(Box::new(EnvoyTrustedHeader::new(mappings)))
    }

    fn get_type(&self) -> Option<proxy_wasm::types::ContextType> {
        Some(proxy_wasm::types::ContextType::HttpContext)
    }
}

struct EnvoyTrustedHeader {
    mappings: Vec<Mapping>,
}

impl EnvoyTrustedHeader {
    fn new(mappings: Vec<Mapping>) -> Self {
        Self { mappings }
    }
}

impl proxy_wasm::traits::Context for EnvoyTrustedHeader {}

impl proxy_wasm::traits::HttpContext for EnvoyTrustedHeader {
    fn on_http_request_headers(&mut self, _: usize, _: bool) -> proxy_wasm::types::Action {
        if let Some(value) = self.get_http_request_header("x-forwarded-client-cert") {
            let x_forwarded_client_cert = XForwardedClientCert::from(value.as_ref());

            for mapping in &self.mappings {
                let trusted = mapping.trusted_certs.iter().any(|trusted_cert| {
                    trusted(&trusted_cert.by, &x_forwarded_client_cert.by)
                        && trusted(&trusted_cert.hash, &x_forwarded_client_cert.hash)
                        && trusted(&trusted_cert.cert, &x_forwarded_client_cert.cert)
                        && trusted(&trusted_cert.chain, &x_forwarded_client_cert.chain)
                        && trusted(&trusted_cert.subject, &x_forwarded_client_cert.subject)
                        && trusted(&trusted_cert.uri, &x_forwarded_client_cert.uri)
                        && trusted(&trusted_cert.dns, &x_forwarded_client_cert.dns)
                });
                if !trusted {
                    self.set_http_request_header(&mapping.header_name, None);
                }
            }
        }
        proxy_wasm::types::Action::Continue
    }
}

fn trusted(expected: &Option<String>, actual: &Option<String>) -> bool {
    match (expected, actual) {
        (Some(expected), Some(actual)) => expected == actual,
        (Some(_), None) => false,
        (None, Some(_)) => true,
        (None, None) => true,
    }
}
