package org.opentick.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.opentick.android.data.TaskEntity
import org.opentick.android.data.TimelineScale
import org.opentick.android.ui.tasks.DragMode
import org.opentick.android.ui.tasks.GroupUnit
import org.opentick.android.ui.tasks.Span
import org.opentick.android.ui.tasks.dragSpan
import org.opentick.android.ui.tasks.scheduleOf
import org.opentick.android.ui.tasks.schedulePatch
import org.opentick.android.ui.tasks.spanPatch
import org.opentick.android.ui.tasks.taskSpan
import org.opentick.android.ui.tasks.timelineGroups
import org.opentick.android.ui.tasks.timelineRange
import org.opentick.android.ui.tasks.withDates
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.LocalDateTime

/** Same cases as web/src/lib/timeline.test.ts. */
class TimelineTest {
    private fun day(m: Int, d: Int) = LocalDate.of(2026, m, d)
    private fun at(m: Int, d: Int, h: Int = 0, min: Int = 0) = LocalDateTime.of(2026, m, d, h, min)

    private val blank = TaskEntity(id = 1, projectId = 1, title = "T")
    private fun allDay(from: LocalDateTime, to: LocalDateTime? = null) = blank.withDates(schedulePatch(from, true, to))
    private fun timed(from: LocalDateTime, to: LocalDateTime? = null) = blank.withDates(schedulePatch(from, false, to))

    @Test
    fun taskSpanCoversTheDaysOfATask() {
        assertNull(taskSpan(blank))
        assertEquals(Span(day(10, 5), day(10, 5)), taskSpan(allDay(at(10, 5))))
        assertEquals(Span(day(10, 5), day(10, 5)), taskSpan(timed(at(10, 5, 14, 30))))
        assertEquals(Span(day(10, 5), day(10, 8)), taskSpan(allDay(at(10, 5), at(10, 8))))
        assertEquals(Span(day(10, 5), day(10, 7)), taskSpan(timed(at(10, 5, 22), at(10, 7, 2))))
        // A timed block ending at midnight doesn't count that day.
        assertEquals(Span(day(10, 5), day(10, 6)), taskSpan(timed(at(10, 5, 9), at(10, 7))))
    }

    @Test
    fun dragSpanMovesAndResizes() {
        val span = Span(day(10, 5), day(10, 8))
        assertEquals(Span(day(10, 8), day(10, 11)), dragSpan(span, DragMode.MOVE, 3))
        assertEquals(Span(day(9, 30), day(10, 3)), dragSpan(span, DragMode.MOVE, -5))
        assertEquals(Span(day(10, 3), day(10, 8)), dragSpan(span, DragMode.START, -2))
        assertEquals(Span(day(10, 8), day(10, 8)), dragSpan(span, DragMode.START, 9))
        assertEquals(Span(day(10, 5), day(10, 10)), dragSpan(span, DragMode.END, 2))
        assertEquals(Span(day(10, 5), day(10, 5)), dragSpan(span, DragMode.END, -9))
    }

    @Test
    fun spanPatchRoundTrips() {
        val target = Span(day(11, 2), day(11, 4))
        for (t in listOf(allDay(at(10, 5)), allDay(at(10, 5), at(10, 8)), timed(at(10, 5, 9)), timed(at(10, 5, 9), at(10, 6, 17)))) {
            assertEquals(target, taskSpan(t.withDates(spanPatch(t, target))))
        }
    }

    @Test
    fun spanPatchKeepsAllDayAndTimes() {
        val multi = blank.withDates(spanPatch(allDay(at(10, 5)), Span(day(10, 5), day(10, 7))))
        val s = scheduleOf(multi)!!
        assertTrue(s.allDay)
        assertEquals(at(10, 5), s.start)
        assertEquals(LocalDateTime.of(2026, 10, 7, 23, 59, 59), s.end)

        val shrunk = spanPatch(allDay(at(10, 5), at(10, 8)), Span(day(10, 6), day(10, 6)))
        assertNull(shrunk.start)
        assertNull(shrunk.end)
        assertTrue(scheduleOf(blank.withDates(shrunk))!!.allDay)

        val moved = scheduleOf(blank.withDates(spanPatch(timed(at(10, 5, 9, 15), at(10, 6, 17, 45)), Span(day(10, 12), day(10, 14)))))!!
        assertEquals(at(10, 12, 9, 15), moved.start)
        assertEquals(at(10, 14, 17, 45), moved.end)

        // One day left with the end time before the start time: keep just the start.
        assertEquals(schedulePatch(at(10, 6, 22), false), spanPatch(timed(at(10, 5, 22), at(10, 6, 2)), Span(day(10, 6), day(10, 6))))
    }

    @Test
    fun rangePadsAroundTodayAndStretchesToTasks() {
        val today = day(10, 22)
        val (start, days) = timelineRange(emptyList(), TimelineScale.DAY, today, DayOfWeek.MONDAY)
        assertTrue(start <= day(10, 8))
        assertTrue(days > 56)
        assertEquals(DayOfWeek.MONDAY, start.dayOfWeek)
        assertTrue(timelineRange(listOf(Span(day(1, 3), day(1, 3))), TimelineScale.WEEK, today, DayOfWeek.MONDAY).first <= day(1, 3))
        val far = LocalDate.of(2010, 1, 1)
        assertEquals(2026, timelineRange(listOf(Span(far, far)), TimelineScale.WEEK, today, DayOfWeek.MONDAY).first.year)
        assertEquals(1, timelineRange(emptyList(), TimelineScale.MONTH, today, DayOfWeek.MONDAY).first.dayOfMonth)
    }

    @Test
    fun groupsSplitAtBoundaries() {
        val g = timelineGroups(day(9, 25), 40, GroupUnit.MONTH, DayOfWeek.MONDAY)
        assertEquals(listOf(0 to 6, 6 to 31, 37 to 3), g.map { it.offset to it.days })
        assertEquals(day(10, 1), g[1].date)
        for (unit in GroupUnit.entries) {
            val all = timelineGroups(day(9, 25), 400, unit, DayOfWeek.SUNDAY)
            assertEquals(400, all.sumOf { it.days })
            assertEquals(0, all.first().offset)
        }
    }
}
