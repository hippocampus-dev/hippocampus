use tauri::Manager;

struct Gemini<R: tauri::Runtime>(tauri::plugin::PluginHandle<R>);

#[derive(Debug, serde::Serialize, serde::Deserialize)]
pub struct CategorizeResponse {
    pub category: String,
}

#[derive(Debug, serde::Serialize, serde::Deserialize)]
pub struct ToolCall {
    pub tool: String,
}

#[derive(Debug, serde::Serialize, serde::Deserialize)]
pub struct ParseIntentResponse {
    pub tools: Vec<ToolCall>,
}

#[tauri::command]
async fn categorize<R: tauri::Runtime>(
    app: tauri::AppHandle<R>,
    content: String,
) -> Result<CategorizeResponse, String> {
    #[derive(serde::Serialize)]
    struct Request {
        content: String,
    }

    app.state::<Gemini<R>>()
        .0
        .run_mobile_plugin("categorize", Request { content })
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn parse_intent<R: tauri::Runtime>(
    app: tauri::AppHandle<R>,
    transcript: String,
) -> Result<ParseIntentResponse, String> {
    #[derive(serde::Serialize)]
    struct Request {
        transcript: String,
    }

    app.state::<Gemini<R>>()
        .0
        .run_mobile_plugin("parseIntent", Request { transcript })
        .map_err(|e| e.to_string())
}

pub fn init<R: tauri::Runtime>() -> tauri::plugin::TauriPlugin<R> {
    tauri::plugin::Builder::new("gemini")
        .invoke_handler(tauri::generate_handler![categorize, parse_intent])
        .setup(|app, api| {
            let handle = api
                .register_android_plugin("com.plugin.gemini", "GeminiPlugin")
                .expect("failed to register gemini plugin");
            app.manage(Gemini(handle));
            Ok(())
        })
        .build()
}
