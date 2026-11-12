package org.opentick.android

import org.junit.Assert.assertEquals
import org.junit.Test
import org.opentick.android.ui.calendar.HOUR_DP
import org.opentick.android.ui.calendar.HourScale
import org.opentick.android.ui.calendar.NIGHT_DP
import org.opentick.android.ui.calendar.initialTimelineScrollDp
import java.time.LocalTime

/** Same mapping as web/src/components/calendar/layout.ts (yOf / minOf). */
class HourScaleTest {
    private val folded = HourScale(collapsed = true)
    private val full = HourScale(collapsed = false)

    @Test
    fun fullDayIsLinear() {
        assertEquals(24 * HOUR_DP, full.totalDp, 0.01f)
        assertEquals(9 * HOUR_DP, full.y(9 * 60), 0.01f)
        assertEquals(9 * 60 + 30, full.minAt(9.5f * HOUR_DP))
        assertEquals((1..23).toList(), full.hourLines)
    }

    @Test
    fun nightHoursFoldIntoBands() {
        // 0–6 and 21–24 are one band each, the 15 day hours keep their height.
        assertEquals(2 * NIGHT_DP + 15 * HOUR_DP, folded.totalDp, 0.01f)
        assertEquals(0f, folded.y(0), 0.01f)
        assertEquals(NIGHT_DP / 2, folded.y(3 * 60), 0.01f)
        assertEquals(NIGHT_DP, folded.y(6 * 60), 0.01f)
        assertEquals(NIGHT_DP + 3 * HOUR_DP, folded.y(9 * 60), 0.01f)
        assertEquals(NIGHT_DP + 15 * HOUR_DP, folded.y(21 * 60), 0.01f)
        assertEquals(folded.totalDp, folded.y(24 * 60), 0.01f)
        assertEquals((7..20).toList(), folded.hourLines)
    }

    @Test
    fun minAtInvertsY() {
        for (min in listOf(0, 90, 6 * 60, 9 * 60 + 30, 14 * 60, 21 * 60, 22 * 60 + 30)) {
            assertEquals(min, folded.minAt(folded.y(min) + 0.001f))
            assertEquals(min, full.minAt(full.y(min) + 0.001f))
        }
        assertEquals(24 * 60 - 1, folded.minAt(folded.totalDp + 50))
    }

    @Test
    fun startsAtTheTopWhenFolded() {
        assertEquals(0f, initialTimelineScrollDp(true, LocalTime.of(15, 0)), 0.01f)
        assertEquals(11 * HOUR_DP, initialTimelineScrollDp(false, LocalTime.of(15, 0)), 0.01f)
    }
}
