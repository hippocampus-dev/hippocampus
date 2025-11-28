use proxy_wasm::traits::Context;
use proxy_wasm::traits::HttpContext;

use metrics::*;

#[cfg(not(test))]
#[unsafe(no_mangle)]
pub fn _start() {
    proxy_wasm::set_log_level(MyFrom::from(default_log_level().as_ref()));
    proxy_wasm::set_root_context(|_| -> Box<dyn proxy_wasm::traits::RootContext> {
        Box::new(FallbackFilterRoot::new())
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

mod serdex {
    pub mod std {
        pub mod time {
            const MINUTE_IN_SECONDS: u64 = 60;
            const HOUR_IN_SECONDS: u64 = 60 * MINUTE_IN_SECONDS;
            const DAY_IN_SECONDS: u64 = 24 * HOUR_IN_SECONDS;
            const WEEK_IN_SECONDS: u64 = 7 * DAY_IN_SECONDS;
            const YEAR_IN_SECONDS: u64 = 365 * DAY_IN_SECONDS;

            static UNITS: std::sync::LazyLock<std::collections::HashMap<&'static str, u64>> =
                std::sync::LazyLock::new(|| {
                    let mut m = std::collections::HashMap::new();
                    m.insert("ns", 1);
                    m.insert("us", 1_000);
                    m.insert("ms", 1_000_000);
                    m.insert("s", 1_000_000_000);
                    m.insert("m", MINUTE_IN_SECONDS * 1_000_000_000);
                    m.insert("h", HOUR_IN_SECONDS * 1_000_000_000);
                    m.insert("d", DAY_IN_SECONDS * 1_000_000_000);
                    m.insert("w", WEEK_IN_SECONDS * 1_000_000_000);
                    m.insert("y", YEAR_IN_SECONDS * 1_000_000_000);
                    m
                });

            /// https://github.com/serde-rs/serde/issues/339#issuecomment-539453327
            #[derive(Clone, Copy, Debug, Default)]
            pub struct Duration(std::time::Duration);

            impl From<&Duration> for std::time::Duration {
                fn from(value: &Duration) -> Self {
                    value.0
                }
            }

            impl serde::Serialize for Duration {
                fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
                where
                    S: serde::ser::Serializer,
                {
                    let mut total = self.0.as_nanos() as u64;
                    let mut buffer = String::new();

                    let mut units = UNITS.iter().collect::<Vec<_>>();
                    units.sort_by(|a, b| b.1.cmp(a.1));

                    for (unit, &multiplier) in units {
                        if unit == &"w" {
                            continue;
                        }
                        let value = total / multiplier;
                        if value > 0 {
                            total -= value * multiplier;
                            buffer.push_str(&value.to_string());
                            buffer.push_str(unit);
                        }
                    }

                    serializer.serialize_str(&buffer)
                }
            }

            impl<'de> serde::Deserialize<'de> for Duration {
                fn deserialize<D>(deserializer: D) -> Result<Duration, D::Error>
                where
                    D: serde::de::Deserializer<'de>,
                {
                    deserializer.deserialize_str(DurationVisitor)
                }
            }

            struct DurationVisitor;

            impl<'de> serde::de::Visitor<'de> for DurationVisitor {
                type Value = Duration;

                fn expecting(&self, formatter: &mut std::fmt::Formatter) -> std::fmt::Result {
                    formatter.write_str("a string representing a duration")
                }

                fn visit_str<E>(self, value: &str) -> Result<Duration, E>
                where
                    E: serde::de::Error,
                {
                    let mut total = 0;
                    let mut buffer = String::new();

                    let mut chars = value.chars().peekable();
                    while let Some(c) = chars.next() {
                        match c {
                            '0'..='9' => {
                                buffer.push(c);
                            }
                            'n' | 'u' | 'm' | 's' | 'h' | 'd' | 'w' | 'y' => {
                                let mut unit = c.to_string();
                                if let Some(&next) = chars.peek() {
                                    match next {
                                        's' => {
                                            // ns, us, ms
                                            unit.push(next);
                                            chars.next();
                                        }
                                        '0'..='9' => {}
                                        ' ' => {}
                                        _ => return Err(E::custom("invalid duration unit")),
                                    }
                                }

                                if let Ok(value) = buffer.parse::<u64>() {
                                    if let Some(&multiplier) = UNITS.get(&unit.as_str()) {
                                        total += value * multiplier;
                                    }
                                }
                                buffer.clear();
                            }
                            _ => continue,
                        }
                    }

                    if !buffer.is_empty() {
                        return Err(E::custom("invalid duration format"));
                    }

                    Ok(Duration(std::time::Duration::from_nanos(total)))
                }
            }

            #[cfg(test)]
            mod tests {
                use super::*;

                #[test]
                fn test_duration_serialize() {
                    let duration = Duration(std::time::Duration::from_secs(1));
                    let serialized = serde_json::to_string(&duration).unwrap();
                    assert_eq!(serialized, "\"1s\"");
                }

                #[test]
                fn test_duration_serialize_long() {
                    let duration = Duration(std::time::Duration::from_secs(1_000_000_000));
                    let serialized = serde_json::to_string(&duration).unwrap();
                    assert_eq!(serialized, "\"31y259d1h46m40s\"");
                }

                #[test]
                fn test_duration_deserialize() {
                    let deserialized: Duration = serde_json::from_str("\"1s\"").unwrap();
                    assert_eq!(deserialized.0.as_secs(), 1);
                }

                #[test]
                fn test_duration_deserialize_long() {
                    let deserialized: Duration =
                        serde_json::from_str("\"31y259d1h46m40s\"").unwrap();
                    assert_eq!(deserialized.0.as_secs(), 1_000_000_000);
                }

                #[test]
                fn test_invalid_duration_deserialize() {
                    let deserialized: Result<Duration, _> = serde_json::from_str("\"1\"");
                    assert!(deserialized.is_err());
                    let deserialized: Result<Duration, _> = serde_json::from_str("\"1mm\"");
                    assert!(deserialized.is_err());
                }
            }
        }
    }
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct Fallback {
    cluster: String,
    headers: Vec<(String, String)>,
    #[serde(default = "serdex::std::time::Duration::default")]
    timeout: serdex::std::time::Duration,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct Configuration {
    #[serde(default = "default_log_level")]
    log_level: String,
    #[serde(default)]
    fallback_status_codes: Vec<u32>,
    #[serde(default)]
    fallback_on: Vec<FallbackOn>,
    fallback: Fallback,
    queue_name: Option<String>,
    #[serde(default = "default_response_code_label")]
    response_code_label: String,
}

fn default_log_level() -> String {
    "info".to_string()
}

fn default_response_code_label() -> String {
    "response_code".to_string()
}

struct FallbackFilterRoot {
    configuration: Option<Configuration>,
    queue_id: Option<u32>,
}

impl FallbackFilterRoot {
    fn new() -> Self {
        Self {
            configuration: None,
            queue_id: None,
        }
    }

    fn set_configuration(&mut self, configuration: Configuration) {
        self.configuration = Some(configuration);
    }

    fn set_queue_id(&mut self, queue_id: u32) {
        self.queue_id = Some(queue_id);
    }
}

impl proxy_wasm::traits::Context for FallbackFilterRoot {}

impl proxy_wasm::traits::RootContext for FallbackFilterRoot {
    fn on_configure(&mut self, _plugin_configuration_size: usize) -> bool {
        if let Some(configuration) = self.get_vm_configuration() {
            match serde_json::from_slice::<Configuration>(&configuration) {
                Ok(configuration) => {
                    proxy_wasm::set_log_level(MyFrom::from(configuration.log_level.as_ref()));

                    if let Some(queue_name) = &configuration.queue_name {
                        match proxy_wasm::hostcalls::register_shared_queue(queue_name) {
                            Ok(queue_id) => {
                                self.set_queue_id(queue_id);
                            }
                            Err(status) => {
                                log::error!("{:?}", status);
                                return false;
                            }
                        }
                    }

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
        let fallback_status_codes = self.configuration.clone().map(|c| c.fallback_status_codes);
        let fallback_on = self.configuration.clone().map(|c| c.fallback_on);
        let fallback = self.configuration.clone().map(|c| c.fallback);
        let response_code_label = self.configuration.clone().map(|c| c.response_code_label);
        Some(Box::new(FallbackFilter::new(
            fallback_status_codes,
            fallback_on,
            fallback,
            self.queue_id,
            response_code_label,
        )))
    }

    fn get_type(&self) -> Option<proxy_wasm::types::ContextType> {
        Some(proxy_wasm::types::ContextType::HttpContext)
    }
}

#[derive(Clone, Debug, PartialEq, serde::Serialize, serde::Deserialize)]
enum FallbackOn {
    #[serde(rename = "5xx")]
    _5xx,
    #[serde(rename = "gateway-error")]
    GatewayError,
    #[serde(rename = "retryable-4xx")]
    Retryable4xx,
    #[serde(rename = "fallback-status-codes")]
    FallbackStatusCodes,
    #[serde(rename = "always")]
    Always,
}

struct FallbackFilter {
    fallback_status_codes: Option<Vec<u32>>,
    fallback_on: Option<Vec<FallbackOn>>,
    fallback: Option<Fallback>,
    queue_id: Option<u32>,
    response_code_label: Option<String>,
}

impl FallbackFilter {
    fn new(
        fallback_status_codes: Option<Vec<u32>>,
        fallback_on: Option<Vec<FallbackOn>>,
        fallback: Option<Fallback>,
        queue_id: Option<u32>,
        response_code_label: Option<String>,
    ) -> Self {
        Self {
            fallback_status_codes,
            fallback_on,
            fallback,
            queue_id,
            response_code_label,
        }
    }
}

impl proxy_wasm::traits::Context for FallbackFilter {
    fn on_http_call_response(&mut self, _: u32, _: usize, body_size: usize, _: usize) {
        if let Some(body) = self.get_http_call_response_body(0, body_size) {
            self.send_http_response(
                503,
                vec![("content-type", "text/html; charset=utf-8")],
                Some(&body),
            );
            return;
        }
        self.resume_http_request();
    }
}

impl proxy_wasm::traits::HttpContext for FallbackFilter {
    fn on_http_response_headers(&mut self, _: usize, _: bool) -> proxy_wasm::types::Action {
        let fallback = match &self.fallback {
            Some(fallback) => fallback,
            None => return proxy_wasm::types::Action::Continue,
        };

        let f = |value: &str| {
            if let Ok(value) = value.parse::<u32>() {
                if let Some(fallback_on) = &self.fallback_on {
                    if fallback_on.contains(&FallbackOn::_5xx) && value >= 500 && value < 600 {
                        return true;
                    }
                    if fallback_on.contains(&FallbackOn::GatewayError) && value == 502
                        || value == 503
                        || value == 504
                    {
                        return true;
                    }
                    if fallback_on.contains(&FallbackOn::Retryable4xx) && value == 409 {
                        return true;
                    }
                    if fallback_on.contains(&FallbackOn::FallbackStatusCodes) {
                        if let Some(fallback_status_codes) = &self.fallback_status_codes {
                            if fallback_status_codes.contains(&value) {
                                return true;
                            }
                        }
                    }
                    if fallback_on.contains(&FallbackOn::Always) {
                        return true;
                    }
                }
            }
            false
        };

        if let Some(value) = self.get_http_response_header(":status") {
            if f(value.as_ref())
                && self
                    .dispatch_http_call(
                        fallback.cluster.as_str(),
                        fallback
                            .headers
                            .iter()
                            .map(|header| (header.0.as_str(), header.1.as_str()))
                            .collect(),
                        None,
                        Vec::new(),
                        (&fallback.timeout).into(),
                    )
                    .is_ok()
            {
                if let Some(queue_id) = self.queue_id {
                    let mut label = std::collections::HashMap::new();
                    label.insert(
                        self.response_code_label
                            .clone()
                            .unwrap_or(default_response_code_label()),
                        value.clone(),
                    );
                    if let Ok(bytes) = serde_json::to_vec(&Task {
                        operation: Operation::IncrementMetric(1),
                        metric: Metric {
                            name: "fallback_responses_total".to_string(),
                            label,
                            metric_type: MetricType::Counter,
                        },
                    }) {
                        if let Err(status) = self.enqueue_shared_queue(queue_id, Some(&bytes)) {
                            log::error!("{:?}", status);
                        }
                    }
                }
                return proxy_wasm::types::Action::Pause;
            }
        }
        proxy_wasm::types::Action::Continue
    }
}
