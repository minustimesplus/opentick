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

private fun localized(skeleton: String): DateTimeFormatter {
    val locale = Locale.getDefault()
    return DateTimeFormatter.ofPattern(android.text.format.DateFormat.getBestDateTimePattern(locale, skeleton), locale)
}

/** "6 Nov" / "Nov 6" depending on the locale, like TickTick. */
fun formatShortDate(date: LocalDate): String =
    date.format(localized(if (date.year == today().year) "dMMM" else "dMMMyyyy"))

/** "Today", "Tomorrow", "Yesterday", or "6 Nov" */
fun formatDay(date: LocalDate): String {
    val diff = ChronoUnit.DAYS.between(today(), date)
    return when (diff) {
        0L -> "Today"
        1L -> "Tomorrow"
        -1L -> "Yesterday"
        else -> formatShortDate(date)
    }
}

/** Long day label for headers: "Wed, 11 Nov" */
fun formatLongDay(date: LocalDate): String = date.format(localized("EEEdMMM"))

/** Task row label: "Today", "14:30" for today with a time, "6 Nov" */
fun formatDue(ms: Long): String {
    val date = ms.toLocalDate()
    if (!isAllDay(ms) && date == today()) return formatTime(ms)
    return formatDay(date)
}

/** Label with the time: "Today, 14:30", "6 Nov, 09:00" */
fun formatDueLong(ms: Long): String {
    val day = formatDay(ms.toLocalDate())
    return if (isAllDay(ms)) day else "$day, ${formatTime(ms)}"
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
