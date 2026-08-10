use proxy_wasm::traits::Context;

#[cfg(not(test))]
#[unsafe(no_mangle)]
pub fn _start() {
    proxy_wasm::set_log_level(proxy_wasm::types::LogLevel::Trace);
    proxy_wasm::set_http_context(|_, _| -> Box<dyn proxy_wasm::traits::HttpContext> {
        Box::new(HeaderDebugger)
    });
}

struct HeaderDebugger;

impl proxy_wasm::traits::Context for HeaderDebugger {}

impl proxy_wasm::traits::HttpContext for HeaderDebugger {
    fn on_http_response_headers(&mut self, _: usize, _: bool) -> proxy_wasm::types::Action {
        for (name, masked_value) in self
            .get_http_response_headers()
            .into_iter()
            .map(mask_sensitive_info)
        {
            log::info!("{}: {}", name, masked_value);
        }
        proxy_wasm::types::Action::Continue
    }
}

fn mask_sensitive_info<'a, S, C>((name, value): (S, C)) -> (S, std::borrow::Cow<'a, str>)
where
    S: AsRef<str>,
    C: Into<std::borrow::Cow<'a, str>>,
{
    if name.as_ref().eq_ignore_ascii_case("authorization") {
        (name, std::borrow::Cow::Borrowed("********"))
    } else {
        (name, value.into())
    }
}
