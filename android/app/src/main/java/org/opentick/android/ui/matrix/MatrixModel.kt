package org.opentick.android.ui.matrix

import org.opentick.android.data.MatrixGroup
import org.opentick.android.data.MatrixSort
import org.opentick.android.data.TaskEntity
import org.opentick.android.data.Views
import org.opentick.android.data.viewKey
import org.opentick.android.ui.Data
import org.opentick.android.util.toLocalDate
import java.text.Collator
import java.time.LocalDate
import java.time.temporal.ChronoUnit

/** A quadrant: [id] is the web app's quadrant id (1-4), the key of its synced settings. */
data class Quadrant(val id: Int, val roman: String, val title: String, val priority: Int)

// TickTick's layout: I and III on top, II and IV below.
val QUADRANT_ROWS = listOf(
    listOf(Quadrant(1, "I", "Urgent & Important", 3), Quadrant(3, "III", "Urgent & Unimportant", 1)),
    listOf(Quadrant(2, "II", "Not Urgent & Important", 2), Quadrant(4, "IV", "Not Urgent & Unimportant", 0)),
)

data class MatrixTaskGroup(val key: String, val title: String, val header: Boolean, val tasks: List<TaskEntity>)

fun level(priority: Int) = priority.coerceIn(0, 3)

private val PRIORITY_NAMES = listOf("None", "Low", "Medium", "High")
private val DATE_BUCKETS = listOf("overdue" to "Overdue", "today" to "Today", "tomorrow" to "Tomorrow", "next7" to "Next 7 Days", "later" to "Later", "noDate" to "No Date")

private fun scheduled(t: TaskEntity): Long? = t.dueDate ?: t.startDate

private fun dateBucket(t: TaskEntity, today: LocalDate): String {
    val day = scheduled(t)?.toLocalDate() ?: return "noDate"
    val diff = ChronoUnit.DAYS.between(today, day)
    return when {
        diff < 0 -> if (t.done) "today" else "overdue"
        diff == 0L -> "today"
        diff == 1L -> "tomorrow"
        diff < 7 -> "next7"
        else -> "later"
    }
}

private fun comparator(sort: MatrixSort, order: List<Long>): Comparator<TaskEntity> {
    val byDate = compareBy<TaskEntity> { scheduled(it) ?: Long.MAX_VALUE }.thenBy { it.viewKey() }
    return when (sort) {
        MatrixSort.DATE -> byDate
        MatrixSort.PRIORITY -> compareByDescending<TaskEntity> { it.priority }.then(byDate)
        MatrixSort.TITLE -> Collator.getInstance().let { c -> Comparator<TaskEntity> { a, b -> c.compare(a.title, b.title) }.thenBy { it.viewKey() } }
        MatrixSort.CREATED -> compareByDescending<TaskEntity> { it.created }.thenByDescending { it.viewKey() }
        MatrixSort.CUSTOM -> {
            // Tasks not placed yet (new ones) go on top, newest first, like the web app.
            val index = order.withIndex().associate { (i, id) -> id to i }
            compareBy<TaskEntity> { if (index[it.viewKey()] == null) 0 else 1 }
                .thenByDescending { if (index[it.viewKey()] == null) it.created else 0L }
                .thenBy { index[it.viewKey()] ?: 0 }
        }
    }
}

/** The tasks of a quadrant, grouped and sorted by its synced settings. Mirrors web MatrixView.vue. */
fun matrixGroups(q: Quadrant, d: Data, views: Views, today: LocalDate, now: Long = System.currentTimeMillis()): List<MatrixTaskGroup> {
    val view = views.quadrant(q.id)
    val lists = views.matrixLists.toSet()
    val showDone = views.matrixShowDone
    val source = d.tasks.filter { t ->
        val project = d.projectMap[t.projectId] ?: return@filter false
        (!t.done || showDone && t.doneAt != null && now - t.doneAt < 7 * 86_400_000L) &&
            level(t.priority) == q.priority &&
            (lists.isEmpty() || project.viewKey() in lists)
    }
    val groups = LinkedHashMap<String, Pair<Pair<String, Boolean>, MutableList<TaskEntity>>>()
    fun add(key: String, title: () -> String, header: Boolean, t: TaskEntity) {
        groups.getOrPut(key) { (title() to header) to mutableListOf() }.second += t
    }
    for (t in source) {
        when (view.group) {
            MatrixGroup.PRIORITY -> {
                val p = if (q.priority == 3) 3 else t.priority
                add("p$p", { PRIORITY_NAMES[level(p)] }, q.priority != 0, t)
            }
            MatrixGroup.LIST -> add("l${t.projectId}", {
                val p = d.projectMap[t.projectId]
                if (p?.isInbox == true) "Inbox" else p?.title.orEmpty()
            }, true, t)
            MatrixGroup.DATE -> dateBucket(t, today).let { b -> add("d$b", { DATE_BUCKETS.first { it.first == b }.second }, true, t) }
            MatrixGroup.NONE -> add("all", { "" }, false, t)
        }
    }
    val projectIndex = d.projects.withIndex().associate { (i, p) -> p.id to i }
    fun rank(key: String): Int = when (view.group) {
        MatrixGroup.PRIORITY -> -(key.drop(1).toIntOrNull() ?: 0)
        MatrixGroup.LIST -> projectIndex[key.drop(1).toLongOrNull()] ?: 0
        MatrixGroup.DATE -> DATE_BUCKETS.indexOfFirst { it.first == key.drop(1) }
        MatrixGroup.NONE -> 0
    }
    val cmp = compareBy<TaskEntity> { it.done }.then(comparator(view.sort, views.matrixOrder(q.id)))
    return groups.entries
        .sortedBy { rank(it.key) }
        .map { (key, v) -> MatrixTaskGroup(key, v.first.first, v.first.second, v.second.sortedWith(cmp)) }
}
