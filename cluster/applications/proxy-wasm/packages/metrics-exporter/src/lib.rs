use proxy_wasm::traits::Context;

use metrics::*;

#[cfg(not(test))]
#[no_mangle]
pub fn _start() {
    proxy_wasm::set_log_level(MyFrom::from(default_log_level().as_ref()));
    proxy_wasm::set_root_context(|_| -> Box<dyn proxy_wasm::traits::RootContext> {
        Box::new(MetricsExporterRoot::new())
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
    #[serde(default = "default_prefix")]
    prefix: String,
    #[serde(default = "default_field_separator")]
    field_separator: String,
    #[serde(default = "default_value_separator")]
    value_separator: String,
    vm_id: String,
    queue_name: String,
}

fn default_log_level() -> String {
    "info".to_string()
}

fn default_prefix() -> String {
    "".to_string()
}

fn default_field_separator() -> String {
    ";.;".to_string()
}

fn default_value_separator() -> String {
    "=.=".to_string()
}

struct MetricMap(std::collections::HashMap<Metric, u32>);

impl MetricMap {
    fn get_or_define<S>(
        &mut self,
        metric: &Metric,
        prefix: S,
        field_separator: S,
        value_separator: S,
    ) -> Option<u32>
    where
        S: AsRef<str>,
    {
        match self.0.get(metric) {
            Some(id) => Some(*id),
            None => {
                let metric_type = match metric.metric_type {
                    MetricType::Counter => proxy_wasm::types::MetricType::Counter,
                    MetricType::Gauge => proxy_wasm::types::MetricType::Gauge,
                    MetricType::Histogram => proxy_wasm::types::MetricType::Histogram,
                };
                let mut name: Vec<&str> = vec![prefix.as_ref(), &metric.name];
                for (k, v) in &metric.label {
                    name.push(field_separator.as_ref());
                    name.push(k);
                    name.push(value_separator.as_ref());
                    name.push(v);
                }
                match proxy_wasm::hostcalls::define_metric(metric_type, &name.join("")) {
                    Ok(id) => {
                        self.0.insert(metric.clone(), id);
                        Some(id)
                    }
                    Err(status) => {
                        log::error!("{:?}", status);
                        None
                    }
                }
            }
        }
    }
}

struct MetricsExporterRoot {
    configuration: Option<Configuration>,
    queue_id: Option<u32>,
    metric_map: MetricMap,
}

impl MetricsExporterRoot {
    fn new() -> Self {
        Self {
            configuration: None,
            queue_id: None,
            metric_map: MetricMap(std::collections::HashMap::new()),
        }
    }

    fn set_configuration(&mut self, configuration: Configuration) {
        self.configuration = Some(configuration);
    }

    fn set_queue_id(&mut self, queue_id: u32) {
        self.queue_id = Some(queue_id);
    }

    fn execute_task_loop(&mut self) {
        let configuration = match &self.configuration {
            Some(configuration) => configuration,
            None => return,
        };
        let queue_id = match self.queue_id {
            Some(queue_id) => queue_id,
            None => return,
        };

        if let Some(task) = self
            .dequeue_shared_queue(queue_id)
            .ok()
            .and_then(|task| task)
        {
            if let Ok(task) = serde_json::from_slice::<Task>(&task) {
                match task.operation {
                    Operation::IncrementMetric(value) => {
                        if let Some(metric_id) = self.metric_map.get_or_define(
                            &task.metric,
                            &configuration.prefix,
                            &configuration.field_separator,
                            &configuration.value_separator,
                        ) {
                            if let Err(status) =
                                proxy_wasm::hostcalls::increment_metric(metric_id, value)
                            {
                                log::error!("{:?}", status);
                            }
                        }
                    }
                    Operation::RecordMetric(value) => {
                        if let Some(metric_id) = self.metric_map.get_or_define(
                            &task.metric,
                            &configuration.prefix,
                            &configuration.field_separator,
                            &configuration.value_separator,
                        ) {
                            if let Err(status) =
                                proxy_wasm::hostcalls::record_metric(metric_id, value)
                            {
                                log::error!("{:?}", status);
                            }
                        }
                    }
                }
                self.execute_task_loop();
            }
        }
    }
}

impl proxy_wasm::traits::Context for MetricsExporterRoot {}

impl proxy_wasm::traits::RootContext for MetricsExporterRoot {
    fn on_configure(&mut self, _plugin_configuration_size: usize) -> bool {
        if let Some(configuration) = self.get_vm_configuration() {
            match serde_json::from_slice::<Configuration>(&configuration) {
                Ok(configuration) => {
                    proxy_wasm::set_log_level(MyFrom::from(configuration.log_level.as_ref()));

                    if let Some(queue_id) =
                        self.resolve_shared_queue(&configuration.vm_id, &configuration.queue_name)
                    {
                        self.set_queue_id(queue_id);
                    }

                    self.set_configuration(configuration);
                }
                Err(e) => log::error!("{:?}", e),
            }
        }

        self.set_tick_period(std::time::Duration::from_secs(10));

        true
    }

    fn on_tick(&mut self) {
        self.execute_task_loop();
    }

    fn create_http_context(
        &self,
        _context_id: u32,
    ) -> Option<Box<dyn proxy_wasm::traits::HttpContext>> {
        Some(Box::new(MetricsExporter))
    }

    fn get_type(&self) -> Option<proxy_wasm::types::ContextType> {
        Some(proxy_wasm::types::ContextType::HttpContext)
    }
}

struct MetricsExporter;

impl proxy_wasm::traits::Context for MetricsExporter {}

impl proxy_wasm::traits::HttpContext for MetricsExporter {}
