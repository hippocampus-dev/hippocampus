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

@InvokeArg
class CategorizeArgs {
    lateinit var content: String
}

@InvokeArg
class ParseIntentArgs {
    lateinit var transcript: String
    var availableGroups: List<String> = emptyList()
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

                val groupsList = if (args.availableGroups.isEmpty()) {
                    "(none)"
                } else {
                    args.availableGroups.joinToString(", ") { "\"$it\"" }
                }

                val prompt = """
You are a timer app voice assistant. Analyze user input and return JSON with tool calls.

Available tools:
- start_timer(): Start the timer
- pause_timer(): Pause or stop the timer
- reset_timer(): Reset all timers
- next_timer(): Skip to next timer
- load_group(group_name): Load a timer group. Groups: [$groupsList]

Rules:
- Return {"tools": [...]} array with one or more tool calls
- Multiple tools can be called in sequence (e.g., load then start)
- If not a timer command, return {"tools": []}
- For load_group, use exact group name from list

Examples:
User: "start"
{"tools": [{"tool": "start_timer"}]}

User: "stop"
{"tools": [{"tool": "pause_timer"}]}

User: "use Morning Routine"
{"tools": [{"tool": "load_group", "parameters": {"group_name": "Morning Routine"}}]}

User: "start Morning Routine"
{"tools": [{"tool": "load_group", "parameters": {"group_name": "Morning Routine"}}, {"tool": "start_timer"}]}

User: "hello"
{"tools": []}

User: "${args.transcript}"
""".trimIndent()

                Log.d("GeminiPlugin", "parseIntent called with transcript length=${args.transcript.length}")

                val rawResponse = model.generateContent(prompt)
                    .candidates.firstOrNull()?.text?.trim() ?: "{\"tools\": []}"

                Log.d("GeminiPlugin", "parseIntent response length=${rawResponse.length}")

                val jsonString = extractJson(rawResponse)
                val result = parseToolsResponse(jsonString, args.availableGroups)
                invoke.resolve(result)
            } catch (e: Exception) {
                Log.e("GeminiPlugin", "parseIntent failed", e)
                invoke.reject(e.message ?: "Intent parsing failed")
            }
        }
    }

    private fun extractJson(response: String): String {
        var json = response.trim()
        if (json.startsWith("```json")) {
            json = json.removePrefix("```json")
        }
        if (json.startsWith("```")) {
            json = json.removePrefix("```")
        }
        if (json.endsWith("```")) {
            json = json.removeSuffix("```")
        }
        return json.trim()
    }

    private fun parseToolsResponse(jsonString: String, availableGroups: List<String>): JSObject {
        val result = JSObject()
        val toolsArray = org.json.JSONArray()
        try {
            val json = org.json.JSONObject(jsonString)
            val tools = json.optJSONArray("tools") ?: org.json.JSONArray()

            for (i in 0 until tools.length()) {
                val toolObj = tools.getJSONObject(i)
                val tool = toolObj.optString("tool", null)
                if (tool.isNullOrBlank()) continue

                val parsedTool = JSObject()
                parsedTool.put("tool", tool)

                when (tool) {
                    "load_group" -> {
                        val parameters = toolObj.optJSONObject("parameters")
                        val groupName = parameters?.optString("group_name", null)
                        val paramsObj = JSObject()
                        if (groupName.isNullOrBlank()) {
                            paramsObj.put("group_name", null as String?)
                        } else {
                            val matchedGroup = availableGroups.find {
                                it.equals(groupName, ignoreCase = true)
                            } ?: availableGroups.find {
                                it.contains(groupName, ignoreCase = true)
                            }
                            paramsObj.put("group_name", matchedGroup)
                        }
                        parsedTool.put("parameters", paramsObj)
                    }
                    else -> {
                        parsedTool.put("parameters", JSObject())
                    }
                }
                toolsArray.put(parsedTool)
            }
        } catch (e: Exception) {
            Log.w("GeminiPlugin", "JSON parse failed: ${e.message}")
        }
        result.put("tools", toolsArray)
        return result
    }
}
