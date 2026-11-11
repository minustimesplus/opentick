package org.opentick.android.ui.calendar

import androidx.compose.ui.graphics.Color
import org.opentick.android.data.TaskEntity
import org.opentick.android.ui.Data
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.parseHex
import org.opentick.android.util.isAllDay
import org.opentick.android.util.toLocalDate
import org.opentick.android.util.toLocalDateTime
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.temporal.WeekFields
import java.util.Locale

enum class CalView(val title: String) { LIST("List"), YEAR("Year"), MONTH("Month"), WEEK("Week"), THREE_DAY("3 Day"), DAY("Day") }

/** A task placed on one day of the calendar. Minutes are from midnight; null means all day. */
data class CalItem(val task: TaskEntity, val startMin: Int?, val endMin: Int?, val color: Color)

/** Tasks without an end show as a block this long. */
const val DEFAULT_MINUTES = 60

fun weekStartDay(pref: Int): DayOfWeek =
    if (pref in 1..7) DayOfWeek.of(pref) else WeekFields.of(Locale.getDefault()).firstDayOfWeek

fun startOfWeek(date: LocalDate, first: DayOfWeek): LocalDate {
    var d = date
    while (d.dayOfWeek != first) d = d.minusDays(1)
    return d
}

fun taskColor(task: TaskEntity, d: Data): Color =
    d.projectMap[task.projectId]?.hexColor?.takeIf { it.isNotBlank() }?.let { parseHex(it) } ?: Accent

/**
 * Everything on [day]: timed blocks from start/end dates or the due time,
 * and all-day items (all-day due dates, or ranges covering the whole day).
 */
fun itemsOn(day: LocalDate, d: Data, showCompleted: Boolean): List<CalItem> {
    val out = ArrayList<CalItem>()
    for (t in d.tasks) {
        if (t.done && !showCompleted) continue
        val color = taskColor(t, d)
        val s = t.startDate
        val e = t.endDate
        if (s != null && e != null && e > s) {
            val sd = s.toLocalDate()
            val ed = e.toLocalDate()
            if (day < sd || day > ed) continue
            val allDay = s.toLocalDateTime().let { it.hour == 0 && it.minute == 0 } && isAllDay(e)
            if (allDay || (day != sd && day != ed)) {
                out += CalItem(t, null, null, color)
            } else {
                val start = if (day == sd) s.toLocalDateTime().let { it.hour * 60 + it.minute } else 0
                val end = if (day == ed) e.toLocalDateTime().let { it.hour * 60 + it.minute } else 24 * 60
                out += CalItem(t, start, maxOf(end, start + 15), color)
            }
            continue
        }
        val due = t.dueDate ?: continue
        if (due.toLocalDate() != day) continue
        if (isAllDay(due)) {
            out += CalItem(t, null, null, color)
        } else {
            val start = due.toLocalDateTime().let { it.hour * 60 + it.minute }
            out += CalItem(t, start, minOf(start + DEFAULT_MINUTES, 24 * 60), color)
        }
    }
    return out.sortedWith(compareBy<CalItem> { it.startMin ?: -1 }.thenBy { it.task.done }.thenBy { it.task.title })
}

/** Side by side lanes for overlapping blocks: item to (lane, lanes in its cluster). */
fun layoutLanes(items: List<CalItem>): Map<CalItem, Pair<Int, Int>> {
    val timed = items.filter { it.startMin != null }.sortedBy { it.startMin }
    val result = HashMap<CalItem, Pair<Int, Int>>()
    var cluster = mutableListOf<CalItem>()
    var laneEnds = mutableListOf<Int>()
    var clusterEnd = -1
    fun flush() {
        val lanes = laneEnds.size.coerceAtLeast(1)
        for (c in cluster) result[c] = result[c]!!.first to lanes
        cluster = mutableListOf(); laneEnds = mutableListOf()
    }
    for (it in timed) {
        if (it.startMin!! >= clusterEnd && cluster.isNotEmpty()) flush()
        var lane = laneEnds.indexOfFirst { end -> end <= it.startMin }
        if (lane == -1) { lane = laneEnds.size; laneEnds.add(it.endMin!!) } else laneEnds[lane] = it.endMin!!
        result[it] = lane to 0
        cluster.add(it)
        clusterEnd = maxOf(clusterEnd, it.endMin)
    }
    if (cluster.isNotEmpty()) flush()
    return result
}
