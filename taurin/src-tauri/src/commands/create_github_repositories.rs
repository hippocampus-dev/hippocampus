const MAX_RETRIES: usize = 3;
const ROW_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(30);

#[tauri::command]
#[ipc::estringify]
pub async fn read_github_repositories(
    _app_handle: tauri::AppHandle,
    path: String,
) -> Result<ipc::types::CsvTable, Box<dyn std::error::Error + Send + Sync + 'static>> {
    internal::read_table(std::fs::File::open(path)?)
}

#[tauri::command]
pub async fn create_github_repositories(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
    header: Vec<String>,
    records: Vec<Vec<String>>,
    concurrency: usize,
    event_tx: tauri::ipc::Channel<ipc::types::RepositoryCreationEvent>,
) -> Result<(), ipc::CommandError> {
    let mut cancelled = state
        .lock()
        .await
        .create_github_repositories
        .cancellation
        .begin();

    tokio::select! {
        result = internal::create_repositories(header, records, concurrency, &event_tx) => Ok(result?),
        Ok(()) = cancelled.changed() => Err(ipc::CommandError::Cancelled),
    }
}

#[tauri::command]
#[ipc::estringify]
pub async fn save_github_repositories_result(
    _app_handle: tauri::AppHandle,
    path: String,
    table: Vec<Vec<String>>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let mut file = std::fs::File::create(path)?;
    std::io::Write::write_all(&mut file, "\u{feff}".as_bytes())?;
    let mut writer = csv::WriterBuilder::new()
        .terminator(csv::Terminator::CRLF)
        .from_writer(file);
    for record in table {
        writer.write_record(record)?;
    }
    writer.flush()?;
    Ok(())
}

#[tauri::command]
pub async fn cancel_create_github_repositories(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<(), ipc::CommandError> {
    state
        .lock()
        .await
        .create_github_repositories
        .cancellation
        .cancel();
    Ok(())
}

mod internal {
    use super::*;

    use futures_util::StreamExt;

    #[derive(Debug, PartialEq, serde::Deserialize)]
    pub(super) struct Row {
        pub(super) repository: String,
        #[serde(default)]
        pub(super) description: String,
        #[serde(default)]
        pub(super) visibility: String,
    }

    #[derive(serde::Deserialize)]
    struct Repository {
        html_url: String,
    }

    pub(super) fn read_table(
        reader: impl std::io::Read,
    ) -> Result<ipc::types::CsvTable, Box<dyn std::error::Error + Send + Sync + 'static>> {
        let mut reader = csv::ReaderBuilder::new().flexible(true).from_reader(reader);
        let header: Vec<String> = reader.headers()?.iter().map(String::from).collect();
        let mut records = Vec::new();
        for record in reader.records() {
            let record = record?;
            records.push(
                (0..header.len())
                    .map(|index| record.get(index).unwrap_or("").to_string())
                    .collect(),
            );
        }
        Ok(ipc::types::CsvTable { header, records })
    }

    pub(super) async fn create_repositories(
        header: Vec<String>,
        records: Vec<Vec<String>>,
        concurrency: usize,
        event_tx: &tauri::ipc::Channel<ipc::types::RepositoryCreationEvent>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let headers = csv::StringRecord::from(header);

        let client = bakery::Client::new(crate::commands::BAKERY_URL, 0);
        let token = client.get_value(crate::commands::COOKIE_NAME).await?;

        let https = {
            use hyper_rustls::ConfigBuilderExt;
            let config = rustls::ClientConfig::builder()
                .with_safe_defaults()
                .with_native_roots()
                .with_no_client_auth();
            hyper_rustls::HttpsConnectorBuilder::new()
                .with_tls_config(config)
                .https_only()
                .enable_http1()
                .build()
        };
        let client: hyper::Client<_, hyper::Body> = hyper::Client::builder().build(https);

        let results: Vec<Result<(), tauri::Error>> =
            futures_util::stream::iter(records.into_iter().enumerate())
                .map(|(index, record)| {
                    let client = &client;
                    let token = &token;
                    let headers = &headers;
                    async move {
                        let index = index as u64;
                        event_tx.send(ipc::types::RepositoryCreationEvent::Running { index })?;
                        let result: Result<
                            String,
                            Box<dyn std::error::Error + Send + Sync + 'static>,
                        > = match csv::StringRecord::from(record).deserialize::<Row>(Some(headers))
                        {
                            Ok(row) => tokio::time::timeout(ROW_TIMEOUT, async {
                                let private = match row.visibility.as_str() {
                                    "" | "private" => true,
                                    "public" => false,
                                    visibility => {
                                        return Err(format!(
                                            "visibility must be public or private: {visibility}"
                                        )
                                        .into());
                                    }
                                };
                                let body = serde_json::to_vec(&serde_json::json!({
                                    "name": row.repository,
                                    "description": row.description,
                                    "private": private,
                                }))?;

                                let mut retries = 0;
                                loop {
                                    let request = hyper::Request::post(format!(
                                        "{}/user/repos",
                                        crate::commands::GITHUB_CREDENTIAL_PROXY_URL
                                    ))
                                    .header(hyper::header::ACCEPT, "application/vnd.github+json")
                                    .header(hyper::header::CONTENT_TYPE, "application/json")
                                    .header("X-GitHub-Api-Version", "2022-11-28")
                                    // https://docs.github.com/en/rest/using-the-rest-api/getting-started-with-the-rest-api#user-agent
                                    .header(hyper::header::USER_AGENT, "taurin")
                                    .header(
                                        hyper::header::COOKIE,
                                        format!("{}={}", crate::commands::COOKIE_NAME, token),
                                    )
                                    .body(hyper::Body::from(body.clone()))?;

                                    let response = client.request(request).await?;
                                    let status = response.status();
                                    let retry_after = response
                                        .headers()
                                        .get(hyper::header::RETRY_AFTER)
                                        .and_then(|v| v.to_str().ok())
                                        .and_then(|v| v.parse::<u64>().ok());
                                    let bytes = hyper::body::to_bytes(response.into_body()).await?;

                                    if status.is_success() {
                                        let repository: Repository =
                                            serde_json::from_slice(&bytes)?;
                                        return Ok(repository.html_url);
                                    }

                                    // https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#exceeding-the-rate-limit
                                    if let Some(seconds) = retry_after
                                        && (status == hyper::StatusCode::FORBIDDEN
                                            || status == hyper::StatusCode::TOO_MANY_REQUESTS)
                                        && retries < MAX_RETRIES
                                    {
                                        retries += 1;
                                        tokio::time::sleep(std::time::Duration::from_secs(seconds))
                                            .await;
                                        continue;
                                    }

                                    return Err(format!(
                                        "failed to create repository: status {}: {}",
                                        status,
                                        String::from_utf8_lossy(&bytes)
                                    )
                                    .into());
                                }
                            })
                            .await
                            .unwrap_or_else(|e| Err(e.into())),
                            Err(e) => Err(e.into()),
                        };
                        event_tx.send(match result {
                            Ok(html_url) => {
                                ipc::types::RepositoryCreationEvent::Created { index, html_url }
                            }
                            Err(e) => ipc::types::RepositoryCreationEvent::Failed {
                                index,
                                message: e.to_string(),
                            },
                        })
                    }
                })
                .buffer_unordered(concurrency)
                .collect()
                .await;
        results.into_iter().collect::<Result<(), _>>()?;

        Ok(())
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn test_read_table() {
            let table = read_table(
                "repository,description,visibility\nfoo,\"bar, \"\"baz\"\"\",public\nqux,,\nquux,corge\n"
                    .as_bytes(),
            )
            .unwrap();
            assert_eq!(
                table,
                ipc::types::CsvTable {
                    header: vec![
                        "repository".to_string(),
                        "description".to_string(),
                        "visibility".to_string(),
                    ],
                    records: vec![
                        vec![
                            "foo".to_string(),
                            "bar, \"baz\"".to_string(),
                            "public".to_string()
                        ],
                        vec!["qux".to_string(), "".to_string(), "".to_string()],
                        vec!["quux".to_string(), "corge".to_string(), "".to_string()],
                    ],
                }
            );
        }

        #[test]
        fn test_deserialize_row() {
            let mut reader = csv::Reader::from_reader(
                "repository,description,visibility\nfoo,\"bar, \"\"baz\"\"\",public\nqux,,\n"
                    .as_bytes(),
            );
            let headers = reader.headers().unwrap().clone();
            assert_eq!(
                reader
                    .records()
                    .map(|record| record.unwrap().deserialize::<Row>(Some(&headers)).unwrap())
                    .collect::<Vec<_>>(),
                vec![
                    Row {
                        repository: "foo".to_string(),
                        description: "bar, \"baz\"".to_string(),
                        visibility: "public".to_string(),
                    },
                    Row {
                        repository: "qux".to_string(),
                        description: "".to_string(),
                        visibility: "".to_string(),
                    },
                ]
            );
        }
    }
}
