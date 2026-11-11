package org.opentick.android.sync

import android.content.Context
import androidx.work.BackoffPolicy
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
import org.opentick.android.data.AccountMode
import org.opentick.android.data.SyncState
import java.util.concurrent.TimeUnit

/** Runs a sync when the network is available; retries with backoff while offline. */
class SyncWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        val app = applicationContext as OpenTickApp
        if (app.prefs.mode.value != AccountMode.SERVER) return Result.success()
        if (app.repo.sync()) return Result.success()
        val state = app.repo.syncState.value
        // Expired sessions need the user; retrying won't help.
        if (app.repo.authExpired.value) return Result.success()
        return if (state is SyncState.Failed && runAttemptCount < 8) Result.retry() else Result.success()
    }

    companion object {
        private val online = Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build()

        /** Queues a sync a couple of seconds from now, after any running one. */
        fun requestSoon(context: Context) {
            val req = OneTimeWorkRequestBuilder<SyncWorker>()
                .setConstraints(online)
                .setInitialDelay(2, TimeUnit.SECONDS)
                .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 15, TimeUnit.SECONDS)
                .build()
            WorkManager.getInstance(context).enqueueUniqueWork("sync-now", ExistingWorkPolicy.APPEND_OR_REPLACE, req)
        }

        fun schedulePeriodic(context: Context) {
            val req = PeriodicWorkRequestBuilder<SyncWorker>(30, TimeUnit.MINUTES).setConstraints(online).build()
            WorkManager.getInstance(context).enqueueUniquePeriodicWork("sync-periodic", ExistingPeriodicWorkPolicy.KEEP, req)
        }

        fun cancelAll(context: Context) {
            WorkManager.getInstance(context).cancelUniqueWork("sync-now")
            WorkManager.getInstance(context).cancelUniqueWork("sync-periodic")
        }
    }
}
