package org.opentick.android

import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollToNode
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.click
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import com.github.takahirom.roborazzi.captureRoboImage
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import org.junit.Assume.assumeTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.opentick.android.data.AccountMode
import org.opentick.android.ui.habits.HabitInput
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import java.time.LocalDate
import java.util.Locale
import org.opentick.android.ui.people.DeviceContact
import org.opentick.android.ui.people.DeviceNumber

/**
 * Renders the Habits and People tabs with data from a running server, on its own
 * (it signs in app-wide, which the other screenshot tests don't expect):
 *   OPENTICK_SCREENSHOT_SERVER=http://localhost:3456 ./gradlew testDebugUnitTest --tests '*HabitsPeopleScreenshotTest*' -Pscreenshots
 * Signs up a fresh account and fills it. PNGs land in app/build/screenshots.
 */
@RunWith(org.robolectric.RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = "en-rGB-w393dp-h851dp-440dpi")
class HabitsPeopleScreenshotTest {
    @get:Rule val compose = createEmptyComposeRule()

    private val server = System.getenv("OPENTICK_SCREENSHOT_SERVER")
    private val app get() = ApplicationProvider.getApplicationContext<OpenTickApp>()

    @Before
    fun setUp() {
        assumeTrue(System.getProperty("opentick.screenshots") == "true" && !server.isNullOrBlank())
        Locale.setDefault(Locale.UK)
        RuntimeEnvironment.setFontScale(1.1f)
        val api = app.api
        val prefs = app.prefs
        val user = "droidshot" + System.nanoTime().toString().takeLast(9)
        api.register(server!!, user, "$user@example.com", "password12345")
        val (token, refresh) = api.login(server, user, "password12345", null)
        prefs.serverUrl = server
        prefs.accessToken = token
        prefs.refreshToken = refresh
        prefs.setMode(AccountMode.SERVER)
        prefs.setHabitWeightOpen(true)
        seed()
    }

    private fun seed() {
        val api = app.api
        val today = LocalDate.now()
        val t = today.toString()
        val water = api.createHabit(HabitInput("Drink water", "💧", "15AABF", "daily", 0, 8, "glasses", false).toJson())
        val read = api.createHabit(HabitInput("Read 20 minutes", "📚", "7048E8", "daily", 0, 1, "", false).toJson())
        val gym = api.createHabit(HabitInput("Workout", "💪", "F76707", "weekdays", 0b0101010, 1, "", false).toJson())
        val weigh = api.createHabit(HabitInput("Weigh in", "⚖️", "40C057", "daily", 0, 1, "", true).toJson())
        for (d in 1L..12L) {
            val day = today.minusDays(d)
            api.setCheckin(water.id, day.toString(), if (d % 5 == 0L) 5 else 8, t)
            if (d % 3 != 0L) api.setCheckin(read.id, day.toString(), 1, t)
            if (gym.weekdays and (1 shl (day.dayOfWeek.value % 7)) != 0) api.setCheckin(gym.id, day.toString(), 1, t)
        }
        api.setCheckin(water.id, t, 5, t)
        api.setCheckin(read.id, t, 1, t)
        for (d in 30L downTo 1L step 3) api.setWeight(today.minusDays(d).toString(), 74.6 - (30 - d) * 0.05 + (d % 2) * 0.2, t)
        check(weigh.tracksWeight)

        fun contact(name: String, title: String, company: String, location: String, email: String = "", phone: String = "") =
            api.createContact(buildJsonObject {
                put("full_name", name); put("title", title); put("company", company); put("location", location)
                if (email.isNotEmpty()) put("email", email)
                if (phone.isNotEmpty()) put("phone", phone)
            }, false)
        val ada = contact("Ada Lovelace", "Analyst", "Analytical Engines", "London, UK", "ada@example.com", "+44 20 7946 0000")
        contact("Alan Turing", "Researcher", "Bletchley Park", "Milton Keynes, UK")
        val grace = contact("Grace Hopper", "Rear Admiral", "US Navy", "Arlington, VA", "grace@example.com")
        contact("Katherine Johnson", "Mathematician", "NASA", "Hampton, VA")
        contact("Edsger Dijkstra", "Professor", "UT Austin", "Austin, TX")
        api.setContactGroup("Mentors", listOf(ada.id, grace.id), on = true, create = true)
        api.setContactGroup("Friends", listOf(ada.id), on = true, create = true)
        api.setContactTag("math", listOf(ada.id), on = true)
        api.saveContactEntry(ada.id, "experiences", null, buildJsonObject { put("company", "Analytical Engines"); put("title", "Analyst"); put("start_date", "1842") })
        api.saveContactEntry(ada.id, "educations", null, buildJsonObject { put("school", "University of London"); put("degree", "Mathematics"); put("start_year", "1840"); put("end_year", "1842") })
        api.saveContactEntry(ada.id, "notes", null, buildJsonObject { put("body", "Ask about the notes on the Analytical Engine.") })
    }

    private fun waitFor(text: String) = compose.waitUntil(20_000) { compose.onAllNodesWithText(text, substring = true).fetchSemanticsNodes().isNotEmpty() }

    private fun shot(name: String) {
        compose.waitForIdle()
        compose.onRoot().captureRoboImage("build/screenshots/$name.png")
    }

    @Test
    fun habitsAndPeople() {
        ActivityScenario.launch(MainActivity::class.java).use {
            compose.onNodeWithContentDescription("Habit").performClick()
            waitFor("Drink water")
            waitFor("kg in")
            shot("habits_1_list")
            // The row's middle holds the day dots; tap its title side to open it.
            compose.onNodeWithText("Drink water").performTouchInput { click(androidx.compose.ui.geometry.Offset(width * 0.2f, centerY)) }
            waitFor("Longest streak")
            compose.waitUntil(20_000) { compose.onAllNodesWithText("Total check-ins").fetchSemanticsNodes().isNotEmpty() }
            shot("habits_2_detail")
            compose.onNodeWithContentDescription("Edit habit").performClick()
            waitFor("Weigh-in habit")
            shot("habits_3_edit")
            it.onActivity { a -> a.onBackPressedDispatcher.onBackPressed() }
            compose.waitForIdle()
            it.onActivity { a -> a.onBackPressedDispatcher.onBackPressed() }

            compose.onNodeWithContentDescription("People").performClick()
            waitFor("Grace Hopper")
            shot("people_1_list")
            compose.onNodeWithText("Ada Lovelace").performClick()
            waitFor("Add details")
            shot("people_2_detail")
            // [0] is the contact page; the other scrollable list is the closed drawer.
            compose.onAllNodes(androidx.compose.ui.test.hasScrollToNodeAction())[0]
                .performScrollToNode(androidx.compose.ui.test.hasText("Ask about the notes", substring = true))
            waitFor("University of London")
            shot("people_3_history")
        }
    }

    @Test
    fun phoneContacts() {
        ActivityScenario.launch(MainActivity::class.java).use {
            compose.onNodeWithContentDescription("People").performClick()
            waitFor("Grace Hopper")
            compose.onNodeWithContentDescription("Numbers from phone contacts").performClick()
            waitFor("Compare phone contacts")
            shot("people_4_phone_intro")
            // Robolectric has no address book; hand the store the phone's contacts.
            it.onActivity { a ->
                androidx.lifecycle.ViewModelProvider(a)[org.opentick.android.ui.MainViewModel::class.java].people.scanPhone {
                    listOf(
                        DeviceContact("Alan Turing", numbers = listOf(DeviceNumber("+44 7700 900461", "Mobile"))),
                        DeviceContact("Amazing Grace", listOf("grace@example.com"), listOf(DeviceNumber("+1 703 555 0142", "Work"), DeviceNumber("+1 703 555 0199", "Mobile"))),
                        DeviceContact("Ada Lovelace", numbers = listOf(DeviceNumber("+44 7700 900815", "Mobile"))),
                        DeviceContact("Dijkstra, Edsger", numbers = listOf(DeviceNumber("+1 512 555 0110", "Home"))),
                        DeviceContact("Pizza place", numbers = listOf(DeviceNumber("+44 20 7946 0999", "Work"))),
                    )
                }
            }
            waitFor("suggestions")
            shot("people_5_phone_suggestions")
            compose.onAllNodesWithText("Add number")[0].performClick()
            waitFor("Added the number")
            shot("people_6_phone_added")
        }
    }
}
