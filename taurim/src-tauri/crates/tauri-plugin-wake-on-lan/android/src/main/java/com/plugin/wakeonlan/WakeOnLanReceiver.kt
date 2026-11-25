package com.plugin.wakeonlan

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

class WakeOnLanReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        when (intent.action) {
            WakeOnLan.ACTION_WAKE -> {
                WakeOnLan.schedule(context)
                val schedule = WakeOnLan.load(context)
                if (!schedule.enabled) {
                    return
                }
                val pendingResult = goAsync()
                CoroutineScope(Dispatchers.IO).launch {
                    try {
                        WakeOnLan.send(schedule.macAddress)
                        Log.d("WakeOnLanReceiver", "Sent scheduled magic packet")
                    } catch (e: Exception) {
                        Log.e("WakeOnLanReceiver", "Failed to send scheduled magic packet", e)
                    } finally {
                        pendingResult.finish()
                    }
                }
            }
            Intent.ACTION_BOOT_COMPLETED, Intent.ACTION_MY_PACKAGE_REPLACED -> {
                WakeOnLan.schedule(context)
                Log.d("WakeOnLanReceiver", "Rescheduled after ${intent.action}")
            }
        }
    }
}
