package com.plugin.wakeonlan

import android.app.AlarmManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId
import java.time.ZonedDateTime

data class Schedule(val macAddress: String, val time: LocalTime, val enabled: Boolean)

object WakeOnLan {
    const val ACTION_WAKE = "com.plugin.wakeonlan.ACTION_WAKE"

    private const val PREFERENCES_NAME = "wake_on_lan"
    private const val KEY_MAC_ADDRESS = "mac_address"
    private const val KEY_HOUR = "hour"
    private const val KEY_MINUTE = "minute"
    private const val KEY_ENABLED = "enabled"

    private const val PORT = 9

    fun load(context: Context): Schedule {
        val preferences = context.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)
        return Schedule(
            preferences.getString(KEY_MAC_ADDRESS, "") ?: "",
            LocalTime.of(preferences.getInt(KEY_HOUR, 7), preferences.getInt(KEY_MINUTE, 0)),
            preferences.getBoolean(KEY_ENABLED, false),
        )
    }

    fun save(context: Context, schedule: Schedule) {
        context.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE).edit()
            .putString(KEY_MAC_ADDRESS, schedule.macAddress)
            .putInt(KEY_HOUR, schedule.time.hour)
            .putInt(KEY_MINUTE, schedule.time.minute)
            .putBoolean(KEY_ENABLED, schedule.enabled)
            .apply()
    }

    fun parseMacAddress(text: String): ByteArray {
        require(Regex("[0-9A-Fa-f]{2}([:-][0-9A-Fa-f]{2}){5}").matches(text)) { "Invalid MAC address" }
        val octets = text.split(':', '-')
        return ByteArray(6) { octets[it].toInt(16).toByte() }
    }

    // https://en.wikipedia.org/wiki/Wake-on-LAN#Magic_packet
    fun send(macAddress: String) {
        val mac = parseMacAddress(macAddress)
        val payload = ByteArray(6) { 0xFF.toByte() } + ByteArray(6 * 16) { mac[it % 6] }
        DatagramSocket().use { socket ->
            socket.broadcast = true
            socket.send(DatagramPacket(payload, payload.size, InetAddress.getByName("255.255.255.255"), PORT))
        }
    }

    fun schedule(context: Context) {
        val alarmManager = context.getSystemService(AlarmManager::class.java)
        val intent = PendingIntent.getBroadcast(
            context,
            0,
            Intent(context, WakeOnLanReceiver::class.java).setAction(ACTION_WAKE),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        alarmManager.cancel(intent)

        val schedule = load(context)
        if (!schedule.enabled) {
            return
        }

        val now = ZonedDateTime.now()
        var next = LocalDate.now().atTime(schedule.time).atZone(ZoneId.systemDefault())
        if (!next.isAfter(now)) {
            next = next.plusDays(1)
        }
        alarmManager.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, next.toInstant().toEpochMilli(), intent)
    }
}
