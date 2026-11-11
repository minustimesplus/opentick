package org.opentick.android.ui.tasks

import org.opentick.android.data.TaskEntity
import org.opentick.android.data.TimelineScale
import org.opentick.android.util.isAllDay
import org.opentick.android.util.startOfWeek
import org.opentick.android.util.toLocalDateTime
import org.opentick.android.util.toMillis
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.LocalDateTime
import java.time.LocalTime
import java.time.temporal.ChronoUnit

// The timeline (gantt) view, ported from web/src/lib/timeline.ts. It works in
// whole days: every dated task is a bar from its first to its last day.

/** When a task is planned, read the same way as the web app's getSchedule(). */
data class Schedule(val allDay: Boolean, val start: LocalDateTime, val end: LocalDateTime?)

fun scheduleOf(t: TaskEntity): Schedule? {
    val s = t.startDate
    val e = t.endDate
    if (s != null && e != null && e > s) {
        val start = s.toLocalDateTime()
        return Schedule(start.hour == 0 && start.minute == 0 && isAllDay(e), start, e.toLocalDateTime())
    }
    val date = t.dueDate ?: t.startDate ?: return null
    val allDay = isAllDay(date)
    val dt = date.toLocalDateTime()
    return Schedule(allDay, if (allDay) dt.toLocalDate().atStartOfDay() else dt, null)
}

/** New due, start and end dates (epoch millis) for a task. */
data class DatePatch(val due: Long?, val start: Long?, val end: Long?)

/** Same rules as the web app's schedulePatch(): a range needs both ends, all-day dates sit at 23:59. */
fun schedulePatch(date: LocalDateTime?, allDay: Boolean, end: LocalDateTime? = null): DatePatch {
    if (date == null) return DatePatch(null, null, null)
    if (allDay) {
        val due = date.toLocalDate().atTime(23, 59).toMillis()
        if (end != null && end.toLocalDate() > date.toLocalDate()) {
            return DatePatch(due, date.toLocalDate().atStartOfDay().toMillis(), end.toLocalDate().atTime(23, 59, 59).toMillis())
        }
        return DatePatch(due, null, null)
    }
    if (end != null && end > date) return DatePatch(date.toMillis(), date.toMillis(), end.toMillis())
    return DatePatch(date.toMillis(), null, null)
}

fun TaskEntity.withDates(p: DatePatch) = copy(dueDate = p.due, startDate = p.start, endDate = p.end)

/** First and last day (inclusive) a bar covers. */
data class Span(val start: LocalDate, val end: LocalDate)

private fun LocalDateTime.atMidnight() = toLocalTime() == LocalTime.MIDNIGHT

fun taskSpan(t: TaskEntity): Span? {
    val s = scheduleOf(t) ?: return null
    val start = s.start.toLocalDate()
    val e = s.end ?: return Span(start, start)
    // A timed block ending at midnight does not cover the next day.
    val end = if (!s.allDay && e.atMidnight()) e.toLocalDate().minusDays(1) else e.toLocalDate()
    return Span(start, maxOf(start, end))
}

enum class DragMode { MOVE, START, END }

/** The span after dragging the bar (or one of its edges) by [days]. An edge never crosses the other. */
fun dragSpan(span: Span, mode: DragMode, days: Long): Span = when (mode) {
    DragMode.MOVE -> Span(span.start.plusDays(days), span.end.plusDays(days))
    DragMode.START -> Span(minOf(span.start.plusDays(days), span.end), span.end)
    DragMode.END -> Span(span.start, maxOf(span.end.plusDays(days), span.start))
}

/** Dates that put a task on [span], keeping the times of day of a timed task. */
fun spanPatch(t: TaskEntity, span: Span): DatePatch {
    val s = scheduleOf(t)
    if (s == null || s.allDay) return schedulePatch(span.start.atStartOfDay(), true, span.end.atStartOfDay())
    val start = span.start.atTime(s.start.toLocalTime())
    if (s.end == null && span.end == span.start) return schedulePatch(start, false)
    val end = if (s.end != null && s.end.atMidnight()) span.end.plusDays(1).atStartOfDay()
    else span.end.atTime((s.end ?: s.start).toLocalTime())
    // Shrunk to one day with the end time before the start time: keep just the start.
    return if (end > start) schedulePatch(start, false, end) else schedulePatch(start, false)
}

private val PAD = mapOf(TimelineScale.DAY to (14L to 42L), TimelineScale.WEEK to (35L to 120L), TimelineScale.MONTH to (90L to 365L))
/** Tasks further away than this from today don't stretch the timeline. */
private const val MAX_REACH = 730L

/** First day and number of days the timeline shows: around today, stretched to cover the tasks. */
fun timelineRange(spans: List<Span>, scale: TimelineScale, today: LocalDate, first: DayOfWeek): Pair<LocalDate, Int> {
    val (before, after) = PAD.getValue(scale)
    var start = today.minusDays(before)
    var end = today.plusDays(after)
    for (s in spans) {
        if (s.start < start && ChronoUnit.DAYS.between(s.start, today) <= MAX_REACH) start = s.start
        if (s.end > end && ChronoUnit.DAYS.between(today, s.end) <= MAX_REACH) end = s.end
    }
    if (scale == TimelineScale.MONTH) {
        start = start.minusDays(7).withDayOfMonth(1)
        end = end.plusDays(40).withDayOfMonth(1).minusDays(1)
    } else {
        start = startOfWeek(start.minusDays(7), first)
        end = startOfWeek(end.plusDays(14), first).minusDays(1)
    }
    return start to (ChronoUnit.DAYS.between(start, end).toInt() + 1)
}

enum class GroupUnit { DAY, WEEK, MONTH, YEAR }

/** A header cell: [days] days starting [offset] days after the timeline's first day. */
data class TimelineGroup(val date: LocalDate, val offset: Int, val days: Int)

/** Splits [days] days from [start] at day, week, month or year boundaries. Edge groups may be partial. */
fun timelineGroups(start: LocalDate, days: Int, unit: GroupUnit, first: DayOfWeek): List<TimelineGroup> {
    val out = mutableListOf<TimelineGroup>()
    var offset = 0
    var d = start
    while (offset < days) {
        val next = when (unit) {
            GroupUnit.DAY -> d.plusDays(1)
            GroupUnit.WEEK -> startOfWeek(d, first).plusDays(7)
            GroupUnit.MONTH -> d.withDayOfMonth(1).plusMonths(1)
            GroupUnit.YEAR -> d.withDayOfYear(1).plusYears(1)
        }
        val len = minOf(ChronoUnit.DAYS.between(d, next).toInt(), days - offset)
        out += TimelineGroup(d, offset, len)
        offset += len
        d = d.plusDays(len.toLong())
    }
    return out
}

/** Header units per scale: the top row and the bottom row. */
fun timelineUnits(scale: TimelineScale): Pair<GroupUnit, GroupUnit> = when (scale) {
    TimelineScale.MONTH -> GroupUnit.YEAR to GroupUnit.MONTH
    TimelineScale.WEEK -> GroupUnit.MONTH to GroupUnit.WEEK
    TimelineScale.DAY -> GroupUnit.MONTH to GroupUnit.DAY
}
