package org.opentick.android.ui.tasks

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.gestures.waitForUpOrCancellation
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.pointer.positionChange
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import org.opentick.android.data.TaskEntity
import org.opentick.android.data.TimelineScale
import org.opentick.android.ui.Data
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.calendar.weekStartDay
import org.opentick.android.ui.components.TaskRow
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.parseHex
import org.opentick.android.util.formatShortDate
import org.opentick.android.util.today
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.format.TextStyle
import java.time.temporal.ChronoUnit
import java.util.Locale
import kotlin.math.roundToInt

private val ROW = 36.dp
/** Bars narrower than this show their title beside them. */
private val MIN_LABELLED = 72.dp
/** Bars at least this wide can be stretched by their ends. */
private val MIN_RESIZABLE = 40.dp
private val EDGE = 14.dp

private data class Bar(val task: TaskEntity, val span: Span)

/** A bar being dragged: [dx] pixels so far. */
private data class Drag(val taskId: Long, val mode: DragMode, val dx: Float)

/**
 * The timeline (gantt) layout of a list, like web TimelineView.vue: every dated
 * task is a bar from its first to its last day. Long press a bar and drag to
 * move it, or drag its ends to change its dates; tasks without a date are listed below.
 */
@Composable
fun TimelineView(vm: MainViewModel, data: Data, tasks: List<TaskEntity>) {
    val p = LocalExtra.current
    val views by vm.views.collectAsState()
    val weekPref by vm.prefs.weekStart.collectAsState()
    val scale = views.timelineScale
    val first = weekStartDay(weekPref)
    val today = today()
    val density = LocalDensity.current
    val dw = scale.dayWidth.dp
    val dwPx = with(density) { dw.toPx() }

    val bars = remember(tasks) {
        tasks.mapNotNull { t -> taskSpan(t)?.let { Bar(t, it) } }
            .sortedWith(compareBy<Bar> { it.span.start }.thenBy { it.span.end }.thenBy { it.task.id })
    }
    val undated = remember(tasks) { tasks.filter { taskSpan(it) == null }.sortedWith(compareBy<TaskEntity> { it.done }.thenBy { it.id }) }
    val (start, days) = remember(bars, scale, first, today) { timelineRange(bars.map { it.span }, scale, today, first) }
    val (topUnit, bottomUnit) = timelineUnits(scale)
    val top = remember(start, days, topUnit, first) { timelineGroups(start, days, topUnit, first) }
    val bottom = remember(start, days, bottomUnit, first) { timelineGroups(start, days, bottomUnit, first) }
    fun x(d: LocalDate): Dp = dw * ChronoUnit.DAYS.between(start, d).toFloat()

    val hs = rememberScrollState()
    val scope = rememberCoroutineScope()
    var drag by remember { mutableStateOf<Drag?>(null) }

    BoxWithConstraints(Modifier.fillMaxSize()) {
        val viewport = with(density) { maxWidth.toPx() }
        fun todayScroll() = (with(density) { x(today).toPx() } - viewport / 3).roundToInt().coerceAtLeast(0)
        LaunchedEffect(scale) { hs.scrollTo(todayScroll()) }
        // Keep the same days on screen when the timeline grows to the left.
        var lastStart by remember { mutableStateOf(start) }
        LaunchedEffect(start) {
            val shift = ChronoUnit.DAYS.between(start, lastStart) * dwPx
            if (shift != 0f) hs.scrollTo((hs.value + shift).roundToInt().coerceAtLeast(0))
            lastStart = start
        }

        Column(Modifier.fillMaxSize()) {
            Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "Today", fontSize = 14.sp, fontWeight = FontWeight.SemiBold, color = Accent,
                    modifier = Modifier.clip(RoundedCornerShape(10.dp)).clickable { scope.launch { hs.animateScrollTo(todayScroll()) } }.padding(horizontal = 10.dp, vertical = 6.dp),
                )
                Spacer(Modifier.weight(1f))
                Row(Modifier.clip(RoundedCornerShape(10.dp)).background(p.card).padding(2.dp)) {
                    for (s in TimelineScale.entries) {
                        Text(
                            s.label, fontSize = 13.sp, color = if (s == scale) Color.White else p.textSecondary,
                            modifier = Modifier.clip(RoundedCornerShape(8.dp)).background(if (s == scale) Accent else Color.Transparent)
                                .clickable { vm.setTimelineScale(s) }.padding(horizontal = 12.dp, vertical = 5.dp),
                        )
                    }
                }
            }

            Column(Modifier.weight(1f).verticalScroll(rememberScrollState())) {
                Box(Modifier.horizontalScroll(hs)) {
                    Column(Modifier.width(dw * days.toFloat())) {
                        // Header: months (or years) over days, weeks or months.
                        Box(Modifier.fillMaxWidth().height(22.dp)) {
                            for (g in top) {
                                // Stays at the left edge while its month (or year) is on screen.
                                val from = g.offset * dwPx
                                val to = (g.offset + g.days) * dwPx
                                Text(
                                    topLabel(g.date, topUnit), fontSize = 12.sp, fontWeight = FontWeight.SemiBold, color = p.textSecondary, maxLines = 1,
                                    modifier = Modifier
                                        .offset { IntOffset(hs.value.toFloat().coerceIn(from, maxOf(from, to - 120.dp.toPx())).roundToInt(), 0) }
                                        .padding(start = 6.dp),
                                )
                            }
                        }
                        Box(Modifier.fillMaxWidth().height(if (scale == TimelineScale.DAY) 36.dp else 22.dp)) {
                            for (g in bottom) {
                                val isToday = scale == TimelineScale.DAY && g.date == today
                                Column(
                                    Modifier.offset(x = dw * g.offset.toFloat()).width(dw * g.days.toFloat()),
                                    horizontalAlignment = if (scale == TimelineScale.DAY) Alignment.CenterHorizontally else Alignment.Start,
                                ) {
                                    if (scale == TimelineScale.DAY) Text(g.date.dayOfWeek.getDisplayName(TextStyle.NARROW, Locale.getDefault()), fontSize = 10.sp, color = p.textTertiary)
                                    Text(
                                        bottomLabel(g.date, bottomUnit), fontSize = 11.sp, maxLines = 1,
                                        color = if (isToday) Accent else p.textTertiary, fontWeight = if (isToday) FontWeight.Bold else FontWeight.Normal,
                                        modifier = if (scale == TimelineScale.DAY) Modifier else Modifier.padding(start = 4.dp),
                                    )
                                }
                            }
                        }
                        Box(Modifier.fillMaxWidth().height(ROW * maxOf(bars.size, 1).toFloat())) {
                            Grid(bottom, scale, dw, today, start, first, p.divider)
                            for ((i, bar) in bars.withIndex()) {
                                val d = drag?.takeIf { it.taskId == bar.task.id }
                                // The bar stays put while dragged (so the finger's position stays meaningful); an outline shows where it goes.
                                if (d != null) {
                                    val to = dragSpan(bar.span, d.mode, (d.dx / dwPx).roundToInt().toLong())
                                    Box(
                                        Modifier.offset(x = x(to.start) + 1.dp, y = ROW * i.toFloat() + 4.dp).width(spanWidth(to, dw)).height(ROW - 8.dp)
                                            .border(2.dp, barColor(bar.task, data), RoundedCornerShape(8.dp)),
                                    )
                                }
                                TimelineBar(
                                    bar.task, bar.span, i, x(bar.span.start), dw, d != null, barColor(bar.task, data),
                                    onTap = { vm.openTaskId.value = bar.task.id },
                                    onDrag = { mode, dx -> drag = Drag(bar.task.id, mode, (drag?.takeIf { it.taskId == bar.task.id }?.dx ?: 0f) + dx) },
                                    onEnd = { moved ->
                                        val days = drag?.takeIf { it.taskId == bar.task.id }?.let { (it.dx / dwPx).roundToInt().toLong() } ?: 0L
                                        val mode = drag?.mode ?: DragMode.MOVE
                                        drag = null
                                        if (!moved) vm.actionsTaskId.value = bar.task.id
                                        else if (days != 0L) vm.setDates(bar.task.id, spanPatch(bar.task, dragSpan(bar.span, mode, days)))
                                    },
                                )
                            }
                        }
                    }
                }

                if (undated.isNotEmpty()) {
                    Text("No Date", fontSize = 14.sp, fontWeight = FontWeight.Bold, color = p.textSecondary, modifier = Modifier.padding(start = 18.dp, top = 18.dp, bottom = 6.dp))
                    Column(Modifier.padding(horizontal = 16.dp).fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(p.card).padding(vertical = 4.dp)) {
                        for (t in undated) {
                            androidx.compose.runtime.key(t.id) {
                                TaskRow(
                                    task = t,
                                    project = data.projectMap[t.projectId],
                                    labels = t.labelIdList.mapNotNull { data.labelMap[it] },
                                    showProject = false,
                                    onToggle = { vm.toggleDone(t) },
                                    onClick = { vm.openTaskId.value = t.id },
                                    onDelete = { vm.delete(t) },
                                    onLongClick = { vm.actionsTaskId.value = t.id },
                                )
                            }
                        }
                    }
                }
                Spacer(Modifier.height(110.dp))
            }
        }
    }
}

/** Day lines, shaded weekends (day scale) and the today line. */
@Composable
private fun Grid(bottom: List<TimelineGroup>, scale: TimelineScale, dw: Dp, today: LocalDate, start: LocalDate, first: DayOfWeek, line: Color) {
    Canvas(Modifier.fillMaxSize()) {
        val w = dw.toPx()
        for (g in bottom) {
            val left = g.offset * w
            if (scale == TimelineScale.DAY && (g.date.dayOfWeek == DayOfWeek.SATURDAY || g.date.dayOfWeek == DayOfWeek.SUNDAY)) {
                drawRect(line.copy(alpha = 0.35f), Offset(left, 0f), androidx.compose.ui.geometry.Size(w, size.height))
            }
            drawLine(line, Offset(left, 0f), Offset(left, size.height), strokeWidth = 1f)
        }
        val tx = (ChronoUnit.DAYS.between(start, today) + 0.5f) * w
        drawLine(Accent, Offset(tx, 0f), Offset(tx, size.height), strokeWidth = 2.dp.toPx())
    }
}

@Composable
private fun TimelineBar(
    t: TaskEntity,
    span: Span,
    row: Int,
    left: Dp,
    dw: Dp,
    dragging: Boolean,
    color: Color,
    onTap: () -> Unit,
    onDrag: (DragMode, Float) -> Unit,
    onEnd: (moved: Boolean) -> Unit,
) {
    val p = LocalExtra.current
    val width = spanWidth(span, dw)
    val labelled = width >= MIN_LABELLED
    val resizable = width >= MIN_RESIZABLE
    val density = LocalDensity.current
    val edge = with(density) { EDGE.toPx() }
    val widthPx = with(density) { width.toPx() }
    // The gesture outlives recompositions; always call the latest callbacks.
    val tap by rememberUpdatedState(onTap)
    val dragTo by rememberUpdatedState(onDrag)
    val end by rememberUpdatedState(onEnd)
    Row(Modifier.offset(x = left + 1.dp, y = ROW * row.toFloat() + 6.dp), verticalAlignment = Alignment.CenterVertically) {
        Box(
            Modifier.width(width).height(ROW - 12.dp).clip(RoundedCornerShape(6.dp))
                .background(color.copy(alpha = if (t.done) 0.35f else if (dragging) 0.7f else 0.9f))
                .pointerInput(t.id, resizable, widthPx) {
                    awaitEachGesture {
                        val down = awaitFirstDown()
                        // A tap opens the task; a long press starts dragging. A sideways swipe scrolls the timeline.
                        var up: Any? = null
                        val finished = withTimeoutOrNull(viewConfiguration.longPressTimeoutMillis) { up = waitForUpOrCancellation(); true }
                        if (finished == true) {
                            if (up != null) tap()
                            return@awaitEachGesture
                        }
                        val mode = when {
                            !resizable -> DragMode.MOVE
                            down.position.x < edge -> DragMode.START
                            down.position.x > widthPx - edge -> DragMode.END
                            else -> DragMode.MOVE
                        }
                        dragTo(mode, 0f)
                        var moved = false
                        while (true) {
                            val change = awaitPointerEvent().changes.firstOrNull { it.id == down.id } ?: break
                            if (!change.pressed) break
                            val dx = change.positionChange().x
                            if (dx != 0f) {
                                moved = true
                                dragTo(mode, dx)
                            }
                            change.consume()
                        }
                        end(moved)
                    }
                },
            contentAlignment = Alignment.CenterStart,
        ) {
            if (resizable) {
                // Grips at both ends.
                Box(Modifier.align(Alignment.CenterStart).padding(start = 4.dp).width(2.dp).fillMaxHeight(0.5f).background(Color.White.copy(alpha = 0.6f)))
                Box(Modifier.align(Alignment.CenterEnd).padding(end = 4.dp).width(2.dp).fillMaxHeight(0.5f).background(Color.White.copy(alpha = 0.6f)))
            }
            if (labelled) BarTitle(t, Color.White, Modifier.padding(horizontal = 10.dp))
        }
        if (!labelled) BarTitle(t, p.text, Modifier.padding(start = 6.dp).width(220.dp))
    }
}

private fun spanWidth(span: Span, dw: Dp): Dp = dw * (ChronoUnit.DAYS.between(span.start, span.end) + 1).toFloat() - 2.dp

@Composable
private fun BarTitle(t: TaskEntity, color: Color, modifier: Modifier) {
    Text(
        t.title, fontSize = 12.sp, fontWeight = FontWeight.Medium, color = color, maxLines = 1, overflow = TextOverflow.Ellipsis,
        textDecoration = if (t.done) TextDecoration.LineThrough else null, modifier = modifier,
    )
}

private fun barColor(t: TaskEntity, d: Data): Color =
    d.projectMap[t.projectId]?.hexColor?.takeIf { it.isNotBlank() }?.let { parseHex(it) } ?: Accent

private fun topLabel(d: LocalDate, unit: GroupUnit): String =
    if (unit == GroupUnit.YEAR) d.year.toString() else "${d.month.getDisplayName(TextStyle.FULL, Locale.getDefault())} ${d.year}"

private fun bottomLabel(d: LocalDate, unit: GroupUnit): String = when (unit) {
    GroupUnit.DAY -> d.dayOfMonth.toString()
    GroupUnit.WEEK -> formatShortDate(d)
    else -> d.month.getDisplayName(TextStyle.SHORT, Locale.getDefault())
}
