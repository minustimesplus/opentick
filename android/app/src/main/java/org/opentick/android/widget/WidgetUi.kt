package org.opentick.android.widget

import android.appwidget.AppWidgetManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.glance.ColorFilter
import androidx.glance.GlanceId
import androidx.glance.GlanceModifier
import androidx.glance.Image
import androidx.glance.ImageProvider
import androidx.glance.action.Action
import androidx.glance.action.clickable
import androidx.glance.appwidget.GlanceAppWidgetManager
import androidx.glance.appwidget.action.actionStartActivity
import androidx.glance.appwidget.appWidgetBackground
import androidx.glance.appwidget.cornerRadius
import androidx.glance.background
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
import androidx.glance.text.FontWeight
import androidx.glance.text.Text
import androidx.glance.text.TextStyle
import androidx.glance.unit.ColorProvider
import org.opentick.android.MainActivity
import org.opentick.android.R
import androidx.glance.color.ColorProvider as DayNight

// Shared look of the home screen widgets, after TickTick's: a rounded card,
// translucent by default, with a bold title, + and ⋮ at the top.

/** Per widget settings, kept in the widget's Glance state. */
object WidgetKeys {
    /** The list a tasks widget shows; see [parseWidgetKey]. */
    val LIST = stringPreferencesKey("list")
    /** 0 follows the system's dark mode, 1 light, 2 dark. */
    val THEME = intPreferencesKey("theme")
    /** Background opacity in percent. */
    val OPACITY = intPreferencesKey("opacity")
    /** Month shown by the calendar widget, relative to this month. */
    val MONTH_OFFSET = intPreferencesKey("month_offset")
    /** Page of a list that doesn't fit, from 0. */
    val PAGE = intPreferencesKey("page")
}

const val THEME_AUTO = 0
const val THEME_LIGHT = 1
const val THEME_DARK = 2
const val DEFAULT_OPACITY = 90

val WidgetAccent = Color(0xFF4772FA)
val WidgetRed = Color(0xFFE5483F)

/** Colors of a widget for its theme and opacity. */
class WidgetStyle(val theme: Int, opacity: Int) {
    private val alpha = opacity.coerceIn(0, 100) / 100f

    private fun pick(day: Color, night: Color): ColorProvider = when (theme) {
        THEME_LIGHT -> ColorProvider(day)
        THEME_DARK -> ColorProvider(night)
        else -> DayNight(day, night)
    }

    val bg = pick(Color.White.copy(alpha = alpha), Color(0xFF202020).copy(alpha = alpha))
    val text = pick(Color(0xFF191919), Color(0xFFEDEDED))
    /** On [text]-colored shapes, e.g. the day in the calendar icon. */
    val onText = pick(Color.White, Color(0xFF202020))
    val secondary = pick(Color(0xFF707175), Color(0xFFA0A0A4))
    val tertiary = pick(Color(0xFFA2A2A2), Color(0xFF6E6E72))
    val divider = pick(Color(0x14000000), Color(0x1FFFFFFF))
    /** Empty day squares and tiles. */
    val faint = pick(Color(0x0F000000), Color(0x1AFFFFFF))
    val prioNone = pick(Color(0xFFABABAB), Color(0xFF8A8A8E))
    val accent = ColorProvider(WidgetAccent)
    val red = ColorProvider(WidgetRed)
    val white = ColorProvider(Color.White)

    fun priority(p: Int): ColorProvider = when {
        p >= 3 -> ColorProvider(Color(0xFFDE433C))
        p == 2 -> ColorProvider(Color(0xFFF9B00C))
        p == 1 -> ColorProvider(Color(0xFF4772FA))
        else -> prioNone
    }

    companion object {
        fun of(prefs: Preferences) = WidgetStyle(prefs[WidgetKeys.THEME] ?: THEME_AUTO, prefs[WidgetKeys.OPACITY] ?: DEFAULT_OPACITY)
    }
}

/** The rounded card every widget sits on. */
@Composable
fun WidgetCard(style: WidgetStyle, modifier: GlanceModifier = GlanceModifier, content: @Composable ColumnScope.() -> Unit) {
    Box(
        GlanceModifier.fillMaxSize().appWidgetBackground().cornerRadius(18.dp)
            // Below Android 12 cornerRadius does nothing; the tinted shape rounds the corners there.
            .background(ImageProvider(R.drawable.widget_bg), colorFilter = ColorFilter.tint(style.bg)),
    ) {
        Column(modifier.fillMaxSize(), content = content)
    }
}

@Composable
fun WidgetIcon(res: Int, color: ColorProvider, size: Dp, action: Action?, description: String? = null, padding: Dp = 8.dp) {
    var m = GlanceModifier.padding(padding)
    if (action != null) m = m.clickable(action)
    Box(m, contentAlignment = Alignment.Center) {
        Image(ImageProvider(res), description, GlanceModifier.size(size), colorFilter = ColorFilter.tint(color))
    }
}

/** Title with an optional ▾ (opens the widget's settings), then + and ⋮, like TickTick. */
@Composable
fun WidgetHeader(
    style: WidgetStyle,
    title: String,
    titleAction: Action?,
    addAction: Action?,
    moreAction: Action,
    dropdown: Boolean = false,
    leading: (@Composable () -> Unit)? = null,
    center: (@Composable () -> Unit)? = null,
) {
    Row(GlanceModifier.fillMaxWidth().padding(start = 14.dp, end = 2.dp, top = 6.dp), verticalAlignment = Alignment.CenterVertically) {
        if (leading != null) { leading(); Spacer(GlanceModifier.width(8.dp)) }
        if (center != null) {
            Box(GlanceModifier.defaultWeight(), contentAlignment = Alignment.Center) { center() }
        } else {
            var m = GlanceModifier.defaultWeight().padding(vertical = 8.dp)
            if (titleAction != null) m = m.clickable(titleAction)
            Row(m, verticalAlignment = Alignment.CenterVertically) {
                Text(title, style = TextStyle(color = style.text, fontSize = 17.sp, fontWeight = FontWeight.Bold), maxLines = 1)
                if (dropdown) {
                    Spacer(GlanceModifier.width(2.dp))
                    Image(ImageProvider(R.drawable.ic_widget_dropdown), null, GlanceModifier.size(16.dp), colorFilter = ColorFilter.tint(style.text))
                }
            }
        }
        if (addAction != null) WidgetIcon(R.drawable.ic_widget_add, style.text, 22.dp, addAction, "Add task")
        WidgetIcon(R.drawable.ic_widget_more, style.text, 20.dp, moreAction, "Widget settings", padding = 6.dp)
    }
}

/** A centered note, e.g. "No tasks" or "Sign in". */
@Composable
fun WidgetMessage(style: WidgetStyle, text: String, action: Action? = null) {
    var m = GlanceModifier.fillMaxSize().padding(16.dp)
    if (action != null) m = m.clickable(action)
    Box(m, contentAlignment = Alignment.Center) {
        Text(text, style = TextStyle(color = style.tertiary, fontSize = 14.sp))
    }
}

/** A thin line between parts of a widget. */
@Composable
fun WidgetDivider(style: WidgetStyle, vertical: Boolean = false) {
    Box(
        if (vertical) GlanceModifier.width(1.dp).fillMaxHeight().background(style.divider)
        else GlanceModifier.fillMaxWidth().height(1.dp).background(style.divider),
    ) {}
}

// ─── Opening the app ─────────────────────────────────────────────────────

/**
 * Opens the app. Every intent gets its own data URI so the launcher doesn't
 * reuse one widget's pending intent for another's extras.
 */
fun appIntent(context: Context, path: String, extras: Intent.() -> Unit = {}): Intent =
    Intent(context, MainActivity::class.java).apply {
        data = Uri.parse("opentick-widget://$path")
        flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
        putExtra(MainActivity.EXTRA_FROM_WIDGET, true)
        extras()
    }

fun openTask(context: Context, id: Long): Action =
    actionStartActivity(appIntent(context, "task/$id") { putExtra("open_task", id) })

/** Opens a list in the Tasks tab, with quick add open if [add]. */
fun openList(context: Context, listKey: String, add: Boolean): Action =
    actionStartActivity(appIntent(context, "list/$listKey/$add") {
        putExtra(MainActivity.EXTRA_WIDGET_LIST, listKey)
        putExtra(MainActivity.EXTRA_QUICK_ADD, add)
    })

fun openTab(context: Context, tab: String, add: Boolean = false, habit: Long? = null): Action =
    actionStartActivity(appIntent(context, "tab/$tab/$add/${habit ?: ""}") {
        putExtra("open_tab", tab)
        putExtra(MainActivity.EXTRA_QUICK_ADD, add)
        if (habit != null) putExtra(MainActivity.EXTRA_OPEN_HABIT, habit)
    })

/** The widget's settings: list, theme and opacity. */
suspend fun openSettings(context: Context, id: GlanceId): Action {
    val widgetId = GlanceAppWidgetManager(context).getAppWidgetId(id)
    return actionStartActivity(
        Intent(context, WidgetSettingsActivity::class.java).apply {
            data = Uri.parse("opentick-widget://settings/$widgetId")
            flags = Intent.FLAG_ACTIVITY_NEW_TASK
            putExtra(AppWidgetManager.EXTRA_APPWIDGET_ID, widgetId)
        },
    )
}

// ─── Pages ───────────────────────────────────────────────────────────────

private val PageStep = androidx.glance.action.ActionParameters.Key<Int>("page_step")

/** ‹ and › under a widget's list, like TickTick's "1/3". */
class PageAction : androidx.glance.appwidget.action.ActionCallback {
    override suspend fun onAction(context: Context, glanceId: GlanceId, parameters: androidx.glance.action.ActionParameters) {
        val step = parameters[PageStep] ?: return
        androidx.glance.appwidget.state.updateAppWidgetState(context, glanceId) { p ->
            p[WidgetKeys.PAGE] = ((p[WidgetKeys.PAGE] ?: 0) + step).coerceAtLeast(0)
        }
        val widgetId = GlanceAppWidgetManager(context).getAppWidgetId(glanceId)
        val provider = AppWidgetManager.getInstance(context).getAppWidgetInfo(widgetId)?.provider?.className
        Widgets.forProvider(provider)?.update(context, glanceId)
    }
}

/** How [count] items split into pages of [perPage], and which one [page] shows. */
data class Pages(val count: Int, val perPage: Int, val page: Int) {
    val total = if (count == 0) 1 else (count + perPage - 1) / perPage
    val current = page.coerceIn(0, total - 1)
    fun <T> of(items: List<T>): List<T> = items.drop(current * perPage).take(perPage)

    companion object {
        /**
         * Pages for items [itemHeight] high in [height] dp: all of them if they
         * fit, else as many as fit above the [FOOTER_HEIGHT] footer.
         */
        fun fit(count: Int, height: Float, itemHeight: Float, page: Int): Pages {
            val all = (height / itemHeight).toInt().coerceAtLeast(1)
            if (count <= all) return Pages(count, all, 0)
            return Pages(count, ((height - FOOTER_HEIGHT) / itemHeight).toInt().coerceAtLeast(1), page)
        }
    }
}

const val FOOTER_HEIGHT = 30f

@Composable
fun PageFooter(style: WidgetStyle, pages: Pages) {
    if (pages.total <= 1) return
    fun step(n: Int) = androidx.glance.appwidget.action.actionRunCallback<PageAction>(androidx.glance.action.actionParametersOf(PageStep to n))
    Row(GlanceModifier.fillMaxWidth().height(FOOTER_HEIGHT.dp).padding(horizontal = 6.dp), verticalAlignment = Alignment.CenterVertically) {
        // Image tints ignore alpha, so an arrow that can't be used isn't drawn at all.
        if (pages.current > 0) WidgetIcon(R.drawable.ic_widget_prev, style.secondary, 14.dp, step(-1), "Previous page", padding = 7.dp)
        else Spacer(GlanceModifier.width(28.dp))
        Text(
            "${pages.current + 1}/${pages.total}", GlanceModifier.defaultWeight(),
            style = TextStyle(color = style.tertiary, fontSize = 12.sp, textAlign = androidx.glance.text.TextAlign.Center),
        )
        if (pages.current < pages.total - 1) WidgetIcon(R.drawable.ic_widget_next, style.secondary, 14.dp, step(1), "Next page", padding = 7.dp)
        else Spacer(GlanceModifier.width(28.dp))
    }
}
