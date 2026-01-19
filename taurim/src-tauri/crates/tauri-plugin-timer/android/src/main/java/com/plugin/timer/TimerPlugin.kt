package com.plugin.timer

import android.app.Activity
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.os.Build
import android.util.Log
import app.tauri.annotation.Command
import app.tauri.annotation.InvokeArg
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Invoke
import app.tauri.plugin.JSObject
import app.tauri.plugin.Plugin
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch

@InvokeArg
class StartTimerArgs {
    var duration_seconds: Int = 0
}

@TauriPlugin
class TimerPlugin(private val activity: Activity) : Plugin(activity) {
    private val scope = CoroutineScope(Dispatchers.Main + SupervisorJob())

    @Volatile
    private var remainingSeconds: Int = 0

    @Volatile
    private var isRunning: Boolean = false

    private val timerReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            when (intent.action) {
                TimerService.ACTION_TICK -> {
                    remainingSeconds = intent.getIntExtra(TimerService.EXTRA_REMAINING, 0)
                    isRunning = true
                    Log.d("TimerPlugin", "Tick: $remainingSeconds seconds remaining")
                }
                TimerService.ACTION_COMPLETE -> {
                    remainingSeconds = 0
                    isRunning = false
                    Log.d("TimerPlugin", "Timer completed")
                }
            }
        }
    }

    override fun load(webView: android.webkit.WebView) {
        super.load(webView)

        val filter = IntentFilter().apply {
            addAction(TimerService.ACTION_TICK)
            addAction(TimerService.ACTION_COMPLETE)
        }

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            activity.registerReceiver(timerReceiver, filter, Context.RECEIVER_NOT_EXPORTED)
        } else {
            @Suppress("UnspecifiedRegisterReceiverFlag")
            activity.registerReceiver(timerReceiver, filter)
        }
    }

    @Command
    fun startTimer(invoke: Invoke) {
        val args = invoke.parseArgs(StartTimerArgs::class.java)
        val durationSeconds = args.duration_seconds

        Log.d("TimerPlugin", "Starting timer for $durationSeconds seconds")

        scope.launch {
            try {
                remainingSeconds = durationSeconds
                isRunning = true

                val intent = Intent(activity, TimerService::class.java).apply {
                    action = TimerService.ACTION_START
                    putExtra(TimerService.EXTRA_DURATION, durationSeconds)
                }

                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                    activity.startForegroundService(intent)
                } else {
                    activity.startService(intent)
                }

                invoke.resolve()
            } catch (e: Exception) {
                Log.e("TimerPlugin", "Failed to start timer", e)
                invoke.reject(e.message ?: "Failed to start timer")
            }
        }
    }

    @Command
    fun stopTimer(invoke: Invoke) {
        Log.d("TimerPlugin", "Stopping timer")

        scope.launch {
            try {
                isRunning = false

                val intent = Intent(activity, TimerService::class.java).apply {
                    action = TimerService.ACTION_STOP
                }
                activity.stopService(intent)

                invoke.resolve()
            } catch (e: Exception) {
                Log.e("TimerPlugin", "Failed to stop timer", e)
                invoke.reject(e.message ?: "Failed to stop timer")
            }
        }
    }

    @Command
    fun getRemaining(invoke: Invoke) {
        val result = JSObject()
        result.put("remaining_seconds", remainingSeconds)
        result.put("is_running", isRunning)
        invoke.resolve(result)
    }
}
