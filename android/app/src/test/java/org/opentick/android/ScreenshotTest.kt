package org.opentick.android

import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.longClick
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTouchInput
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import com.github.takahirom.roborazzi.captureRoboImage
import kotlinx.coroutines.runBlocking
import org.junit.Assume.assumeTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.opentick.android.data.AccountMode
import org.opentick.android.data.LabelEntity
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.TaskEntity
import org.opentick.android.util.allDayMillis
import org.opentick.android.util.atTimeMillis
import org.opentick.android.util.today
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import java.time.LocalTime
import java.util.Locale

/**
 * Renders the main screens at the size and font scale of a typical phone
 * (1080x2340, 440 dpi, font scale 1.1) with data resembling a real account,
 * to compare against TickTick:
 *   ./gradlew testDebugUnitTest --tests '*ScreenshotTest*' -Pscreenshots
 * PNGs land in app/build/screenshots.
 */
@RunWith(org.robolectric.RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = "en-rGB-w393dp-h851dp-440dpi")
class ScreenshotTest {
    @get:Rule val compose = createEmptyComposeRule()

    private val app get() = ApplicationProvider.getApplicationContext<OpenTickApp>()

    @Before
    fun setUp() {
        assumeTrue(System.getProperty("opentick.screenshots") == "true")
        Locale.setDefault(Locale.UK)
        RuntimeEnvironment.setFontScale(1.1f)
        seed()
    }

    private fun seed() = runBlocking(kotlinx.coroutines.Dispatchers.IO) {
        val db = app.db
        db.clearAllTables()
        app.prefs.setMode(AccountMode.LOCAL)
        app.prefs.displayName = "Alex"
        val p = db.projects()
        val inbox = p.insert(ProjectEntity(title = "Inbox", isInbox = true, position = -1.0))
        p.insert(ProjectEntity(serverId = 100, title = "Calendar", position = 1.0))
        val lectures = p.insert(ProjectEntity(serverId = 101, title = "Lectures", hexColor = "f6c84c", parentServerId = 100, position = 1.1))
        val events = p.insert(ProjectEntity(serverId = 102, title = "Events", hexColor = "2fcf6f", parentServerId = 100, position = 1.2))
        val training = p.insert(ProjectEntity(serverId = 103, title = "Sport", hexColor = "dde84a", parentServerId = 100, position = 1.3))
        val spai = p.insert(ProjectEntity(title = "AI Society", position = 2.0))
        p.insert(ProjectEntity(title = "Club", position = 3.0))
        val personal = p.insert(ProjectEntity(title = "🚴 Personal", position = 4.0))
        val scpo = p.insert(ProjectEntity(title = "🏔️ University", hexColor = "02dab1", position = 5.0))
        p.insert(ProjectEntity(title = "🛒 Shopping", position = 6.0))
        p.insert(ProjectEntity(title = "💻 Side Project", position = 7.0))
        p.insert(ProjectEntity(title = "📒 Notes", position = 8.0))
        db.labels().insert(LabelEntity(title = "uni"))

        val t = today()
        fun day(n: Long) = allDayMillis(t.plusDays(n))
        fun at(n: Long, h: Int, m: Int) = atTimeMillis(t.plusDays(n), LocalTime.of(h, m))
        val tasks = db.tasks()
        val list = listOf(
            TaskEntity(projectId = scpo, title = "Email thesis advisor", dueDate = day(0), priority = 3),
            TaskEntity(projectId = scpo, title = "Mock trial preparation", dueDate = day(-5), priority = 2, description = "Notes"),
            TaskEntity(projectId = scpo, title = "Internship application", dueDate = day(-7), priority = 2),
            TaskEntity(projectId = scpo, title = "Schedule prize call", dueDate = day(-7), priority = 1),
            TaskEntity(projectId = scpo, title = "Follow up with the entrepreneurship centre", dueDate = day(-7), priority = 1),
            TaskEntity(projectId = scpo, title = "Read English article", dueDate = day(-7), priority = 2, description = "Link"),
            TaskEntity(projectId = scpo, title = "Housing application?", dueDate = day(-35), priority = 1, description = "Form"),
            TaskEntity(projectId = inbox, title = "Book train tickets", dueDate = day(1), priority = 3),
            TaskEntity(projectId = personal, title = "Scholarship essay", priority = 2),
            TaskEntity(projectId = personal, title = "Message mentor", priority = 2),
            TaskEntity(projectId = spai, title = "Partnership slides", priority = 2),
            TaskEntity(projectId = scpo, title = "sociology research project", dueDate = day(23)),
            TaskEntity(projectId = personal, title = "Reach out to research institutes"),
            TaskEntity(projectId = personal, title = "Plan social media post"),
            TaskEntity(projectId = personal, title = "Sign up to take language tests", dueDate = day(26)),
            TaskEntity(projectId = lectures, title = "Foreign Policy lecture", startDate = at(1, 10, 10), endDate = at(1, 12, 0)),
            TaskEntity(projectId = lectures, title = "Law seminar", startDate = at(1, 14, 30), endDate = at(1, 16, 30)),
            TaskEntity(projectId = events, title = "Conference", startDate = at(1, 18, 0), endDate = at(1, 19, 30)),
            TaskEntity(projectId = events, title = "Team lunch", startDate = at(2, 12, 30), endDate = at(2, 13, 30)),
            TaskEntity(projectId = training, title = "Training", startDate = at(3, 9, 0), endDate = at(3, 11, 30)),
            TaskEntity(projectId = training, title = "Volunteer meeting", dueDate = day(4)),
        )
        for (x in list) tasks.insert(x)
    }

    private fun shot(name: String) {
        compose.waitForIdle()
        compose.onRoot().captureRoboImage("build/screenshots/$name.png")
    }

    @Test
    fun screens() {
        ActivityScenario.launch(MainActivity::class.java).use {
            shot("1_today")
            compose.onNodeWithContentDescription("Open lists").performClick()
            shot("2_drawer")
            compose.onNodeWithContentDescription("Settings").performClick()
            shot("3_settings")
            compose.onNodeWithText("Tab Bar").performClick()
            shot("4_tabbar")
            it.onActivity { a -> a.onBackPressedDispatcher.onBackPressed(); a.onBackPressedDispatcher.onBackPressed() }
            compose.onNodeWithContentDescription("Eisenhower Matrix").performClick()
            shot("5_matrix")
            compose.onNodeWithContentDescription("Calendar").performClick()
            shot("6_calendar_week")
            compose.onNodeWithContentDescription("Calendar view").performClick()
            shot("7_calendar_menu")
            compose.onNodeWithText("Month").performClick()
            shot("8_calendar_month")
            compose.onNodeWithContentDescription("Calendar view").performClick()
            compose.onNodeWithText("Week").performClick()
            compose.onNodeWithContentDescription("Task").performClick()
            compose.onNodeWithContentDescription("Add task").performClick()
            shot("9_quick_add")
        }
    }

    @Test
    fun matrixAndTaskActions() {
        ActivityScenario.launch(MainActivity::class.java).use {
            compose.onNodeWithContentDescription("Eisenhower Matrix").performClick()
            compose.onNodeWithContentDescription("Quadrant I options").performClick()
            shot("10_matrix_quadrant_menu")
            compose.onNodeWithText("List").performClick()
            shot("11_matrix_grouped_by_list")
            compose.onNodeWithContentDescription("Edit matrix").performClick()
            shot("12_matrix_edit")
            compose.onNodeWithText("Done").performClick()
            compose.onNodeWithText("Email thesis advisor").performTouchInput { longClick() }
            shot("13_task_actions")
        }
    }
}
