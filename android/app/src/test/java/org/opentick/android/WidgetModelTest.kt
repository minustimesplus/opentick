package org.opentick.android

import org.junit.Assert.assertEquals
import org.junit.Test
import org.opentick.android.data.ApiHabit
import org.opentick.android.data.ApiHabitCheckin
import org.opentick.android.data.LabelEntity
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.TaskEntity
import org.opentick.android.data.Views
import org.opentick.android.ui.Data
import org.opentick.android.ui.ListSel
import org.opentick.android.ui.SmartList
import org.opentick.android.util.allDayMillis
import org.opentick.android.util.today
import org.opentick.android.widget.Pages
import org.opentick.android.widget.forDay
import org.opentick.android.widget.monthWeeks
import org.opentick.android.widget.nestSubtasks
import org.opentick.android.widget.parseWidgetKey
import org.opentick.android.widget.resolveWidgetList
import org.opentick.android.widget.weekDays
import org.opentick.android.widget.widgetHabits
import org.opentick.android.widget.widgetKey
import org.opentick.android.widget.widgetTasks
import org.opentick.android.widget.widgetValueOn
import kotlinx.serialization.json.JsonObject
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.YearMonth

class WidgetModelTest {
    private val today = today()

    private fun data(vararg tasks: TaskEntity) = Data(
        projects = listOf(ProjectEntity(id = 1, serverId = 1, title = "Inbox", isInbox = true), ProjectEntity(id = 2, serverId = 2, title = "Work")),
        labels = listOf(LabelEntity(id = 7, title = "uni")),
        tasks = tasks.toList(),
    )

    @Test fun listKeysRoundTrip() {
        for (sel in listOf(ListSel.Smart(SmartList.TODAY), ListSel.Smart(SmartList.NEXT7), ListSel.Project(12), ListSel.Label(3))) {
            assertEquals(sel, parseWidgetKey(sel.widgetKey()))
        }
        // Nothing set, garbage, or a list the widget can't show: Today.
        assertEquals(ListSel.Smart(SmartList.TODAY), parseWidgetKey(null))
        assertEquals(ListSel.Smart(SmartList.TODAY), parseWidgetKey("p:x"))
        assertEquals(ListSel.Smart(SmartList.TODAY), parseWidgetKey("completed"))
    }

    @Test fun deletedListFallsBackToToday() {
        val d = data()
        assertEquals(ListSel.Project(2), resolveWidgetList(ListSel.Project(2), d))
        assertEquals(ListSel.Smart(SmartList.TODAY), resolveWidgetList(ListSel.Project(99), d))
        assertEquals(ListSel.Label(7), resolveWidgetList(ListSel.Label(7), d))
        assertEquals(ListSel.Smart(SmartList.TODAY), resolveWidgetList(ListSel.Label(8), d))
    }

    @Test fun widgetShowsOpenTasksOfTheList() {
        val d = data(
            TaskEntity(id = 1, projectId = 2, title = "due today", dueDate = allDayMillis(today)),
            TaskEntity(id = 2, projectId = 2, title = "done today", dueDate = allDayMillis(today), done = true),
            TaskEntity(id = 3, projectId = 2, title = "tomorrow", dueDate = allDayMillis(today.plusDays(1))),
            TaskEntity(id = 4, projectId = 1, title = "inbox, no date"),
        )
        val views = Views(JsonObject(emptyMap()))
        assertEquals(listOf(1L), widgetTasks(ListSel.Smart(SmartList.TODAY), d, views).map { it.id })
        assertEquals(listOf(4L), widgetTasks(ListSel.Smart(SmartList.INBOX), d, views).map { it.id })
        assertEquals(setOf(1L, 3L), widgetTasks(ListSel.Project(2), d, views).map { it.id }.toSet())
    }

    @Test fun subtasksGoBelowTheirParent() {
        val parent = TaskEntity(id = 1, serverId = 10, projectId = 2, title = "parent")
        val other = TaskEntity(id = 2, serverId = 20, projectId = 2, title = "other")
        val child = TaskEntity(id = 3, serverId = 30, projectId = 2, title = "child", parentServerId = 10)
        val grandchild = TaskEntity(id = 4, serverId = 40, projectId = 2, title = "grandchild", parentServerId = 30)
        val orphan = TaskEntity(id = 5, serverId = 50, projectId = 2, title = "orphan", parentServerId = 99)
        val rows = nestSubtasks(listOf(grandchild, parent, other, child, orphan))
        assertEquals(listOf(1L to 0, 3L to 1, 4L to 2, 2L to 0, 5L to 0), rows.map { (t, depth) -> t.id to depth })
    }

    @Test fun parentCycleStillShowsEveryTask() {
        val a = TaskEntity(id = 1, serverId = 10, projectId = 2, title = "a", parentServerId = 20)
        val b = TaskEntity(id = 2, serverId = 20, projectId = 2, title = "b", parentServerId = 10)
        assertEquals(setOf(1L, 2L), nestSubtasks(listOf(a, b)).map { it.first.id }.toSet())
    }

    @Test fun monthGridHasWholeWeeks() {
        // November 2026 starts on a Sunday and ends on a Monday.
        val weeks = monthWeeks(YearMonth.of(2026, 11), DayOfWeek.MONDAY)
        assertEquals(6, weeks.size)
        assertEquals(LocalDate.of(2026, 10, 26), weeks.first().first())
        assertEquals(LocalDate.of(2026, 12, 6), weeks.last().last())
        assertEquals(5, monthWeeks(YearMonth.of(2026, 11), DayOfWeek.SUNDAY).size)
        assertEquals(DayOfWeek.SUNDAY, weekDays(LocalDate.of(2026, 11, 18), DayOfWeek.SUNDAY).first().dayOfWeek)
    }

    @Test fun habitsOfToday() {
        val wednesday = LocalDate.of(2026, 11, 18)
        val weekdaysOnly = 0b0111110
        val habits = listOf(
            ApiHabit(id = 1, title = "b", position = 2.0),
            ApiHabit(id = 2, title = "a", position = 1.0),
            ApiHabit(id = 3, title = "archived", archived = true),
            ApiHabit(id = 4, title = "weekdays", weekdays = weekdaysOnly, position = 3.0),
        )
        assertEquals(listOf(2L, 1L, 4L), widgetHabits(habits, wednesday).map { it.id })
        assertEquals(listOf(2L, 1L), widgetHabits(habits, wednesday.plusDays(3)).map { it.id })
    }

    @Test fun habitsLoadedYesterdayStartTodayAtZero() {
        val day = LocalDate.of(2026, 11, 18)
        val h = ApiHabit(id = 1, todayValue = 1, recentCheckins = listOf(ApiHabitCheckin(day.toString(), 1)))
        assertEquals(1, listOf(h).forDay(day, day).single().todayValue)
        assertEquals(0, listOf(h).forDay(day, day.plusDays(1)).single().todayValue)
        assertEquals(0, h.widgetValueOn(day.minusDays(3), day))
        assertEquals(1, h.widgetValueOn(day, day))
        assertEquals(null, h.widgetValueOn(day.plusDays(1), day))
        assertEquals(null, h.widgetValueOn(day.minusDays(7), day))
    }

    @Test fun pagesOnlyWhenTheListDoesNotFit() {
        assertEquals(1, Pages.fit(5, 200f, 40f, 0).total)
        val p = Pages.fit(12, 200f, 40f, 9)
        assertEquals(4, p.perPage) // (200 - 30) / 40 rows above the footer
        assertEquals(3, p.total)
        assertEquals(2, p.current) // clamped to the last page
        assertEquals((8 until 12).toList(), p.of((0 until 12).toList()))
    }
}
