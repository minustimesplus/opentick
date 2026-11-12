package org.opentick.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test
import org.opentick.android.data.ApiCalendarEvent
import org.opentick.android.data.ApiCalendarSubscription
import org.opentick.android.data.TaskEntity
import org.opentick.android.ui.Data
import org.opentick.android.ui.calendar.DEFAULT_CALENDAR_COLOR
import org.opentick.android.ui.calendar.ExtEvent
import org.opentick.android.ui.calendar.eventItemOn
import org.opentick.android.ui.calendar.eventTimeOn
import org.opentick.android.ui.calendar.itemsOn
import org.opentick.android.ui.calendar.toExtEvent
import java.time.LocalDate
import java.time.LocalDateTime
import java.time.ZoneId
import java.util.Locale

class CalendarEventsTest {
    private fun day(m: Int, d: Int) = LocalDate.of(2026, m, d)
    private fun at(m: Int, d: Int, h: Int = 0, min: Int = 0) = LocalDateTime.of(2026, m, d, h, min)
    private fun event(start: LocalDateTime, end: LocalDateTime, allDay: Boolean = false, title: String = "E") =
        ExtEvent("1:$title", 1, "Cal", title, "", "", "", start, end, allDay, false, "ff0000")

    @Test
    fun parsesApiEventsIntoLocalTime() {
        val sub = ApiCalendarSubscription(id = 7, title = "Holidays", url = "https://x/ics", hexColor = "")
        val api = ApiCalendarEvent(id = "u1", title = "Meet", start = "2026-11-12T08:00:00Z", end = "2026-11-12T09:30:00.5Z")
        val e = api.toExtEvent(sub, ZoneId.of("Europe/Berlin"))!!
        assertEquals("7:u1", e.key)
        assertEquals(at(11, 12, 9), e.start)
        assertEquals(at(11, 12, 10, 30).plusNanos(500_000_000), e.end)
        assertEquals("Holidays", e.calendar)
        assertEquals(DEFAULT_CALENDAR_COLOR, e.hexColor)
        assertNull(ApiCalendarEvent(id = "bad", start = "nope").toExtEvent(sub))
    }

    @Test
    fun timedEventIsABlockOnItsDay() {
        val e = event(at(11, 12, 9), at(11, 12, 10, 30))
        val item = eventItemOn(day(11, 12), e)!!
        assertEquals(9 * 60, item.startMin)
        assertEquals(10 * 60 + 30, item.endMin)
        assertNull(eventItemOn(day(11, 11), e))
        assertNull(eventItemOn(day(11, 13), e))
    }

    @Test
    fun allDayEventsEndBeforeTheirExclusiveMidnight() {
        val e = event(at(11, 12), at(11, 14), allDay = true)
        assertNotNull(eventItemOn(day(11, 12), e))
        assertNull(eventItemOn(day(11, 12), e)!!.startMin)
        assertNotNull(eventItemOn(day(11, 13), e))
        assertNull(eventItemOn(day(11, 14), e))
        assertEquals("All day", eventTimeOn(e, day(11, 13)))
    }

    @Test
    fun multiDayTimedEventSpansDays() {
        val e = event(at(11, 12, 22), at(11, 14, 2))
        val first = eventItemOn(day(11, 12), e)!!
        assertEquals(22 * 60, first.startMin)
        assertEquals(24 * 60, first.endMin)
        assertNull(eventItemOn(day(11, 13), e)!!.startMin) // the full middle day is an all-day chip
        val last = eventItemOn(day(11, 14), e)!!
        assertEquals(0, last.startMin)
        assertEquals(2 * 60, last.endMin)
        // Ending at midnight doesn't reach into the next day.
        val toMidnight = event(at(11, 12, 20), at(11, 13))
        assertEquals(24 * 60, eventItemOn(day(11, 12), toMidnight)!!.endMin)
        assertNull(eventItemOn(day(11, 13), toMidnight))
    }

    @Test
    fun shortEventsGetAMinimumHeight() {
        val e = event(at(11, 12, 9), at(11, 12, 9))
        assertEquals(9 * 60 + 15, eventItemOn(day(11, 12), e)!!.endMin)
    }

    @Test
    fun timesInTheListView() {
        val prev = Locale.getDefault()
        Locale.setDefault(Locale.GERMANY)
        try {
            assertEquals("09:00 – 10:30", eventTimeOn(event(at(11, 12, 9), at(11, 12, 10, 30)), day(11, 12)))
            val long = event(at(11, 12, 22), at(11, 14, 2))
            assertEquals("From 22:00", eventTimeOn(long, day(11, 12)))
            assertEquals("All day", eventTimeOn(long, day(11, 13)))
            assertEquals("Until 02:00", eventTimeOn(long, day(11, 14)))
        } finally {
            Locale.setDefault(prev)
        }
    }

    @Test
    fun itemsOnMixesTasksAndEvents() {
        val task = TaskEntity(id = 1, projectId = 1, title = "Task", dueDate = at(11, 12, 8).atZone(ZoneId.systemDefault()).toInstant().toEpochMilli())
        val items = itemsOn(day(11, 12), Data(tasks = listOf(task)), true, listOf(event(at(11, 12, 7), at(11, 12, 7, 30), title = "Early")))
        assertEquals(listOf("Early", "Task"), items.map { it.title })
        assertNotNull(items[0].event)
        assertNotNull(items[1].task)
    }
}
