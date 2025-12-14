use pprof::protos::Message;

pub async fn profile(
    axum::extract::Query(params): axum::extract::Query<std::collections::HashMap<String, String>>,
) -> impl axum::response::IntoResponse {
    let mut headers = axum::http::HeaderMap::new();
    match handle(params.get("seconds").and_then(|i| i.parse::<u64>().ok())).await {
        Ok(gzip_body) => {
            headers.insert(
                axum::http::header::CONTENT_LENGTH,
                axum::http::HeaderValue::from(gzip_body.len()),
            );
            headers.insert(
                axum::http::header::CONTENT_TYPE,
                axum::http::HeaderValue::from_static("application/octet-stream"),
            );
            (
                axum::http::StatusCode::OK,
                headers,
                axum::response::Response::new(axum::body::Body::from(gzip_body)),
            )
        }
        Err(e) => {
            tracing::error!("{}", e);
            (
                axum::http::StatusCode::INTERNAL_SERVER_ERROR,
                headers,
                axum::response::Response::new(axum::body::Body::from("internal server error")),
            )
        }
    }
}

async fn handle(seconds: Option<u64>) -> Result<Vec<u8>, Box<dyn std::error::Error + Send + Sync>> {
    let mut duration = std::time::Duration::from_secs(30);
    if let Some(seconds) = seconds {
        duration = std::time::Duration::from_secs(seconds);
    }
    let guard = pprof::ProfilerGuard::new(1_000_000)?;
    std::thread::sleep(duration);

    let mut body = Vec::new();
    if let Ok(report) = guard.report().build() {
        let profile = report.pprof()?;
        profile.write_to_vec(&mut body)?;
    }

    let mut encoder = libflate::gzip::Encoder::new(Vec::new())?;
    std::io::copy(&mut &body[..], &mut encoder)?;
    let gzip_body = encoder.finish().into_result()?;
    Ok(gzip_body)
}
