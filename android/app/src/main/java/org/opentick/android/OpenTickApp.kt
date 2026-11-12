package org.opentick.android

import android.app.Application
import androidx.lifecycle.DefaultLifecycleObserver
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ProcessLifecycleOwner
import androidx.work.Configuration
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.debounce
import kotlinx.coroutines.launch
import org.opentick.android.auth.AuthManager
import org.opentick.android.data.AccountMode
import org.opentick.android.data.Api
import org.opentick.android.data.AppDatabase
import org.opentick.android.data.Prefs
import org.opentick.android.data.Repository
import org.opentick.android.reminders.Reminders
import org.opentick.android.sync.SyncWorker
import org.opentick.android.update.UpdateWorker
import org.opentick.android.update.Updater

class OpenTickApp : Application(), Configuration.Provider {
    lateinit var prefs: Prefs
    lateinit var api: Api
    lateinit var repo: Repository
    lateinit var auth: AuthManager
    lateinit var reminders: Reminders
    lateinit var db: AppDatabase
    lateinit var updater: Updater
    val scope = CoroutineScope(SupervisorJob())

    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().build()

    @OptIn(FlowPreview::class)
    override fun onCreate() {
        super.onCreate()
        prefs = Prefs(this)
        api = Api(prefs)
        db = AppDatabase.create(this)
        repo = Repository(db, api, prefs) { SyncWorker.requestSoon(this) }
        auth = AuthManager(this, prefs, api, repo)
        reminders = Reminders(this, prefs)
        updater = Updater(this, prefs)

        if (prefs.mode.value == AccountMode.SERVER) SyncWorker.schedulePeriodic(this)
        UpdateWorker.schedulePeriodic(this)

        // Keep reminder alarms in step with the tasks.
        scope.launch {
            combine(repo.allTasks, repo.allProjects) { tasks, projects -> tasks to projects }
                .debounce(500)
                .collect { (tasks, projects) -> reminders.reschedule(tasks, projects) }
        }

        // Sync whenever the app comes to the foreground.
        ProcessLifecycleOwner.get().lifecycle.addObserver(object : DefaultLifecycleObserver {
            override fun onStart(owner: LifecycleOwner) {
                if (prefs.mode.value == AccountMode.SERVER) scope.launch { repo.sync() }
                UpdateWorker.checkIfDue(this@OpenTickApp, prefs.lastUpdateCheck)
            }
        })
    }
}
