package org.opentick.android.util

import java.time.DayOfWeek
import java.time.Instant
import java.time.LocalDate
import java.time.LocalDateTime
import java.time.LocalTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.time.format.TextStyle
import java.time.temporal.ChronoUnit
import java.util.Locale

// All-day tasks are stored as due at 23:59 local time, like the web app.

val zone: ZoneId get() = ZoneId.systemDefault()

fun Long.toLocalDateTime(): LocalDateTime = LocalDateTime.ofInstant(Instant.ofEpochMilli(this), zone)
fun Long.toLocalDate(): LocalDate = toLocalDateTime().toLocalDate()
fun LocalDateTime.toMillis(): Long = atZone(zone).toInstant().toEpochMilli()

fun isAllDay(ms: Long): Boolean = ms.toLocalDateTime().let { it.hour == 23 && it.minute == 59 }

fun allDayMillis(date: LocalDate): Long = date.atTime(23, 59).toMillis()
fun atTimeMillis(date: LocalDate, time: LocalTime): Long = date.atTime(time).toMillis()

fun today(): LocalDate = LocalDate.now(zone)

fun isOverdue(ms: Long, now: Long = System.currentTimeMillis()): Boolean =
    if (isAllDay(ms)) ms.toLocalDate().isBefore(today()) else ms < now

private val timeFmt: DateTimeFormatter get() = DateTimeFormatter.ofLocalizedTime(FormatStyle.SHORT)

fun formatTime(ms: Long): String = ms.toLocalDateTime().toLocalTime().format(timeFmt)

/** "Today", "Tomorrow", "Mon", "Nov 14", "Nov 14, 2027" */
fun formatDay(date: LocalDate): String {
    val t = today()
    val diff = ChronoUnit.DAYS.between(t, date)
    return when {
        diff == 0L -> "Today"
        diff == 1L -> "Tomorrow"
        diff == -1L -> "Yesterday"
        diff in 2..6 -> date.dayOfWeek.getDisplayName(TextStyle.SHORT, Locale.getDefault())
        date.year == t.year -> date.format(DateTimeFormatter.ofPattern("MMM d", Locale.getDefault()))
        else -> date.format(DateTimeFormatter.ofPattern("MMM d, yyyy", Locale.getDefault()))
    }
}

/** Short label for the task row: "Today", "Tomorrow 15:00", "Nov 14" */
fun formatDue(ms: Long): String {
    val day = formatDay(ms.toLocalDate())
    return if (isAllDay(ms)) day else "$day ${formatTime(ms)}"
}

enum class Section(val title: String) {
    OVERDUE("Overdue"), TODAY("Today"), TOMORROW("Tomorrow"), NEXT7("Next 7 Days"), LATER("Later"), NODATE("No Date")
}

fun sectionOf(dueDate: Long?): Section {
    if (dueDate == null) return Section.NODATE
    if (isOverdue(dueDate)) return Section.OVERDUE
    val diff = ChronoUnit.DAYS.between(today(), dueDate.toLocalDate())
    return when {
        diff <= 0L -> Section.TODAY
        diff == 1L -> Section.TOMORROW
        diff < 7 -> Section.NEXT7
        else -> Section.LATER
    }
}

fun startOfWeek(date: LocalDate, weekStart: DayOfWeek = DayOfWeek.MONDAY): LocalDate {
    var d = date
    while (d.dayOfWeek != weekStart) d = d.minusDays(1)
    return d
}
