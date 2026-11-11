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
import org.opentick.android.data.Views
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

enum class SmartList(val title: String) {
    TODAY("Today"), TOMORROW("Tomorrow"), NEXT7("Next 7 Days"), INBOX("Inbox"), ALL("All"), COMPLETED("Completed"),
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
            SmartList.NEXT7, SmartList.COMPLETED -> GroupMode.DATE
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
                SmartList.COMPLETED -> d.tasks.filter { it.done }.sortedByDescending { it.doneAt ?: 0 }
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
