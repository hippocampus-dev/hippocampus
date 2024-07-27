use proxy_wasm::types::Action;
use serde::Serializer;

#[cfg(not(test))]
#[no_mangle]
pub fn _start() {
    proxy_wasm::set_log_level(MyFrom::from(default_log_level().as_ref()));
    proxy_wasm::set_root_context(|_| -> Box<dyn proxy_wasm::traits::RootContext> {
        Box::new(CookieManipulatorRoot::new())
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

#[derive(Clone, Debug)]
pub struct SerializableSameSite(cookie::SameSite);

impl From<&SerializableSameSite> for cookie::SameSite {
    fn from(value: &SerializableSameSite) -> Self {
        value.0
    }
}

impl serde::Serialize for SerializableSameSite {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        match self.0 {
            cookie::SameSite::Strict => serializer.serialize_str("Strict"),
            cookie::SameSite::Lax => serializer.serialize_str("Lax"),
            cookie::SameSite::None => serializer.serialize_str("None"),
        }
    }
}

impl<'de> serde::Deserialize<'de> for SerializableSameSite {
    fn deserialize<D>(deserializer: D) -> Result<SerializableSameSite, D::Error>
    where
        D: serde::de::Deserializer<'de>,
    {
        deserializer.deserialize_str(SerializableSameSiteVisitor)
    }
}

struct SerializableSameSiteVisitor;

impl<'de> serde::de::Visitor<'de> for SerializableSameSiteVisitor {
    type Value = SerializableSameSite;

    fn expecting(&self, formatter: &mut std::fmt::Formatter) -> std::fmt::Result {
        formatter.write_str("a string representing a SameSite value")
    }

    fn visit_str<E>(self, value: &str) -> Result<SerializableSameSite, E>
    where
        E: serde::de::Error,
    {
        match value {
            "Strict" => Ok(SerializableSameSite(cookie::SameSite::Strict)),
            "Lax" => Ok(SerializableSameSite(cookie::SameSite::Lax)),
            "None" => Ok(SerializableSameSite(cookie::SameSite::None)),
            _ => Err(serde::de::Error::custom("invalid SameSite value")),
        }
    }
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct Cookie {
    name: String,
    value: String,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct SetCookie {
    name: String,
    value: Option<String>,
    http_only: Option<bool>,
    secure: Option<bool>,
    same_site: Option<SerializableSameSite>,
    partitioned: Option<bool>,
    max_age: Option<time::Duration>,
    path: Option<String>,
    domain: Option<String>,
    expires: Option<time::OffsetDateTime>,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct RequestOperation {
    set: Option<Vec<Cookie>>,
    remove: Option<Vec<String>>,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct ResponseOperation {
    set: Option<Vec<SetCookie>>,
    remove: Option<Vec<String>>,
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
struct Cookies {
    request: Option<RequestOperation>,
    response: Option<ResponseOperation>,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct Configuration {
    #[serde(default = "default_log_level")]
    log_level: String,
    cookies: Cookies,
}

fn default_log_level() -> String {
    "info".to_string()
}

struct CookieManipulatorRoot {
    configuration: Option<Configuration>,
}

impl CookieManipulatorRoot {
    fn new() -> Self {
        Self {
            configuration: None,
        }
    }

    fn set_configuration(&mut self, configuration: Configuration) {
        self.configuration = Some(configuration);
    }
}

impl proxy_wasm::traits::Context for CookieManipulatorRoot {}

impl proxy_wasm::traits::RootContext for CookieManipulatorRoot {
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
        let cookies = self
            .configuration
            .clone()
            .map(|c| c.cookies)
            .unwrap_or_default();
        Some(Box::new(CookieManipulator::new(cookies)))
    }

    fn get_type(&self) -> Option<proxy_wasm::types::ContextType> {
        Some(proxy_wasm::types::ContextType::HttpContext)
    }
}

struct CookieManipulator {
    cookies: Cookies,
}

impl CookieManipulator {
    fn new(cookies: Cookies) -> Self {
        Self { cookies }
    }
}

impl proxy_wasm::traits::Context for CookieManipulator {}

impl proxy_wasm::traits::HttpContext for CookieManipulator {
    fn on_http_request_headers(&mut self, _: usize, _: bool) -> proxy_wasm::types::Action {
        if let Some(operation) = &self.cookies.request {
            if let Some(value) = self.get_http_request_header("cookie") {
                let mut new_cookies = Vec::new();

                for cookie in cookie::Cookie::split_parse(value) {
                    if let Ok(c) = &cookie {
                        if let Some(remove) = &operation.remove {
                            if remove.contains(&c.name().to_string()) {
                                continue;
                            }
                        }

                        let mut new_cookie = c.clone();

                        if let Some(set) = &operation.set {
                            for s in set {
                                if c.name() == s.name {
                                    new_cookie.set_value(&s.value);
                                }
                            }
                        }

                        new_cookies.push(new_cookie);
                    }
                }

                if new_cookies.is_empty() {
                    self.set_http_request_header("cookie", None);
                    return proxy_wasm::types::Action::Continue;
                }

                let new_cookie = new_cookies
                    .iter()
                    .map(|c| c.to_string())
                    .collect::<Vec<String>>()
                    .join("; ");
                self.set_http_request_header("cookie", Some(&new_cookie));
            }
        }
        proxy_wasm::types::Action::Continue
    }

    fn on_http_response_headers(&mut self, _num_headers: usize, _end_of_stream: bool) -> Action {
        if let Some(operation) = &self.cookies.response {
            let mut set_cookies = vec![];
            // To send multiple cookies, multiple Set-Cookie headers should be sent in the same response.
            // https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie
            for (key, value) in self.get_http_response_headers() {
                if key != "set-cookie" {
                    continue;
                }
                if let Ok(mut c) = cookie::Cookie::parse(value) {
                    if let Some(remove) = &operation.remove {
                        if remove.contains(&c.name().to_string()) {
                            continue;
                        }
                    }

                    if let Some(set) = &operation.set {
                        for s in set {
                            if c.name() == s.name {
                                if let Some(value) = &s.value {
                                    c.set_value(value);
                                }
                                if let Some(http_only) = s.http_only {
                                    c.set_http_only(http_only);
                                }
                                if let Some(secure) = s.secure {
                                    c.set_secure(secure);
                                }
                                if let Some(same_site) = &s.same_site {
                                    c.set_same_site(Some(same_site.into()));
                                }
                                if let Some(partitioned) = s.partitioned {
                                    c.set_partitioned(partitioned);
                                }
                                if let Some(max_age) = s.max_age {
                                    c.set_max_age(max_age);
                                }
                                if let Some(path) = &s.path {
                                    c.set_path(path);
                                }
                                if let Some(domain) = &s.domain {
                                    c.set_domain(domain);
                                }
                                if let Some(expires) = &s.expires {
                                    c.set_expires(*expires);
                                }
                            }
                        }
                    }

                    set_cookies.push(c.to_string());
                }
            }

            self.set_http_response_header("set-cookie", None);
            for set_cookie in set_cookies {
                self.add_http_response_header("set-cookie", &set_cookie);
            }
        }
        proxy_wasm::types::Action::Continue
    }
}
