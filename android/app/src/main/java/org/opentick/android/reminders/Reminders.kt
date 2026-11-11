package org.opentick.android.reminders

import android.Manifest
import android.app.AlarmManager
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import org.opentick.android.MainActivity
import org.opentick.android.OpenTickApp
import org.opentick.android.R
import org.opentick.android.data.Prefs
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.TaskEntity
import org.opentick.android.util.isAllDay

/**
 * Local notifications at a task's due time. They are scheduled on the device,
 * so they fire offline too. All-day tasks remind at 9:00 on the day.
 */
class Reminders(private val context: Context, private val prefs: Prefs) {
    private val alarms = context.getSystemService(AlarmManager::class.java)

    init {
        val channel = NotificationChannel(CHANNEL, "Task reminders", NotificationManager.IMPORTANCE_HIGH)
        context.getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    fun reschedule(tasks: List<TaskEntity>, @Suppress("UNUSED_PARAMETER") projects: List<ProjectEntity>) {
        val now = System.currentTimeMillis()
        val horizon = now + 14L * 24 * 3600 * 1000
        val wanted = if (!prefs.remindersEnabled) emptyMap() else tasks
            .filter { !it.done && !it.deleted && it.dueDate != null }
            .mapNotNull { t ->
                val at = if (isAllDay(t.dueDate!!)) t.dueDate - (23 * 60 + 59 - 9 * 60) * 60_000L else t.dueDate
                if (at in (now + 1)..horizon) t to at else null
            }
            // Android caps alarms per app; the nearest ones matter.
            .sortedBy { it.second }
            .take(100)
            .associate { (t, at) -> t.id.toString() to (t to at) }

        for (old in prefs.scheduledReminders - wanted.keys) {
            alarms.cancel(pending(old.toLong(), "", 0))
        }
        for ((_, pair) in wanted) {
            val (t, at) = pair
            alarms.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pending(t.id, t.title, at))
        }
        prefs.scheduledReminders = wanted.keys
    }

    private fun pending(taskId: Long, title: String, at: Long): PendingIntent {
        val intent = Intent(context, ReminderReceiver::class.java)
            .putExtra("task_id", taskId).putExtra("title", title).putExtra("at", at)
        return PendingIntent.getBroadcast(
            context, taskId.toInt(), intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
    }

    companion object {
        const val CHANNEL = "reminders"
    }
}

class ReminderReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val taskId = intent.getLongExtra("task_id", 0)
        val app = context.applicationContext as OpenTickApp
        if (intent.action == ACTION_COMPLETE) {
            val result = goAsync()
            app.scope.launch {
                app.repo.setDone(taskId, true)
                NotificationManagerCompat.from(context).cancel(taskId.toInt())
                result.finish()
            }
            return
        }
        if (Build.VERSION.SDK_INT >= 33 &&
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) return
        val result = goAsync()
        app.scope.launch {
            // The task may have been completed or deleted since scheduling.
            val task = app.repo.getTask(taskId)
            if (task != null && !task.done && !task.deleted) {
                val open = PendingIntent.getActivity(
                    context, taskId.toInt(),
                    Intent(context, MainActivity::class.java).putExtra("open_task", taskId)
                        .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP),
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
                )
                val complete = PendingIntent.getBroadcast(
                    context, taskId.toInt(),
                    Intent(context, ReminderReceiver::class.java).setAction(ACTION_COMPLETE).putExtra("task_id", taskId),
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
                )
                val n = NotificationCompat.Builder(context, Reminders.CHANNEL)
                    .setSmallIcon(R.drawable.ic_notification)
                    .setColor(0xFF4772FA.toInt())
                    .setContentTitle(task.title)
                    .setContentText(org.opentick.android.util.formatDue(task.dueDate!!))
                    .setContentIntent(open)
                    .setAutoCancel(true)
                    .setPriority(NotificationCompat.PRIORITY_HIGH)
                    .addAction(0, "Complete", complete)
                    .build()
                runCatching { NotificationManagerCompat.from(context).notify(taskId.toInt(), n) }
            }
            result.finish()
        }
    }

    companion object {
        const val ACTION_COMPLETE = "org.opentick.android.COMPLETE"
    }
}

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED && intent.action != Intent.ACTION_MY_PACKAGE_REPLACED) return
        val app = context.applicationContext as OpenTickApp
        val result = goAsync()
        app.scope.launch {
            app.reminders.reschedule(app.repo.allTasks.first(), app.repo.allProjects.first())
            result.finish()
        }
    }
}
