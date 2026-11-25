const HOST_NAME: &str = "dev.kaidotio.taurin";
// Derived from extension/manifest.json's "key"
const ALLOWED_ORIGINS: &[&str] = &["chrome-extension://phegfihdgknjjilfnlhljhkbljdbpogk/"];
// https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging#native-messaging-host-protocol
const MAX_INCOMING_MESSAGE_LENGTH: usize = 64 * 1024 * 1024;
const MAX_OUTGOING_MESSAGE_LENGTH: usize = 1024 * 1024;
const CONNECT_MESSAGE_TYPE: &str = "connect";
const BRIDGE_ENDPOINT_FILE_NAME: &str = if cfg!(debug_assertions) {
    "dev.kaidotio.taurin.dev.json"
} else {
    "dev.kaidotio.taurin.json"
};
const BRIDGE_HANDSHAKE_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(5);
const BRIDGE_RECONNECT_INTERVAL: std::time::Duration = std::time::Duration::from_secs(1);

pub trait Handler {
    fn handle(
        &self,
        message: serde_json::Value,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync + 'static>>;
}

impl<F> Handler for F
where
    F: Fn(
        serde_json::Value,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync + 'static>>,
{
    fn handle(
        &self,
        message: serde_json::Value,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync + 'static>> {
        self(message)
    }
}

#[derive(Default)]
pub struct Router {
    handlers: std::collections::HashMap<String, Box<dyn Handler>>,
}

impl Router {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn register(
        mut self,
        message_type: impl Into<String>,
        handler: impl Handler + 'static,
    ) -> Self {
        self.handlers.insert(message_type.into(), Box::new(handler));
        self
    }

    pub fn dispatch(
        &self,
        message: serde_json::Value,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync + 'static>> {
        let message_type = message
            .get("type")
            .and_then(serde_json::Value::as_str)
            .ok_or("missing message type")?;
        let handler = self
            .handlers
            .get(message_type)
            .ok_or_else(|| format!("unknown message type: {message_type}"))?;
        handler.handle(message)
    }

    pub fn serve(
        &self,
        reader: &mut impl std::io::Read,
        writer: &mut (impl std::io::Write + Send),
        endpoint_path: Option<&std::path::Path>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let writer = &std::sync::Mutex::new(writer);
        let stop_signal = &(std::sync::Mutex::new(false), std::sync::Condvar::new());
        let bridge_stream: &std::sync::Mutex<Option<std::net::TcpStream>> =
            &std::sync::Mutex::new(None);

        std::thread::scope(|scope| {
            let mut bridge_started = false;

            let result = (|| -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
                while let Some(message) = read_message(reader)? {
                    if let Some(message_type) =
                        message.get("type").and_then(serde_json::Value::as_str)
                    {
                        if message_type == CONNECT_MESSAGE_TYPE {
                            if !bridge_started && let Some(endpoint_path) = endpoint_path {
                                bridge_started = true;
                                scope.spawn(move || {
                                    loop {
                                        let endpoint =
                                            std::fs::read(endpoint_path).ok().and_then(|bytes| {
                                                serde_json::from_slice::<BridgeEndpoint>(&bytes)
                                                    .ok()
                                            });
                                        if let Some(endpoint) = endpoint {
                                            let address = std::net::SocketAddr::from((
                                                [127, 0, 0, 1],
                                                endpoint.port,
                                            ));
                                            if let Ok(mut stream) =
                                                std::net::TcpStream::connect_timeout(
                                                    &address,
                                                    BRIDGE_HANDSHAKE_TIMEOUT,
                                                )
                                                && stream
                                                    .set_read_timeout(Some(
                                                        BRIDGE_HANDSHAKE_TIMEOUT,
                                                    ))
                                                    .is_ok()
                                                && stream
                                                    .set_write_timeout(Some(
                                                        BRIDGE_HANDSHAKE_TIMEOUT,
                                                    ))
                                                    .is_ok()
                                                && write_message(
                                                    &mut stream,
                                                    &serde_json::json!({ "token": endpoint.token }),
                                                )
                                                .is_ok()
                                                && matches!(read_message(&mut stream), Ok(Some(_)))
                                                && stream.set_read_timeout(None).is_ok()
                                                && stream.set_write_timeout(None).is_ok()
                                                && let Ok(writer_stream) = stream.try_clone()
                                            {
                                                let mut registered = false;
                                                {
                                                    let stopped = stop_signal.0.lock().unwrap();
                                                    if !*stopped {
                                                        *bridge_stream.lock().unwrap() =
                                                            Some(writer_stream);
                                                        registered = true;
                                                    }
                                                }
                                                if !registered {
                                                    let _ =
                                                        stream.shutdown(std::net::Shutdown::Both);
                                                    break;
                                                }
                                                while let Ok(Some(outgoing)) =
                                                    read_message(&mut stream)
                                                {
                                                    let mut locked_writer = writer.lock().unwrap();
                                                    if write_message(
                                                        &mut **locked_writer,
                                                        &outgoing,
                                                    )
                                                    .is_err()
                                                    {
                                                        break;
                                                    }
                                                }
                                                *bridge_stream.lock().unwrap() = None;
                                            }
                                        }

                                        let stopped = stop_signal.0.lock().unwrap();
                                        if *stopped {
                                            break;
                                        }
                                        let (stopped, _) = stop_signal
                                            .1
                                            .wait_timeout(stopped, BRIDGE_RECONNECT_INTERVAL)
                                            .unwrap();
                                        if *stopped {
                                            break;
                                        }
                                    }
                                });
                            }
                            let mut locked_writer = writer.lock().unwrap();
                            write_message(&mut **locked_writer, &serde_json::json!({}))?;
                        } else {
                            let response = self.dispatch(message)?;
                            let mut locked_writer = writer.lock().unwrap();
                            write_message(&mut **locked_writer, &response)?;
                        }
                    } else if message.get("id").is_some() {
                        let mut locked_stream = bridge_stream.lock().unwrap();
                        if let Some(stream) = locked_stream.as_mut()
                            && write_message_with_limit(
                                stream,
                                &message,
                                MAX_INCOMING_MESSAGE_LENGTH,
                            )
                            .is_err()
                        {
                            let _ = stream.shutdown(std::net::Shutdown::Both);
                            *locked_stream = None;
                        }
                    } else {
                        self.dispatch(message)?;
                    }
                }
                Ok(())
            })();

            let active_stream = {
                let mut stopped = stop_signal.0.lock().unwrap();
                *stopped = true;
                bridge_stream.lock().unwrap().take()
            };
            stop_signal.1.notify_all();
            if let Some(stream) = active_stream {
                let _ = stream.shutdown(std::net::Shutdown::Both);
            }

            result
        })
    }
}

// https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging#native-messaging-host
pub fn is_native_messaging(arguments: impl IntoIterator<Item = String>) -> bool {
    arguments
        .into_iter()
        .any(|argument| argument.starts_with("chrome-extension://"))
}

pub fn read_message(
    reader: &mut impl std::io::Read,
) -> Result<Option<serde_json::Value>, Box<dyn std::error::Error + Send + Sync + 'static>> {
    let mut length_bytes = [0u8; 4];
    loop {
        match reader.read(&mut length_bytes[..1]) {
            Ok(0) => return Ok(None),
            Ok(_) => break,
            Err(e) if e.kind() == std::io::ErrorKind::Interrupted => continue,
            Err(e) => return Err(e.into()),
        }
    }
    reader.read_exact(&mut length_bytes[1..])?;

    let length = u32::from_ne_bytes(length_bytes) as usize;
    if length > MAX_INCOMING_MESSAGE_LENGTH {
        return Err(format!("incoming message length exceeds limit: {length}").into());
    }
    let mut payload = vec![0u8; length];
    reader.read_exact(&mut payload)?;
    Ok(Some(serde_json::from_slice(&payload)?))
}

fn encode_message(
    message: &serde_json::Value,
    max_length: usize,
) -> Result<Vec<u8>, Box<dyn std::error::Error + Send + Sync + 'static>> {
    let payload = serde_json::to_vec(message)?;
    if payload.len() > max_length {
        return Err(format!("outgoing message length exceeds limit: {}", payload.len()).into());
    }
    let length = u32::try_from(payload.len())?;
    let mut frame = Vec::with_capacity(4 + payload.len());
    frame.extend_from_slice(&length.to_ne_bytes());
    frame.extend_from_slice(&payload);
    Ok(frame)
}

fn write_message_with_limit(
    writer: &mut impl std::io::Write,
    message: &serde_json::Value,
    max_length: usize,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let frame = encode_message(message, max_length)?;
    writer.write_all(&frame)?;
    writer.flush()?;
    Ok(())
}

pub fn write_message(
    writer: &mut impl std::io::Write,
    message: &serde_json::Value,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    write_message_with_limit(writer, message, MAX_OUTGOING_MESSAGE_LENGTH)
}

pub fn default_router() -> Router {
    Router::new().register("create_github_repository", |_message| {
        Ok(serde_json::json!({}))
    })
}

pub fn run() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let mut stdin = std::io::stdin().lock();
    let mut stdout = std::io::stdout();
    default_router().serve(&mut stdin, &mut stdout, Some(&bridge_endpoint_path()))
}

#[derive(serde::Serialize, serde::Deserialize)]
struct BridgeEndpoint {
    port: u16,
    token: String,
}

fn bridge_endpoint_path() -> std::path::PathBuf {
    #[cfg(target_os = "linux")]
    let directory = std::env::var_os("XDG_RUNTIME_DIR")
        .filter(|value| !value.is_empty())
        .map(std::path::PathBuf::from)
        .or_else(|| {
            std::env::var_os("XDG_STATE_HOME")
                .filter(|value| !value.is_empty())
                .map(|value| std::path::PathBuf::from(value).join(HOST_NAME))
        })
        .or_else(|| {
            std::env::var_os("HOME")
                .filter(|value| !value.is_empty())
                .map(|value| {
                    std::path::PathBuf::from(value)
                        .join(".local")
                        .join("state")
                        .join(HOST_NAME)
                })
        })
        .unwrap_or_else(std::env::temp_dir);
    #[cfg(not(target_os = "linux"))]
    let directory = std::env::temp_dir();

    directory.join(BRIDGE_ENDPOINT_FILE_NAME)
}

struct PendingRequest {
    connection_id: u64,
    sender: tokio::sync::oneshot::Sender<Result<serde_json::Value, String>>,
}

struct PendingRequestGuard<'a> {
    pending: &'a std::sync::Mutex<std::collections::HashMap<u64, PendingRequest>>,
    request_id: u64,
}

impl Drop for PendingRequestGuard<'_> {
    fn drop(&mut self) {
        self.pending.lock().unwrap().remove(&self.request_id);
    }
}

struct BridgeInner {
    endpoint_path: std::path::PathBuf,
    token: String,
    port: u16,
    next_request_id: std::sync::atomic::AtomicU64,
    next_connection_id: std::sync::atomic::AtomicU64,
    connections: std::sync::Mutex<
        Vec<(
            u64,
            std::sync::Arc<(std::net::TcpStream, std::sync::Mutex<()>)>,
        )>,
    >,
    pending: std::sync::Mutex<std::collections::HashMap<u64, PendingRequest>>,
    stopped: std::sync::atomic::AtomicBool,
}

impl BridgeInner {
    fn shutdown(&self) {
        if self
            .stopped
            .swap(true, std::sync::atomic::Ordering::Relaxed)
        {
            return;
        }
        let _ =
            std::net::TcpStream::connect(std::net::SocketAddr::from(([127, 0, 0, 1], self.port)));
        let (connections, pending) = {
            let connections = std::mem::take(&mut *self.connections.lock().unwrap());
            let pending = std::mem::take(&mut *self.pending.lock().unwrap());
            (connections, pending)
        };
        for (_, connection) in connections {
            let _ = connection.0.shutdown(std::net::Shutdown::Both);
        }
        for (_, request) in pending {
            let _ = request
                .sender
                .send(Err("native messaging host disconnected".to_string()));
        }
        if let Ok(bytes) = std::fs::read(&self.endpoint_path)
            && let Ok(endpoint) = serde_json::from_slice::<BridgeEndpoint>(&bytes)
            && endpoint.token == self.token
        {
            let _ = std::fs::remove_file(&self.endpoint_path);
        }
    }
}

impl Drop for BridgeInner {
    fn drop(&mut self) {
        self.shutdown();
    }
}

#[derive(Clone)]
pub struct Bridge {
    inner: std::sync::Arc<BridgeInner>,
}

impl std::fmt::Debug for Bridge {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Bridge")
            .field("endpoint_path", &self.inner.endpoint_path)
            .field("port", &self.inner.port)
            .finish()
    }
}

impl Bridge {
    pub fn start() -> Result<Self, Box<dyn std::error::Error + Send + Sync + 'static>> {
        Self::start_at(bridge_endpoint_path())
    }

    fn start_at(
        endpoint_path: std::path::PathBuf,
    ) -> Result<Self, Box<dyn std::error::Error + Send + Sync + 'static>> {
        use base64::Engine;

        let mut token_bytes = [0u8; 32];
        getrandom::getrandom(&mut token_bytes)?;
        let token = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(token_bytes);

        let listener = std::net::TcpListener::bind("127.0.0.1:0")?;
        let port = listener.local_addr()?.port();

        let bytes = serde_json::to_vec(&BridgeEndpoint {
            port,
            token: token.clone(),
        })?;
        if let Some(parent) = endpoint_path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        #[cfg(unix)]
        {
            use std::io::Write;
            use std::os::unix::fs::OpenOptionsExt;

            let mut suffix_bytes = [0u8; 8];
            getrandom::getrandom(&mut suffix_bytes)?;
            let suffix = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(suffix_bytes);
            let temporary_path = endpoint_path.with_extension(format!("json.{suffix}.tmp"));
            let mut file = std::fs::OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(&temporary_path)?;
            let write_result = (|| -> std::io::Result<()> {
                file.write_all(&bytes)?;
                file.sync_all()?;
                drop(file);
                std::fs::rename(&temporary_path, &endpoint_path)
            })();
            if let Err(e) = write_result {
                let _ = std::fs::remove_file(&temporary_path);
                return Err(e.into());
            }
        }
        #[cfg(not(unix))]
        {
            std::fs::write(&endpoint_path, &bytes)?;
        }

        let mut request_id_bytes = [0u8; 8];
        getrandom::getrandom(&mut request_id_bytes[..6])?;
        let initial_request_id = u64::from_le_bytes(request_id_bytes).max(1);

        let inner = std::sync::Arc::new(BridgeInner {
            endpoint_path,
            token,
            port,
            next_request_id: std::sync::atomic::AtomicU64::new(initial_request_id),
            next_connection_id: std::sync::atomic::AtomicU64::new(1),
            connections: std::sync::Mutex::new(Vec::new()),
            pending: std::sync::Mutex::new(std::collections::HashMap::new()),
            stopped: std::sync::atomic::AtomicBool::new(false),
        });

        let accept_inner = std::sync::Arc::downgrade(&inner);
        std::thread::spawn(move || {
            for incoming in listener.incoming() {
                if accept_inner
                    .upgrade()
                    .is_none_or(|inner| inner.stopped.load(std::sync::atomic::Ordering::Relaxed))
                {
                    break;
                }
                let Ok(mut reader_stream) = incoming else {
                    continue;
                };
                let connection_inner = accept_inner.clone();
                std::thread::spawn(move || {
                    if reader_stream
                        .set_read_timeout(Some(BRIDGE_HANDSHAKE_TIMEOUT))
                        .is_err()
                    {
                        return;
                    }
                    let Ok(Some(handshake)) = read_message(&mut reader_stream) else {
                        return;
                    };
                    let Some(received_token) =
                        handshake.get("token").and_then(serde_json::Value::as_str)
                    else {
                        return;
                    };
                    if reader_stream.set_read_timeout(None).is_err() {
                        return;
                    }
                    let Ok(mut writer_stream) = reader_stream.try_clone() else {
                        return;
                    };

                    let expected_token = {
                        let Some(inner) = connection_inner.upgrade() else {
                            return;
                        };
                        if inner.stopped.load(std::sync::atomic::Ordering::Relaxed) {
                            return;
                        }
                        inner.token.clone()
                    };
                    let expected = expected_token.as_bytes();
                    let actual = received_token.as_bytes();
                    if expected.len() != actual.len()
                        || expected
                            .iter()
                            .zip(actual.iter())
                            .fold(0u8, |difference, (left, right)| difference | (left ^ right))
                            != 0
                    {
                        return;
                    }
                    if writer_stream
                        .set_write_timeout(Some(BRIDGE_HANDSHAKE_TIMEOUT))
                        .is_err()
                        || write_message(&mut writer_stream, &serde_json::json!({})).is_err()
                        || writer_stream.set_write_timeout(None).is_err()
                    {
                        return;
                    }

                    let connection_id = {
                        let Some(inner) = connection_inner.upgrade() else {
                            return;
                        };
                        if inner.stopped.load(std::sync::atomic::Ordering::Relaxed) {
                            return;
                        }
                        let connection_id = inner
                            .next_connection_id
                            .fetch_add(1, std::sync::atomic::Ordering::Relaxed);
                        inner.connections.lock().unwrap().push((
                            connection_id,
                            std::sync::Arc::new((writer_stream, std::sync::Mutex::new(()))),
                        ));
                        connection_id
                    };

                    while let Ok(Some(message)) = read_message(&mut reader_stream) {
                        let Some(request_id) =
                            message.get("id").and_then(serde_json::Value::as_u64)
                        else {
                            continue;
                        };
                        let pending_sender = {
                            let Some(inner) = connection_inner.upgrade() else {
                                break;
                            };
                            let mut pending = inner.pending.lock().unwrap();
                            if pending
                                .get(&request_id)
                                .is_some_and(|entry| entry.connection_id == connection_id)
                            {
                                pending.remove(&request_id).map(|entry| entry.sender)
                            } else {
                                None
                            }
                        };
                        if let Some(sender) = pending_sender {
                            let response = if let Some(error) =
                                message.get("error").and_then(serde_json::Value::as_str)
                            {
                                Err(error.to_string())
                            } else {
                                Ok(message
                                    .get("result")
                                    .cloned()
                                    .unwrap_or(serde_json::Value::Null))
                            };
                            let _ = sender.send(response);
                        }
                    }

                    let disconnected_senders: Vec<_> =
                        if let Some(inner) = connection_inner.upgrade() {
                            let mut connections = inner.connections.lock().unwrap();
                            connections.retain(|(id, _)| *id != connection_id);
                            let mut pending = inner.pending.lock().unwrap();
                            let keys: Vec<u64> = pending
                                .iter()
                                .filter_map(|(key, entry)| {
                                    (entry.connection_id == connection_id).then_some(*key)
                                })
                                .collect();
                            keys.into_iter()
                                .filter_map(|key| pending.remove(&key).map(|entry| entry.sender))
                                .collect()
                        } else {
                            Vec::new()
                        };
                    for sender in disconnected_senders {
                        let _ = sender.send(Err("native messaging host disconnected".to_string()));
                    }
                });
            }
        });

        Ok(Self { inner })
    }

    pub fn shutdown(&self) {
        self.inner.shutdown();
    }

    pub async fn send(
        &self,
        mut message: serde_json::Value,
        timeout: std::time::Duration,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync + 'static>> {
        let request_id = self
            .inner
            .next_request_id
            .fetch_add(1, std::sync::atomic::Ordering::Relaxed);
        {
            let object = message
                .as_object_mut()
                .ok_or("outgoing message must be a JSON object")?;
            if !object.get("type").is_some_and(serde_json::Value::is_string) {
                return Err("missing message type".into());
            }
            object.insert("id".to_string(), serde_json::json!(request_id));
        }
        let frame = encode_message(&message, MAX_OUTGOING_MESSAGE_LENGTH)?;
        let deadline = tokio::time::Instant::now() + timeout;

        let (_guard, receiver) = loop {
            let remaining = deadline.saturating_duration_since(tokio::time::Instant::now());
            if remaining.is_zero() {
                return Err("native messaging request timed out".into());
            }

            let (connection_id, writer, receiver) = {
                let connections = self.inner.connections.lock().unwrap();
                let Some((connection_id, writer)) = connections.last().cloned() else {
                    return Err("no native messaging host connected".into());
                };
                let (sender, receiver) = tokio::sync::oneshot::channel();
                self.inner.pending.lock().unwrap().insert(
                    request_id,
                    PendingRequest {
                        connection_id,
                        sender,
                    },
                );
                (connection_id, writer, receiver)
            };
            let guard = PendingRequestGuard {
                pending: &self.inner.pending,
                request_id,
            };

            let write_connection = std::sync::Arc::clone(&writer);
            let write_frame = frame.clone();
            let write_result = tokio::task::spawn_blocking(move || -> std::io::Result<()> {
                use std::io::Write;

                let _lock = write_connection.1.lock().unwrap();
                let mut stream = &write_connection.0;
                stream.set_write_timeout(Some(remaining))?;
                let result = (|| -> std::io::Result<()> {
                    stream.write_all(&write_frame)?;
                    stream.flush()
                })();
                let _ = stream.set_write_timeout(None);
                if result.is_err() {
                    let _ = stream.shutdown(std::net::Shutdown::Both);
                }
                result
            })
            .await;

            if matches!(write_result, Ok(Ok(()))) {
                break (guard, receiver);
            }

            drop(guard);
            self.inner
                .connections
                .lock()
                .unwrap()
                .retain(|(id, _)| *id != connection_id);
            let _ = writer.0.shutdown(std::net::Shutdown::Both);
        };

        match tokio::time::timeout_at(deadline, receiver).await {
            Ok(Ok(Ok(value))) => Ok(value),
            Ok(Ok(Err(error))) => Err(error.into()),
            Ok(Err(_)) => Err("native messaging host disconnected".into()),
            Err(_) => Err("native messaging request timed out".into()),
        }
    }
}

fn manifest_bytes(
    executable: &std::path::Path,
) -> Result<Vec<u8>, Box<dyn std::error::Error + Send + Sync + 'static>> {
    let path = executable
        .to_str()
        .ok_or("executable path is not valid UTF-8")?;
    Ok(serde_json::to_vec_pretty(&serde_json::json!({
        "name": HOST_NAME,
        "description": env!("CARGO_PKG_NAME"),
        "path": path,
        "type": "stdio",
        "allowed_origins": ALLOWED_ORIGINS,
    }))?)
}

fn write_manifest_if_changed(
    directory: &std::path::Path,
    manifest: &[u8],
) -> Result<std::path::PathBuf, Box<dyn std::error::Error + Send + Sync + 'static>> {
    std::fs::create_dir_all(directory)?;
    let path = directory.join(format!("{HOST_NAME}.json"));
    if std::fs::read(&path).ok().as_deref() != Some(manifest) {
        std::fs::write(&path, manifest)?;
    }
    Ok(path)
}

pub fn register_host(
    app_handle: &tauri::AppHandle,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    use tauri::Manager;

    #[cfg(target_os = "linux")]
    let executable = match app_handle.env().appimage {
        Some(appimage) => std::path::PathBuf::from(appimage),
        None => std::env::current_exe()?,
    };
    #[cfg(not(target_os = "linux"))]
    let executable = std::env::current_exe()?;

    let manifest = manifest_bytes(&executable)?;

    #[cfg(target_os = "linux")]
    {
        let mut directories = Vec::new();
        let mut last_error: Option<Box<dyn std::error::Error + Send + Sync + 'static>> = None;
        match app_handle.path().config_dir() {
            Ok(config_directory) => {
                directories.push(
                    config_directory
                        .join("google-chrome")
                        .join("NativeMessagingHosts"),
                );
            }
            Err(e) => last_error = Some(e.into()),
        }
        // files/home/kai/.config/openbox/menu.xml launches Chrome with --user-data-dir=/var/google-chrome
        let custom_user_data_directory = std::path::Path::new("/var/google-chrome");
        if custom_user_data_directory.is_dir() {
            directories.push(custom_user_data_directory.join("NativeMessagingHosts"));
        }
        for directory in directories {
            if let Err(e) = write_manifest_if_changed(&directory, &manifest) {
                last_error = Some(e);
            }
        }
        if let Some(error) = last_error {
            return Err(error);
        }
    }

    #[cfg(target_os = "macos")]
    {
        let directory = app_handle
            .path()
            .config_dir()?
            .join("Google")
            .join("Chrome")
            .join("NativeMessagingHosts");
        write_manifest_if_changed(&directory, &manifest)?;
    }

    #[cfg(target_os = "windows")]
    {
        use std::os::windows::process::CommandExt;

        let directory = app_handle.path().app_data_dir()?;
        let manifest_path = write_manifest_if_changed(&directory, &manifest)?;

        // https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
        const CREATE_NO_WINDOW: u32 = 0x0800_0000;
        let output = std::process::Command::new("reg")
            .args([
                "add",
                &format!("HKCU\\Software\\Google\\Chrome\\NativeMessagingHosts\\{HOST_NAME}"),
                "/ve",
                "/t",
                "REG_SZ",
                "/d",
            ])
            .arg(&manifest_path)
            .arg("/f")
            .creation_flags(CREATE_NO_WINDOW)
            .output()?;
        if !output.status.success() {
            return Err(format!(
                "reg add exited with {}: {}",
                output.status,
                String::from_utf8_lossy(&output.stderr)
            )
            .into());
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    struct EchoHandler;

    impl Handler for EchoHandler {
        fn handle(
            &self,
            message: serde_json::Value,
        ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync + 'static>> {
            Ok(message)
        }
    }

    #[test]
    fn test_is_native_messaging() {
        assert!(!is_native_messaging(Vec::<String>::new()));
        assert!(!is_native_messaging(vec!["--version".to_string()]));
        assert!(is_native_messaging(vec![
            "chrome-extension://phegfihdgknjjilfnlhljhkbljdbpogk/".to_string(),
        ]));
        assert!(is_native_messaging(vec![
            "--parent-window=12345".to_string(),
            "chrome-extension://phegfihdgknjjilfnlhljhkbljdbpogk/".to_string(),
        ]));
    }

    #[test]
    fn test_router_dispatch() {
        let router = Router::new()
            .register("echo", EchoHandler)
            .register("ping", |_message| Ok(serde_json::json!({ "pong": true })));

        assert_eq!(
            router
                .dispatch(serde_json::json!({ "type": "echo", "value": 42 }))
                .unwrap(),
            serde_json::json!({ "type": "echo", "value": 42 })
        );
        assert_eq!(
            router
                .dispatch(serde_json::json!({ "type": "ping" }))
                .unwrap(),
            serde_json::json!({ "pong": true })
        );
        assert!(router.dispatch(serde_json::json!({})).is_err());
        assert!(
            router
                .dispatch(serde_json::json!({ "type": "unknown" }))
                .is_err()
        );
    }

    #[test]
    fn test_serve_roundtrip() {
        let router = default_router().register("echo", EchoHandler);
        let mut input = Vec::new();
        write_message(
            &mut input,
            &serde_json::json!({
                "type": "create_github_repository",
                "fileName": "repos.csv",
                "records": [],
                "results": [],
                "csv": "repository,html_url,error"
            }),
        )
        .unwrap();
        write_message(
            &mut input,
            &serde_json::json!({ "type": "echo", "ok": true }),
        )
        .unwrap();

        let mut output = Vec::new();
        router
            .serve(&mut std::io::Cursor::new(input), &mut output, None)
            .unwrap();

        let mut cursor = std::io::Cursor::new(output);
        assert_eq!(
            read_message(&mut cursor).unwrap(),
            Some(serde_json::json!({}))
        );
        assert_eq!(
            read_message(&mut cursor).unwrap(),
            Some(serde_json::json!({ "type": "echo", "ok": true }))
        );
        assert_eq!(read_message(&mut cursor).unwrap(), None);
    }

    #[test]
    fn test_read_message_truncated_length() {
        let mut cursor = std::io::Cursor::new(vec![1u8, 0u8]);
        assert!(read_message(&mut cursor).is_err());
    }

    #[test]
    fn test_message_length_limits() {
        let oversized_length = (MAX_INCOMING_MESSAGE_LENGTH as u32 + 1).to_ne_bytes();
        let mut cursor = std::io::Cursor::new(oversized_length);
        assert!(read_message(&mut cursor).is_err());

        let mut output = Vec::new();
        let oversized_payload = serde_json::json!({
            "data": "a".repeat(MAX_OUTGOING_MESSAGE_LENGTH),
        });
        assert!(write_message(&mut output, &oversized_payload).is_err());
    }

    #[test]
    fn test_manifest_bytes() {
        let bytes = manifest_bytes(std::path::Path::new("/usr/local/bin/taurin")).unwrap();
        let manifest: serde_json::Value = serde_json::from_slice(&bytes).unwrap();
        assert_eq!(
            manifest,
            serde_json::json!({
                "name": "dev.kaidotio.taurin",
                "description": "taurin",
                "path": "/usr/local/bin/taurin",
                "type": "stdio",
                "allowed_origins": ["chrome-extension://phegfihdgknjjilfnlhljhkbljdbpogk/"],
            })
        );
    }

    fn unique_test_endpoint_path(name: &str) -> std::path::PathBuf {
        use base64::Engine;

        let mut suffix_bytes = [0u8; 8];
        getrandom::getrandom(&mut suffix_bytes).unwrap();
        let suffix = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(suffix_bytes);
        std::env::temp_dir().join(format!("taurin-native-messaging-{name}-{suffix}.json"))
    }

    #[tokio::test]
    async fn test_bridge_roundtrip() {
        let endpoint_path = unique_test_endpoint_path("roundtrip");
        let bridge = Bridge::start_at(endpoint_path.clone()).unwrap();

        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let mut extension_stream =
            std::net::TcpStream::connect(listener.local_addr().unwrap()).unwrap();
        let (mut host_reader, _) = listener.accept().unwrap();
        let mut host_writer = host_reader.try_clone().unwrap();
        let host_endpoint_path = endpoint_path.clone();
        let host_handle = std::thread::spawn(move || {
            default_router()
                .serve(
                    &mut host_reader,
                    &mut host_writer,
                    Some(&host_endpoint_path),
                )
                .unwrap();
        });

        write_message(
            &mut extension_stream,
            &serde_json::json!({ "type": "connect" }),
        )
        .unwrap();
        assert_eq!(
            read_message(&mut extension_stream).unwrap(),
            Some(serde_json::json!({}))
        );

        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(5);
        while bridge.inner.connections.lock().unwrap().is_empty() {
            assert!(std::time::Instant::now() < deadline);
            tokio::time::sleep(std::time::Duration::from_millis(10)).await;
        }

        let extension_thread = std::thread::spawn(move || {
            let first = read_message(&mut extension_stream).unwrap().unwrap();
            assert_eq!(first["type"], "echo");
            assert_eq!(first["value"], 42);
            write_message(
                &mut extension_stream,
                &serde_json::json!({
                    "id": first["id"],
                    "result": { "echoed": first["value"] },
                }),
            )
            .unwrap();

            let second = read_message(&mut extension_stream).unwrap().unwrap();
            assert_eq!(second["type"], "unknown");
            write_message(
                &mut extension_stream,
                &serde_json::json!({
                    "id": second["id"],
                    "error": "unknown message type: unknown",
                }),
            )
            .unwrap();

            let third = read_message(&mut extension_stream).unwrap().unwrap();
            assert_eq!(third["type"], "large_reply");
            write_message_with_limit(
                &mut extension_stream,
                &serde_json::json!({
                    "id": third["id"],
                    "result": { "data": "a".repeat(MAX_OUTGOING_MESSAGE_LENGTH + 1) },
                }),
                MAX_INCOMING_MESSAGE_LENGTH,
            )
            .unwrap();

            write_message(
                &mut extension_stream,
                &serde_json::json!({ "type": "create_github_repository" }),
            )
            .unwrap();
            assert_eq!(
                read_message(&mut extension_stream).unwrap(),
                Some(serde_json::json!({}))
            );
            let _ = extension_stream.shutdown(std::net::Shutdown::Both);
        });

        let echoed = bridge
            .send(
                serde_json::json!({ "type": "echo", "value": 42 }),
                std::time::Duration::from_secs(5),
            )
            .await
            .unwrap();
        assert_eq!(echoed, serde_json::json!({ "echoed": 42 }));

        let error = bridge
            .send(
                serde_json::json!({ "type": "unknown" }),
                std::time::Duration::from_secs(5),
            )
            .await
            .unwrap_err();
        assert_eq!(error.to_string(), "unknown message type: unknown");

        let large = bridge
            .send(
                serde_json::json!({ "type": "large_reply" }),
                std::time::Duration::from_secs(5),
            )
            .await
            .unwrap();
        assert_eq!(
            large["data"].as_str().unwrap().len(),
            MAX_OUTGOING_MESSAGE_LENGTH + 1
        );

        extension_thread.join().unwrap();
        host_handle.join().unwrap();
        drop(bridge);
        assert!(!endpoint_path.exists());
    }

    #[tokio::test]
    async fn test_bridge_auth_timeout_and_disconnect() {
        let endpoint_path = unique_test_endpoint_path("errors");
        let bridge = Bridge::start_at(endpoint_path.clone()).unwrap();

        let endpoint: BridgeEndpoint =
            serde_json::from_slice(&std::fs::read(&endpoint_path).unwrap()).unwrap();

        let mut rogue = std::net::TcpStream::connect(std::net::SocketAddr::from((
            [127, 0, 0, 1],
            endpoint.port,
        )))
        .unwrap();
        write_message(&mut rogue, &serde_json::json!({ "token": "wrong" })).unwrap();
        assert_eq!(read_message(&mut rogue).unwrap(), None);

        let no_host_error = bridge
            .send(
                serde_json::json!({ "type": "ping" }),
                std::time::Duration::from_millis(50),
            )
            .await
            .unwrap_err();
        assert_eq!(
            no_host_error.to_string(),
            "no native messaging host connected"
        );

        let mut silent_host = std::net::TcpStream::connect(std::net::SocketAddr::from((
            [127, 0, 0, 1],
            endpoint.port,
        )))
        .unwrap();
        write_message(
            &mut silent_host,
            &serde_json::json!({ "token": endpoint.token }),
        )
        .unwrap();
        assert_eq!(
            read_message(&mut silent_host).unwrap(),
            Some(serde_json::json!({}))
        );

        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(5);
        while bridge.inner.connections.lock().unwrap().is_empty() {
            assert!(std::time::Instant::now() < deadline);
            tokio::time::sleep(std::time::Duration::from_millis(10)).await;
        }

        let oversized_error = bridge
            .send(
                serde_json::json!({
                    "type": "ping",
                    "data": "a".repeat(MAX_OUTGOING_MESSAGE_LENGTH),
                }),
                std::time::Duration::from_millis(50),
            )
            .await
            .unwrap_err();
        assert!(
            oversized_error
                .to_string()
                .starts_with("outgoing message length exceeds limit:")
        );
        assert_eq!(bridge.inner.connections.lock().unwrap().len(), 1);

        let timeout_error = bridge
            .send(
                serde_json::json!({ "type": "ping" }),
                std::time::Duration::from_millis(50),
            )
            .await
            .unwrap_err();
        assert_eq!(
            timeout_error.to_string(),
            "native messaging request timed out"
        );
        assert!(bridge.inner.pending.lock().unwrap().is_empty());
        let _ = read_message(&mut silent_host).unwrap();

        let closer = std::thread::spawn(move || {
            let _ = read_message(&mut silent_host).unwrap();
            drop(silent_host);
        });

        let disconnect_error = bridge
            .send(
                serde_json::json!({ "type": "ping" }),
                std::time::Duration::from_secs(5),
            )
            .await
            .unwrap_err();
        closer.join().unwrap();
        assert_eq!(
            disconnect_error.to_string(),
            "native messaging host disconnected"
        );
        drop(bridge);
        assert!(!endpoint_path.exists());
    }
}
