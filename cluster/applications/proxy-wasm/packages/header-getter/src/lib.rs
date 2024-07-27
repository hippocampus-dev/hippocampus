use proxy_wasm::traits::Context;

#[cfg(not(test))]
#[no_mangle]
pub fn _start() {
    proxy_wasm::set_log_level(MyFrom::from(default_log_level().as_ref()));
    proxy_wasm::set_root_context(|_| -> Box<dyn proxy_wasm::traits::RootContext> {
        Box::new(HeaderGetterRoot::new())
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
struct Mapping {
    key: String,
    header_name: String,
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

struct HeaderGetterRoot {
    configuration: Option<Configuration>,
}

impl HeaderGetterRoot {
    fn new() -> Self {
        Self {
            configuration: None,
        }
    }

    fn set_configuration(&mut self, configuration: Configuration) {
        self.configuration = Some(configuration);
    }
}

impl proxy_wasm::traits::Context for HeaderGetterRoot {}

impl proxy_wasm::traits::RootContext for HeaderGetterRoot {
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
        Some(Box::new(HeaderGetter::new(mappings)))
    }

    fn get_type(&self) -> Option<proxy_wasm::types::ContextType> {
        Some(proxy_wasm::types::ContextType::HttpContext)
    }
}

struct HeaderGetter {
    mappings: Vec<Mapping>,
}

impl HeaderGetter {
    fn new(mappings: Vec<Mapping>) -> Self {
        Self { mappings }
    }
}

impl proxy_wasm::traits::Context for HeaderGetter {}

impl proxy_wasm::traits::HttpContext for HeaderGetter {
    fn on_http_request_headers(&mut self, _: usize, _: bool) -> proxy_wasm::types::Action {
        for mapping in &self.mappings {
            if let Some(value) = self.get_http_request_header(&mapping.header_name) {
                self.set_shared_data(&mapping.key, Some(value.as_bytes()), None)
                    .unwrap();
            }
        }
        proxy_wasm::types::Action::Continue
    }
}
