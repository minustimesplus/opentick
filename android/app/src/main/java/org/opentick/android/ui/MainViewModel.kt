package org.opentick.android.ui

import android.app.Application
import android.net.Uri
import androidx.compose.runtime.mutableStateListOf
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.opentick.android.OpenTickApp
import org.opentick.android.data.AccountMode
import org.opentick.android.data.ApiTask
import org.opentick.android.data.LabelEntity
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.AgentConfig
import org.opentick.android.data.AgentDispatch
import org.opentick.android.data.TaskEntity
import org.opentick.android.data.ListLayout
import org.opentick.android.data.ListSort
import org.opentick.android.data.TimelineScale
import org.opentick.android.data.Views
import org.opentick.android.data.Visibility
import org.opentick.android.ui.tasks.DatePatch
import org.opentick.android.ui.tasks.KanbanBucket
import org.opentick.android.ui.tasks.POSITION_STEP
import org.opentick.android.ui.tasks.Placement
import org.opentick.android.ui.tasks.layoutProject
import org.opentick.android.ui.tasks.listKey
import org.opentick.android.ui.tasks.sortTasks
import org.opentick.android.ui.tasks.topPosition
import org.opentick.android.ui.tasks.withDates
import org.opentick.android.data.joinIds
import org.opentick.android.util.QuickAddParser
import org.opentick.android.util.allDayMillis
import org.opentick.android.util.atTimeMillis
import org.opentick.android.util.isOverdue
import org.opentick.android.util.toLocalDate
import org.opentick.android.util.today
import java.time.LocalDate
import java.time.temporal.ChronoUnit

/** Which list the Tasks tab shows. */
sealed interface ListSel {
    data class Smart(val kind: SmartList) : ListSel
    data class Project(val id: Long) : ListSel
    data class Label(val id: Long) : ListSel
}

/** [key] is the list's segment in the web app's /tasks/:list route, which keys its synced settings. */
enum class SmartList(val title: String, val key: String) {
    TODAY("Today", "today"), TOMORROW("Tomorrow", "tomorrow"), NEXT7("Next 7 Days", "week"), INBOX("Inbox", "inbox"),
    ALL("All", "all"), COMPLETED("Completed", "completed"), WONT_DO("Won't Do", "wontdo"),
}

/** How a list's tasks are split into cards. */
enum class GroupMode { LIST, DATE, PRIORITY, NONE }

/** A snackbar with an action button, e.g. Undo. */
data class Snack(val text: String, val action: String? = null, val run: (() -> Unit)? = null)

/** Full screen pages on top of the tabs. */
enum class Page { SETTINGS, TAB_BAR, APPEARANCE, DATE_TIME, NOTIFICATIONS, POMO_SETTINGS, GENERAL, ACCOUNT, ABOUT, SEARCH, TRASH }

data class Data(
    val projects: List<ProjectEntity> = emptyList(),
    val labels: List<LabelEntity> = emptyList(),
    val tasks: List<TaskEntity> = emptyList(),
) {
    val projectMap = projects.associateBy { it.id }
    val labelMap = labels.associateBy { it.id }
    val inbox = projects.firstOrNull { it.isInbox }
    val open = tasks.filter { !it.done }
    private val byServerId = projects.filter { it.serverId != null }.associateBy { it.serverId }
    /** Lists inside a folder, keyed by the folder's local id. */
    val children: Map<Long, List<ProjectEntity>> = projects
        .filter { it.parentServerId != null && byServerId[it.parentServerId] != null }
        .groupBy { byServerId[it.parentServerId]!!.id }
    /** Top level lists (and folders), without the inbox. */
    val topLevel = projects.filter { !it.isInbox && (it.parentServerId == null || byServerId[it.parentServerId] == null) }
}

class MainViewModel(app: Application) : AndroidViewModel(app) {
    private val ot = app as OpenTickApp
    val repo = ot.repo
    val prefs = ot.prefs
    val auth = ot.auth

    val data: StateFlow<Data> = combine(repo.allProjects, repo.allLabels, repo.allTasks) { p, l, t -> Data(p, l, t) }
        .stateIn(viewModelScope, SharingStarted.Eagerly, Data())

    val syncState = repo.syncState
    val authExpired = repo.authExpired
    val pendingCount = repo.pendingCount.stateIn(viewModelScope, SharingStarted.Eagerly, 0)

    val selection = MutableStateFlow<ListSel>(ListSel.Smart(SmartList.TODAY))
    val openTaskId = MutableStateFlow<Long?>(null)
    /** Task whose actions sheet is open (long press on a task). */
    val actionsTaskId = MutableStateFlow<Long?>(null)
    /** Task and agent ("claude" or "instinct") of the open send-to-agent dialog. */
    val agentSend = MutableStateFlow<Pair<Long, String>?>(null)
    /** View settings synced with the web and desktop apps. */
    val views: StateFlow<Views> = prefs.views
    val message = MutableStateFlow<String?>(null)
    val snack = MutableStateFlow<Snack?>(null)
    val pages = mutableStateListOf<Page>()

    fun open(page: Page) { pages.add(page) }
    fun back() { if (pages.isNotEmpty()) pages.removeAt(pages.lastIndex) }

    init {
        viewModelScope.launch {
            prefs.mode.collect { if (it != AccountMode.NONE && data.value.inbox == null) repo.ensureInbox() }
        }
    }

    fun sync() = viewModelScope.launch { repo.sync() }

    fun title(sel: ListSel, d: Data): String = when (sel) {
        is ListSel.Smart -> sel.kind.title
        is ListSel.Project -> d.projectMap[sel.id]?.title ?: "List"
        is ListSel.Label -> d.labelMap[sel.id]?.let { "#" + it.title } ?: "Tag"
    }

    fun groupMode(sel: ListSel): GroupMode = when (sel) {
        is ListSel.Smart -> when (sel.kind) {
            SmartList.TODAY, SmartList.TOMORROW, SmartList.ALL -> GroupMode.LIST
            SmartList.NEXT7, SmartList.COMPLETED, SmartList.WONT_DO -> GroupMode.DATE
            SmartList.INBOX -> GroupMode.NONE
        }
        is ListSel.Project -> if (data.value.children.containsKey(sel.id)) GroupMode.LIST else GroupMode.NONE
        is ListSel.Label -> GroupMode.LIST
    }

    /** Tasks that belong to a list, including completed ones. */
    fun tasksFor(sel: ListSel, d: Data): List<TaskEntity> {
        val now = System.currentTimeMillis()
        val t = today()
        fun dayDiff(task: TaskEntity) = task.dueDate?.let { ChronoUnit.DAYS.between(t, it.toLocalDate()) }
        return when (sel) {
            is ListSel.Smart -> when (sel.kind) {
                SmartList.TODAY -> d.tasks.filter { task ->
                    val dd = dayDiff(task) ?: return@filter false
                    if (task.done) dd == 0L || (task.doneAt?.toLocalDate() == t && dd <= 0)
                    else dd <= 0L || isOverdue(task.dueDate!!, now)
                }
                SmartList.TOMORROW -> d.tasks.filter { dayDiff(it) == 1L }
                SmartList.NEXT7 -> d.tasks.filter { task ->
                    val dd = dayDiff(task) ?: return@filter false
                    dd < 7 && (dd >= 0 || !task.done)
                }
                SmartList.INBOX -> d.tasks.filter { it.projectId == d.inbox?.id }
                SmartList.ALL -> d.tasks
                SmartList.COMPLETED -> d.tasks.filter { it.done && !it.wontDo }.sortedByDescending { it.doneAt ?: 0 }
                SmartList.WONT_DO -> d.tasks.filter { it.done && it.wontDo }.sortedByDescending { it.doneAt ?: 0 }
            }
            is ListSel.Project -> {
                // A folder shows the tasks of all its lists.
                val ids = setOf(sel.id) + d.children[sel.id].orEmpty().map { it.id }
                d.tasks.filter { it.projectId in ids }
            }
            is ListSel.Label -> d.tasks.filter { sel.id in it.labelIdList }
        }
    }

    fun counts(d: Data): Map<Any, Int> {
        val m = HashMap<Any, Int>()
        for (k in listOf(SmartList.TODAY, SmartList.TOMORROW, SmartList.NEXT7, SmartList.INBOX, SmartList.ALL)) {
            m[k] = tasksFor(ListSel.Smart(k), d).count { !it.done }
        }
        for (task in d.open) {
            m[task.projectId] = (m[task.projectId] ?: 0) + 1
            for (l in task.labelIdList) m["l$l"] = (m["l$l"] ?: 0) + 1
        }
        for ((folder, kids) in d.children) m[folder] = (m[folder] ?: 0) + kids.sumOf { m[it.id] ?: 0 }
        return m
    }

    /** Defaults for a task added while looking at a list, like TickTick. */
    fun defaultsFor(sel: ListSel): Triple<Long?, LocalDate?, Long?> = when (sel) {
        is ListSel.Smart -> when (sel.kind) {
            SmartList.TODAY, SmartList.NEXT7 -> Triple(null, today(), null)
            SmartList.TOMORROW -> Triple(null, today().plusDays(1), null)
            else -> Triple(null, null, null)
        }
        is ListSel.Project -> Triple(sel.id, null, null)
        is ListSel.Label -> Triple(null, null, sel.id)
    }

    /** Creates a task from quick-add text. Explicit picks win over parsed tokens. */
    fun quickAdd(
        text: String,
        projectId: Long?,
        pickedDue: Long?,
        datePicked: Boolean,
        priority: Int,
        labels: List<String>,
        description: String = "",
    ) {
        if (text.isBlank()) return
        viewModelScope.launch {
            val p = QuickAddParser.parse(text)
            val parsedDue = p.date?.let { date -> p.time?.let { atTimeMillis(date, it) } ?: allDayMillis(date) }
            val due = if (datePicked) pickedDue else parsedDue ?: pickedDue
            val listId = p.list?.let { name -> data.value.projects.firstOrNull { it.title.equals(name, true) || org.opentick.android.util.splitEmoji(it.title).second.equals(name, true) }?.id } ?: projectId
            repo.createTask(
                title = p.title,
                projectId = listId,
                dueDate = due,
                priority = p.priority?.takeIf { priority == 0 } ?: priority,
                labelTitles = (labels + p.labels).distinct(),
                description = description,
            )
        }
    }

    fun toggleDone(task: TaskEntity) = viewModelScope.launch { repo.setDone(task.id, !task.done) }

    fun update(id: Long, edit: (TaskEntity) -> TaskEntity) = viewModelScope.launch { repo.updateTask(id, edit) }

    private var lastDeleted: TaskEntity? = null
    fun delete(task: TaskEntity) = viewModelScope.launch {
        lastDeleted = task
        repo.deleteTask(task.id)
    }

    fun undoDelete() = viewModelScope.launch { lastDeleted?.let { repo.restoreTask(it) }; lastDeleted = null }

    fun updateViews(edit: (Views) -> Views) {
        val next = edit(views.value)
        if (next != views.value) repo.setViews(next)
    }

    // ─── Per-list settings, synced with the web app ────────────────────

    fun listSort(sel: ListSel, d: Data = data.value): ListSort = listKey(sel, d)?.let { views.value.listSort(it) } ?: ListSort.DATE
    fun setListSort(sel: ListSel, v: ListSort) = listKey(sel, data.value)?.let { k -> updateViews { it.withListSort(k, v) } }

    fun showCompleted(sel: ListSel, d: Data = data.value): Boolean = listKey(sel, d)?.let { views.value.listShowCompleted(it) } ?: true
    fun setShowCompleted(sel: ListSel, on: Boolean) = listKey(sel, data.value)?.let { k -> updateViews { it.withListShowCompleted(k, on) } }

    /** Kanban and timeline exist for lists only; everything else is a plain list. */
    fun layout(sel: ListSel, d: Data = data.value): ListLayout =
        if (layoutProject(sel, d) == null) ListLayout.LIST else listKey(sel, d)?.let { views.value.listLayout(it) } ?: ListLayout.LIST
    fun setLayout(sel: ListSel, v: ListLayout) = listKey(sel, data.value)?.let { k -> updateViews { it.withListLayout(k, v) } }

    fun setTimelineScale(v: TimelineScale) = updateViews { it.withTimelineScale(v) }

    /** Open tasks of a list in its synced sort order. */
    fun sorted(sel: ListSel, tasks: List<TaskEntity>, d: Data = data.value): List<TaskEntity> {
        val sort = listSort(sel, d)
        val positions = if (sort == ListSort.CUSTOM) layoutProject(sel, d)?.let { this.positions.value[it] } else null
        return sortTasks(tasks, sort, sel, positions)
    }

    // ─── Smart lists (web Settings > Smart List) ───────────────────────

    fun smartLists(): Map<String, Visibility> = views.value.smartLists(prefs.smartLists.value)

    fun setSmartList(key: String, v: Visibility) = updateViews { it.withSmartList(key, v, prefs.smartLists.value) }

    /** Whether the trash has anything in it, asked from the server for "show if not empty". */
    val trashNonEmpty = MutableStateFlow(false)

    fun probeTrash() {
        if (smartLists()["trash"] != Visibility.AUTO || prefs.mode.value != AccountMode.SERVER) return
        viewModelScope.launch {
            runCatching { withContext(Dispatchers.IO) { ot.api.trashNonEmpty() } }.onSuccess { trashNonEmpty.value = it }
        }
    }

    /** Smart lists the drawer shows, by web key. "Show if not empty" is decided like on the web. */
    fun visibleSmartLists(d: Data, counts: Map<Any, Int>, trashFull: Boolean): Set<String> {
        fun nonEmpty(key: String): Boolean = when (key) {
            "today" -> (counts[SmartList.TODAY] ?: 0) > 0
            "tomorrow" -> (counts[SmartList.TOMORROW] ?: 0) > 0
            "next7" -> (counts[SmartList.NEXT7] ?: 0) > 0
            "inbox" -> (counts[SmartList.INBOX] ?: 0) > 0
            "all" -> (counts[SmartList.ALL] ?: 0) > 0
            "tags" -> d.labels.isNotEmpty()
            "completed" -> d.tasks.any { it.done && !it.wontDo }
            "wontDo" -> d.tasks.any { it.done && it.wontDo }
            "trash" -> trashFull
            else -> false
        }
        return smartLists().filter { (k, v) -> v == Visibility.SHOW || (v == Visibility.AUTO && nonEmpty(k)) }.keys
    }

    // ─── Manual order (Custom sort), from the list's list view on the server ──

    /** Per list (local id): task server id → position. Loaded when a list is sorted by Custom. */
    val positions = MutableStateFlow<Map<Long, Map<Long, Double>>>(emptyMap())

    fun loadPositions(projectId: Long) {
        if (prefs.mode.value != AccountMode.SERVER) return
        val sid = data.value.projectMap[projectId]?.serverId ?: return
        viewModelScope.launch {
            runCatching {
                withContext(Dispatchers.IO) {
                    val view = ot.api.projectViews(sid).let { vs -> vs.firstOrNull { it.viewKind == "list" } ?: vs.firstOrNull() } ?: return@withContext null
                    ot.api.viewTasks(sid, view.id, "done = false").associate { it.id to it.position }
                }
            }.getOrNull()?.let { map -> positions.value = positions.value + (projectId to map) }
        }
    }

    // ─── Kanban (online: sections live on the server) ──────────────────

    data class Kanban(
        val projectId: Long,
        val loading: Boolean = true,
        val error: String? = null,
        val viewId: Long = 0,
        val buckets: List<KanbanBucket> = emptyList(),
        val placement: Map<Long, Placement> = emptyMap(),
        val defaultBucket: Long = 0,
        val doneBucket: Long = 0,
    )

    val kanban = MutableStateFlow<Kanban?>(null)

    private fun projectServerId(projectId: Long) = data.value.projectMap[projectId]?.serverId

    fun loadKanban(projectId: Long) = viewModelScope.launch {
        val prev = kanban.value?.takeIf { it.projectId == projectId }
        kanban.value = (prev ?: Kanban(projectId)).copy(loading = true, error = null)
        if (prefs.mode.value != AccountMode.SERVER) {
            kanban.value = Kanban(projectId, loading = false, error = "Kanban sections are stored on the server. Sign in to an OpenTick server to use them.")
            return@launch
        }
        if (projectServerId(projectId) == null) repo.sync()
        val sid = projectServerId(projectId)
        if (sid == null) {
            kanban.value = Kanban(projectId, loading = false, error = "Connect to the internet to see this list's sections.")
            return@launch
        }
        kanban.value = try {
            withContext(Dispatchers.IO) {
                val view = ot.api.projectViews(sid).firstOrNull { it.viewKind == "kanban" }
                    ?: return@withContext Kanban(projectId, loading = false, error = "This list has no kanban view.")
                val detail = ot.api.projectView(sid, view.id)
                val buckets = ot.api.bucketsWithTasks(sid, view.id)
                Kanban(
                    projectId, loading = false, viewId = view.id,
                    buckets = buckets.map { KanbanBucket(it.id, it.title, it.position) },
                    placement = buckets.flatMap { b -> b.tasks.orEmpty().map { it.id to Placement(b.id, it.position) } }.toMap(),
                    defaultBucket = detail.defaultBucketId, doneBucket = detail.doneBucketId,
                )
            }
        } catch (e: Exception) {
            (prev ?: Kanban(projectId)).copy(loading = false, error = if (e is java.io.IOException && e !is org.opentick.android.data.ApiException) "Connect to the internet to see this list's sections." else e.message ?: "Couldn't load sections")
        }
    }

    private fun kanbanAction(block: suspend (Kanban, Long) -> Unit) = viewModelScope.launch {
        val k = kanban.value ?: return@launch
        val sid = projectServerId(k.projectId) ?: return@launch
        try {
            withContext(Dispatchers.IO) { block(k, sid) }
        } catch (e: Exception) {
            message.value = if (e is java.io.IOException && e !is org.opentick.android.data.ApiException) "Connect to the internet to change sections." else e.message ?: "Something went wrong"
            loadKanban(k.projectId)
        }
    }

    /** Moves a card to the top of another section. */
    fun moveToBucket(task: TaskEntity, bucket: Long, column: List<TaskEntity>) = kanbanAction { k, sid ->
        val id = serverIdOf(task.id) ?: throw java.io.IOException("offline")
        val position = topPosition(column, k.placement, except = task.id)
        kanban.value = k.copy(placement = k.placement + (id to Placement(bucket, position)))
        ot.api.moveToBucket(sid, k.viewId, bucket, id)
        ot.api.setPosition(id, k.viewId, position)
        // Entering or leaving the done bucket changes whether the task is done.
        if (k.doneBucket != 0L && (bucket == k.doneBucket) != task.done) repo.sync()
    }

    /** Adds a task (quick add syntax) at the top of a section. */
    fun addToBucket(text: String, bucket: Long, column: List<TaskEntity>) = kanbanAction { k, sid ->
        val p = QuickAddParser.parse(text)
        if (p.title.isBlank()) return@kanbanAction
        val due = p.date?.let { date -> p.time?.let { atTimeMillis(date, it) } ?: allDayMillis(date) }
        val local = repo.createTask(title = p.title, projectId = k.projectId, dueDate = due, priority = p.priority ?: 0, labelTitles = p.labels)
        val id = serverIdOf(local) ?: return@kanbanAction // stays in the default section until online
        val position = topPosition(column, k.placement)
        kanban.value = kanban.value?.let { it.copy(placement = it.placement + (id to Placement(bucket, position))) }
        ot.api.moveToBucket(sid, k.viewId, bucket, id)
        ot.api.setPosition(id, k.viewId, position)
    }

    fun addSection(title: String) = kanbanAction { k, sid ->
        val b = ot.api.createBucket(sid, k.viewId, title, (k.buckets.lastOrNull()?.position ?: 0.0) + POSITION_STEP)
        kanban.value = kanban.value?.let { it.copy(buckets = it.buckets + KanbanBucket(b.id, b.title.ifBlank { title }, b.position)) }
    }

    fun renameSection(bucket: KanbanBucket, title: String) = kanbanAction { k, sid ->
        kanban.value = k.copy(buckets = k.buckets.map { if (it.id == bucket.id) it.copy(title = title) else it })
        ot.api.updateBucket(sid, k.viewId, bucket.id, title, bucket.position)
    }

    fun deleteSection(bucket: KanbanBucket) = kanbanAction { k, sid ->
        ot.api.deleteBucket(sid, k.viewId, bucket.id)
        loadKanban(k.projectId)
    }

    /** Sets a task's dates (timeline drag). */
    fun setDates(taskId: Long, p: DatePatch) = update(taskId) { it.withDates(p) }

    fun toggleWontDo(task: TaskEntity) = update(task.id) {
        if (it.done && it.wontDo) it.copy(done = false, wontDo = false, doneAt = null)
        else it.copy(done = true, wontDo = true, doneAt = System.currentTimeMillis())
    }

    fun duplicate(task: TaskEntity) = viewModelScope.launch {
        repo.duplicateTask(task.id)?.let { message.value = "Duplicated" }
    }

    // ─── AI agents (server accounts only) ──────────────────────────────

    /** Agents are offered with a server account and the AI feature switched on (web Settings > Features). */
    fun agentsAvailable(): Boolean = prefs.mode.value == AccountMode.SERVER && views.value.feature("ai")

    suspend fun agentConfig(): AgentConfig = withContext(Dispatchers.IO) { ot.api.agentConfig() }

    /** The server id of a task, syncing first if it was made offline. */
    private suspend fun serverIdOf(taskId: Long): Long? {
        val t = repo.getTask(taskId) ?: return null
        if (t.serverId != null && !t.dirty) return t.serverId
        repo.sync()
        return repo.getTask(taskId)?.serverId
    }

    suspend fun agentPrompt(taskId: Long, extra: String): String {
        val id = serverIdOf(taskId) ?: throw java.io.IOException("Connect to the internet to send this task.")
        return withContext(Dispatchers.IO) { ot.api.agentPrompt(id, extra) }
    }

    suspend fun sendToAgent(taskId: Long, agent: String, extra: String): AgentDispatch {
        val id = serverIdOf(taskId) ?: throw java.io.IOException("Connect to the internet to send this task.")
        return withContext(Dispatchers.IO) { ot.api.sendToAgent(id, agent, extra) }
    }

    /** Link to the task in the web app. */
    fun webLink(task: TaskEntity): String? {
        val sid = task.serverId ?: return null
        if (prefs.mode.value != AccountMode.SERVER) return null
        val project = data.value.projectMap[task.projectId]?.serverId ?: return null
        return "${prefs.serverUrl}/tasks/p$project/$sid"
    }

    fun setLabels(taskId: Long, titles: List<String>) = viewModelScope.launch {
        val ids = titles.map { repo.ensureLabel(it) }
        repo.updateTask(taskId) { it.copy(labelIds = joinIds(ids)) }
    }

    fun createProject(title: String, color: String) = viewModelScope.launch {
        val id = repo.createProject(title, color)
        selection.value = ListSel.Project(id)
    }

    fun updateProject(id: Long, title: String, color: String) = viewModelScope.launch { repo.updateProject(id, title, color) }

    fun deleteProject(id: Long) = viewModelScope.launch {
        repo.deleteProject(id)
        selection.value = ListSel.Smart(SmartList.INBOX)
    }

    fun createLabel(title: String) = viewModelScope.launch { repo.ensureLabel(title.removePrefix("#")) }

    fun deleteLabel(id: Long) = viewModelScope.launch {
        repo.deleteLabel(id)
        selection.value = ListSel.Smart(SmartList.TODAY)
    }

    // ─── Trash (server side, online only) ──────────────────────────────

    sealed interface TrashState {
        data object Loading : TrashState
        data class Loaded(val tasks: List<ApiTask>) : TrashState
        data class Failed(val message: String) : TrashState
    }

    val trash = MutableStateFlow<TrashState>(TrashState.Loading)

    fun loadTrash() = viewModelScope.launch {
        if (prefs.mode.value != AccountMode.SERVER) {
            trash.value = TrashState.Failed("Deleted tasks are removed right away when you use OpenTick without an account.")
            return@launch
        }
        trash.value = TrashState.Loading
        trash.value = try {
            TrashState.Loaded(withContext(Dispatchers.IO) { ot.api.trash() })
        } catch (e: Exception) {
            TrashState.Failed("Connect to the internet to see deleted tasks.")
        }
    }

    fun trashAction(action: suspend () -> Unit) = viewModelScope.launch {
        try {
            withContext(Dispatchers.IO) { action() }
            repo.sync()
        } catch (e: Exception) {
            message.value = e.message ?: "Something went wrong"
        }
        loadTrash()
    }

    fun restoreFromTrash(id: Long) = trashAction { ot.api.restoreFromTrash(id) }
    fun deleteForever(id: Long) = trashAction { ot.api.deleteForever(id) }
    fun emptyTrash() = trashAction { ot.api.emptyTrash() }

    fun handleOAuthCallback(uri: Uri, onResult: (String?) -> Unit) = viewModelScope.launch {
        try {
            auth.handleCallback(uri)
            onResult(null)
        } catch (e: Exception) {
            onResult(e.message ?: "Sign in failed")
        }
    }

    fun signOut() = viewModelScope.launch {
        auth.signOut()
        pages.clear()
        selection.value = ListSel.Smart(SmartList.TODAY)
    }
}
