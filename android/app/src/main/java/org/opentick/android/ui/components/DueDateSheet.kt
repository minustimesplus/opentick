package org.opentick.android.ui.components

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.AccessTime
import androidx.compose.material.icons.outlined.CalendarMonth
import androidx.compose.material.icons.outlined.DateRange
import androidx.compose.material.icons.outlined.EventBusy
import androidx.compose.material.icons.outlined.LightMode
import androidx.compose.material.icons.outlined.WbTwilight
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TimePicker
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.material3.rememberTimePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
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
import java.time.Instant
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneOffset

/**
 * TickTick-style date picker: quick picks on top, a month calendar, and an
 * optional time. Calls [onPick] with the new due date (null = no date).
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DueDateSheet(current: Long?, onPick: (Long?) -> Unit, onDismiss: () -> Unit) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val initialDate = current?.toLocalDate() ?: today()
    var time by remember { mutableStateOf(current?.takeIf { !isAllDay(it) }?.toLocalDateTime()?.toLocalTime()) }
    val dateState = rememberDatePickerState(
        initialSelectedDateMillis = initialDate.atStartOfDay().toInstant(ZoneOffset.UTC).toEpochMilli(),
    )
    var showTime by remember { mutableStateOf(false) }

    fun selectedDate(): LocalDate = dateState.selectedDateMillis
        ?.let { Instant.ofEpochMilli(it).atZone(ZoneOffset.UTC).toLocalDate() } ?: initialDate

    fun commit(date: LocalDate) {
        onPick(time?.let { atTimeMillis(date, it) } ?: allDayMillis(date))
    }

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet, containerColor = MaterialTheme.colorScheme.surface) {
        Column(Modifier.verticalScroll(rememberScrollState()).navigationBarsPadding()) {
            Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp), horizontalArrangement = Arrangement.SpaceEvenly) {
                QuickPick(Icons.Outlined.LightMode, "Today") { commit(today()) }
                QuickPick(Icons.Outlined.WbTwilight, "Tomorrow") { commit(today().plusDays(1)) }
                QuickPick(Icons.Outlined.DateRange, "Next Mon") {
                    var d = today().plusDays(1)
                    while (d.dayOfWeek != DayOfWeek.MONDAY) d = d.plusDays(1)
                    commit(d)
                }
                QuickPick(Icons.Outlined.EventBusy, "No date") { onPick(null) }
            }
            DatePicker(
                state = dateState,
                title = null,
                headline = null,
                showModeToggle = false,
                colors = DatePickerDefaults.colors(containerColor = MaterialTheme.colorScheme.surface),
            )
            HorizontalDivider(color = LocalExtra.current.divider)
            Row(
                Modifier.fillMaxWidth().clickable { showTime = true }.padding(horizontal = 20.dp, vertical = 14.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(Icons.Outlined.AccessTime, null, tint = LocalExtra.current.textSecondary)
                Spacer(Modifier.size(16.dp))
                Text("Time", Modifier.weight(1f))
                Text(time?.let { formatTime(atTimeMillis(today(), it)) } ?: "None", color = if (time != null) Accent else LocalExtra.current.textTertiary)
                if (time != null) {
                    TextButton(onClick = { time = null }) { Text("Clear") }
                }
            }
            HorizontalDivider(color = LocalExtra.current.divider)
            Row(Modifier.fillMaxWidth().padding(12.dp), horizontalArrangement = Arrangement.End) {
                TextButton(onClick = onDismiss) { Text("Cancel") }
                TextButton(onClick = { commit(selectedDate()) }) { Text("OK") }
            }
        }
    }

    if (showTime) {
        val tp = rememberTimePickerState(time?.hour ?: 9, time?.minute ?: 0)
        AlertDialog(
            onDismissRequest = { showTime = false },
            confirmButton = { TextButton(onClick = { time = LocalTime.of(tp.hour, tp.minute); showTime = false }) { Text("OK") } },
            dismissButton = { TextButton(onClick = { showTime = false }) { Text("Cancel") } },
            text = { TimePicker(state = tp) },
        )
    }
}

@Composable
private fun QuickPick(icon: ImageVector, label: String, onClick: () -> Unit) {
    Column(
        Modifier.clickable(onClick = onClick).padding(horizontal = 10.dp, vertical = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Icon(icon, null, tint = Accent)
        Spacer(Modifier.height(4.dp))
        Text(label, fontSize = 12.sp, color = LocalExtra.current.textSecondary)
    }
}
