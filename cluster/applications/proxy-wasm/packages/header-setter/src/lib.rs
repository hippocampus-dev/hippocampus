use proxy_wasm::traits::Context;

#[cfg(not(test))]
#[unsafe(no_mangle)]
pub fn _start() {
    proxy_wasm::set_log_level(MyFrom::from(default_log_level().as_ref()));
    proxy_wasm::set_root_context(|_| -> Box<dyn proxy_wasm::traits::RootContext> {
        Box::new(HeaderSetterRoot::new())
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
    r#override: bool,
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

struct HeaderSetterRoot {
    configuration: Option<Configuration>,
}

impl HeaderSetterRoot {
    fn new() -> Self {
        Self {
            configuration: None,
        }
    }

    fn set_configuration(&mut self, configuration: Configuration) {
        self.configuration = Some(configuration);
    }
}

impl proxy_wasm::traits::Context for HeaderSetterRoot {}

impl proxy_wasm::traits::RootContext for HeaderSetterRoot {
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
        context_id: u32,
    ) -> Option<Box<dyn proxy_wasm::traits::HttpContext>> {
        let mappings = self
            .configuration
            .clone()
            .map(|c| c.mappings)
            .unwrap_or_default();
        Some(Box::new(HeaderSetter::new(context_id, mappings)))
    }

    fn get_type(&self) -> Option<proxy_wasm::types::ContextType> {
        Some(proxy_wasm::types::ContextType::HttpContext)
    }
}

struct HeaderSetter {
    context_id: u32,
    mappings: Vec<Mapping>,
}

impl HeaderSetter {
    fn new(context_id: u32, mappings: Vec<Mapping>) -> Self {
        Self {
            context_id,
            mappings,
        }
    }
}

impl proxy_wasm::traits::Context for HeaderSetter {}

impl proxy_wasm::traits::HttpContext for HeaderSetter {
    fn on_http_request_headers(&mut self, _: usize, _: bool) -> proxy_wasm::types::Action {
        let key = self.context_id.to_string();
        if let (Some(bytes), _) = self.get_shared_data(&key) {
            if let Ok(data) = serde_json::from_slice::<serde_json::Value>(&bytes) {
                for mapping in &self.mappings {
                    if let Some(value) = data.get(&mapping.key).and_then(|v| v.as_str()) {
                        if mapping.r#override {
                            self.set_http_request_header(&mapping.header_name, Some(value));
                        } else {
                            let existing = self.get_http_request_header(&mapping.header_name);
                            if existing.is_none() {
                                self.set_http_request_header(&mapping.header_name, Some(value));
                            }
                        }
                    }
                }
            }
        }
        proxy_wasm::types::Action::Continue
    }
}
