package com.plugin.autoupdate

import android.app.Activity
import android.app.ActivityManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageInstaller
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import android.util.Log
import androidx.core.content.ContextCompat
import androidx.core.content.IntentCompat
import app.tauri.annotation.Command
import app.tauri.annotation.InvokeArg
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Invoke
import app.tauri.plugin.JSObject
import app.tauri.plugin.Plugin
import java.io.File
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch

@InvokeArg
class AuthorizeArgs {
    var url: String = ""
}

@InvokeArg
class InstallUpdateArgs {
    var path: String = ""
}

@TauriPlugin
class AutoUpdatePlugin(private val activity: Activity) : Plugin(activity) {
    private val supervisorJob = SupervisorJob()
    private val scope = CoroutineScope(Dispatchers.IO + supervisorJob)
    private val handler = Handler(Looper.getMainLooper())

    private var pendingAuthorization: Invoke? = null

    private var receiverRegistered: Boolean = false

    private var unknownAppSourcesRequested: Boolean = false

    private val installReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            when (val status = intent.getIntExtra(PackageInstaller.EXTRA_STATUS, PackageInstaller.STATUS_FAILURE)) {
                PackageInstaller.STATUS_PENDING_USER_ACTION -> {
                    Log.d("AutoUpdatePlugin", "Install needs user action")
                    IntentCompat.getParcelableExtra(intent, Intent.EXTRA_INTENT, Intent::class.java)?.let {
                        activity.startActivity(it)
                    }
                }
                PackageInstaller.STATUS_SUCCESS -> Log.d("AutoUpdatePlugin", "Update installed")
                else -> Log.e(
                    "AutoUpdatePlugin",
                    "Install failed: status=$status message=${intent.getStringExtra(PackageInstaller.EXTRA_STATUS_MESSAGE)}"
                )
            }
        }
    }

    override fun load(webView: android.webkit.WebView) {
        super.load(webView)

        ContextCompat.registerReceiver(
            activity,
            installReceiver,
            IntentFilter(ACTION_INSTALL_STATUS),
            ContextCompat.RECEIVER_NOT_EXPORTED
        )
        receiverRegistered = true
    }

    override fun onDestroy() {
        if (receiverRegistered) {
            activity.unregisterReceiver(installReceiver)
        }
        handler.removeCallbacksAndMessages(null)
        supervisorJob.cancel()
        super.onDestroy()
    }

    override fun onNewIntent(intent: Intent) {
        val data = intent.data ?: return
        if (intent.action != Intent.ACTION_VIEW || data.host != CALLBACK_HOST || data.path != CALLBACK_PATH) {
            return
        }
        val invoke = pendingAuthorization ?: return
        pendingAuthorization = null
        handler.removeCallbacksAndMessages(null)
        Log.d("AutoUpdatePlugin", "Received sign-in callback")
        val result = JSObject()
        data.queryParameterNames.forEach { result.put(it, data.getQueryParameter(it)) }
        invoke.resolve(result)
    }

    @Command
    fun authorize(invoke: Invoke) {
        val url = invoke.parseArgs(AuthorizeArgs::class.java).url
        pendingAuthorization = invoke
        handler.postDelayed({
            pendingAuthorization?.reject("No sign-in callback within ${AUTHORIZE_TIMEOUT_MILLISECONDS}ms")
            pendingAuthorization = null
        }, AUTHORIZE_TIMEOUT_MILLISECONDS)
        activity.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)))
    }

    @Command
    fun installUpdate(invoke: Invoke) {
        val apk = File(invoke.parseArgs(InstallUpdateArgs::class.java).path)

        @Suppress("DEPRECATION")
        val timerRunning = activity.getSystemService(ActivityManager::class.java)
            .getRunningServices(Int.MAX_VALUE)
            .any { it.service.className == TIMER_SERVICE_CLASS_NAME }
        if (timerRunning) {
            invoke.reject("$TIMER_SERVICE_CLASS_NAME is running")
            return
        }
        if (!activity.packageManager.canRequestPackageInstalls()) {
            if (!unknownAppSourcesRequested) {
                unknownAppSourcesRequested = true
                activity.startActivity(
                    Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:${activity.packageName}"))
                )
            }
            invoke.reject("Installing unknown apps is not allowed for ${activity.packageName}")
            return
        }

        scope.launch {
            val installer = activity.packageManager.packageInstaller
            var sessionId: Int? = null
            try {
                val params = PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL).apply {
                    setAppPackageName(activity.packageName)
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                        setRequireUserAction(PackageInstaller.SessionParams.USER_ACTION_NOT_REQUIRED)
                    }
                }
                sessionId = installer.createSession(params)
                installer.openSession(sessionId).use { session ->
                    session.openWrite(apk.name, 0, apk.length()).use { output ->
                        apk.inputStream().use { it.copyTo(output) }
                        session.fsync(output)
                    }
                    val intent = Intent(ACTION_INSTALL_STATUS).setPackage(activity.packageName)
                    val pendingIntent = PendingIntent.getBroadcast(
                        activity,
                        sessionId,
                        intent,
                        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_MUTABLE
                    )
                    Log.d("AutoUpdatePlugin", "Committing install session $sessionId")
                    session.commit(pendingIntent.intentSender)
                }
                invoke.resolve()
            } catch (e: Exception) {
                Log.e("AutoUpdatePlugin", "Failed to install update", e)
                sessionId?.let { installer.abandonSession(it) }
                invoke.reject(e.message ?: "Failed to install update")
            }
        }
    }

    companion object {
        const val ACTION_INSTALL_STATUS = "com.plugin.autoupdate.INSTALL_STATUS"
        const val TIMER_SERVICE_CLASS_NAME = "com.plugin.timer.TimerService"
        const val CALLBACK_HOST = "tauri-releases-public.kaidotio.dev"
        const val CALLBACK_PATH = "/taurim/callback"
        const val AUTHORIZE_TIMEOUT_MILLISECONDS = 600_000L
    }
}
