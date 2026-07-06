package com.plugin.timer

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Context
import android.content.Intent
import android.media.AudioAttributes
import android.media.MediaPlayer
import android.media.RingtoneManager
import android.os.Build
import android.os.CountDownTimer
import android.os.Handler
import android.os.IBinder
import android.os.VibrationEffect
import android.os.Vibrator
import android.os.VibratorManager
import android.util.Log
import androidx.core.app.NotificationCompat

class TimerService : Service() {
    private var timer: CountDownTimer? = null
    private var remainingSeconds: Int = 0
    private var alarmPlayer: MediaPlayer? = null
    private val handler = Handler(android.os.Looper.getMainLooper())

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                val durationSeconds = intent.getIntExtra(EXTRA_DURATION, 0)
                if (durationSeconds > 0) {
                    startTimer(durationSeconds)
                }
            }
            ACTION_STOP -> {
                handler.removeCallbacksAndMessages(null)
                stopTimer()
                stopAlarmSound()
                stopSelf()
            }
            ACTION_STOP_ALARM -> {
                handler.removeCallbacksAndMessages(null)
                stopAlarmSound()
                stopSelf()
            }
        }
        return START_NOT_STICKY
    }

    private fun startTimer(durationSeconds: Int) {
        timer?.cancel()

        remainingSeconds = durationSeconds
        val notification = createNotification(remainingSeconds)
        startForeground(NOTIFICATION_ID, notification)

        timer = object : CountDownTimer(durationSeconds * 1000L, 1000L) {
            override fun onTick(millisUntilFinished: Long) {
                remainingSeconds = (millisUntilFinished / 1000).toInt()
                updateNotification(remainingSeconds)
                broadcastTick(remainingSeconds)
            }

            override fun onFinish() {
                remainingSeconds = 0
                broadcastComplete()
                playAlarmSound()
                vibrate()
                showCompletionNotification()

                handler.postDelayed({
                    stopAlarmSound()
                    stopSelf()
                }, ALARM_TIMEOUT_MS)
            }
        }.start()

        Log.d("TimerService", "Timer started for $durationSeconds seconds")
    }

    private fun stopTimer() {
        timer?.cancel()
        timer = null
        Log.d("TimerService", "Timer stopped")
    }

    private fun broadcastTick(remaining: Int) {
        val intent = Intent(ACTION_TICK).apply {
            putExtra(EXTRA_REMAINING, remaining)
            setPackage(packageName)
        }
        sendBroadcast(intent)
    }

    private fun broadcastComplete() {
        val intent = Intent(ACTION_COMPLETE).apply {
            setPackage(packageName)
        }
        sendBroadcast(intent)
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "Timer",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Timer notifications"
                setShowBadge(false)
            }

            val completionChannel = NotificationChannel(
                COMPLETION_CHANNEL_ID,
                "Timer Complete",
                NotificationManager.IMPORTANCE_HIGH
            ).apply {
                description = "Timer completion alerts"
                enableVibration(true)
                vibrationPattern = longArrayOf(0, 200, 100, 200)
                setSound(null, null)
            }

            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(channel)
            manager.createNotificationChannel(completionChannel)
        }
    }

    private fun createNotification(remainingSeconds: Int): Notification {
        val minutes = remainingSeconds / 60
        val seconds = remainingSeconds % 60
        val timeText = String.format("%02d:%02d", minutes, seconds)

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("Timer Running")
            .setContentText(timeText)
            .setSmallIcon(android.R.drawable.ic_lock_idle_alarm)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
            .build()
    }

    private fun updateNotification(remainingSeconds: Int) {
        val notification = createNotification(remainingSeconds)
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, notification)
    }

    private fun showCompletionNotification() {
        val notification = NotificationCompat.Builder(this, COMPLETION_CHANNEL_ID)
            .setContentTitle("Timer Complete!")
            .setContentText("Your timer has finished")
            .setSmallIcon(android.R.drawable.ic_lock_idle_alarm)
            .setAutoCancel(true)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .build()

        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(COMPLETION_NOTIFICATION_ID, notification)
    }

    private fun playAlarmSound() {
        stopAlarmSound()

        val player = MediaPlayer()
        try {
            val uri = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_ALARM)
                ?: RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION)

            player.setDataSource(applicationContext, uri)
            player.setAudioAttributes(
                AudioAttributes.Builder()
                    .setUsage(AudioAttributes.USAGE_ALARM)
                    .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
                    .build()
            )
            player.isLooping = true
            player.prepare()
            player.start()
            alarmPlayer = player

            Log.d("TimerService", "Alarm sound started")
        } catch (e: Exception) {
            player.release()
            Log.e("TimerService", "Failed to play alarm sound", e)
        }
    }

    private fun stopAlarmSound() {
        val player = alarmPlayer
        alarmPlayer = null
        try {
            player?.let {
                if (it.isPlaying) {
                    it.stop()
                }
                it.release()
            }
        } catch (e: Exception) {
            Log.e("TimerService", "Failed to stop alarm sound", e)
        }
    }

    private fun vibrate() {
        try {
            val vibrator = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                val manager = getSystemService(Context.VIBRATOR_MANAGER_SERVICE) as VibratorManager
                manager.defaultVibrator
            } else {
                @Suppress("DEPRECATION")
                getSystemService(Context.VIBRATOR_SERVICE) as Vibrator
            }

            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                val pattern = longArrayOf(0, 200, 100, 200, 100, 200)
                vibrator.vibrate(VibrationEffect.createWaveform(pattern, -1))
            } else {
                @Suppress("DEPRECATION")
                vibrator.vibrate(longArrayOf(0, 200, 100, 200, 100, 200), -1)
            }
        } catch (e: Exception) {
            Log.e("TimerService", "Failed to vibrate", e)
        }
    }

    override fun onDestroy() {
        handler.removeCallbacksAndMessages(null)
        timer?.cancel()
        stopAlarmSound()
        super.onDestroy()
    }

    companion object {
        const val CHANNEL_ID = "timer_channel"
        const val COMPLETION_CHANNEL_ID = "timer_completion_channel"
        const val NOTIFICATION_ID = 1001
        const val COMPLETION_NOTIFICATION_ID = 1002

        const val ACTION_START = "com.plugin.timer.ACTION_START"
        const val ACTION_STOP = "com.plugin.timer.ACTION_STOP"
        const val ACTION_STOP_ALARM = "com.plugin.timer.ACTION_STOP_ALARM"
        const val ACTION_TICK = "com.plugin.timer.ACTION_TICK"
        const val ACTION_COMPLETE = "com.plugin.timer.ACTION_COMPLETE"

        const val EXTRA_DURATION = "duration"
        const val EXTRA_REMAINING = "remaining"

        const val ALARM_TIMEOUT_MS = 5L * 60L * 1000L
    }
}
