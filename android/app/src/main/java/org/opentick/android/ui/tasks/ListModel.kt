package org.opentick.android.ui.tasks

import org.opentick.android.data.ListSort
import org.opentick.android.data.TaskEntity
import org.opentick.android.data.viewKey
import org.opentick.android.ui.Data
import org.opentick.android.ui.ListSel
import org.opentick.android.ui.SmartList
import org.opentick.android.util.isOverdue
import org.opentick.android.util.splitEmoji
import org.opentick.android.util.toLocalDate
import org.opentick.android.util.today
import java.time.temporal.ChronoUnit
import java.text.Collator

/**
 * The key of a list's synced settings (sort, layout, show completed): the web
 * app's /tasks/:list segment, so both apps share them. Lists made offline use
 * their negated local id ("p-5") until they reach the server.
 */
fun listKey(sel: ListSel, d: Data): String? = when (sel) {
    is ListSel.Smart -> sel.kind.key
    is ListSel.Project -> d.projectMap[sel.id]?.let { if (it.isInbox) "inbox" else "p${it.viewKey()}" }
    is ListSel.Label -> d.labelMap[sel.id]?.let { "l${it.viewKey()}" }
}

/** The list whose kanban and timeline layouts apply, like the web: lists and the inbox, not smart lists or tags. */
fun layoutProject(sel: ListSel, d: Data): Long? = when (sel) {
    is ListSel.Project -> sel.id
    is ListSel.Smart -> if (sel.kind == SmartList.INBOX) d.inbox?.id else null
    is ListSel.Label -> null
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

/** Completed and Won't Do list closed tasks; they have no sort, layout or add button. */
fun isClosedList(sel: ListSel) = sel == ListSel.Smart(SmartList.COMPLETED) || sel == ListSel.Smart(SmartList.WONT_DO)

/** The day a task is planned for: the start of a range, else its due date. */
private fun dateValue(t: TaskEntity): Long =
    t.startDate?.takeIf { t.endDate != null && t.endDate > it } ?: t.dueDate ?: t.startDate ?: Long.MAX_VALUE

private val titleCollator: Collator get() = Collator.getInstance().apply { strength = Collator.SECONDARY }

/**
 * Sorts open tasks like the web app's list view (TasksView.vue). [positions] is
 * the manual order of the list (task server id → position) for Custom.
 *
 * Today sorted by date keeps TickTick Android's order, latest dates first.
 */
fun sortTasks(tasks: List<TaskEntity>, sort: ListSort, sel: ListSel, positions: Map<Long, Double>? = null): List<TaskEntity> {
    val byId = compareBy<TaskEntity> { it.viewKey() }
    val cmp: Comparator<TaskEntity> = when (sort) {
        ListSort.DATE -> if (sel == ListSel.Smart(SmartList.TODAY)) {
            compareByDescending<TaskEntity> { it.dueDate ?: 0 }.thenBy { it.created }.thenBy { it.id }
        } else {
            compareBy<TaskEntity> { dateValue(it) }.thenByDescending { it.priority.coerceIn(0, 3) }.then(byId)
        }
        ListSort.PRIORITY -> compareByDescending<TaskEntity> { it.priority.coerceIn(0, 3) }.thenBy { dateValue(it) }.then(byId)
        ListSort.TITLE -> titleCollator.let { c -> Comparator<TaskEntity> { a, b -> c.compare(splitEmoji(a.title).second, splitEmoji(b.title).second) }.then(byId) }
        ListSort.CREATED -> compareByDescending<TaskEntity> { it.created }.then(byId)
        ListSort.CUSTOM -> compareBy<TaskEntity> { t -> t.serverId?.let { positions?.get(it) } ?: Double.MAX_VALUE }.then(byId)
    }
    return tasks.sortedWith(cmp)
}

// ─── Kanban ────────────────────────────────────────────────────────────

/** A kanban column: a bucket ("section") of the list's kanban view. */
data class KanbanBucket(val id: Long, val title: String, val position: Double)

/** Where a task sits on the board: its bucket and its position in it. */
data class Placement(val bucket: Long, val position: Double)

data class KanbanColumn(val bucket: KanbanBucket, val tasks: List<TaskEntity>)

/**
 * Splits a list's tasks into columns, like web KanbanBoard.vue: done tasks go to
 * the done bucket if there is one, unplaced tasks (e.g. made offline) to the default bucket.
 */
fun kanbanColumns(
    tasks: List<TaskEntity>,
    buckets: List<KanbanBucket>,
    placement: Map<Long, Placement>,
    defaultBucket: Long,
    doneBucket: Long,
): List<KanbanColumn> {
    val fallback = defaultBucket.takeIf { id -> buckets.any { it.id == id } } ?: buckets.firstOrNull()?.id ?: 0
    fun bucketOf(t: TaskEntity): Long {
        val p = t.serverId?.let { placement[it] }
        if (t.done && doneBucket != 0L) return doneBucket
        if (p != null && !(p.bucket == doneBucket && !t.done) && buckets.any { it.id == p.bucket }) return p.bucket
        return fallback
    }
    fun posOf(t: TaskEntity) = t.serverId?.let { placement[it]?.position } ?: Double.MAX_VALUE
    val byBucket = tasks.groupBy(::bucketOf)
    return buckets.map { b ->
        KanbanColumn(b, byBucket[b.id].orEmpty().sortedWith(compareBy<TaskEntity> { posOf(it) }.thenBy { it.viewKey() }))
    }
}

/** Spacing of positions in a view, as on the web. */
const val POSITION_STEP = 65536.0

/** Position that puts a card at the top of [column] (new and moved cards go on top, as in TickTick). */
fun topPosition(column: List<TaskEntity>, placement: Map<Long, Placement>, except: Long? = null): Double {
    val first = column.firstOrNull { it.id != except }?.serverId?.let { placement[it]?.position }
    return if (first != null && first > 0) first / 2 else POSITION_STEP
}
