package org.opentick.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.opentick.android.util.QuickAddParser
import java.time.LocalDate
import java.time.LocalTime

class QuickAddParserTest {
    private val now = LocalDate.of(2026, 11, 11) // a Wednesday

    @Test
    fun plainTitle() {
        val p = QuickAddParser.parse("Buy milk", now)
        assertEquals("Buy milk", p.title)
        assertNull(p.date)
        assertNull(p.priority)
    }

    @Test
    fun dateTimePriorityTagsList() {
        val p = QuickAddParser.parse("Call mom tomorrow 5pm #family #phone !high ~Personal", now)
        assertEquals("Call mom", p.title)
        assertEquals(now.plusDays(1), p.date)
        assertEquals(LocalTime.of(17, 0), p.time)
        assertEquals(3, p.priority)
        assertEquals(listOf("family", "phone"), p.labels)
        assertEquals("Personal", p.list)
    }

    @Test
    fun weekdayAndTwentyFourHour() {
        val p = QuickAddParser.parse("Standup monday at 09:30", now)
        assertEquals("Standup", p.title)
        assertEquals(LocalDate.of(2026, 11, 16), p.date)
        assertEquals(LocalTime.of(9, 30), p.time)
    }

    @Test
    fun inDaysAndShortPriority() {
        val p = QuickAddParser.parse("Renew passport in 3 days !m", now)
        assertEquals("Renew passport", p.title)
        assertEquals(now.plusDays(3), p.date)
        assertEquals(2, p.priority)
    }

    @Test
    fun hashInsideWordIsKept() {
        val p = QuickAddParser.parse("Fix C# build", now)
        assertEquals("Fix C# build", p.title)
        assertEquals(emptyList<String>(), p.labels)
    }
}
