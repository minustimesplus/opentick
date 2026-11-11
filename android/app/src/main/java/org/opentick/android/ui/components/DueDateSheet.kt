package org.opentick.android.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.ChevronLeft
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.outlined.AccessTime
import androidx.compose.material.icons.outlined.DateRange
import androidx.compose.material.icons.outlined.EventBusy
import androidx.compose.material.icons.outlined.LightMode
import androidx.compose.material.icons.outlined.WbTwilight
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TimePicker
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.material3.rememberTimePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.ui.calendar.startOfWeek
import org.opentick.android.ui.calendar.weekStartDay
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.util.allDayMillis
import org.opentick.android.util.atTimeMillis
import org.opentick.android.util.formatTime
import org.opentick.android.util.isAllDay
import org.opentick.android.util.toLocalDate
import org.opentick.android.util.toLocalDateTime
import org.opentick.android.util.today
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.LocalTime
import java.time.YearMonth
import java.time.format.TextStyle
import java.util.Locale

/**
 * TickTick-style date picker: quick picks, a month calendar and a time row.
 * Calls [onPick] with the new due date (null = no date).
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DueDateSheet(current: Long?, onPick: (Long?) -> Unit, onDismiss: () -> Unit, repeatLabel: String? = null) {
    val p = LocalExtra.current
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var date by remember { mutableStateOf(current?.toLocalDate() ?: today()) }
    var month by remember { mutableStateOf(YearMonth.from(date)) }
    var time by remember { mutableStateOf(current?.takeIf { !isAllDay(it) }?.toLocalDateTime()?.toLocalTime()) }
    var showTime by remember { mutableStateOf(false) }

    fun commit(d: LocalDate = date) = onPick(time?.let { atTimeMillis(d, it) } ?: allDayMillis(d))

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet, containerColor = p.card, shape = RoundedCornerShape(topStart = 20.dp, topEnd = 20.dp)) {
        Column(Modifier.verticalScroll(rememberScrollState()).navigationBarsPadding()) {
            Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp), horizontalArrangement = Arrangement.SpaceEvenly) {
                QuickPick(Icons.Outlined.LightMode, "Today") { commit(today()) }
                QuickPick(Icons.Outlined.WbTwilight, "Tomorrow") { commit(today().plusDays(1)) }
                QuickPick(Icons.Outlined.DateRange, "Next Mon") {
                    var d = today().plusDays(1)
                    while (d.dayOfWeek != DayOfWeek.MONDAY) d = d.plusDays(1)
                    commit(d)
                }
                QuickPick(Icons.Outlined.EventBusy, "No Date") { onPick(null) }
            }
            Spacer(Modifier.height(6.dp))
            MonthPicker(month, date, { month = it }) { date = it }
            HorizontalDivider(Modifier.padding(horizontal = 20.dp, vertical = 6.dp), color = p.divider)
            OptionRow(Icons.Outlined.AccessTime, "Time", time?.let { formatTime(atTimeMillis(today(), it)) } ?: "None", time != null) { showTime = true }
            if (repeatLabel != null) OptionRow(Icons.Outlined.DateRange, "Repeat", repeatLabel, true, null)
            Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp)) {
                TextButton(onClick = { onPick(null) }) { Text("Clear", color = p.textSecondary) }
                Spacer(Modifier.weight(1f))
                TextButton(onClick = onDismiss) { Text("Cancel", color = p.textSecondary) }
                TextButton(onClick = { commit() }) { Text("OK", color = Accent, fontWeight = FontWeight.SemiBold) }
            }
        }
    }

    if (showTime) {
        val tp = rememberTimePickerState(time?.hour ?: 9, time?.minute ?: 0)
        AlertDialog(
            onDismissRequest = { showTime = false },
            confirmButton = { TextButton(onClick = { time = LocalTime.of(tp.hour, tp.minute); showTime = false }) { Text("OK") } },
            dismissButton = {
                Row {
                    if (time != null) TextButton(onClick = { time = null; showTime = false }) { Text("No Time") }
                    TextButton(onClick = { showTime = false }) { Text("Cancel") }
                }
            },
            text = { TimePicker(state = tp) },
            containerColor = p.card,
        )
    }
}

/** A month of days; the selected day in a blue circle, today in blue. */
@Composable
fun MonthPicker(month: YearMonth, selected: LocalDate?, onMonth: (YearMonth) -> Unit, onPick: (LocalDate) -> Unit) {
    val p = LocalExtra.current
    val first = weekStartDay(0)
    Column(Modifier.padding(horizontal = 16.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(start = 6.dp)) {
            Text(
                month.month.getDisplayName(TextStyle.FULL_STANDALONE, Locale.getDefault()).replaceFirstChar { it.uppercase() } +
                    if (month.year != today().year) " ${month.year}" else "",
                fontSize = 16.sp, fontWeight = FontWeight.Bold, color = p.text, modifier = Modifier.weight(1f),
            )
            IconButton(onClick = { onMonth(month.minusMonths(1)) }) { Icon(Icons.Filled.ChevronLeft, "Previous month", tint = p.textSecondary) }
            IconButton(onClick = { onMonth(month.plusMonths(1)) }) { Icon(Icons.Filled.ChevronRight, "Next month", tint = p.textSecondary) }
        }
        Row {
            for (i in 0 until 7) {
                Text(first.plus(i.toLong()).getDisplayName(TextStyle.NARROW, Locale.getDefault()), Modifier.weight(1f), textAlign = TextAlign.Center, fontSize = 12.sp, color = p.textTertiary)
            }
        }
        val start = startOfWeek(month.atDay(1), first)
        for (w in 0 until 6) {
            Row {
                for (i in 0 until 7) {
                    val d = start.plusDays((w * 7 + i).toLong())
                    val inMonth = d.month == month.month
                    val isSel = d == selected
                    Box(Modifier.weight(1f).height(42.dp), contentAlignment = Alignment.Center) {
                        Box(
                            Modifier.size(36.dp).clip(CircleShape).background(if (isSel) Accent else Color.Transparent).clickable { onPick(d) },
                            contentAlignment = Alignment.Center,
                        ) {
                            Text(
                                "${d.dayOfMonth}", fontSize = 15.sp,
                                fontWeight = if (isSel || d == today()) FontWeight.SemiBold else FontWeight.Normal,
                                color = when {
                                    isSel -> Color.White
                                    d == today() -> Accent
                                    !inMonth -> p.textTertiary.copy(alpha = 0.6f)
                                    else -> p.text
                                },
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun OptionRow(icon: ImageVector, title: String, value: String, active: Boolean, onClick: (() -> Unit)?) {
    val p = LocalExtra.current
    Row(
        Modifier.fillMaxWidth().then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier).padding(horizontal = 24.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, null, tint = p.textSecondary, modifier = Modifier.size(22.dp))
        Spacer(Modifier.width(16.dp))
        Text(title, Modifier.weight(1f), fontSize = 16.sp, color = p.text)
        Text(value, fontSize = 14.sp, color = if (active) Accent else p.textTertiary)
        if (onClick != null) Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, null, tint = p.count, modifier = Modifier.size(20.dp))
    }
}

@Composable
private fun QuickPick(icon: ImageVector, label: String, onClick: () -> Unit) {
    val p = LocalExtra.current
    Column(
        Modifier.clip(RoundedCornerShape(12.dp)).clickable(onClick = onClick).padding(horizontal = 10.dp, vertical = 10.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Icon(icon, null, tint = Accent, modifier = Modifier.size(24.dp))
        Spacer(Modifier.height(5.dp))
        Text(label, fontSize = 12.sp, color = p.textSecondary)
    }
}

