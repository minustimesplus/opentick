package org.opentick.android.ui.calendar

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.ui.Data
import org.opentick.android.ui.components.TaskRowContent
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.eventColors
import org.opentick.android.util.formatLongDay
import org.opentick.android.util.toLocalDate
import org.opentick.android.util.today
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.YearMonth
import java.time.format.TextStyle
import java.util.Locale

/** Month grid with task titles inside the day cells, like TickTick's month view. */
@Composable
fun MonthGrid(month: YearMonth, first: DayOfWeek, data: Data, showCompleted: Boolean, events: List<ExtEvent>, onDay: (LocalDate) -> Unit, onItem: (CalItem) -> Unit) {
    val p = LocalExtra.current
    val start = startOfWeek(month.atDay(1), first)
    val weeks = ((java.time.temporal.ChronoUnit.DAYS.between(start, month.atEndOfMonth()) + 1 + 6) / 7).toInt()
    Column(Modifier.fillMaxSize()) {
        Row(Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
            for (i in 0 until 7) {
                Text(
                    first.plus(i.toLong()).getDisplayName(TextStyle.SHORT, Locale.getDefault()), Modifier.weight(1f),
                    textAlign = TextAlign.Center, fontSize = 12.sp, color = p.weekday,
                )
            }
        }
        Column(Modifier.fillMaxSize().clip(RoundedCornerShape(topStart = 12.dp, topEnd = 12.dp)).background(p.gridBg)) {
            for (w in 0 until weeks) {
                if (w > 0) HorizontalDivider(color = p.gridLine)
                Row(Modifier.weight(1f).fillMaxWidth()) {
                    for (i in 0 until 7) {
                        val day = start.plusDays((w * 7 + i).toLong())
                        MonthCell(day, day.month == month.month, itemsOn(day, data, showCompleted, events), Modifier.weight(1f), { onDay(day) }, onItem)
                        if (i < 6) Box(Modifier.width(1.dp).fillMaxHeight().background(p.gridLine))
                    }
                }
            }
        }
    }
}

@Composable
private fun MonthCell(day: LocalDate, inMonth: Boolean, items: List<CalItem>, modifier: Modifier, onClick: () -> Unit, onItem: (CalItem) -> Unit) {
    val p = LocalExtra.current
    val isToday = day == today()
    BoxWithConstraints(modifier.fillMaxSize().clickable(onClick = onClick)) {
        val chipH = 16.dp
        val max = ((maxHeight - 26.dp) / (chipH + 2.dp)).toInt().coerceAtLeast(0)
        Column(Modifier.fillMaxSize().padding(top = 3.dp, start = 1.dp, end = 1.dp), horizontalAlignment = Alignment.CenterHorizontally) {
            Box(Modifier.size(21.dp).clip(CircleShape).background(if (isToday) Accent else Color.Transparent), contentAlignment = Alignment.Center) {
                Text(
                    "${day.dayOfMonth}", fontSize = 12.sp, fontWeight = if (isToday) FontWeight.Bold else FontWeight.Normal,
                    color = when {
                        isToday -> Color.White
                        !inMonth -> p.textTertiary.copy(alpha = 0.6f)
                        else -> p.text
                    },
                )
            }
            Spacer(Modifier.height(2.dp))
            val shown = if (items.size > max) items.take((max - 1).coerceAtLeast(0)) else items
            for (it in shown) {
                val (bg, fg) = eventColors(it.color, p.dark)
                Text(
                    it.title, fontSize = 10.sp, lineHeight = 12.sp, color = fg, maxLines = 1, softWrap = false, overflow = TextOverflow.Clip,
                    textDecoration = if (it.done) TextDecoration.LineThrough else null,
                    modifier = Modifier.fillMaxWidth().padding(bottom = 2.dp).height(chipH).clip(RoundedCornerShape(3.dp)).background(bg)
                        .clickable { onItem(it) }.padding(horizontal = 3.dp, vertical = 1.dp),
                )
            }
            if (items.size > shown.size) Text("+${items.size - shown.size}", fontSize = 10.sp, color = p.textSecondary)
        }
    }
}

/** Twelve small months. */
@Composable
fun YearGrid(year: Int, first: DayOfWeek, data: Data, onMonth: (YearMonth) -> Unit) {
    val p = LocalExtra.current
    val busy = remember(data, year) { data.tasks.mapNotNull { t -> (t.dueDate ?: t.startDate)?.toLocalDate() }.filter { it.year == year }.toSet() }
    LazyVerticalGrid(GridCells.Fixed(3), contentPadding = PaddingValues(12.dp), horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
        items((1..12).map { YearMonth.of(year, it) }) { m ->
            Column(Modifier.clip(RoundedCornerShape(10.dp)).clickable { onMonth(m) }.padding(4.dp)) {
                Text(
                    m.month.getDisplayName(TextStyle.SHORT, Locale.getDefault()), fontSize = 14.sp, fontWeight = FontWeight.Bold,
                    color = if (m == YearMonth.now()) Accent else p.text, modifier = Modifier.padding(bottom = 4.dp),
                )
                val start = startOfWeek(m.atDay(1), first)
                for (w in 0 until 6) {
                    Row {
                        for (i in 0 until 7) {
                            val d = start.plusDays((w * 7 + i).toLong())
                            val inMonth = d.month == m.month
                            Box(Modifier.weight(1f).aspectRatio(1f).clip(CircleShape).background(if (d == today() && inMonth) Accent else Color.Transparent), contentAlignment = Alignment.Center) {
                                if (inMonth) Text(
                                    "${d.dayOfMonth}", fontSize = 7.sp,
                                    color = when {
                                        d == today() -> Color.White
                                        d in busy -> Accent
                                        else -> p.textSecondary
                                    },
                                    fontWeight = if (d in busy) FontWeight.Bold else FontWeight.Normal,
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

/** How far ahead the List view looks. */
const val AGENDA_DAYS = 90L

/** Upcoming days with tasks and events, each in a card. */
@Composable
fun AgendaList(
    data: Data,
    showCompleted: Boolean,
    events: List<ExtEvent>,
    onToggle: (Long) -> Unit,
    onTask: (Long) -> Unit,
    onEvent: (ExtEvent) -> Unit,
    onLongTask: (Long) -> Unit = {},
) {
    val p = LocalExtra.current
    val days = remember(data, showCompleted, events) {
        (0L until AGENDA_DAYS).map { today().plusDays(it) }.map { it to itemsOn(it, data, showCompleted, events) }.filter { it.second.isNotEmpty() }
    }
    if (days.isEmpty()) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { Text("Nothing planned", color = p.textTertiary) }
        return
    }
    LazyColumn(contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 110.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        for ((day, items) in days) {
            item(key = day.toString()) {
                Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(p.card)) {
                    Text(
                        (if (day == today()) "Today · " else "") + formatLongDay(day),
                        fontSize = 16.sp, fontWeight = FontWeight.Bold, color = if (day == today()) Accent else p.text,
                        modifier = Modifier.padding(start = 18.dp, top = 15.dp, bottom = 6.dp),
                    )
                    for (it in items) {
                        val t = it.task
                        if (t != null) {
                            TaskRowContent(t, data.projectMap[t.projectId], t.labelIdList.mapNotNull { l -> data.labelMap[l] }, true, { onToggle(t.id) }, { onTask(t.id) }, { onLongTask(t.id) })
                        } else {
                            EventRow(it, day) { onEvent(it.event!!) }
                        }
                    }
                    Spacer(Modifier.height(6.dp))
                }
            }
        }
    }
}

/** A subscribed calendar's event in the List view: a colored bar, the title and its time. */
@Composable
private fun EventRow(item: CalItem, day: LocalDate, onClick: () -> Unit) {
    val p = LocalExtra.current
    val e = item.event!!
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onClick).padding(horizontal = 18.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.width(4.dp).height(30.dp).clip(RoundedCornerShape(2.dp)).background(item.color))
        Spacer(Modifier.width(14.dp))
        Column(Modifier.weight(1f)) {
            Text(item.title, fontSize = 15.sp, color = p.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(
                listOf(eventTimeOn(e, day), e.calendar).filter { it.isNotBlank() }.joinToString(" · "),
                fontSize = 12.sp, color = p.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
    }
}
