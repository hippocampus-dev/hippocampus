package com.plugin.speech

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.speech.RecognitionListener
import android.speech.RecognizerIntent
import android.speech.SpeechRecognizer
import android.util.Log
import app.tauri.annotation.Command
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Invoke
import app.tauri.plugin.JSObject
import app.tauri.plugin.Plugin

@TauriPlugin
class SpeechPlugin(private val activity: Activity) : Plugin(activity) {
    private var recognizer: SpeechRecognizer? = null
    private var audioManager: AudioManager? = null
    private var focusRequest: AudioFocusRequest? = null
    private var sessionToken: Long = 0
    private var backoffMs: Long = 100
    private var isListeningActive = false
    private val mainHandler = Handler(Looper.getMainLooper())

    override fun load(webView: android.webkit.WebView) {
        super.load(webView)
        audioManager = activity.getSystemService(Context.AUDIO_SERVICE) as AudioManager
        Log.d("SpeechPlugin", "Plugin loaded")
    }

    override fun onDestroy() {
        mainHandler.removeCallbacksAndMessages(null)
        recognizer?.destroy()
        recognizer = null
        abandonAudioFocus()
        super.onDestroy()
    }

    @Command
    fun startListening(invoke: Invoke) {
        mainHandler.post {
            try {
                if (!SpeechRecognizer.isRecognitionAvailable(activity)) {
                    Log.e("SpeechPlugin", "Speech recognition not available")
                    invoke.reject("Speech recognition not available on this device")
                    return@post
                }

                sessionToken++
                isListeningActive = true
                backoffMs = 100

                requestAudioFocus()
                initRecognizer()
                recognizer?.startListening(createRecognizerIntent())

                emitState("listening")
                Log.d("SpeechPlugin", "Started listening (session=$sessionToken)")
                invoke.resolve()
            } catch (e: Exception) {
                Log.e("SpeechPlugin", "Failed to start listening", e)
                invoke.reject(e.message ?: "Failed to start listening")
            }
        }
    }

    @Command
    fun stopListening(invoke: Invoke) {
        mainHandler.post {
            try {
                sessionToken++
                isListeningActive = false

                recognizer?.stopListening()
                recognizer?.destroy()
                recognizer = null

                abandonAudioFocus()

                emitState("idle")
                Log.d("SpeechPlugin", "Stopped listening")
                invoke.resolve()
            } catch (e: Exception) {
                Log.e("SpeechPlugin", "Failed to stop listening", e)
                invoke.reject(e.message ?: "Failed to stop listening")
            }
        }
    }

    private fun initRecognizer() {
        recognizer?.destroy()
        recognizer = SpeechRecognizer.createSpeechRecognizer(activity).apply {
            setRecognitionListener(createListener())
        }
    }

    private fun createRecognizerIntent(): Intent {
        return Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
            putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
            putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true)
            putExtra(RecognizerIntent.EXTRA_MAX_RESULTS, 1)
        }
    }

    private fun createListener(): RecognitionListener {
        val capturedToken = sessionToken
        return object : RecognitionListener {
            override fun onReadyForSpeech(params: Bundle?) {
                Log.d("SpeechPlugin", "Ready for speech")
            }

            override fun onBeginningOfSpeech() {
                Log.d("SpeechPlugin", "Speech started")
            }

            override fun onRmsChanged(rmsdB: Float) {}

            override fun onBufferReceived(buffer: ByteArray?) {}

            override fun onEndOfSpeech() {
                Log.d("SpeechPlugin", "Speech ended")
            }

            override fun onError(error: Int) {
                if (capturedToken != sessionToken) {
                    Log.d("SpeechPlugin", "Ignoring stale error callback")
                    return
                }

                val errorName = getErrorName(error)
                Log.d("SpeechPlugin", "Recognition error: $errorName ($error)")

                when (error) {
                    SpeechRecognizer.ERROR_NO_MATCH,
                    SpeechRecognizer.ERROR_SPEECH_TIMEOUT -> {
                        scheduleRestart(capturedToken)
                    }
                    SpeechRecognizer.ERROR_RECOGNIZER_BUSY,
                    SpeechRecognizer.ERROR_CLIENT -> {
                        scheduleRestartWithBackoff(capturedToken)
                    }
                    SpeechRecognizer.ERROR_NETWORK,
                    SpeechRecognizer.ERROR_NETWORK_TIMEOUT -> {
                        emitError("NETWORK_ERROR", "Network unavailable for speech recognition")
                        scheduleRestartWithBackoff(capturedToken)
                    }
                    SpeechRecognizer.ERROR_INSUFFICIENT_PERMISSIONS -> {
                        emitError("PERMISSION_DENIED", "Microphone permission denied")
                        emitState("error")
                    }
                    SpeechRecognizer.ERROR_AUDIO -> {
                        scheduleRestartWithBackoff(capturedToken)
                    }
                    else -> {
                        scheduleRestartWithBackoff(capturedToken)
                    }
                }
            }

            override fun onResults(results: Bundle?) {
                if (capturedToken != sessionToken) {
                    Log.d("SpeechPlugin", "Ignoring stale results callback")
                    return
                }

                val matches = results?.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION)
                val transcript = matches?.firstOrNull() ?: ""

                if (transcript.isNotEmpty()) {
                    Log.d("SpeechPlugin", "Final result received (length=${transcript.length})")
                    emitResult(transcript, true)
                }

                scheduleRestart(capturedToken)
            }

            override fun onPartialResults(partialResults: Bundle?) {
                if (capturedToken != sessionToken) return

                val matches = partialResults?.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION)
                val transcript = matches?.firstOrNull() ?: ""

                if (transcript.isNotEmpty()) {
                    Log.d("SpeechPlugin", "Partial result received (length=${transcript.length})")
                    emitResult(transcript, false)
                }
            }

            override fun onEvent(eventType: Int, params: Bundle?) {}
        }
    }

    private fun scheduleRestart(token: Long) {
        mainHandler.postDelayed({
            if (token == sessionToken && isListeningActive) {
                backoffMs = 100
                initRecognizer()
                recognizer?.startListening(createRecognizerIntent())
                Log.d("SpeechPlugin", "Restarted listening")
            }
        }, 200)
    }

    private fun scheduleRestartWithBackoff(token: Long) {
        mainHandler.postDelayed({
            if (token == sessionToken && isListeningActive) {
                initRecognizer()
                recognizer?.startListening(createRecognizerIntent())
                backoffMs = minOf(backoffMs * 2, 5000)
                Log.d("SpeechPlugin", "Restarted listening with backoff=${backoffMs}ms")
            }
        }, backoffMs)
    }

    private fun requestAudioFocus() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            focusRequest = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN_TRANSIENT)
                .setOnAudioFocusChangeListener { focusChange ->
                    when (focusChange) {
                        AudioManager.AUDIOFOCUS_LOSS,
                        AudioManager.AUDIOFOCUS_LOSS_TRANSIENT -> {
                            Log.d("SpeechPlugin", "Audio focus lost")
                        }
                    }
                }
                .build()
            audioManager?.requestAudioFocus(focusRequest!!)
        } else {
            @Suppress("DEPRECATION")
            audioManager?.requestAudioFocus(
                { focusChange ->
                    when (focusChange) {
                        AudioManager.AUDIOFOCUS_LOSS,
                        AudioManager.AUDIOFOCUS_LOSS_TRANSIENT -> {
                            Log.d("SpeechPlugin", "Audio focus lost")
                        }
                    }
                },
                AudioManager.STREAM_MUSIC,
                AudioManager.AUDIOFOCUS_GAIN_TRANSIENT
            )
        }
    }

    private fun abandonAudioFocus() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            focusRequest?.let { audioManager?.abandonAudioFocusRequest(it) }
        } else {
            @Suppress("DEPRECATION")
            audioManager?.abandonAudioFocus(null)
        }
    }

    private fun emitResult(transcript: String, isFinal: Boolean) {
        val payload = JSObject()
        payload.put("transcript", transcript)
        payload.put("isFinal", isFinal)
        trigger("result", payload)
    }

    private fun emitError(code: String, message: String) {
        val payload = JSObject()
        payload.put("code", code)
        payload.put("message", message)
        trigger("error", payload)
    }

    private fun emitState(state: String) {
        val payload = JSObject()
        payload.put("state", state)
        trigger("state", payload)
    }

    private fun getErrorName(error: Int): String {
        return when (error) {
            SpeechRecognizer.ERROR_AUDIO -> "ERROR_AUDIO"
            SpeechRecognizer.ERROR_CLIENT -> "ERROR_CLIENT"
            SpeechRecognizer.ERROR_INSUFFICIENT_PERMISSIONS -> "ERROR_INSUFFICIENT_PERMISSIONS"
            SpeechRecognizer.ERROR_NETWORK -> "ERROR_NETWORK"
            SpeechRecognizer.ERROR_NETWORK_TIMEOUT -> "ERROR_NETWORK_TIMEOUT"
            SpeechRecognizer.ERROR_NO_MATCH -> "ERROR_NO_MATCH"
            SpeechRecognizer.ERROR_RECOGNIZER_BUSY -> "ERROR_RECOGNIZER_BUSY"
            SpeechRecognizer.ERROR_SERVER -> "ERROR_SERVER"
            SpeechRecognizer.ERROR_SPEECH_TIMEOUT -> "ERROR_SPEECH_TIMEOUT"
            else -> "UNKNOWN_ERROR"
        }
    }
}
