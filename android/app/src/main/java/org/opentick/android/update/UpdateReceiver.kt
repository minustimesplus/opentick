package org.opentick.android.update

import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentSender
import android.content.pm.PackageInstaller
import android.os.Build
import org.opentick.android.MainActivity
import org.opentick.android.OpenTickApp

/** Receives PackageInstaller's result for an update session. */
class UpdateReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != ACTION_STATUS) return
        val app = context.applicationContext as OpenTickApp
        val updater = app.updater
        val versionName = intent.getStringExtra(EXTRA_VERSION_NAME) ?: ""
        val silent = intent.getBooleanExtra(EXTRA_SILENT, false)

        when (val status = intent.getIntExtra(PackageInstaller.EXTRA_STATUS, PackageInstaller.STATUS_FAILURE)) {
            PackageInstaller.STATUS_PENDING_USER_ACTION -> {
                val confirm = confirmIntent(intent) ?: return
                confirm.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                // In the app: show Android's install prompt now. Otherwise leave it in a notification.
                if (Updater.appVisible() && runCatching { context.startActivity(confirm) }.isSuccess) return
                val pending = PendingIntent.getActivity(
                    context, 0, confirm, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
                )
                updater.notify("OpenTick $versionName is ready to install", "Tap to update.", pending)
            }
            PackageInstaller.STATUS_SUCCESS -> {
                // Runs in the updated app. Prompted installs already showed Android's own "App installed".
                if (silent) {
                    val open = PendingIntent.getActivity(
                        context, 0, Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
                        PendingIntent.FLAG_IMMUTABLE,
                    )
                    updater.notify("OpenTick updated to $versionName", "Installed in the background.", open)
                } else {
                    updater.cancelNotification()
                }
            }
            PackageInstaller.STATUS_FAILURE_ABORTED -> updater.onInstallFailed("Update cancelled")
            else -> {
                val message = intent.getStringExtra(PackageInstaller.EXTRA_STATUS_MESSAGE)
                updater.onInstallFailed("Update failed" + (message?.let { ": $it" } ?: " ($status)"))
            }
        }
    }

    @Suppress("DEPRECATION")
    private fun confirmIntent(intent: Intent): Intent? =
        if (Build.VERSION.SDK_INT >= 33) intent.getParcelableExtra(Intent.EXTRA_INTENT, Intent::class.java)
        else intent.getParcelableExtra(Intent.EXTRA_INTENT)

    companion object {
        private const val ACTION_STATUS = "org.opentick.android.UPDATE_STATUS"
        private const val EXTRA_VERSION_NAME = "version_name"
        private const val EXTRA_SILENT = "silent"

        fun statusIntent(context: Context, sessionId: Int, info: UpdateInfo, silent: Boolean): IntentSender {
            val intent = Intent(context, UpdateReceiver::class.java).setAction(ACTION_STATUS)
                .putExtra(EXTRA_VERSION_NAME, info.versionName)
                .putExtra(EXTRA_SILENT, silent)
            // Mutable: the installer adds the status extras.
            val flags = PendingIntent.FLAG_UPDATE_CURRENT or
                (if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) PendingIntent.FLAG_MUTABLE else 0)
            return PendingIntent.getBroadcast(context, sessionId, intent, flags).intentSender
        }
    }
}
