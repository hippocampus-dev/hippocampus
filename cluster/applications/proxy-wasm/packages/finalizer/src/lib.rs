use proxy_wasm::traits::Context;

#[cfg(not(test))]
#[unsafe(no_mangle)]
pub fn _start() {
    proxy_wasm::set_log_level(MyFrom::from(default_log_level().as_ref()));
    proxy_wasm::set_root_context(|_| -> Box<dyn proxy_wasm::traits::RootContext> {
        Box::new(FinalizerRoot)
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

struct FinalizerRoot;

impl proxy_wasm::traits::Context for FinalizerRoot {}

impl proxy_wasm::traits::RootContext for FinalizerRoot {
    fn on_configure(&mut self, _plugin_configuration_size: usize) -> bool {
        if let Some(configuration) = self
            .get_plugin_configuration()
            .or_else(|| self.get_vm_configuration())
        {
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
        Some(Box::new(Finalizer { context_id }))
    }

    fn get_type(&self) -> Option<proxy_wasm::types::ContextType> {
        Some(proxy_wasm::types::ContextType::HttpContext)
    }
}

struct Finalizer {
    context_id: u32,
}

impl proxy_wasm::traits::Context for Finalizer {}

impl proxy_wasm::traits::HttpContext for Finalizer {
    fn on_http_request_headers(&mut self, _: usize, _: bool) -> proxy_wasm::types::Action {
        proxy_wasm::types::Action::Continue
    }

    fn on_log(&mut self) {
        let key = self.context_id.to_string();
        let _ = self.set_shared_data(&key, None, None);
    }
}
