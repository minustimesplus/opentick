package org.opentick.android

import android.graphics.drawable.GradientDrawable
import android.view.View
import android.widget.FrameLayout
import androidx.compose.ui.unit.DpSize
import androidx.compose.ui.unit.dp
import androidx.datastore.preferences.core.mutablePreferencesOf
import androidx.glance.ExperimentalGlanceApi
import androidx.glance.appwidget.GlanceAppWidget
import androidx.glance.appwidget.compose
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import org.junit.Assume.assumeTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.opentick.android.data.AccountMode
import org.opentick.android.data.ApiHabit
import org.opentick.android.data.ApiHabitCheckin
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.TaskEntity
import org.opentick.android.util.allDayMillis
import org.opentick.android.util.atTimeMillis
import org.opentick.android.util.today
import org.opentick.android.widget.AddTaskWidget
import org.opentick.android.widget.CalendarWidget
import org.opentick.android.widget.MatrixWidget
import org.opentick.android.widget.THEME_DARK
import org.opentick.android.widget.THEME_LIGHT
import org.opentick.android.widget.TasksWidget
import org.opentick.android.widget.TodayHabitsWidget
import org.opentick.android.widget.WeeklyHabitsWidget
import org.opentick.android.widget.WidgetKeys
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import java.time.LocalTime
import java.util.Locale

/**
 * Renders the home screen widgets on a wallpaper-like background, to compare
 * with TickTick's (help.ticktick.com, "Widgets", Android):
 *   ./gradlew testDebugUnitTest --tests '*WidgetScreenshotTest*' -Pscreenshots
 * PNGs land in app/build/screenshots/widget_*.png.
 */
@RunWith(org.robolectric.RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = "en-rGB-w393dp-h851dp-440dpi")
class WidgetScreenshotTest {
    private val app get() = ApplicationProvider.getApplicationContext<OpenTickApp>()

    @Before
    fun setUp() {
        assumeTrue(System.getProperty("opentick.screenshots") == "true")
        Locale.setDefault(Locale.UK)
        seed()
    }

    private fun seed() = runBlocking(Dispatchers.IO) {
        val db = app.db
        db.clearAllTables()
        app.prefs.setMode(AccountMode.SERVER)
        app.prefs.serverUrl = "https://example.invalid"
        val p = db.projects()
        val inbox = p.insert(ProjectEntity(serverId = 1, title = "Inbox", isInbox = true, position = -1.0))
        val work = p.insert(ProjectEntity(serverId = 2, title = "💼 Work", hexColor = "f6c84c", position = 1.0))
        val life = p.insert(ProjectEntity(serverId = 3, title = "🏡 Life", hexColor = "2fcf6f", position = 2.0))
        val t = today()
        fun day(n: Long) = allDayMillis(t.plusDays(n))
        fun at(n: Long, h: Int, m: Int) = atTimeMillis(t.plusDays(n), LocalTime.of(h, m))
        val tasks = db.tasks()
        tasks.insert(TaskEntity(serverId = 10, projectId = work, title = "New product promotion", dueDate = day(-2), priority = 0))
        tasks.insert(TaskEntity(serverId = 11, projectId = work, title = "Publicity materials", dueDate = day(0), parentServerId = 10))
        tasks.insert(TaskEntity(serverId = 12, projectId = work, title = "Guest invitation", dueDate = day(0), priority = 3, parentServerId = 10))
        tasks.insert(TaskEntity(serverId = 13, projectId = work, title = "Make a poster", dueDate = day(0), parentServerId = 11))
        tasks.insert(TaskEntity(serverId = 14, projectId = work, title = "Venue set-up", dueDate = at(0, 9, 0), priority = 3))
        tasks.insert(TaskEntity(serverId = 15, projectId = life, title = "Yoga lessons", dueDate = at(0, 18, 0), priority = 2))
        tasks.insert(TaskEntity(serverId = 16, projectId = inbox, title = "Organize my wardrobe", dueDate = day(0), priority = 1))
        tasks.insert(TaskEntity(serverId = 17, projectId = life, title = "Camping plan", dueDate = day(3), startDate = at(3, 10, 0), endDate = at(3, 12, 0)))
        tasks.insert(TaskEntity(serverId = 18, projectId = work, title = "Data analysis report", dueDate = day(1), priority = 3))
        tasks.insert(TaskEntity(serverId = 19, projectId = work, title = "Weekly Summary", dueDate = day(5), priority = 1))
        tasks.insert(TaskEntity(serverId = 20, projectId = life, title = "Dancing lessons 💃", dueDate = day(-4), priority = 0))

        fun recent(vararg daysAgo: Long) = daysAgo.map { ApiHabitCheckin(t.minusDays(it).toString(), 1) }
        app.habitWidgets.publish(
            listOf(
                ApiHabit(1, "Early to Rise", "☀️", "FAB005", position = 1.0, currentStreak = 0, recentCheckins = recent(1, 2, 3, 5)),
                ApiHabit(2, "Drink Water", "💧", "12B886", goal = 8, unit = "Cup", todayValue = 8, currentStreak = 4, position = 2.0, recentCheckins = recent(0, 1, 2, 3, 4)),
                ApiHabit(3, "Learn New Words", "📚", "4772FA", goal = 5, unit = "Page", todayValue = 3, position = 3.0, recentCheckins = recent(2, 4)),
                ApiHabit(4, "Daily Check-in", "✅", "40C057", todayValue = 1, currentStreak = 12, position = 4.0, recentCheckins = recent(0, 1, 3, 4, 5, 6)),
                ApiHabit(5, "Read", "📖", "15AABF", position = 5.0, recentCheckins = recent(1, 2, 6)),
            ),
        )
    }

    @OptIn(ExperimentalGlanceApi::class)
    private fun render(name: String, widget: GlanceAppWidget, w: Int, h: Int, theme: Int = THEME_DARK, list: String? = null) {
        val state = mutablePreferencesOf(WidgetKeys.THEME to theme, WidgetKeys.OPACITY to 88)
        if (list != null) state[WidgetKeys.LIST] = list
        val rv = runBlocking { widget.compose(app, size = DpSize(w.dp, h.dp), state = state) }
        val density = app.resources.displayMetrics.density
        val pad = (16 * density).toInt()
        val activity = org.robolectric.Robolectric.buildActivity(android.app.Activity::class.java).setup().get()
        val root = FrameLayout(activity).apply {
            background = GradientDrawable(GradientDrawable.Orientation.TL_BR, intArrayOf(0xFF6D5BD0.toInt(), 0xFF3E8EDC.toInt(), 0xFF7FB7E8.toInt()))
            setPadding(pad, pad, pad, pad)
        }
        val view = rv.apply(activity, root)
        root.addView(view, FrameLayout.LayoutParams((w * density).toInt(), (h * density).toInt()))
        activity.setContentView(root, android.view.ViewGroup.LayoutParams(((w + 32) * density).toInt(), ((h + 32) * density).toInt()))
        val spec = { px: Int -> View.MeasureSpec.makeMeasureSpec(px, View.MeasureSpec.EXACTLY) }
        val tw = ((w + 32) * density).toInt()
        val th = ((h + 32) * density).toInt()
        // Lists fill in over a few layout passes.
        repeat(3) {
            root.measure(spec(tw), spec(th))
            root.layout(0, 0, tw, th)
            org.robolectric.shadows.ShadowLooper.idleMainLooper()
        }
        val bitmap = android.graphics.Bitmap.createBitmap(tw, th, android.graphics.Bitmap.Config.ARGB_8888)
        root.draw(android.graphics.Canvas(bitmap))
        val out = java.io.File("build/screenshots/widget_$name.png").apply { parentFile?.mkdirs() }
        out.outputStream().use { bitmap.compress(android.graphics.Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test fun tasksToday() = render("tasks_today", TasksWidget(), 330, 290)
    @Test fun tasksLight() = render("tasks_light", TasksWidget(), 330, 290, theme = THEME_LIGHT)
    @Test fun tasksNext7() = render("tasks_next7", TasksWidget(), 250, 200, list = "week")
    @Test fun addTask() = render("add_task", AddTaskWidget(), 150, 150)
    @Test fun matrix() = render("matrix", MatrixWidget(), 330, 400)
    @Test fun calendar() = render("calendar", CalendarWidget(), 330, 360)
    @Test fun todayHabits() = render("today_habits", TodayHabitsWidget(), 330, 200)
    @Test fun weeklyHabits() = render("weekly_habits", WeeklyHabitsWidget(), 330, 240)
}
