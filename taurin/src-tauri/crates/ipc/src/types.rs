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
