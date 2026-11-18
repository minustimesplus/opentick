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
import androidx.glance.appwidget.state.updateAppWidgetState
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
import androidx.glance.text.FontWeight
import androidx.glance.text.Text
import androidx.glance.text.TextAlign
import androidx.glance.text.TextStyle
import androidx.glance.unit.ColorProvider
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.first
import org.opentick.android.OpenTickApp
import org.opentick.android.R
import org.opentick.android.data.AccountMode
import org.opentick.android.data.TaskEntity
import org.opentick.android.ui.Data
import org.opentick.android.ui.ListSel
import org.opentick.android.ui.SmartList
import org.opentick.android.ui.calendar.itemsOn
import org.opentick.android.ui.calendar.weekStartDay
import org.opentick.android.ui.matrix.QUADRANT_ROWS
import org.opentick.android.ui.matrix.Quadrant
import org.opentick.android.ui.matrix.matrixGroups
import org.opentick.android.ui.theme.parseHex
import org.opentick.android.util.formatDue
import org.opentick.android.util.isOverdue
import org.opentick.android.util.today
import java.time.LocalDate
import java.time.YearMonth
import java.time.format.TextStyle as DateTextStyle
import java.util.Locale

// Widgets for tasks. They read the local database, so they work offline and
// show changes right away; ticking a task off syncs it like in the app.

private val OpenTickApp.widgetData: Flow<Data>
    get() = combine(repo.allProjects, repo.allLabels, repo.allTasks) { p, l, t -> Data(p, l, t) }

private fun Context.app() = applicationContext as OpenTickApp

/** Base of the task widgets: per widget settings in Preferences, the app's data as it changes. */
abstract class TaskDataWidget : GlanceAppWidget() {
    override val sizeMode = SizeMode.Exact
    override val stateDefinition: GlanceStateDefinition<*> = PreferencesGlanceStateDefinition

    override suspend fun provideGlance(context: Context, id: GlanceId) {
        val app = context.app()
        val initial = app.widgetData.first()
        val settings = openSettings(context, id)
        provideContent {
            val d by app.widgetData.collectAsState(initial)
            val mode by app.prefs.mode.collectAsState()
            val prefs = currentState<Preferences>()
            val style = WidgetStyle.of(prefs)
            WidgetCard(style) {
                Content(context, id, d, prefs, style, settings, signedIn = mode != AccountMode.NONE)
            }
        }
    }

    @Composable
    abstract fun ColumnScope.Content(
        context: Context, id: GlanceId, d: Data, prefs: Preferences, style: WidgetStyle,
        settings: androidx.glance.action.Action, signedIn: Boolean,
    )
}

@Composable
private fun SignIn(style: WidgetStyle, context: Context) =
    WidgetMessage(style, "Sign in to OpenTick", openTab(context, "tasks"))

private val TaskIdKey = ActionParameters.Key<Long>("task_id")

/** Ticks a task off from a widget. */
class CompleteTaskAction : ActionCallback {
    override suspend fun onAction(context: Context, glanceId: GlanceId, parameters: ActionParameters) {
        val id = parameters[TaskIdKey] ?: return
        context.app().repo.setDone(id, true)
        Widgets.updateTasks(context)
    }
}

private fun listColor(t: TaskEntity, d: Data): ColorProvider? =
    d.projectMap[t.projectId]?.hexColor?.takeIf { it.isNotBlank() }?.let { ColorProvider(parseHex(it)) }

// ─── Tasks ───────────────────────────────────────────────────────────────

/** A list's open tasks: tap the box to complete, the title to open, + to add. */
class TasksWidget : TaskDataWidget() {
    @Composable
    override fun ColumnScope.Content(
        context: Context, id: GlanceId, d: Data, prefs: Preferences, style: WidgetStyle,
        settings: androidx.glance.action.Action, signedIn: Boolean,
    ) {
        val sel = resolveWidgetList(parseWidgetKey(prefs[WidgetKeys.LIST]), d)
        val key = sel.widgetKey()
        val views by context.app().prefs.views.collectAsState()
        WidgetHeader(
            style, widgetListTitle(sel, d), titleAction = settings, dropdown = true,
            addAction = if (signedIn) openList(context, key, add = true) else null, moreAction = settings,
        )
        if (!signedIn) return SignIn(style, context)
        val rows = nestSubtasks(widgetTasks(sel, d, views))
        if (rows.isEmpty()) return WidgetMessage(style, "No tasks", openList(context, key, add = false))
        val now = System.currentTimeMillis()
        // Header ~50dp, then rows; small widgets get denser rows. A list that doesn't fit gets pages, like TickTick.
        val height = LocalSize.current.height.value - 50f - 4f
        val compact = height < 200f
        val pages = Pages.fit(rows.size, height, if (compact) 32f else 40f, prefs[WidgetKeys.PAGE] ?: 0)
        Column(GlanceModifier.fillMaxWidth().defaultWeight()) {
            for ((t, depth) in pages.of(rows)) TaskLine(context, style, t, depth, listColor(t, d), now, compact)
        }
        PageFooter(style, pages)
    }
}

@Composable
private fun TaskLine(context: Context, style: WidgetStyle, t: TaskEntity, depth: Int, bar: ColorProvider?, now: Long, compact: Boolean) {
    Row(GlanceModifier.fillMaxWidth().height(if (compact) 32.dp else 40.dp), verticalAlignment = Alignment.CenterVertically) {
        // The list's color down the left edge, like TickTick.
        Box(GlanceModifier.width(3.dp).fillMaxHeight().background(bar ?: ColorProvider(Color.Transparent))) {}
        Spacer(GlanceModifier.width((depth * 24).dp))
        Box(
            GlanceModifier.padding(start = 9.dp, end = 10.dp, top = 6.dp, bottom = 6.dp)
                .clickable(actionRunCallback<CompleteTaskAction>(actionParametersOf(TaskIdKey to t.id))),
            contentAlignment = Alignment.Center,
        ) {
            Image(ImageProvider(R.drawable.ic_widget_box), "Complete", GlanceModifier.size(if (compact) 16.dp else 18.dp), colorFilter = ColorFilter.tint(style.priority(t.priority)))
        }
        Row(GlanceModifier.defaultWeight().fillMaxHeight().clickable(openTask(context, t.id)), verticalAlignment = Alignment.CenterVertically) {
            Text(t.title, GlanceModifier.defaultWeight(), style = TextStyle(color = style.text, fontSize = if (compact) 13.sp else 15.sp), maxLines = 1)
            t.dueDate?.let { due ->
                Text(
                    formatDue(due), GlanceModifier.padding(start = 8.dp, end = 12.dp),
                    style = TextStyle(color = if (isOverdue(due, now)) style.red else style.tertiary, fontSize = 12.sp), maxLines = 1,
                )
            } ?: Spacer(GlanceModifier.width(12.dp))
        }
    }
}

// ─── Add Task ────────────────────────────────────────────────────────────

/** A small card that opens quick add for the inbox. */
class AddTaskWidget : TaskDataWidget() {
    @Composable
    override fun ColumnScope.Content(
        context: Context, id: GlanceId, d: Data, prefs: Preferences, style: WidgetStyle,
        settings: androidx.glance.action.Action, signedIn: Boolean,
    ) {
        val add = if (signedIn) openList(context, ListSel.Smart(SmartList.INBOX).widgetKey(), add = true) else openTab(context, "tasks")
        Box(GlanceModifier.fillMaxSize().clickable(add).padding(16.dp)) {
            Column(GlanceModifier.fillMaxSize()) {
                Text("Add Task", style = TextStyle(color = style.text, fontSize = 17.sp, fontWeight = FontWeight.Bold), maxLines = 1)
                Spacer(GlanceModifier.height(4.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Image(ImageProvider(R.drawable.ic_widget_inbox), null, GlanceModifier.size(13.dp), colorFilter = ColorFilter.tint(style.secondary))
                    Spacer(GlanceModifier.width(5.dp))
                    Text(d.inbox?.title ?: "Inbox", style = TextStyle(color = style.secondary, fontSize = 12.sp), maxLines = 1)
                }
            }
            Box(GlanceModifier.fillMaxSize(), contentAlignment = Alignment.BottomEnd) {
                Image(ImageProvider(R.drawable.ic_widget_add), "Add task", GlanceModifier.size(26.dp), colorFilter = ColorFilter.tint(style.accent))
            }
        }
    }
}

// ─── Eisenhower Matrix ───────────────────────────────────────────────────

private fun quadrantColor(q: Quadrant) = ColorProvider(
    when (q.id) { 1 -> Color(0xFFF0656C); 2 -> Color(0xFFFFAF00); 3 -> Color(0xFF4772FA); else -> Color(0xFF0BCD9B) },
)

/** The four quadrants with their tasks, laid out like the Matrix tab. */
class MatrixWidget : TaskDataWidget() {
    @Composable
    override fun ColumnScope.Content(
        context: Context, id: GlanceId, d: Data, prefs: Preferences, style: WidgetStyle,
        settings: androidx.glance.action.Action, signedIn: Boolean,
    ) {
        val views by context.app().prefs.views.collectAsState()
        WidgetHeader(
            style, "Eisenhower Matrix", titleAction = openTab(context, "matrix"),
            addAction = if (signedIn) openTab(context, "matrix", add = true) else null, moreAction = settings,
            leading = { Image(ImageProvider(R.drawable.ic_widget_matrix), null, GlanceModifier.size(20.dp), colorFilter = ColorFilter.tint(style.accent)) },
        )
        if (!signedIn) return SignIn(style, context)
        val today = today()
        // Header ~50dp; each quadrant has a 26dp title and 24dp lines.
        val quadrantHeight = (LocalSize.current.height.value - 54f) / 2f
        val lines = ((quadrantHeight - 30f) / 24f).toInt().coerceAtLeast(1)
        Column(GlanceModifier.fillMaxSize().padding(start = 10.dp, end = 10.dp, bottom = 8.dp)) {
            WidgetDivider(style)
            QUADRANT_ROWS.forEachIndexed { i, row ->
                if (i > 0) WidgetDivider(style)
                Row(GlanceModifier.fillMaxWidth().defaultWeight()) {
                    row.forEachIndexed { j, q ->
                        if (j > 0) WidgetDivider(style, vertical = true)
                        val tasks = matrixGroups(q, d, views, today).flatMap { it.tasks }.filter { !it.done }
                        QuadrantBox(context, style, q, tasks, lines, GlanceModifier.defaultWeight().fillMaxHeight())
                    }
                }
            }
        }
    }
}

@Composable
private fun QuadrantBox(context: Context, style: WidgetStyle, q: Quadrant, tasks: List<TaskEntity>, lines: Int, modifier: GlanceModifier) {
    Column(modifier.padding(horizontal = 6.dp, vertical = 6.dp).clickable(openTab(context, "matrix"))) {
        Text(q.title, style = TextStyle(color = quadrantColor(q), fontSize = 14.sp, fontWeight = FontWeight.Medium), maxLines = 1)
        Spacer(GlanceModifier.height(4.dp))
        val shown = if (tasks.size > lines) tasks.take(lines - 1) else tasks
        for (t in shown) {
            Text(
                t.title, GlanceModifier.fillMaxWidth().height(24.dp).clickable(openTask(context, t.id)),
                style = TextStyle(color = style.text, fontSize = 14.sp), maxLines = 1,
            )
        }
        if (shown.size < tasks.size) {
            Text("+${tasks.size - shown.size}", style = TextStyle(color = style.tertiary, fontSize = 12.sp), maxLines = 1)
        }
        if (tasks.isEmpty()) Text("No tasks", style = TextStyle(color = style.tertiary, fontSize = 13.sp))
    }
}

// ─── Month calendar ──────────────────────────────────────────────────────

private val MonthStep = ActionParameters.Key<Int>("month_step")

/** ‹ and › on the calendar widget; 0 goes back to this month. */
class MonthStepAction : ActionCallback {
    override suspend fun onAction(context: Context, glanceId: GlanceId, parameters: ActionParameters) {
        val step = parameters[MonthStep] ?: 0
        updateAppWidgetState(context, glanceId) { p ->
            p[WidgetKeys.MONTH_OFFSET] = if (step == 0) 0 else (p[WidgetKeys.MONTH_OFFSET] ?: 0) + step
        }
        CalendarWidget().update(context, glanceId)
    }
}

private fun monthStep(step: Int) = actionRunCallback<MonthStepAction>(actionParametersOf(MonthStep to step))

/** The month with each day's tasks as colored chips, like TickTick's month widget. */
class CalendarWidget : TaskDataWidget() {
    @Composable
    override fun ColumnScope.Content(
        context: Context, id: GlanceId, d: Data, prefs: Preferences, style: WidgetStyle,
        settings: androidx.glance.action.Action, signedIn: Boolean,
    ) {
        val app = context.app()
        val weekStart by app.prefs.weekStart.collectAsState()
        val today = today()
        val month = YearMonth.from(today).plusMonths((prefs[WidgetKeys.MONTH_OFFSET] ?: 0).toLong())
        val monthName = month.month.getDisplayName(DateTextStyle.SHORT, Locale.getDefault()) +
            if (month.year != today.year) " ${month.year}" else ""
        WidgetHeader(
            style, monthName, titleAction = monthStep(0),
            addAction = if (signedIn) openList(context, ListSel.Smart(SmartList.TODAY).widgetKey(), add = true) else null,
            moreAction = settings,
            leading = { CalendarBadge(style, today, openTab(context, "calendar")) },
            center = {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    WidgetIcon(R.drawable.ic_widget_prev, style.text, 16.dp, monthStep(-1), "Previous month", padding = 6.dp)
                    Text(
                        monthName, GlanceModifier.clickable(monthStep(0)).padding(horizontal = 6.dp),
                        style = TextStyle(color = style.text, fontSize = 17.sp, fontWeight = FontWeight.Bold), maxLines = 1,
                    )
                    WidgetIcon(R.drawable.ic_widget_next, style.text, 16.dp, monthStep(1), "Next month", padding = 6.dp)
                }
            },
        )
        if (!signedIn) return SignIn(style, context)
        val first = weekStartDay(weekStart)
        val weeks = monthWeeks(month, first)
        val size = LocalSize.current
        // Header 50dp, weekday names 18dp; a cell has its 18dp day number and 13dp chips.
        val cellHeight = (size.height.value - 50f - 18f - 6f) / weeks.size
        val chips = ((cellHeight - 19f) / 13f).toInt().coerceIn(0, 4)
        val open = openTab(context, "calendar")
        Column(GlanceModifier.fillMaxSize().padding(start = 4.dp, end = 4.dp, bottom = 6.dp).clickable(open)) {
            Row(GlanceModifier.fillMaxWidth().height(18.dp)) {
                for (day in weeks.first()) {
                    Text(
                        day.dayOfWeek.getDisplayName(DateTextStyle.SHORT, Locale.getDefault()),
                        GlanceModifier.defaultWeight(),
                        style = TextStyle(color = style.tertiary, fontSize = 10.sp, textAlign = TextAlign.Center), maxLines = 1,
                    )
                }
            }
            for (week in weeks) {
                Row(GlanceModifier.fillMaxWidth().defaultWeight()) {
                    for (day in week) DayCell(style, day, month, today, d, chips, GlanceModifier.defaultWeight().fillMaxHeight())
                }
            }
        }
    }
}

@Composable
private fun CalendarBadge(style: WidgetStyle, today: LocalDate, action: androidx.glance.action.Action) {
    Box(GlanceModifier.size(22.dp).clickable(action), contentAlignment = Alignment.Center) {
        Image(ImageProvider(R.drawable.ic_widget_calendar), null, GlanceModifier.size(22.dp), colorFilter = ColorFilter.tint(style.text))
        Text(
            today.dayOfMonth.toString(), GlanceModifier.padding(top = 4.dp),
            style = TextStyle(color = style.onText, fontSize = 9.sp, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center),
        )
    }
}

@Composable
private fun DayCell(style: WidgetStyle, day: LocalDate, month: YearMonth, today: LocalDate, d: Data, chips: Int, modifier: GlanceModifier) {
    val inMonth = YearMonth.from(day) == month
    Column(modifier.padding(horizontal = 1.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Box(GlanceModifier.size(17.dp).let { if (day == today) it.cornerRadius(9.dp).background(style.accent) else it }, contentAlignment = Alignment.Center) {
            Text(
                day.dayOfMonth.toString(),
                style = TextStyle(
                    color = when { day == today -> style.white; inMonth -> style.text; else -> style.tertiary },
                    fontSize = 10.sp, textAlign = TextAlign.Center,
                    fontWeight = if (day == today) FontWeight.Bold else FontWeight.Normal,
                ),
            )
        }
        if (!inMonth || chips == 0) return@Column
        Spacer(GlanceModifier.height(2.dp))
        val items = itemsOn(day, d, showCompleted = false)
        val shown = if (items.size > chips) items.take(chips - 1) else items
        for (item in shown) {
            Text(
                item.title,
                GlanceModifier.fillMaxWidth().height(12.dp).padding(bottom = 1.dp).cornerRadius(2.dp)
                    .background(ColorProvider(item.color.copy(alpha = 0.75f))).padding(horizontal = 2.dp),
                style = TextStyle(color = style.white, fontSize = 8.sp), maxLines = 1,
            )
        }
        if (shown.size < items.size) {
            Text("+${items.size - shown.size}", style = TextStyle(color = style.secondary, fontSize = 8.sp), maxLines = 1)
        }
    }
}

// ─── Receivers ───────────────────────────────────────────────────────────

class TasksWidgetReceiver : GlanceAppWidgetReceiver() { override val glanceAppWidget: GlanceAppWidget = TasksWidget() }
class AddTaskWidgetReceiver : GlanceAppWidgetReceiver() { override val glanceAppWidget: GlanceAppWidget = AddTaskWidget() }
class MatrixWidgetReceiver : GlanceAppWidgetReceiver() { override val glanceAppWidget: GlanceAppWidget = MatrixWidget() }
class CalendarWidgetReceiver : GlanceAppWidgetReceiver() { override val glanceAppWidget: GlanceAppWidget = CalendarWidget() }
