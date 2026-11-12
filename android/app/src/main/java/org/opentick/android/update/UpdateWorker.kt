package org.opentick.android.update

import android.content.Context
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import org.opentick.android.OpenTickApp
import java.util.concurrent.TimeUnit

/** Checks for an app update and installs it in the background when allowed, else notifies. */
class UpdateWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        val updater = (applicationContext as OpenTickApp).updater
        val info = updater.check() ?: return Result.success()
        if (updater.canInstallSilently()) updater.install(info, silent = true)
        else updater.notifyAvailable(info)
        return Result.success()
    }

    companion object {
        private val CHECK_EVERY = TimeUnit.HOURS.toMillis(6)
        private val online = Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build()

        fun schedulePeriodic(context: Context) {
            if (!Updater.SUPPORTED) return
            val req = PeriodicWorkRequestBuilder<UpdateWorker>(CHECK_EVERY, TimeUnit.MILLISECONDS)
                .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).setRequiresBatteryNotLow(true).build())
                .build()
            WorkManager.getInstance(context).enqueueUniquePeriodicWork("update-periodic", ExistingPeriodicWorkPolicy.KEEP, req)
        }

        /** Checks now if the last check is older than the periodic interval (e.g. when the app opens). */
        fun checkIfDue(context: Context, lastCheck: Long) {
            if (!Updater.SUPPORTED || System.currentTimeMillis() - lastCheck < CHECK_EVERY) return
            val req = OneTimeWorkRequestBuilder<UpdateWorker>().setConstraints(online).build()
            WorkManager.getInstance(context).enqueueUniqueWork("update-now", ExistingWorkPolicy.KEEP, req)
        }
    }
}
