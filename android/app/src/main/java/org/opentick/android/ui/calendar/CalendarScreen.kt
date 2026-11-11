package org.opentick.android.ui.calendar

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.ChevronLeft
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.QuickAddSheet
import org.opentick.android.ui.components.TaskRowContent
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.priorityColor
import org.opentick.android.util.formatDay
import org.opentick.android.util.toLocalDate
import org.opentick.android.util.today
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.YearMonth
import java.time.format.TextStyle
import java.time.temporal.WeekFields
import java.util.Locale

/** Month calendar with the selected day's tasks below, like TickTick's calendar tab. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CalendarScreen(vm: MainViewModel) {
    val data by vm.data.collectAsState()
    var month by rememberSaveable { mutableStateOf(YearMonth.now()) }
    var selected by rememberSaveable { mutableStateOf(today()) }
    var adding by remember { mutableStateOf(false) }
    val extra = LocalExtra.current
    val firstDay: DayOfWeek = WeekFields.of(Locale.getDefault()).firstDayOfWeek

    val byDay = remember(data.tasks) { data.tasks.filter { it.dueDate != null }.groupBy { it.dueDate!!.toLocalDate() } }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background),
                title = {
                    Text(
                        month.month.getDisplayName(TextStyle.FULL, Locale.getDefault()) + if (month.year != today().year) " ${month.year}" else "",
                        fontWeight = FontWeight.Bold,
                    )
                },
                actions = {
                    TextButton(onClick = { month = YearMonth.now(); selected = today() }) { Text("Today") }
                    IconButton(onClick = { month = month.minusMonths(1) }) { Icon(Icons.Filled.ChevronLeft, "Previous month") }
                    IconButton(onClick = { month = month.plusMonths(1) }) { Icon(Icons.Filled.ChevronRight, "Next month") }
                },
            )
        },
        floatingActionButton = {
            FloatingActionButton(onClick = { adding = true }, containerColor = Accent, contentColor = Color.White, shape = CircleShape) {
                Icon(Icons.Filled.Add, "Add task")
            }
        },
    ) { pad ->
        Column(Modifier.padding(pad).fillMaxSize()) {
            Row(Modifier.fillMaxWidth().padding(horizontal = 8.dp)) {
                for (i in 0 until 7) {
                    val d = firstDay.plus(i.toLong())
                    Text(
                        d.getDisplayName(TextStyle.NARROW, Locale.getDefault()), Modifier.weight(1f),
                        textAlign = TextAlign.Center, fontSize = 12.sp, color = extra.textTertiary,
                    )
                }
            }
            val first = month.atDay(1)
            val offset = ((first.dayOfWeek.value - firstDay.value) + 7) % 7
            val start = first.minusDays(offset.toLong())
            val weeks = ((offset + month.lengthOfMonth()) + 6) / 7
            Column(Modifier.padding(horizontal = 8.dp)) {
                for (w in 0 until weeks) {
                    Row(Modifier.fillMaxWidth()) {
                        for (i in 0 until 7) {
                            val day = start.plusDays((w * 7 + i).toLong())
                            DayCell(
                                day, inMonth = day.month == month.month, isSelected = day == selected,
                                tasks = byDay[day].orEmpty().filter { !it.done }.map { it.priority },
                                modifier = Modifier.weight(1f),
                            ) { selected = day; if (day.month != month.month) month = YearMonth.from(day) }
                        }
                    }
                }
            }
            HorizontalDivider(Modifier.padding(top = 6.dp), color = extra.divider)
            Text(formatDay(selected), fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(start = 16.dp, top = 12.dp, bottom = 4.dp))
            val dayTasks = byDay[selected].orEmpty().sortedWith(compareBy({ it.done }, { it.dueDate }))
            if (dayTasks.isEmpty()) {
                Text("No tasks", color = extra.textTertiary, fontSize = 14.sp, modifier = Modifier.padding(16.dp))
            }
            LazyColumn(Modifier.weight(1f)) {
                items(dayTasks, key = { it.id }) { t ->
                    TaskRowContent(
                        t, data.projectMap[t.projectId], t.labelIdList.mapNotNull { data.labelMap[it] }, showProject = true,
                        onToggle = { vm.toggleDone(t) }, onClick = { vm.openTaskId.value = t.id },
                    )
                }
                item { Spacer(Modifier.height(88.dp)) }
            }
        }
    }

    if (adding) {
        QuickAddSheet(
            projects = data.projects, labels = data.labels, defaultProject = null, defaultDate = selected, defaultLabel = null,
            onAdd = { vm.quickAdd(it.text, it.projectId, it.due, it.datePicked, it.priority, it.labels) },
            onDismiss = { adding = false },
        )
    }
}

@Composable
private fun DayCell(day: LocalDate, inMonth: Boolean, isSelected: Boolean, tasks: List<Int>, modifier: Modifier, onClick: () -> Unit) {
    val extra = LocalExtra.current
    val isToday = day == today()
    Column(
        modifier.aspectRatio(1f).clip(CircleShape).clickable(onClick = onClick),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Box(
            Modifier.size(32.dp).clip(CircleShape).background(
                when {
                    isSelected -> Accent
                    isToday -> Accent.copy(alpha = 0.12f)
                    else -> Color.Transparent
                },
            ),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                "${day.dayOfMonth}", fontSize = 14.sp,
                fontWeight = if (isToday || isSelected) FontWeight.Bold else FontWeight.Normal,
                color = when {
                    isSelected -> Color.White
                    isToday -> Accent
                    !inMonth -> extra.textTertiary.copy(alpha = 0.5f)
                    else -> MaterialTheme.colorScheme.onBackground
                },
            )
        }
        Row(Modifier.height(6.dp).padding(top = 2.dp), horizontalArrangement = Arrangement.spacedBy(2.dp)) {
            for (p in tasks.sortedDescending().take(3)) {
                Box(Modifier.size(4.dp).clip(CircleShape).background(priorityColor(p, extra.textTertiary)))
            }
        }
    }
}
