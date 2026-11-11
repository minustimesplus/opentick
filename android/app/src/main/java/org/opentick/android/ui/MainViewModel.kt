package org.opentick.android.ui

import android.app.Application
import android.net.Uri
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import org.opentick.android.OpenTickApp
import org.opentick.android.data.LabelEntity
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.TaskEntity
import org.opentick.android.util.QuickAddParser
import org.opentick.android.util.allDayMillis
import org.opentick.android.util.atTimeMillis
import org.opentick.android.util.isOverdue
import org.opentick.android.util.today
import org.opentick.android.util.toLocalDate
import java.time.LocalDate
import java.time.temporal.ChronoUnit

/** Which list the Tasks tab shows. */
sealed interface ListSel {
    data class Smart(val kind: SmartList) : ListSel
    data class Project(val id: Long) : ListSel
    data class Label(val id: Long) : ListSel
}

enum class SmartList(val title: String) { TODAY("Today"), TOMORROW("Tomorrow"), NEXT7("Next 7 Days"), INBOX("Inbox"), ALL("All"), COMPLETED("Completed") }

data class Data(
    val projects: List<ProjectEntity> = emptyList(),
    val labels: List<LabelEntity> = emptyList(),
    val tasks: List<TaskEntity> = emptyList(),
) {
    val projectMap = projects.associateBy { it.id }
    val labelMap = labels.associateBy { it.id }
    val inbox = projects.firstOrNull { it.isInbox }
    val open = tasks.filter { !it.done }
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
    val message = MutableStateFlow<String?>(null)

    init {
        viewModelScope.launch {
            prefs.mode.collect { if (it != org.opentick.android.data.AccountMode.NONE && data.value.inbox == null) repo.ensureInbox() }
        }
    }

    fun sync() = viewModelScope.launch { repo.sync() }

    fun title(sel: ListSel, d: Data): String = when (sel) {
        is ListSel.Smart -> sel.kind.title
        is ListSel.Project -> d.projectMap[sel.id]?.title ?: "List"
        is ListSel.Label -> d.labelMap[sel.id]?.let { "#" + it.title } ?: "Tag"
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
            is ListSel.Project -> d.tasks.filter { it.projectId == sel.id }
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
        smartParse: Boolean = true,
    ) {
        if (text.isBlank()) return
        viewModelScope.launch {
            val p = if (smartParse) QuickAddParser.parse(text) else null
            val parsedDue = p?.date?.let { date -> p.time?.let { atTimeMillis(date, it) } ?: allDayMillis(date) }
            val due = if (datePicked) pickedDue else parsedDue ?: pickedDue
            val listId = p?.list?.let { name -> data.value.projects.firstOrNull { it.title.equals(name, true) }?.id } ?: projectId
            repo.createTask(
                title = p?.title ?: text.trim(),
                projectId = listId,
                dueDate = due,
                priority = p?.priority?.takeIf { priority == 0 } ?: priority,
                labelTitles = (labels + p?.labels.orEmpty()).distinct(),
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

    fun setLabels(taskId: Long, titles: List<String>) = viewModelScope.launch {
        val ids = titles.map { repo.ensureLabel(it) }
        repo.updateTask(taskId) { it.copy(labelIds = org.opentick.android.data.joinIds(ids)) }
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
        selection.value = ListSel.Smart(SmartList.TODAY)
    }
}
