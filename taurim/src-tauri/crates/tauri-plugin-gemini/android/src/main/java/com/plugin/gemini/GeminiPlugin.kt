package com.plugin.gemini

import android.app.Activity
import android.util.Log
import app.tauri.annotation.Command
import app.tauri.annotation.InvokeArg
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Invoke
import app.tauri.plugin.JSObject
import app.tauri.plugin.Plugin
import com.google.mlkit.genai.common.DownloadStatus
import com.google.mlkit.genai.common.FeatureStatus
import com.google.mlkit.genai.prompt.Generation
import com.google.mlkit.genai.prompt.GenerativeModel
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import org.json.JSONArray
import org.json.JSONObject

@InvokeArg
class CategorizeArgs {
    lateinit var content: String
}

@InvokeArg
class ParseIntentArgs {
    lateinit var transcript: String
}

@TauriPlugin
class GeminiPlugin(private val activity: Activity) : Plugin(activity) {
    private val supervisorJob = SupervisorJob()
    private val scope = CoroutineScope(Dispatchers.IO + supervisorJob)
    @Volatile private var generativeModel: GenerativeModel? = null
    @Volatile private var modelReady = false

    override fun load(webView: android.webkit.WebView) {
        super.load(webView)
        scope.launch {
            try {
                generativeModel = Generation.getClient()
                when (generativeModel?.checkStatus()) {
                    FeatureStatus.DOWNLOADABLE -> {
                        generativeModel?.download()?.collect { status ->
                            if (status == DownloadStatus.DownloadCompleted) modelReady = true
                        }
                    }
                    FeatureStatus.AVAILABLE -> modelReady = true
                    else -> {}
                }
            } catch (e: Exception) {
                Log.e("GeminiPlugin", "Init failed", e)
            }
        }
    }

    override fun onDestroy() {
        supervisorJob.cancel()
        super.onDestroy()
    }

    @Command
    fun categorize(invoke: Invoke) {
        val args = invoke.parseArgs(CategorizeArgs::class.java)

        scope.launch {
            try {
                val model = generativeModel
                if (model == null || !modelReady) {
                    invoke.reject("Model not ready")
                    return@launch
                }

                val prompt = """
                    Categorize into one of: Work, Personal, Shopping, Ideas, Tasks, Notes, Other.
                    Reply with ONLY the category name.

                    Content: ${args.content}
                """.trimIndent()

                val category = model.generateContent(prompt)
                    .candidates.firstOrNull()?.text?.trim() ?: "Other"

                val validCategories = listOf("Work", "Personal", "Shopping", "Ideas", "Tasks", "Notes", "Other")
                val result = JSObject()
                result.put("category", validCategories.find { it.equals(category, ignoreCase = true) } ?: "Other")
                invoke.resolve(result)
            } catch (e: Exception) {
                invoke.reject(e.message ?: "Categorization failed")
            }
        }
    }

    @Command
    fun parseIntent(invoke: Invoke) {
        val args = invoke.parseArgs(ParseIntentArgs::class.java)

        scope.launch {
            try {
                val model = generativeModel
                if (model == null || !modelReady) {
                    invoke.reject("Model not ready")
                    return@launch
                }

                val prompt = """
You are a timer app voice assistant listening while the timer alarm rings. Decide whether the user asks to stop the alarm and return JSON with tool calls.

Available tools:
- stop_alarm(): Stop the ringing alarm

Rules:
- Return {"tools": [...]} array with the tool call
- If the user does not ask to stop the alarm, return {"tools": []}

Examples:
User: "止めて"
{"tools": [{"tool": "stop_alarm"}]}

User: "うるさい、もういいよ"
{"tools": [{"tool": "stop_alarm"}]}

User: "stop"
{"tools": [{"tool": "stop_alarm"}]}

User: "止めないで"
{"tools": []}

User: "hello"
{"tools": []}

User: "${args.transcript}"
""".trimIndent()

                Log.d("GeminiPlugin", "parseIntent called with transcript length=${args.transcript.length}")

                val rawResponse = model.generateContent(prompt)
                    .candidates.firstOrNull()?.text?.trim() ?: "{\"tools\": []}"

                Log.d("GeminiPlugin", "parseIntent response length=${rawResponse.length}")

                var jsonString = rawResponse.trim()
                if (jsonString.startsWith("```json")) {
                    jsonString = jsonString.removePrefix("```json")
                }
                if (jsonString.startsWith("```")) {
                    jsonString = jsonString.removePrefix("```")
                }
                if (jsonString.endsWith("```")) {
                    jsonString = jsonString.removeSuffix("```")
                }
                jsonString = jsonString.trim()

                val result = JSObject()
                val toolsArray = JSONArray()
                try {
                    val json = JSONObject(jsonString)
                    val tools = json.optJSONArray("tools") ?: JSONArray()

                    for (i in 0 until tools.length()) {
                        val toolObj = tools.getJSONObject(i)
                        val tool = toolObj.optString("tool", null)
                        if (tool.isNullOrBlank()) continue

                        val parsedTool = JSObject()
                        parsedTool.put("tool", tool)
                        toolsArray.put(parsedTool)
                    }
                } catch (e: Exception) {
                    Log.w("GeminiPlugin", "JSON parse failed: ${e.message}")
                }
                result.put("tools", toolsArray)
                invoke.resolve(result)
            } catch (e: Exception) {
                Log.e("GeminiPlugin", "parseIntent failed", e)
                invoke.reject(e.message ?: "Intent parsing failed")
            }
        }
    }
}
