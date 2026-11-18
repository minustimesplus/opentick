package org.opentick.android.widget

import org.opentick.android.data.ApiHabit
import org.opentick.android.data.ListSort
import org.opentick.android.data.TaskEntity
import org.opentick.android.data.Views
import org.opentick.android.ui.Data
import org.opentick.android.ui.ListSel
import org.opentick.android.ui.SmartList
import org.opentick.android.ui.habits.isDue
import org.opentick.android.ui.habits.sortedHabits
import org.opentick.android.ui.tasks.listKey
import org.opentick.android.ui.tasks.sortTasks
import org.opentick.android.ui.tasks.tasksFor
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.YearMonth

// What the home screen widgets show, without Glance, so it can be tested.

/** Smart lists a tasks widget can show: the ones with open tasks to tick off. */
val WIDGET_SMART_LISTS = listOf(SmartList.TODAY, SmartList.TOMORROW, SmartList.NEXT7, SmartList.INBOX, SmartList.ALL)

/**
 * How a widget remembers its list: "today", "p:12" (local list id) or "l:3"
 * (local tag id). Local ids stay the same across syncs.
 */
fun ListSel.widgetKey(): String = when (this) {
    is ListSel.Smart -> kind.key
    is ListSel.Project -> "p:$id"
    is ListSel.Label -> "l:$id"
}

fun parseWidgetKey(key: String?): ListSel {
    val k = key.orEmpty()
    k.removePrefix("p:").takeIf { k.startsWith("p:") }?.toLongOrNull()?.let { return ListSel.Project(it) }
    k.removePrefix("l:").takeIf { k.startsWith("l:") }?.toLongOrNull()?.let { return ListSel.Label(it) }
    return ListSel.Smart(SmartList.entries.firstOrNull { it.key == k && it in WIDGET_SMART_LISTS } ?: SmartList.TODAY)
}

/** A list the widget was set to and that was deleted since falls back to Today. */
fun resolveWidgetList(sel: ListSel, d: Data): ListSel = when (sel) {
    is ListSel.Smart -> sel
    is ListSel.Project -> if (sel.id in d.projectMap) sel else ListSel.Smart(SmartList.TODAY)
    is ListSel.Label -> if (sel.id in d.labelMap) sel else ListSel.Smart(SmartList.TODAY)
}

fun widgetListTitle(sel: ListSel, d: Data): String = when (sel) {
    is ListSel.Smart -> sel.kind.title
    is ListSel.Project -> d.projectMap[sel.id]?.title ?: "List"
    is ListSel.Label -> d.labelMap[sel.id]?.let { "#" + it.title } ?: "Tag"
}

/**
 * The open tasks of [sel] in the list's synced sort order. The custom order
 * needs the server, so it falls back to creation order, like the app offline.
 */
fun widgetTasks(sel: ListSel, d: Data, views: Views): List<TaskEntity> {
    val open = tasksFor(sel, d).filter { !it.done && !it.deleted }
    val sort = listKey(sel, d)?.let { views.listSort(it) } ?: ListSort.DATE
    return sortTasks(open, sort, sel)
}

/** The habits due on [day] that aren't archived, in list order. */
fun widgetHabits(habits: List<ApiHabit>, day: LocalDate): List<ApiHabit> =
    habits.filter { !it.archived && it.isDue(day) }.sortedHabits()

/** Whether a habit's goal for today is reached. */
fun ApiHabit.doneToday(): Boolean = todayValue >= goal.coerceAtLeast(1)

/**
 * Habits loaded on [loadedOn] seen on [today]: once the day changed, today's
 * value is only known if a recent check-in has it, else it is 0.
 */
fun List<ApiHabit>.forDay(loadedOn: LocalDate, today: LocalDate): List<ApiHabit> =
    if (loadedOn == today) this
    else map { h -> h.copy(todayValue = h.recentCheckins.orEmpty().firstOrNull { it.date == today.toString() }?.value ?: 0) }

/**
 * Puts subtasks right below their parent, indented one level per depth, like
 * TickTick's task widget. Tasks whose parent isn't in the list stay where
 * they are at the top level.
 */
fun nestSubtasks(tasks: List<TaskEntity>): List<Pair<TaskEntity, Int>> {
    val inList = tasks.mapNotNull { it.serverId }.toSet()
    val children = tasks.filter { it.parentServerId != null && it.parentServerId in inList && it.parentServerId != it.serverId }
        .groupBy { it.parentServerId }
    val childIds = children.values.flatten().map { it.id }.toSet()
    val out = ArrayList<Pair<TaskEntity, Int>>(tasks.size)
    val seen = HashSet<Long>()
    fun add(t: TaskEntity, depth: Int) {
        if (!seen.add(t.id)) return
        out += t to depth
        t.serverId?.let { sid -> children[sid]?.forEach { add(it, minOf(depth + 1, MAX_DEPTH)) } }
    }
    for (t in tasks) if (t.id !in childIds) add(t, 0)
    // A parent cycle leaves tasks unreached; show them at the top level.
    for (t in tasks) add(t, 0)
    return out
}

private const val MAX_DEPTH = 3

/** The 7 days of the week that has [today], starting on [first]. */
fun weekDays(today: LocalDate, first: DayOfWeek): List<LocalDate> {
    var d = today
    while (d.dayOfWeek != first) d = d.minusDays(1)
    return (0L until 7L).map { d.plusDays(it) }
}

/** The weeks of the month grid of [month]: whole weeks from [first], 4 to 6 of them. */
fun monthWeeks(month: YearMonth, first: DayOfWeek): List<List<LocalDate>> {
    val start = weekDays(month.atDay(1), first).first()
    val end = month.atEndOfMonth()
    val weeks = ArrayList<List<LocalDate>>()
    var d = start
    while (d <= end) {
        weeks += (0L until 7L).map { d.plusDays(it) }
        d = d.plusDays(7)
    }
    return weeks
}

/** What a habit had on [day], as far as the widget knows: today and the last 7 days. */
fun ApiHabit.widgetValueOn(day: LocalDate, today: LocalDate): Int? = when {
    day == today -> todayValue
    day > today || day < today.minusDays(6) -> null
    else -> recentCheckins.orEmpty().firstOrNull { it.date == day.toString() }?.value ?: 0
}
