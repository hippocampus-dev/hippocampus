#[doc(hidden)]
pub mod __macro_support {
    use opentelemetry::trace::TraceContextExt;
    use tracing_opentelemetry::OpenTelemetrySpanExt;

    pub fn __traceparent() -> Option<(
        opentelemetry::trace::TraceId,
        opentelemetry::trace::SpanId,
        opentelemetry::trace::TraceFlags,
    )> {
        let context = &tracing::Span::current().context();
        let span = context.span();
        let span_context = span.span_context();

        if !span_context.is_valid() {
            return None;
        }

        let trace_id = span_context.trace_id();
        let span_id = span_context.span_id();
        let trace_flags = span_context.trace_flags();

        Some((trace_id, span_id, trace_flags))
    }
}

#[macro_export]
macro_rules! trace {
    ({ $($field:tt)* }, $($arg:tt)*) => {
        $crate::event!(tracing::Level::TRACE, { $($field)* }, $($arg)*)
    };
    ($($arg:tt)*) => {
        $crate::event!(tracing::Level::TRACE, $($arg)*)
    };
}

#[macro_export]
macro_rules! debug {
    ({ $($field:tt)* }, $($arg:tt)*) => {
        $crate::event!(tracing::Level::DEBUG, { $($field)* }, $($arg)*)
    };
    ($($arg:tt)*) => {
        $crate::event!(tracing::Level::DEBUG, $($arg)*)
    };
}

#[macro_export]
macro_rules! info {
    ({ $($field:tt)* }, $($arg:tt)*) => {
        $crate::event!(tracing::Level::INFO, { $($field)* }, $($arg)*)
    };
    ($($arg:tt)*) => {
        $crate::event!(tracing::Level::INFO, $($arg)*)
    };
}

#[macro_export]
macro_rules! warn {
    ({ $($field:tt)* }, $($arg:tt)*) => {
        $crate::event!(tracing::Level::WARN, { $($field)* }, $($arg)*)
    };
    ($($arg:tt)*) => {
        $crate::event!(tracing::Level::WARN, $($arg)*)
    };
}

#[macro_export]
macro_rules! error {
    ({ $($field:tt)* }, $($arg:tt)*) => {
        $crate::event!(tracing::Level::ERROR, { $($field)* }, $($arg)*)
    };
    ($($arg:tt)*) => {
        $crate::event!(tracing::Level::ERROR, $($arg)*)
    };
}

#[macro_export]
macro_rules! event {
    ($lvl:expr, { $($field:tt)* }, $($arg:tt)*) => {{
        if tracing::enabled!($lvl) {
            match $crate::__macro_support::__traceparent() {
                Some((trace_id, span_id, _)) => tracing::event!($lvl, { "traceid" = trace_id.to_string(), "spanid" = span_id.to_string(), $($field)* }, $($arg)*),
                None => tracing::event!($lvl, { $($field)* }, $($arg)*),
            }
        }
    }};
    ($lvl:expr, $($arg:tt)*) => {{
        if tracing::enabled!($lvl) {
            match $crate::__macro_support::__traceparent() {
                Some((trace_id, span_id, _)) => tracing::event!($lvl, { "traceid" = trace_id.to_string(), "spanid" = span_id.to_string() }, $($arg)*),
                None => tracing::event!($lvl, $($arg)*),
            }
        }
    }};
}
