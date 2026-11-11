package org.opentick.android.data

import android.util.Log
import androidx.room.withTransaction
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import java.io.IOException
import java.time.Instant
import java.time.temporal.ChronoUnit

sealed interface SyncState {
    data object Idle : SyncState
    data object Running : SyncState
    data class Failed(val message: String, val offline: Boolean) : SyncState
}

/**
 * The single source of truth is the local database. The UI only reads and
 * writes Room; [sync] pushes local changes to the server and pulls the
 * server's state back. Everything works offline, changes queue up as dirty
 * rows until the next successful sync.
 */
class Repository(
    private val db: AppDatabase,
    private val api: Api,
    private val prefs: Prefs,
    /** Asks the background scheduler for a sync soon (debounced). */
    private val requestSync: () -> Unit,
) {
    private val projects = db.projects()
    private val labels = db.labels()
    private val tasks = db.tasks()
    private val syncMutex = Mutex()

    private val _syncState = MutableStateFlow<SyncState>(SyncState.Idle)
    val syncState: StateFlow<SyncState> = _syncState

    private val _authExpired = MutableStateFlow(false)
    val authExpired: StateFlow<Boolean> = _authExpired

    val allProjects: Flow<List<ProjectEntity>> = projects.observe()
    val allLabels: Flow<List<LabelEntity>> = labels.observe()
    val allTasks: Flow<List<TaskEntity>> = tasks.observeAll()
    val pendingCount: Flow<Int> = tasks.observePendingCount()
    fun task(id: Long): Flow<TaskEntity?> = tasks.observe(id)

    private fun changed() {
        if (prefs.mode.value == AccountMode.SERVER) requestSync()
    }

    // ─── Local edits ───────────────────────────────────────────────────

    suspend fun ensureInbox(): ProjectEntity = withContext(Dispatchers.IO) {
        projects.inbox() ?: run {
            val id = projects.insert(ProjectEntity(title = "Inbox", isInbox = true, position = -1.0))
            projects.get(id)!!
        }
    }

    suspend fun createTask(
        title: String,
        projectId: Long?,
        dueDate: Long? = null,
        priority: Int = 0,
        labelTitles: List<String> = emptyList(),
        description: String = "",
    ): Long = withContext(Dispatchers.IO) {
        val pid = projectId ?: ensureInbox().id
        val labelIds = labelTitles.map { ensureLabel(it) }
        val id = tasks.insert(
            TaskEntity(
                projectId = pid, title = title.trim(), description = description, dueDate = dueDate,
                priority = priority, labelIds = joinIds(labelIds), dirty = true,
            ),
        )
        changed()
        id
    }

    suspend fun updateTask(id: Long, edit: (TaskEntity) -> TaskEntity) = withContext(Dispatchers.IO) {
        db.withTransaction {
            val before = tasks.get(id) ?: return@withTransaction
            val after = edit(before)
            if (after == before) return@withTransaction
            tasks.update(after.copy(dirty = true, version = before.version + 1, updated = System.currentTimeMillis()))
        }
        changed()
    }

    suspend fun setDone(id: Long, done: Boolean) = updateTask(id) {
        it.copy(done = done, wontDo = false, doneAt = if (done) System.currentTimeMillis() else null)
    }

    suspend fun deleteTask(id: Long) = withContext(Dispatchers.IO) {
        val t = tasks.get(id) ?: return@withContext
        if (t.serverId == null) tasks.hardDelete(id)
        else tasks.update(t.copy(deleted = true, dirty = true, version = t.version + 1))
        changed()
    }

    /** Puts a task back that was deleted a moment ago (undo). */
    suspend fun restoreTask(snapshot: TaskEntity) = withContext(Dispatchers.IO) {
        val current = tasks.get(snapshot.id)
        if (current == null) tasks.insert(snapshot.copy(dirty = true, deleted = false))
        else tasks.update(snapshot.copy(deleted = false, dirty = true, version = current.version + 1))
        changed()
    }

    suspend fun getTask(id: Long): TaskEntity? = withContext(Dispatchers.IO) { tasks.get(id) }

    suspend fun createProject(title: String, hexColor: String = ""): Long = withContext(Dispatchers.IO) {
        val maxPos = projects.all().maxOfOrNull { it.position } ?: 0.0
        val id = projects.insert(ProjectEntity(title = title.trim(), hexColor = hexColor, position = maxPos + 1, dirty = true))
        changed()
        id
    }

    suspend fun updateProject(id: Long, title: String, hexColor: String) = withContext(Dispatchers.IO) {
        val p = projects.get(id) ?: return@withContext
        projects.update(p.copy(title = title.trim(), hexColor = hexColor, dirty = true, version = p.version + 1))
        changed()
    }

    suspend fun deleteProject(id: Long) = withContext(Dispatchers.IO) {
        val p = projects.get(id) ?: return@withContext
        if (p.isInbox) return@withContext
        db.withTransaction {
            tasks.dropUnsyncedInProject(id)
            if (p.serverId == null) {
                projects.hardDelete(id)
            } else {
                // The server deletes the tasks with the project.
                tasks.deleteInProject(id)
                projects.update(p.copy(deleted = true, dirty = true, version = p.version + 1))
            }
        }
        changed()
    }

    /** Returns the local id of the label with this title, creating it if needed. */
    suspend fun ensureLabel(title: String): Long = withContext(Dispatchers.IO) {
        labels.byTitle(title.trim())?.id ?: labels.insert(LabelEntity(title = title.trim(), dirty = true)).also { changed() }
    }

    suspend fun deleteLabel(id: Long) = withContext(Dispatchers.IO) {
        val l = labels.get(id) ?: return@withContext
        db.withTransaction {
            for (t in tasks.all().filter { id in it.labelIdList }) {
                tasks.update(t.copy(labelIds = joinIds(t.labelIdList - id), version = t.version + 1))
            }
            if (l.serverId == null) labels.hardDelete(id)
            else labels.update(l.copy(deleted = true, dirty = true, version = l.version + 1))
        }
        changed()
    }

    /** Wipes everything on sign out. */
    suspend fun clearAll() = withContext(Dispatchers.IO) {
        db.clearAllTables()
    }

    // ─── Sync ──────────────────────────────────────────────────────────

    /** Pushes local changes, then pulls the server state. Returns true on success. */
    suspend fun sync(): Boolean = withContext(Dispatchers.IO) {
        if (prefs.mode.value != AccountMode.SERVER) return@withContext true
        syncMutex.withLock {
            _syncState.value = SyncState.Running
            try {
                val user = api.user()
                prefs.username = user.username
                prefs.displayName = user.name
                pullProjects(user.settings?.defaultProjectId ?: 0)
                pullLabels()
                pushProjects()
                pushLabels()
                pushTasks()
                pullTasks()
                prefs.lastSync = System.currentTimeMillis()
                _authExpired.value = false
                _syncState.value = SyncState.Idle
                true
            } catch (e: AuthExpiredException) {
                _authExpired.value = true
                _syncState.value = SyncState.Failed(e.message ?: "Signed out", offline = false)
                false
            } catch (e: ApiException) {
                Log.w(TAG, "sync failed", e)
                _syncState.value = SyncState.Failed(e.message ?: "Sync failed", offline = false)
                false
            } catch (e: IOException) {
                _syncState.value = SyncState.Failed("Offline. Changes are saved and sync later.", offline = true)
                false
            } catch (e: Exception) {
                Log.e(TAG, "sync crashed", e)
                _syncState.value = SyncState.Failed(e.message ?: "Sync failed", offline = false)
                false
            }
        }
    }

    private suspend fun pullProjects(defaultProjectId: Long) {
        val remote = api.projects().filter { it.id > 0 && !it.isArchived }
        val remoteIds = remote.map { it.id }.toSet()
        db.withTransaction {
            for (r in remote) {
                val local = projects.byServerId(r.id)
                if (local == null) {
                    projects.insert(ProjectEntity(serverId = r.id, title = r.title, hexColor = r.hexColor, position = r.position, isFavorite = r.isFavorite))
                } else if (!local.dirty) {
                    projects.update(local.copy(title = r.title, hexColor = r.hexColor, position = r.position, isFavorite = r.isFavorite))
                }
            }
            // Projects removed (or archived) on the server.
            for (p in projects.all()) {
                if (p.serverId != null && p.serverId !in remoteIds && !p.dirty) {
                    tasks.dropUnsyncedInProject(p.id)
                    tasks.all().filter { it.projectId == p.id }.forEach { tasks.hardDelete(it.id) }
                    projects.hardDelete(p.id)
                }
            }
            val serverInbox = (if (defaultProjectId > 0) projects.byServerId(defaultProjectId) else null)
                ?: projects.all().firstOrNull { it.serverId != null && it.title.equals("Inbox", true) }
                ?: projects.all().firstOrNull { it.serverId != null && !it.deleted }
            if (serverInbox != null) {
                // A local-only Inbox (created offline or before signing in) merges into the server's.
                for (p in projects.all()) {
                    if (p.serverId == null && p.isInbox && p.id != serverInbox.id) {
                        tasks.all().filter { it.projectId == p.id }.forEach { tasks.update(it.copy(projectId = serverInbox.id, dirty = true)) }
                        projects.hardDelete(p.id)
                    }
                }
                projects.setInbox(serverInbox.id)
            }
        }
    }

    private suspend fun pullLabels() {
        val remote = api.labels()
        db.withTransaction {
            for (r in remote) {
                val local = labels.byServerId(r.id)
                    // A label created offline with the same name adopts the server's.
                    ?: labels.all().firstOrNull { it.serverId == null && !it.deleted && it.title.equals(r.title, true) }
                if (local == null) {
                    labels.insert(LabelEntity(serverId = r.id, title = r.title, hexColor = r.hexColor))
                } else if (!local.dirty || local.serverId == null) {
                    labels.update(local.copy(serverId = r.id, title = r.title, hexColor = r.hexColor, dirty = false))
                }
            }
            val ids = remote.map { it.id }.toSet()
            for (l in labels.all()) {
                if (l.serverId != null && l.serverId !in ids && !l.dirty) labels.hardDelete(l.id)
            }
        }
    }

    private suspend fun pushProjects() {
        for (p in projects.all()) {
            try {
                when {
                    p.deleted -> {
                        p.serverId?.let { ignoreNotFound { api.deleteProject(it) } }
                        tasks.all().filter { it.projectId == p.id }.forEach { tasks.hardDelete(it.id) }
                        projects.hardDelete(p.id)
                    }
                    p.serverId == null -> {
                        val created = api.createProject(p.title, p.hexColor)
                        val cur = projects.get(p.id) ?: continue
                        projects.update(cur.copy(serverId = created.id))
                        projects.markClean(p.id, p.version)
                    }
                    p.dirty -> {
                        api.updateProject(p.serverId, p.title, p.hexColor)
                        projects.markClean(p.id, p.version)
                    }
                }
            } catch (e: ApiException) {
                if (!dropRejected(e)) throw e
                if (e.status == 404) projects.hardDelete(p.id) else projects.markClean(p.id, p.version)
            }
        }
    }

    private suspend fun pushLabels() {
        for (l in labels.all()) {
            try {
                when {
                    l.deleted -> {
                        l.serverId?.let { ignoreNotFound { api.deleteLabel(it) } }
                        labels.hardDelete(l.id)
                    }
                    l.serverId == null -> {
                        val created = api.createLabel(l.title, l.hexColor)
                        val cur = labels.get(l.id) ?: continue
                        labels.update(cur.copy(serverId = created.id))
                        labels.markClean(l.id, l.version)
                    }
                    // Label edits other than create/delete aren't offered in the app.
                    l.dirty -> labels.markClean(l.id, l.version)
                }
            } catch (e: ApiException) {
                if (!dropRejected(e)) throw e
                labels.markClean(l.id, l.version)
            }
        }
    }

    private suspend fun pushTasks() {
        val inbox = projects.inbox()
        for (t in tasks.pending()) {
            try {
                if (t.deleted) {
                    t.serverId?.let { ignoreNotFound { api.deleteTask(it) } }
                    tasks.hardDelete(t.id)
                    continue
                }
                val project = projects.get(t.projectId)?.takeIf { !it.deleted } ?: inbox
                val projectServerId = project?.serverId ?: continue // project not on the server yet
                val wantedLabels = t.labelIdList.mapNotNull { labels.get(it)?.serverId }.toSet()
                val response = if (t.serverId == null) {
                    val created = api.createTask(projectServerId, t.toApiFields(projectServerId))
                    // Store the server id right away so a failure below can't duplicate the task.
                    tasks.get(t.id)?.let { tasks.update(it.copy(serverId = created.id)) }
                    if (t.done) api.updateTask(created.id, t.toApiFields(projectServerId)) else created
                } else {
                    api.updateTask(t.serverId, t.toApiFields(projectServerId))
                }
                val had = parseIds(t.syncedLabelServerIds).toSet()
                for (add in wantedLabels - had) runCatching { api.addLabel(response.id, add) }
                for (rm in had - wantedLabels) ignoreNotFound { api.removeLabel(response.id, rm) }

                db.withTransaction {
                    val cur = tasks.get(t.id) ?: return@withTransaction
                    if (cur.version == t.version) {
                        // Take the server's answer: repeating tasks come back with the next date.
                        tasks.update(applyRemote(cur, response, project.id).copy(syncedLabelServerIds = joinIds(wantedLabels), labelIds = cur.labelIds, dirty = false))
                    } else {
                        tasks.update(cur.copy(serverId = response.id, syncedLabelServerIds = joinIds(wantedLabels)))
                    }
                }
            } catch (e: ApiException) {
                if (!dropRejected(e)) throw e
                // Gone on the server, or the server refuses the change: don't retry forever.
                if (e.status == 404 && t.serverId != null) tasks.hardDelete(t.id) else tasks.markClean(t.id, t.version)
                Log.w(TAG, "task ${t.id} rejected: ${e.message}")
            }
        }
    }

    private suspend fun pullTasks() {
        val since = Instant.now().minus(14, ChronoUnit.DAYS).truncatedTo(ChronoUnit.SECONDS)
        val open = api.tasks("done = false")
        val recentDone = api.tasks("done = true && done_at > \"$since\"", maxPages = 4)
        val remote = (open + recentDone).distinctBy { it.id }
        val seen = HashSet<Long>()
        db.withTransaction {
            val projectByServer = projects.all().filter { it.serverId != null }.associateBy { it.serverId!! }
            for (r in remote) {
                val project = projectByServer[r.projectId] ?: continue // e.g. an archived list
                seen += r.id
                val local = tasks.byServerId(r.id)
                if (local != null && (local.dirty || local.deleted)) continue
                val labelIds = r.labels.orEmpty().map { rl ->
                    labels.byServerId(rl.id)?.id ?: labels.insert(LabelEntity(serverId = rl.id, title = rl.title, hexColor = rl.hexColor))
                }
                val base = local ?: TaskEntity(projectId = project.id, title = r.title)
                val merged = applyRemote(base, r, project.id).copy(
                    labelIds = joinIds(labelIds),
                    syncedLabelServerIds = joinIds(r.labels.orEmpty().map { it.id }),
                    dirty = false,
                )
                if (local == null) tasks.insert(merged.copy(id = 0)) else tasks.update(merged)
            }
            for (t in tasks.all()) {
                if (t.serverId != null && t.serverId !in seen && !t.dirty && !t.deleted) tasks.hardDelete(t.id)
            }
        }
    }

    private fun applyRemote(local: TaskEntity, r: ApiTask, projectLocalId: Long) = local.copy(
        serverId = r.id,
        projectId = projectLocalId,
        title = r.title,
        description = r.description,
        done = r.done,
        doneAt = parseServerDate(r.doneAt),
        wontDo = r.wontDo,
        dueDate = parseServerDate(r.dueDate),
        priority = r.priority,
        repeatAfter = r.repeatAfter,
        repeatMode = r.repeatMode,
        repeatRule = r.repeatRule,
        parentServerId = r.relatedTasks?.get("parenttask")?.firstOrNull()?.id ?: local.parentServerId.takeIf { r.relatedTasks == null },
        created = parseServerDate(r.created) ?: local.created,
        updated = parseServerDate(r.updated) ?: local.updated,
    )

    private inline fun ignoreNotFound(block: () -> Unit) {
        try {
            block()
        } catch (e: ApiException) {
            if (e.status != 404 && e.status != 403) throw e
        }
    }

    /** 4xx other than auth/rate limits: the server will never accept this change. */
    private fun dropRejected(e: ApiException) = e.status in 400..499 && e.status != 401 && e.status != 429 && e.status != 408

    companion object {
        private const val TAG = "OpenTickSync"
    }
}
