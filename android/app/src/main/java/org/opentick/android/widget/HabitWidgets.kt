package org.opentick.android.widget

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.datastore.preferences.core.Preferences
import androidx.glance.ColorFilter
import androidx.glance.GlanceId
import androidx.glance.GlanceModifier
import androidx.glance.Image
import androidx.glance.ImageProvider
import androidx.glance.LocalSize
import androidx.glance.action.Action
import androidx.glance.action.ActionParameters
import androidx.glance.action.actionParametersOf
import androidx.glance.action.clickable
import androidx.glance.appwidget.GlanceAppWidget
import androidx.glance.appwidget.GlanceAppWidgetReceiver
import androidx.glance.appwidget.SizeMode
import androidx.glance.appwidget.action.ActionCallback
import androidx.glance.appwidget.action.actionRunCallback
import androidx.glance.appwidget.cornerRadius
import androidx.glance.appwidget.provideContent
import androidx.glance.background
import androidx.glance.currentState
import androidx.glance.layout.Alignment
import androidx.glance.layout.Box
import androidx.glance.layout.Column
import androidx.glance.layout.ColumnScope
import androidx.glance.layout.Row
import androidx.glance.layout.Spacer
import androidx.glance.layout.fillMaxHeight
import androidx.glance.layout.fillMaxSize
import androidx.glance.layout.fillMaxWidth
import androidx.glance.layout.height
import androidx.glance.layout.padding
import androidx.glance.layout.size
import androidx.glance.layout.width
import androidx.glance.state.GlanceStateDefinition
import androidx.glance.state.PreferencesGlanceStateDefinition
import androidx.glance.text.Text
import androidx.glance.text.TextAlign
import androidx.glance.text.TextStyle
import androidx.glance.unit.ColorProvider
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import org.opentick.android.OpenTickApp
import org.opentick.android.R
import org.opentick.android.data.AccountMode
import org.opentick.android.data.ApiHabit
import org.opentick.android.ui.calendar.weekStartDay
import org.opentick.android.ui.habits.DEFAULT_HABIT_COLOR
import org.opentick.android.ui.habits.HabitsStore
import org.opentick.android.ui.habits.isDue
import org.opentick.android.ui.habits.tapValue
import org.opentick.android.ui.habits.withValue
import org.opentick.android.ui.theme.parseHex
import java.time.LocalDate
import java.time.format.TextStyle as DateTextStyle
import java.util.Locale

// Widgets for habits. Habits live on the server, so the widgets show the last
// list they loaded (kept on the device) and reload it when they update. A
// check-in shows at once and rolls back if the server refuses it, like the tab.

@Serializable
data class HabitsSnapshot(
    val server: String = "",
    /** The day the habits were loaded on (YYYY-MM-DD); today's values are relative to it. */
    val day: String = "",
    val habits: List<ApiHabit> = emptyList(),
    val loadedAt: Long = 0,
)

data class HabitsWidgetState(val habits: List<ApiHabit>, val loaded: Boolean, val error: String?, val signedIn: Boolean, val page: Int = 0)

/** What the habit widgets show, shared by all of them and the Habits tab. */
class HabitsWidgetStore(private val app: OpenTickApp) {
    private val sp = app.getSharedPreferences("widget_habits", Context.MODE_PRIVATE)
    private val json = Json { ignoreUnknownKeys = true; explicitNulls = false }
    private val mutex = Mutex()

    private val _snapshot = MutableStateFlow(read())
    val snapshot: StateFlow<HabitsSnapshot> = _snapshot

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error

    private fun read(): HabitsSnapshot =
        runCatching { json.decodeFromString<HabitsSnapshot>(sp.getString("snapshot", null) ?: "") }.getOrDefault(HabitsSnapshot())

    private fun save(s: HabitsSnapshot) {
        sp.edit().putString("snapshot", json.encodeToString(HabitsSnapshot.serializer(), s)).apply()
        _snapshot.value = s
    }

    /** The habits for today, or nothing if they belong to another server or account mode. */
    fun state(s: HabitsSnapshot = _snapshot.value, err: String? = _error.value): HabitsWidgetState {
        val signedIn = app.prefs.mode.value == AccountMode.SERVER
        val mine = signedIn && s.server == app.prefs.serverUrl && s.day.isNotEmpty()
        val today = LocalDate.now()
        val habits = if (mine) s.habits.forDay(LocalDate.parse(s.day), today) else emptyList()
        return HabitsWidgetState(habits, mine, err, signedIn)
    }

    /** Takes the habits the Habits tab loaded or changed. */
    fun publish(habits: List<ApiHabit>) {
        if (app.prefs.mode.value != AccountMode.SERVER) return
        save(HabitsSnapshot(app.prefs.serverUrl, LocalDate.now().toString(), habits, System.currentTimeMillis()))
        _error.value = null
        app.scope.launch { Widgets.updateHabits(app) }
    }

    /** Loads the habits if they're older than a few minutes or from another day. */
    suspend fun refresh(force: Boolean = false) {
        if (app.prefs.mode.value != AccountMode.SERVER) return
        val s = _snapshot.value
        val today = LocalDate.now().toString()
        val fresh = s.server == app.prefs.serverUrl && s.day == today && System.currentTimeMillis() - s.loadedAt < 5 * 60_000
        if (fresh && !force) return
        mutex.withLock {
            try {
                val habits = withContext(Dispatchers.IO) { app.api.habits(today) }
                save(HabitsSnapshot(app.prefs.serverUrl, today, habits, System.currentTimeMillis()))
                _error.value = null
            } catch (e: Exception) {
                _error.value = HabitsStore.errorText(e)
            }
        }
    }

    /** One tap on a habit, like in the tab: yes/no toggles, counters add one. */
    suspend fun tap(id: Long) = mutex.withLock {
        val today = LocalDate.now()
        val before = _snapshot.value
        val h = state(before).habits.firstOrNull { it.id == id } ?: return@withLock
        val value = h.tapValue(h.todayValue)
        fun with(habit: ApiHabit) = before.copy(
            day = today.toString(),
            habits = state(before).habits.map { if (it.id == id) habit else it },
        )
        save(with(h.withValue(today, value, today)))
        Widgets.updateHabits(app)
        try {
            val saved = withContext(Dispatchers.IO) { app.api.setCheckin(id, today.toString(), value, today.toString()) }
            save(with(saved))
            _error.value = null
        } catch (e: Exception) {
            save(before)
            _error.value = HabitsStore.errorText(e)
        }
    }

    fun clear() {
        sp.edit().clear().apply()
        _snapshot.value = HabitsSnapshot()
    }
}

private fun Context.app() = applicationContext as OpenTickApp

private val HabitIdKey = ActionParameters.Key<Long>("habit_id")

/** Checks a habit in from a widget. */
class HabitTapAction : ActionCallback {
    override suspend fun onAction(context: Context, glanceId: GlanceId, parameters: ActionParameters) {
        val id = parameters[HabitIdKey] ?: return
        context.app().habitWidgets.tap(id)
        Widgets.updateHabits(context)
    }
}

/** Reloads the habits (tap on the message when they couldn't load). */
class HabitsRefreshAction : ActionCallback {
    override suspend fun onAction(context: Context, glanceId: GlanceId, parameters: ActionParameters) {
        context.app().habitWidgets.refresh(force = true)
        Widgets.updateHabits(context)
    }
}

fun habitColor(h: ApiHabit): Color = parseHex(h.hexColor.removePrefix("#").ifBlank { DEFAULT_HABIT_COLOR })

/** Weigh-in habits ask for the weight, so they open in the app; the rest check in right away. */
private fun habitAction(context: Context, h: ApiHabit): Action =
    if (h.tracksWeight) openTab(context, "habits", habit = h.id)
    else actionRunCallback<HabitTapAction>(actionParametersOf(HabitIdKey to h.id))

abstract class HabitDataWidget : GlanceAppWidget() {
    override val sizeMode = SizeMode.Exact
    override val stateDefinition: GlanceStateDefinition<*> = PreferencesGlanceStateDefinition

    override suspend fun provideGlance(context: Context, id: GlanceId) {
        val app = context.app()
        val store = app.habitWidgets
        app.scope.launch { if (store.snapshot.value.loadedAt < System.currentTimeMillis() - 5 * 60_000) { store.refresh(); Widgets.updateHabits(app) } }
        val settings = openSettings(context, id)
        provideContent {
            val snap by store.snapshot.collectAsState()
            val err by store.error.collectAsState()
            val prefs = currentState<Preferences>()
            val style = WidgetStyle.of(prefs)
            val s = store.state(snap, err).copy(page = prefs[WidgetKeys.PAGE] ?: 0)
            WidgetCard(style) {
                when {
                    !s.signedIn -> WidgetMessage(style, "Sign in to a server to see your habits", openTab(context, "habits"))
                    !s.loaded -> WidgetMessage(style, s.error ?: "Loading…", actionRunCallback<HabitsRefreshAction>())
                    else -> Content(context, s, style, settings)
                }
            }
        }
    }

    @Composable
    abstract fun ColumnScope.Content(context: Context, s: HabitsWidgetState, style: WidgetStyle, settings: Action)
}

@Composable
private fun HabitEmoji(h: ApiHabit, size: Int, done: Boolean) {
    val color = habitColor(h)
    Box(
        GlanceModifier.size(size.dp).cornerRadius((size / 2).dp).background(ColorProvider(if (done) Color.White else color.copy(alpha = 0.22f))),
        contentAlignment = Alignment.Center,
    ) {
        if (done) {
            Image(ImageProvider(R.drawable.ic_widget_check), "Done", GlanceModifier.size((size * 0.6f).dp), colorFilter = ColorFilter.tint(ColorProvider(color)))
        } else {
            Text(h.icon.ifBlank { "•" }, style = TextStyle(fontSize = (size * 0.5f).sp, color = ColorProvider(color), textAlign = TextAlign.Center))
        }
    }
}

// ─── Today Habits ────────────────────────────────────────────────────────

/** Today's habits as tiles, two per row; a tile fills with the habit's color as it's checked in. */
class TodayHabitsWidget : HabitDataWidget() {
    @Composable
    override fun ColumnScope.Content(context: Context, s: HabitsWidgetState, style: WidgetStyle, settings: Action) {
        val habits = widgetHabits(s.habits, LocalDate.now())
        if (habits.isEmpty()) return WidgetMessage(style, "No habits today", openTab(context, "habits"))
        val size = LocalSize.current
        val columns = if (size.width.value >= 220f) 2 else 1
        val tileWidth = (size.width.value - 16f - 8f * (columns - 1)) / columns
        val rows = habits.chunked(columns)
        val pages = Pages.fit(rows.size, size.height.value - 8f, 64f, s.page)
        Column(GlanceModifier.fillMaxWidth().defaultWeight().padding(start = 8.dp, end = 8.dp, top = 8.dp)) {
            for (row in pages.of(rows)) {
                Row(GlanceModifier.fillMaxWidth().padding(bottom = 8.dp)) {
                    row.forEachIndexed { i, h ->
                        if (i > 0) Spacer(GlanceModifier.width(8.dp))
                        HabitTile(context, style, h, tileWidth)
                    }
                }
            }
        }
        PageFooter(style, pages)
    }
}

@Composable
private fun HabitTile(context: Context, style: WidgetStyle, h: ApiHabit, width: Float) {
    val color = habitColor(h)
    val goal = h.goal.coerceAtLeast(1)
    val progress = (h.todayValue.toFloat() / goal).coerceIn(0f, 1f)
    val done = h.doneToday()
    val sub = if (goal > 1 && !done) "${h.todayValue} / $goal ${h.unit}".trim() else "${h.currentStreak} Day" + if (h.currentStreak == 1) "" else "s"
    Box(GlanceModifier.width(width.dp).height(56.dp).cornerRadius(12.dp).background(style.faint).clickable(habitAction(context, h))) {
        // Checked-in part of the tile in the habit's color: all of it once done.
        if (progress > 0f) {
            Box(GlanceModifier.width((width * progress).dp).fillMaxHeight().cornerRadius(12.dp).background(ColorProvider(color.copy(alpha = if (done) 0.55f else 0.35f)))) {}
        }
        Row(GlanceModifier.fillMaxSize().padding(horizontal = 10.dp), verticalAlignment = Alignment.CenterVertically) {
            HabitEmoji(h, 30, done)
            Spacer(GlanceModifier.width(10.dp))
            Column {
                Text(h.title, style = TextStyle(color = style.text, fontSize = 15.sp), maxLines = 1)
                Text(sub, style = TextStyle(color = style.secondary, fontSize = 12.sp), maxLines = 1)
            }
        }
    }
}

// ─── Weekly Habits ───────────────────────────────────────────────────────

/** Each habit with a square per day of this week, filled in its color when done. */
class WeeklyHabitsWidget : HabitDataWidget() {
    @Composable
    override fun ColumnScope.Content(context: Context, s: HabitsWidgetState, style: WidgetStyle, settings: Action) {
        val weekStart by context.app().prefs.weekStart.collectAsState()
        val today = LocalDate.now()
        val days = weekDays(today, weekStartDay(weekStart))
        val habits = s.habits.filter { !it.archived }.let { widgetHabits(it, today) + it.filter { h -> !h.isDue(today) } }.distinctBy { it.id }
        if (habits.isEmpty()) return WidgetMessage(style, "No habits yet", openTab(context, "habits"))
        val square = ((LocalSize.current.width.value * 0.55f - 12f) / 7f - 6f).coerceIn(14f, 22f)
        Column(GlanceModifier.fillMaxSize().padding(start = 12.dp, end = 12.dp, top = 10.dp, bottom = 4.dp)) {
            Row(GlanceModifier.fillMaxWidth().height(28.dp), verticalAlignment = Alignment.CenterVertically) {
                Spacer(GlanceModifier.defaultWeight())
                for (day in days) {
                    val isToday = day == today
                    Box(GlanceModifier.width((square + 6f).dp), contentAlignment = Alignment.Center) {
                        Box(
                            GlanceModifier.size(22.dp).let { if (isToday) it.cornerRadius(11.dp).background(style.accent) else it },
                            contentAlignment = Alignment.Center,
                        ) {
                            Text(
                                day.dayOfWeek.getDisplayName(DateTextStyle.NARROW, Locale.getDefault()),
                                style = TextStyle(color = if (isToday) style.white else style.tertiary, fontSize = 11.sp, textAlign = TextAlign.Center),
                            )
                        }
                    }
                }
            }
            val pages = Pages.fit(habits.size, LocalSize.current.height.value - 42f, 40f, s.page)
            Column(GlanceModifier.fillMaxWidth().defaultWeight()) {
                for (h in pages.of(habits)) WeekRow(context, style, h, days, today, square)
            }
            PageFooter(style, pages)
        }
    }
}

@Composable
private fun WeekRow(context: Context, style: WidgetStyle, h: ApiHabit, days: List<LocalDate>, today: LocalDate, square: Float) {
    val color = habitColor(h)
    Row(GlanceModifier.fillMaxWidth().height(40.dp), verticalAlignment = Alignment.CenterVertically) {
        Row(GlanceModifier.defaultWeight().clickable(openTab(context, "habits", habit = h.id)), verticalAlignment = Alignment.CenterVertically) {
            HabitEmoji(h, 22, done = false)
            Spacer(GlanceModifier.width(8.dp))
            Text(h.title, style = TextStyle(color = style.text, fontSize = 14.sp), maxLines = 1)
        }
        for (day in days) {
            val value = h.widgetValueOn(day, today)
            val goal = h.goal.coerceAtLeast(1)
            val fill = when {
                value == null || value == 0 -> null
                value >= goal -> color
                else -> color.copy(alpha = 0.45f)
            }
            var m = GlanceModifier.width((square + 6f).dp).height(40.dp)
            if (day == today) m = m.clickable(habitAction(context, h))
            Box(m, contentAlignment = Alignment.Center) {
                Box(
                    GlanceModifier.size(square.dp).cornerRadius(4.dp)
                        .background(fill?.let { ColorProvider(it) } ?: if (h.isDue(day)) style.faint else ColorProvider(Color.Transparent)),
                ) {}
            }
        }
    }
}

class TodayHabitsWidgetReceiver : GlanceAppWidgetReceiver() { override val glanceAppWidget: GlanceAppWidget = TodayHabitsWidget() }
class WeeklyHabitsWidgetReceiver : GlanceAppWidgetReceiver() { override val glanceAppWidget: GlanceAppWidget = WeeklyHabitsWidget() }
