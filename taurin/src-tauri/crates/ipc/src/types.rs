#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[ipc_macro::export("settings.ts")]
#[serde(untagged)]
pub enum Value {
    Bool(bool),
    String(String),
    #[cfg(not(any(target_os = "android", target_os = "ios")))]
    Shortcut(tauri_plugin_global_shortcut::Shortcut),
}

#[derive(Clone, Debug, serde::Serialize)]
#[ipc_macro::export("settings.ts")]
pub enum Kind {
    Switch,
    Input,
    Shortcut,
}

#[derive(Clone, Debug, serde::Serialize)]
#[ipc_macro::export("settings.ts")]
pub struct Setting {
    pub current_value: Value,
    pub kind: Kind,
    pub handler_command: String,
}

#[ipc_macro::export("settings.ts")]
pub type Settings = Vec<Setting>;

#[derive(Clone, Debug, serde::Serialize)]
#[ipc_macro::export("tmux_monitor.ts")]
pub struct TmuxWindow {
    pub session_name: String,
    pub window_index: String,
    pub silent: bool,
    pub title: String,
}

#[ipc_macro::export("tmux_monitor.ts")]
pub type TmuxWindows = Vec<TmuxWindow>;

#[derive(Clone, Debug, PartialEq, serde::Serialize)]
#[ipc_macro::export("create_github_repositories.ts")]
pub struct CsvTable {
    pub header: Vec<String>,
    pub records: Vec<Vec<String>>,
}

#[derive(Clone, Debug, serde::Serialize)]
#[ipc_macro::export("create_github_repositories.ts")]
#[serde(tag = "kind")]
pub enum RepositoryCreationEvent {
    Running { index: u64 },
    Created { index: u64, html_url: String },
    Failed { index: u64, message: String },
}
