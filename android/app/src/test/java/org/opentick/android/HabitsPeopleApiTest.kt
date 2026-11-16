package org.opentick.android

import android.app.Application
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import androidx.test.core.app.ApplicationProvider
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.opentick.android.data.AccountMode
import org.opentick.android.data.Api
import org.opentick.android.data.ApiException
import org.opentick.android.data.Prefs
import org.opentick.android.ui.habits.HabitInput
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import java.io.ByteArrayOutputStream
import java.time.LocalDate
import java.time.YearMonth

/**
 * Habits, weights and People against a running OpenTick server with local accounts:
 *   OPENTICK_TEST_SERVER=http://localhost:3456 ./gradlew testDebugUnitTest
 * Skipped when the variable isn't set.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(application = Application::class, sdk = [34])
class HabitsPeopleApiTest {
    private val server = System.getenv("OPENTICK_TEST_SERVER")
    private lateinit var api: Api
    private val today = LocalDate.now()

    @Before
    fun setUp() {
        assumeTrue("OPENTICK_TEST_SERVER not set", !server.isNullOrBlank())
        val ctx = ApplicationProvider.getApplicationContext<Application>()
        ctx.getSharedPreferences("opentick", 0).edit().clear().commit()
        val prefs = Prefs(ctx)
        api = Api(prefs)
        val user = "droidlife" + System.nanoTime().toString().takeLast(9)
        api.register(server!!, user, "$user@example.com", "password12345")
        val (token, refresh) = api.login(server, user, "password12345", null)
        prefs.serverUrl = server
        prefs.accessToken = token
        prefs.refreshToken = refresh
        prefs.setMode(AccountMode.SERVER)
    }

    @Test
    fun habitsCheckinsStatsAndWeights() {
        val t = today.toString()
        val water = api.createHabit(HabitInput("Water", "💧", "#15AABF", "daily", 0, 3, "glasses", false).toJson())
        assertEquals(127, water.weekdays)
        assertEquals("15AABF", water.hexColor.uppercase())
        val weigh = api.createHabit(HabitInput("Weigh in", "⚖️", "40C057", "weekdays", 0b0111110, 1, "", true).toJson())
        assertEquals("weekdays", weigh.frequency)
        assertTrue(weigh.position > water.position)

        assertEquals(2, api.setCheckin(water.id, t, 2, t).todayValue)
        val yesterday = today.minusDays(1).toString()
        val after = api.setCheckin(water.id, yesterday, 3, t)
        assertEquals(3, after.recentCheckins.orEmpty().first { it.date == yesterday }.value)

        val stats = api.habitStats(water.id, t, YearMonth.from(today).toString())
        assertEquals(2, stats.totalCheckins)
        assertEquals(YearMonth.from(today).lengthOfMonth(), stats.days.orEmpty().size)

        // Logging a weight checks off the weigh-in habit.
        val w = api.setWeight(t, 72.44, t)
        assertEquals(72.4, w.weight, 1e-9)
        assertEquals(1, api.habits(t).first { it.id == weigh.id }.todayValue)
        assertEquals(listOf(72.4), api.weights(today.minusDays(30).toString(), t).map { it.weight })
        api.deleteWeight(t)
        assertTrue(api.weights(today.minusDays(30).toString(), t).isEmpty())

        val moved = api.updateHabit(weigh.id, buildJsonObject { put("position", water.position / 2); put("archived", true) })
        assertTrue(moved.archived)
        assertEquals("Weigh in", moved.title) // merge patch keeps the rest
        assertEquals(listOf(weigh.id, water.id), api.habits(t).sortedBy { it.position }.map { it.id })

        api.setCheckin(water.id, t, 0, t)
        assertEquals(0, api.habits(t).first { it.id == water.id }.todayValue)
        api.deleteHabit(weigh.id)
        assertEquals(listOf(water.id), api.habits(t).map { it.id })
    }

    @Test
    fun contactsGroupsTagsHistoryAndPhoto() {
        val ada = api.createContact(buildJsonObject { put("full_name", "Ada Lovelace"); put("email", "ada@example.com") }, false)
        assertEquals("Ada Lovelace", ada.name)
        val dup = runCatching { api.createContact(buildJsonObject { put("email", "ada@example.com") }, false) }.exceptionOrNull()
        assertEquals(409, (dup as ApiException).status)
        api.createContact(buildJsonObject { put("email", "ada@example.com") }, true)

        val edited = api.updateContact(ada.id, buildJsonObject { put("company", "Analytical Engines") })
        assertEquals("Analytical Engines", edited.company)
        assertTrue("company" in edited.lockedFields.orEmpty())
        val unlocked = api.updateContact(ada.id, buildJsonObject { put("locked_fields", JsonArray(edited.lockedFields.orEmpty().filter { it != "company" }.map { JsonPrimitive(it) })) })
        assertFalse("company" in unlocked.lockedFields.orEmpty())

        api.setContactGroup("Friends", listOf(ada.id), on = true, create = true)
        api.setContactTag("math", listOf(ada.id), on = true)
        var o = api.contactOverview()
        val friends = o.groups.orEmpty().single { it.name == "Friends" }
        val math = o.tags.orEmpty().single { it.name == "math" }
        assertEquals(listOf(friends.id), o.contacts.orEmpty().first { it.id == ada.id }.groupIds)
        api.setContactGroup(friends.id.toString(), listOf(ada.id), on = false)
        api.setContactTag(math.id.toString(), listOf(ada.id), on = false)
        assertTrue(api.contact(ada.id).groupIds.orEmpty().isEmpty())

        api.saveContactEntry(ada.id, "experiences", null, buildJsonObject { put("company", "Babbage & Co"); put("title", "Analyst"); put("start_date", "1842") })
        api.saveContactEntry(ada.id, "educations", null, buildJsonObject { put("school", "Home"); put("degree", "Mathematics") })
        api.saveContactEntry(ada.id, "notes", null, buildJsonObject { put("body", "Wrote the first program") })
        var full = api.contact(ada.id)
        val note = full.notes.orEmpty().single()
        api.saveContactEntry(ada.id, "notes", note.id, buildJsonObject { put("body", "Wrote the first published program") })
        api.deleteContactEntry(ada.id, "educations", full.educations.orEmpty().single().id)
        full = api.contact(ada.id)
        assertEquals("Analyst", full.experiences.orEmpty().single().title)
        assertTrue(full.educations.orEmpty().isEmpty())
        assertEquals("Wrote the first published program", full.notes.orEmpty().single().body)
        assertEquals(listOf("Babbage & Co", "Analyst"), listOf(full.experiences!!.single().company, full.experiences!!.single().title))

        val bmp = Bitmap.createBitmap(64, 64, Bitmap.Config.ARGB_8888).apply { eraseColor(android.graphics.Color.RED) }
        val jpeg = ByteArrayOutputStream().use { bmp.compress(Bitmap.CompressFormat.JPEG, 90, it); it.toByteArray() }
        api.uploadContactPhoto(ada.id, jpeg, "image/jpeg")
        assertTrue(api.contact(ada.id).hasPhoto)
        val photo = api.contactPhoto(ada.id)
        assertNotNull(BitmapFactory.decodeByteArray(photo, 0, photo.size))
        api.deleteContactPhoto(ada.id)
        assertFalse(api.contact(ada.id).hasPhoto)

        val g = api.saveContactGroup(null, "Work", "4772FA", "Colleagues")
        assertEquals("Colleagues", api.saveContactGroup(g.id, "Work", "E03131", "Colleagues").description)
        api.deleteContactGroup(g.id)
        api.deleteContactGroup(friends.id)
        api.deleteContactTag(math.id)
        api.deleteContact(ada.id)
        o = api.contactOverview()
        assertTrue(o.groups.orEmpty().isEmpty())
        assertEquals(1, o.contacts.orEmpty().size)
    }
}
