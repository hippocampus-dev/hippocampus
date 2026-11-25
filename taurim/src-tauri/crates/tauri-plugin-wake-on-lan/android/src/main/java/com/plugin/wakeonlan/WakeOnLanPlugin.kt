package com.plugin.wakeonlan

import android.app.Activity
import android.os.Build
import android.util.Log
import app.tauri.PermissionState
import app.tauri.annotation.Command
import app.tauri.annotation.InvokeArg
import app.tauri.annotation.Permission
import app.tauri.annotation.PermissionCallback
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Invoke
import app.tauri.plugin.JSObject
import app.tauri.plugin.Plugin
import java.time.LocalTime
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch

@InvokeArg
class SetScheduleArgs {
    var mac_address: String = ""
    var hour: Int = 0
    var minute: Int = 0
    var enabled: Boolean = false
}

@InvokeArg
class WakeArgs {
    var mac_address: String = ""
}

// ACCESS_LOCAL_NETWORK and its enforcement arrive in API level 37, above this plugin's compileSdk: https://developer.android.com/privacy-and-security/local-network-permission
@TauriPlugin(
    permissions = [Permission(strings = ["android.permission.ACCESS_LOCAL_NETWORK"], alias = "localNetwork")]
)
class WakeOnLanPlugin(private val activity: Activity) : Plugin(activity) {
    private val supervisorJob = SupervisorJob()
    private val scope = CoroutineScope(Dispatchers.IO + supervisorJob)

    override fun load(webView: android.webkit.WebView) {
        super.load(webView)
        // A force stop cancels the alarm without a broadcast before Android 15: https://developer.android.com/about/versions/15/behavior-changes-all#enhanced-stop-states
        WakeOnLan.schedule(activity)
    }

    override fun onDestroy() {
        supervisorJob.cancel()
        super.onDestroy()
    }

    private fun lacksLocalNetworkPermission(): Boolean =
        Build.VERSION.SDK_INT >= 37 && getPermissionState("localNetwork") != PermissionState.GRANTED

    @Command
    fun getSchedule(invoke: Invoke) {
        val schedule = WakeOnLan.load(activity)
        val result = JSObject()
        result.put("mac_address", schedule.macAddress)
        result.put("hour", schedule.time.hour)
        result.put("minute", schedule.time.minute)
        result.put("enabled", schedule.enabled)
        invoke.resolve(result)
    }

    @Command
    fun setSchedule(invoke: Invoke) {
        val args = invoke.parseArgs(SetScheduleArgs::class.java)
        if (args.enabled && lacksLocalNetworkPermission()) {
            requestPermissionForAlias("localNetwork", invoke, "setSchedulePermissionCallback")
            return
        }
        saveSchedule(invoke, args)
    }

    @PermissionCallback
    private fun setSchedulePermissionCallback(invoke: Invoke) {
        if (lacksLocalNetworkPermission()) {
            Log.e("WakeOnLanPlugin", "Local network permission denied")
            invoke.reject("Local network permission denied")
            return
        }
        saveSchedule(invoke, invoke.parseArgs(SetScheduleArgs::class.java))
    }

    private fun saveSchedule(invoke: Invoke, args: SetScheduleArgs) {
        Log.d("WakeOnLanPlugin", "Setting schedule enabled=${args.enabled}")

        try {
            WakeOnLan.parseMacAddress(args.mac_address)
            WakeOnLan.save(activity, Schedule(args.mac_address, LocalTime.of(args.hour, args.minute), args.enabled))
            WakeOnLan.schedule(activity)
            invoke.resolve()
        } catch (e: Exception) {
            Log.e("WakeOnLanPlugin", "Failed to set schedule", e)
            invoke.reject(e.message ?: "Failed to set schedule")
        }
    }

    @Command
    fun wake(invoke: Invoke) {
        if (lacksLocalNetworkPermission()) {
            requestPermissionForAlias("localNetwork", invoke, "wakePermissionCallback")
            return
        }
        send(invoke)
    }

    @PermissionCallback
    private fun wakePermissionCallback(invoke: Invoke) {
        if (lacksLocalNetworkPermission()) {
            Log.e("WakeOnLanPlugin", "Local network permission denied")
            invoke.reject("Local network permission denied")
            return
        }
        send(invoke)
    }

    private fun send(invoke: Invoke) {
        val args = invoke.parseArgs(WakeArgs::class.java)

        Log.d("WakeOnLanPlugin", "Sending magic packet")

        scope.launch {
            try {
                WakeOnLan.send(args.mac_address)
                invoke.resolve()
            } catch (e: Exception) {
                Log.e("WakeOnLanPlugin", "Failed to send magic packet", e)
                invoke.reject(e.message ?: "Failed to send magic packet")
            }
        }
    }
}
