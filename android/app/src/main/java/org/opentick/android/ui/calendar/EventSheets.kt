package org.opentick.android.ui.calendar

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Link
import androidx.compose.material.icons.outlined.LocationOn
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.Repeat
import androidx.compose.material.icons.outlined.Schedule
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.util.formatLongDay
import java.time.LocalDate
import java.time.LocalDateTime
import java.time.LocalTime
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

private val timeFmt: DateTimeFormatter get() = DateTimeFormatter.ofLocalizedTime(FormatStyle.SHORT)
private fun LocalDateTime.time(): String = toLocalTime().format(timeFmt)

/** The last day an event covers; its end is exclusive. */
fun lastDayOf(e: ExtEvent): LocalDate =
    if (e.end > e.start && e.end.toLocalTime() == LocalTime.MIDNIGHT) e.end.toLocalDate().minusDays(1) else e.end.toLocalDate()

/** When [e] happens on [day], for the List view: "All day", "09:00 – 10:00", "From 22:00", "Until 02:00". */
fun eventTimeOn(e: ExtEvent, day: LocalDate): String {
    val first = e.start.toLocalDate()
    val last = lastDayOf(e)
    return when {
        e.allDay -> "All day"
        first == last -> if (e.end > e.start) "${e.start.time()} – ${e.end.time()}" else e.start.time()
        day == first -> "From ${e.start.time()}"
        day == last && e.end.toLocalTime() != LocalTime.MIDNIGHT -> "Until ${e.end.time()}"
        else -> "All day"
    }
}

/** The full date and time of [e], for its details. */
fun eventWhen(e: ExtEvent): String {
    val first = e.start.toLocalDate()
    val last = lastDayOf(e)
    return when {
        e.allDay && first == last -> "${formatLongDay(first)} · All day"
        e.allDay -> "${formatLongDay(first)} – ${formatLongDay(last)}"
        first == last -> "${formatLongDay(first)} · ${eventTimeOn(e, first)}"
        else -> "${formatLongDay(first)}, ${e.start.time()} – ${formatLongDay(e.end.toLocalDate())}, ${e.end.time()}"
    }
}

/** Details of a subscribed calendar's event. Events are read-only; change them in their own calendar. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EventSheet(event: ExtEvent, onDismiss: () -> Unit) {
    val p = LocalExtra.current
    val context = LocalContext.current
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Column(Modifier.navigationBarsPadding().verticalScroll(rememberScrollState()).padding(start = 22.dp, end = 22.dp, bottom = 24.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Spacer(Modifier.size(10.dp).clip(CircleShape).background(calendarColor(event.hexColor)))
                Spacer(Modifier.width(8.dp))
                Text(event.calendar, fontSize = 13.sp, color = p.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            Spacer(Modifier.size(8.dp))
            Text(event.title.ifBlank { "(No title)" }, fontSize = 20.sp, fontWeight = FontWeight.SemiBold, color = p.text)
            Spacer(Modifier.size(14.dp))
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                DetailRow(Icons.Outlined.Schedule, eventWhen(event))
                if (event.recurring) DetailRow(Icons.Outlined.Repeat, "Repeats")
                if (event.location.isNotBlank()) DetailRow(Icons.Outlined.LocationOn, event.location)
                if (event.url.isNotBlank()) {
                    DetailRow(Icons.Outlined.Link, event.url, color = Accent) {
                        runCatching { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(event.url))) }
                    }
                }
            }
            if (event.description.isNotBlank()) {
                Spacer(Modifier.size(16.dp))
                Text(event.description, fontSize = 14.sp, lineHeight = 20.sp, color = p.text)
            }
        }
    }
}

@Composable
private fun DetailRow(icon: ImageVector, text: String, color: androidx.compose.ui.graphics.Color? = null, onClick: (() -> Unit)? = null) {
    val p = LocalExtra.current
    Row(
        Modifier.fillMaxWidth().then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, null, tint = p.icon, modifier = Modifier.size(20.dp))
        Spacer(Modifier.width(14.dp))
        Text(text, fontSize = 14.sp, color = color ?: p.text, maxLines = 3, overflow = TextOverflow.Ellipsis)
    }
}

/** The subscribed calendars, each with a switch to show or hide it on the calendar tab. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CalendarsSheet(feeds: CalendarFeeds, hidden: Set<Long>, onToggle: (Long, Boolean) -> Unit, onDismiss: () -> Unit) {
    val p = LocalExtra.current
    val subs by feeds.subscriptions.collectAsState()
    val errors by feeds.errors.collectAsState()
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Column(Modifier.navigationBarsPadding().verticalScroll(rememberScrollState()).padding(bottom = 16.dp)) {
            Row(Modifier.fillMaxWidth().padding(start = 22.dp, end = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                Text("Subscribed calendars", fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = p.text, modifier = Modifier.weight(1f))
                IconButton(onClick = { feeds.refresh() }) { Icon(Icons.Outlined.Refresh, "Reload calendars", tint = p.icon) }
            }
            if (subs.isEmpty()) {
                Text(
                    "No subscribed calendars yet. Subscribe to an ICS calendar in the web app's calendar and it shows up here.",
                    fontSize = 14.sp, color = p.textSecondary, modifier = Modifier.padding(horizontal = 22.dp, vertical = 12.dp),
                )
            }
            for (sub in subs) {
                val visible = sub.id !in hidden
                Row(
                    Modifier.fillMaxWidth().clickable { onToggle(sub.id, visible) }.padding(horizontal = 22.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Spacer(Modifier.size(12.dp).clip(CircleShape).background(calendarColor(sub.hexColor)))
                    Spacer(Modifier.width(14.dp))
                    Column(Modifier.weight(1f)) {
                        Text(sub.title, fontSize = 15.sp, color = p.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        errors[sub.id]?.let { Text(it, fontSize = 12.sp, color = p.textSecondary, maxLines = 2, overflow = TextOverflow.Ellipsis) }
                    }
                    Switch(visible, { onToggle(sub.id, visible) }, colors = SwitchDefaults.colors(checkedTrackColor = Accent))
                }
            }
        }
    }
}
