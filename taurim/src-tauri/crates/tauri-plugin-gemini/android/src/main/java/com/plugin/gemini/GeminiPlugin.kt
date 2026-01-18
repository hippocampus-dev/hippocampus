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

@TauriPlugin
class GeminiPlugin(private val activity: Activity) : Plugin(activity) {
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())
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
}
