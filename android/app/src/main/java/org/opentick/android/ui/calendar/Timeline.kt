package org.opentick.android.ui.calendar

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import org.opentick.android.ui.Data
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.NowRed
import org.opentick.android.ui.theme.eventColors
import org.opentick.android.util.today
import java.time.LocalDate
import java.time.LocalTime
import java.time.format.TextStyle
import java.util.Locale

val HOUR_HEIGHT = 44.dp
val GUTTER = 50.dp

/** Weekday names over day numbers; today in a blue circle. */
@Composable
fun DayHeader(days: List<LocalDate>, onDay: (LocalDate) -> Unit = {}) {
    val p = LocalExtra.current
    Row(Modifier.fillMaxWidth().padding(top = 4.dp, bottom = 8.dp)) {
        Spacer(Modifier.width(GUTTER))
        for (d in days) {
            val isToday = d == today()
            Column(Modifier.weight(1f).clickable { onDay(d) }, horizontalAlignment = Alignment.CenterHorizontally) {
                Text(d.dayOfWeek.getDisplayName(TextStyle.SHORT, Locale.getDefault()), fontSize = 12.sp, color = p.weekday)
                Spacer(Modifier.height(6.dp))
                Box(Modifier.size(35.dp).clip(CircleShape).background(if (isToday) Accent else Color.Transparent), contentAlignment = Alignment.Center) {
                    Text("${d.dayOfMonth}", fontSize = 15.sp, color = if (isToday) Color.White else p.text, fontWeight = if (isToday) FontWeight.SemiBold else FontWeight.Normal)
                }
            }
        }
    }
}

/**
 * The hour grid of the Week, 3 Day and Day views: all-day chips on top, then
 * 24 hours of blocks with the current time line.
 */
@Composable
fun Timeline(
    days: List<LocalDate>,
    data: Data,
    showCompleted: Boolean,
    scroll: ScrollState,
    onTask: (Long) -> Unit,
    onSlot: (LocalDate, LocalTime) -> Unit,
    onDay: (LocalDate) -> Unit,
) {
    val p = LocalExtra.current
    val is24 = android.text.format.DateFormat.is24HourFormat(LocalContext.current)
    val items = remember(days, data, showCompleted) { days.associateWith { itemsOn(it, data, showCompleted) } }
    var now by remember { mutableLongStateOf(System.currentTimeMillis()) }
    LaunchedEffect(Unit) { while (true) { delay(60_000); now = System.currentTimeMillis() } }
    val density = LocalDensity.current

    Column(Modifier.fillMaxSize()) {
        DayHeader(days, onDay)
        // All-day strip.
        val allDay = days.map { d -> items[d].orEmpty().filter { it.startMin == null } }
        val rows = allDay.maxOfOrNull { it.size }?.coerceAtMost(3) ?: 0
        Row(Modifier.fillMaxWidth()) {
            Spacer(Modifier.width(GUTTER))
            Row(
                Modifier.weight(1f).clip(RoundedCornerShape(topStart = 12.dp)).background(p.gridBg)
                    .heightIn(min = if (rows == 0) 12.dp else 0.dp).padding(vertical = if (rows == 0) 0.dp else 5.dp),
            ) {
                for ((i, list) in allDay.withIndex()) {
                    Column(Modifier.weight(1f).padding(horizontal = 2.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        val shown = if (list.size > rows) list.take(rows - 1) else list
                        for (it in shown) {
                            val (bg, fg) = eventColors(it.color, p.dark)
                            Text(
                                it.task.title, fontSize = 12.sp, color = fg, maxLines = 1, softWrap = false, overflow = TextOverflow.Clip,
                                textDecoration = if (it.task.done) TextDecoration.LineThrough else null,
                                modifier = Modifier.fillMaxWidth().height(19.dp).clip(RoundedCornerShape(4.dp)).background(bg)
                                    .clickable { onTask(it.task.id) }.padding(horizontal = 4.dp).alpha(if (it.task.done) 0.6f else 1f),
                            )
                        }
                        if (list.size > rows && rows > 0) {
                            Text("+${list.size - shown.size}", fontSize = 11.sp, color = p.textSecondary, modifier = Modifier.height(19.dp).clickable { onDay(days[i]) }.padding(horizontal = 4.dp))
                        }
                    }
                }
            }
        }
        Row(Modifier.fillMaxWidth()) {
            Spacer(Modifier.width(GUTTER))
            HorizontalDivider(color = p.gridLine)
        }
        // Hours.
        Row(Modifier.fillMaxSize().verticalScroll(scroll)) {
            Box(Modifier.width(GUTTER).height(HOUR_HEIGHT * 24 + 12.dp)) {
                for (h in 1..24) {
                    val label = when {
                        h == 24 -> "00"
                        is24 -> "$h"
                        else -> "${if (h % 12 == 0) 12 else h % 12} ${if (h < 12) "AM" else "PM"}"
                    }
                    Text(
                        label, fontSize = if (is24) 12.sp else 10.sp, color = p.textTertiary, maxLines = 1,
                        modifier = Modifier.fillMaxWidth().offset(y = HOUR_HEIGHT * h - 8.dp).padding(end = 6.dp),
                        textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                    )
                }
            }
            BoxWithConstraints(Modifier.weight(1f).height(HOUR_HEIGHT * 24).background(p.gridBg)) {
                val colW = maxWidth / days.size
                val hourPx = with(density) { HOUR_HEIGHT.toPx() }
                Canvas(
                    Modifier.fillMaxSize().pointerInput(days) {
                        detectTapGestures { pos ->
                            val col = (pos.x / (size.width.toFloat() / days.size)).toInt().coerceIn(0, days.lastIndex)
                            val minutes = ((pos.y / hourPx) * 60).toInt() / 30 * 30
                            onSlot(days[col], LocalTime.of((minutes / 60).coerceAtMost(23), minutes % 60))
                        }
                    },
                ) {
                    if (now < 0) return@Canvas // redraws the time line every minute
                    for (h in 1 until 24) drawLine(p.gridLine, Offset(0f, h * hourPx), Offset(size.width, h * hourPx), 1f)
                    val cw = size.width / days.size
                    for (i in 1 until days.size) drawLine(p.gridLine, Offset(i * cw, 0f), Offset(i * cw, size.height), 1f)
                    // Current time: faint across the week, solid in today's column.
                    val ti = days.indexOf(today())
                    if (ti >= 0) {
                        val t = java.time.LocalTime.now()
                        val y = (t.hour * 60 + t.minute) / 60f * hourPx
                        drawLine(NowRed.copy(alpha = 0.35f), Offset(0f, y), Offset(size.width, y), 2f)
                        drawLine(NowRed, Offset(ti * cw, y), Offset((ti + 1) * cw, y), 4f)
                        drawCircle(NowRed, 4.dp.toPx(), Offset(ti * cw, y))
                    }
                }
                for ((i, d) in days.withIndex()) {
                    val timed = items[d].orEmpty().filter { it.startMin != null }
                    val lanes = layoutLanes(timed)
                    for (it in timed) {
                        val (lane, count) = lanes[it] ?: (0 to 1)
                        val w = colW / count
                        val top = HOUR_HEIGHT * (it.startMin!! / 60f)
                        val h = HOUR_HEIGHT * ((it.endMin!! - it.startMin) / 60f)
                        TimelineBlock(it, Modifier.offset(x = colW * i + w * lane, y = top).width(w).height(h).padding(horizontal = 2.dp, vertical = 1.dp)) { onTask(it.task.id) }
                    }
                }
            }
        }
    }
}

@Composable
private fun TimelineBlock(item: CalItem, modifier: Modifier, onClick: () -> Unit) {
    val p = LocalExtra.current
    val (bg, fg) = eventColors(item.color, p.dark)
    val t = item.task
    val time = fun(m: Int) = "%02d:%02d".format(m / 60 % 24, m % 60)
    Column(
        modifier.clip(RoundedCornerShape(4.dp)).background(bg).clickable(onClick = onClick).padding(horizontal = 4.dp, vertical = 3.dp).alpha(if (t.done) 0.55f else 1f),
    ) {
        Text(
            t.title, fontSize = 10.sp, lineHeight = 12.sp, color = fg, overflow = TextOverflow.Clip,
            textDecoration = if (t.done) TextDecoration.LineThrough else null,
        )
        val range = if (t.startDate != null && t.endDate != null) "${time(item.startMin!!)}-${time(item.endMin!!)}" else time(item.startMin!!)
        Text(range, fontSize = 9.sp, lineHeight = 11.sp, color = fg.copy(alpha = 0.75f), maxLines = 1, softWrap = false, overflow = TextOverflow.Clip)
    }
}
