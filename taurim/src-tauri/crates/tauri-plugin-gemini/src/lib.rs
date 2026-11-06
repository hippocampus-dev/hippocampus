use serde::{Deserialize, Serialize};
use tauri::{
    plugin::{Builder, PluginHandle, TauriPlugin},
    AppHandle, Manager, Runtime,
};

#[derive(Debug, Serialize, Deserialize)]
pub struct CategorizeResponse {
    pub category: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ParseIntentResponse {
    pub command: String,
    pub group_name: Option<String>,
}

#[tauri::command]
async fn categorize<R: Runtime>(
    app: AppHandle<R>,
    content: String,
) -> Result<CategorizeResponse, String> {
    #[derive(Serialize)]
    struct Request {
        content: String,
    }

    app.state::<PluginHandle<R>>()
        .run_mobile_plugin("categorize", Request { content })
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn parse_intent<R: Runtime>(
    app: AppHandle<R>,
    transcript: String,
    available_groups: Vec<String>,
) -> Result<ParseIntentResponse, String> {
    #[derive(Serialize)]
    struct Request {
        transcript: String,
        available_groups: Vec<String>,
    }

    app.state::<PluginHandle<R>>()
        .run_mobile_plugin(
            "parseIntent",
            Request {
                transcript,
                available_groups,
            },
        )
        .map_err(|e| e.to_string())
}

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("gemini")
        .invoke_handler(tauri::generate_handler![categorize, parse_intent])
        .setup(|app, api| {
            let handle = api
                .register_android_plugin("com.plugin.gemini", "GeminiPlugin")
                .expect("failed to register gemini plugin");
            app.manage(handle);
            Ok(())
        })
        .build()
}
