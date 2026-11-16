package org.opentick.android

import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.opentick.android.data.ApiContactOverview
import org.opentick.android.data.ApiContactRow
import org.opentick.android.data.ApiHabit
import org.opentick.android.data.ApiHabitCheckin
import org.opentick.android.data.ApiWeight
import org.opentick.android.ui.habits.fmtWeight
import org.opentick.android.ui.habits.isDue
import org.opentick.android.ui.habits.movedPosition
import org.opentick.android.ui.habits.parseWeight
import org.opentick.android.ui.habits.tapValue
import org.opentick.android.ui.habits.valueOn
import org.opentick.android.ui.habits.weightChange
import org.opentick.android.ui.habits.withValue
import org.opentick.android.ui.people.ContactSort
import org.opentick.android.ui.people.PeopleFilter
import org.opentick.android.ui.people.filterContacts
import org.opentick.android.ui.people.fold
import org.opentick.android.ui.people.initials
import org.opentick.android.ui.people.nameColor
import org.opentick.android.ui.people.sortContacts
import java.time.LocalDate

class HabitsPeopleTest {
    private val today = LocalDate.of(2026, 11, 16) // a Monday

    @Test
    fun weekdaysMaskHasSundayAsBitZero() {
        val weekends = ApiHabit(frequency = "weekdays", weekdays = 0b1000001)
        assertTrue(weekends.isDue(LocalDate.of(2026, 11, 15))) // Sunday
        assertTrue(weekends.isDue(LocalDate.of(2026, 11, 14))) // Saturday
        assertFalse(weekends.isDue(today))
        assertTrue(ApiHabit().isDue(today))
    }

    @Test
    fun tapTogglesYesNoAndCountsUpToTheGoal() {
        val yesNo = ApiHabit(goal = 1)
        assertEquals(1, yesNo.tapValue(0))
        assertEquals(0, yesNo.tapValue(1))
        val water = ApiHabit(goal = 3)
        assertEquals(1, water.tapValue(0))
        assertEquals(3, water.tapValue(2))
        assertEquals(0, water.tapValue(3))
    }

    @Test
    fun valuesComeFromTodayOrTheRecentCheckins() {
        val h = ApiHabit(todayValue = 2, recentCheckins = listOf(ApiHabitCheckin("2026-11-14", 5)))
        assertEquals(2, h.valueOn(today, today))
        assertEquals(5, h.valueOn(LocalDate.of(2026, 11, 14), today))
        assertEquals(0, h.valueOn(LocalDate.of(2026, 11, 13), today))

        val cleared = h.withValue(LocalDate.of(2026, 11, 14), 0, today)
        assertEquals(0, cleared.valueOn(LocalDate.of(2026, 11, 14), today))
        assertEquals(2, cleared.todayValue)
        assertEquals(4, h.withValue(today, 4, today).todayValue)
    }

    @Test
    fun movingTakesTheMeanOfTheNewNeighbours() {
        val a = ApiHabit(id = 1, position = 100.0)
        val b = ApiHabit(id = 2, position = 200.0)
        val c = ApiHabit(id = 3, position = 300.0)
        val all = listOf(a, b, c)
        assertEquals(50.0, movedPosition(all, c, 0)!!, 0.0)
        assertEquals(150.0, movedPosition(all, c, 1)!!, 0.0)
        assertEquals(300.0 + 65536.0, movedPosition(all, a, 2)!!, 0.0)
        assertNull(movedPosition(listOf(a), a, 0))
    }

    @Test
    fun weightChangeAndFormatting() {
        val w = listOf(ApiWeight("2026-10-01", 80.0), ApiWeight("2026-11-10", 79.2), ApiWeight("2026-11-16", 78.9))
        assertEquals(-0.3, weightChange(w, 7, today)!!, 1e-9)
        assertEquals(-0.3, weightChange(w, 30, today)!!, 1e-9)
        assertEquals(-1.1, weightChange(w, 60, today)!!, 1e-9)
        assertNull(weightChange(w.take(1), 30, today))
        assertEquals("72.4", fmtWeight(72.4))
        assertEquals("+0.3", fmtWeight(0.3, true))
        assertEquals("−0.3", fmtWeight(-0.3, true))
        assertEquals("0.0", fmtWeight(0.0, true))
        assertEquals("–", fmtWeight(null))
        assertEquals(72.4, parseWeight("72,4")!!, 0.0)
        assertNull(parseWeight("0"))
        assertNull(parseWeight("abc"))
    }

    @Test
    fun foldIgnoresCaseAccentsAndPunctuation() {
        assertEquals(fold("SciencesPo"), fold("Sciences Po"))
        assertEquals(fold("sciences-po"), fold("Sciences Po"))
        assertEquals("munchen", fold("München"))
        assertEquals("strasse", fold("Straße"))
    }

    @Test
    fun initialsAndColorsMatchTheWeb() {
        assertEquals("AL", initials("Ada Lovelace"))
        assertEquals("AL", initials("ada  van lovelace"))
        assertEquals("CH", initials("Cher"))
        assertEquals("?", initials(" "))
        assertEquals("ABCDEF", nameColor("x", "#ABCDEF"))
        // Stable for a name, from the same palette as the web.
        assertEquals(nameColor("Ada Lovelace"), nameColor("Ada Lovelace"))
    }

    @Test
    fun searchNeedsEveryWordAndFiltersByGroupOrTag() {
        val ada = ApiContactRow(id = 1, name = "Ada Lovelace", company = "Analytical Engines", groupIds = listOf(10), schools = listOf("Sciences Po"))
        val bob = ApiContactRow(id = 2, name = "Bob", company = "", location = "Reims", tagIds = listOf(20))
        val all = listOf(ada, bob)
        val tags = mapOf(20L to "climbing")
        val groups = mapOf(10L to "Friends")
        assertEquals(listOf(ada), filterContacts(all, "ada sciencespo", PeopleFilter.All, tags, groups))
        assertEquals(listOf(bob), filterContacts(all, "climbing", PeopleFilter.All, tags, groups))
        assertEquals(listOf(ada), filterContacts(all, "friends", PeopleFilter.All, tags, groups))
        assertEquals(listOf(ada), filterContacts(all, "", PeopleFilter.Group(10), tags, groups))
        assertEquals(listOf(bob), filterContacts(all, "", PeopleFilter.Tag(20), tags, groups))
        assertTrue(filterContacts(all, "ada", PeopleFilter.Tag(20), tags, groups).isEmpty())
    }

    @Test
    fun sortingPutsContactsWithoutTheFieldLast() {
        val a = ApiContactRow(id = 1, name = "Zoe", company = "Acme", updated = "2026-01-01T00:00:00Z")
        val b = ApiContactRow(id = 2, name = "Adam", company = "", updated = "2026-06-01T00:00:00Z")
        val c = ApiContactRow(id = 3, name = "émile", company = "beta", updated = "2026-03-01T00:00:00Z")
        assertEquals(listOf(2L, 3L, 1L), sortContacts(listOf(a, b, c), ContactSort.NAME).map { it.id })
        assertEquals(listOf(1L, 3L, 2L), sortContacts(listOf(a, b, c), ContactSort.COMPANY).map { it.id })
        assertEquals(listOf(2L, 3L, 1L), sortContacts(listOf(a, b, c), ContactSort.RECENT).map { it.id })
    }

    @Test
    fun decodesTheServerShapes() {
        val json = Json { ignoreUnknownKeys = true; explicitNulls = false; coerceInputValues = true }
        val habit = json.decodeFromString<ApiHabit>(
            """{"id":4,"title":"Water","icon":"💧","hex_color":"15AABF","frequency":"daily","weekdays":127,"goal":8,"unit":"glasses",
               "archived":false,"tracks_weight":false,"position":65536,"today_value":3,"current_streak":5,
               "recent_checkins":[{"habit_id":4,"date":"2026-11-15","value":8}],"created":"x","updated":"y"}""",
        )
        assertEquals(8, habit.goal)
        assertEquals(8, habit.valueOn(LocalDate.of(2026, 11, 15), today))
        val o = json.decodeFromString<ApiContactOverview>(
            """{"contacts":[{"id":1,"name":"Ada","tag_ids":null,"group_ids":[2],"schools":null,"has_photo":true}],
               "groups":[{"id":2,"name":"Friends","members":1}],"tags":null}""",
        )
        assertEquals(listOf(2L), o.contacts!!.single().groupIds)
        assertTrue(o.contacts!!.single().hasPhoto)
        assertNull(o.tags)
    }
}
