package org.opentick.android.util

import java.time.DayOfWeek
import java.time.LocalDate
import java.time.LocalTime

/**
 * TickTick-style smart recognition for the quick add bar, matching the web app:
 *   dates     today, tomorrow, tmr, day after tomorrow, monday..sunday, next week, in 3 days
 *   times     3pm, 3:30pm, 15:00, at 15:00
 *   priority  !high !medium !low  (!h !m !l, !3 !2 !1, !!! !! !)
 *   tags      #tag
 *   list      ~List or ^List
 */
data class Parsed(
    val title: String,
    val date: LocalDate?,
    val time: LocalTime?,
    val priority: Int?,
    val labels: List<String>,
    val list: String?,
    val matches: List<String>,
)

object QuickAddParser {
    private val weekdays = mapOf(
        "monday" to DayOfWeek.MONDAY, "mon" to DayOfWeek.MONDAY,
        "tuesday" to DayOfWeek.TUESDAY, "tue" to DayOfWeek.TUESDAY,
        "wednesday" to DayOfWeek.WEDNESDAY, "wed" to DayOfWeek.WEDNESDAY,
        "thursday" to DayOfWeek.THURSDAY, "thu" to DayOfWeek.THURSDAY,
        "friday" to DayOfWeek.FRIDAY, "fri" to DayOfWeek.FRIDAY,
        "saturday" to DayOfWeek.SATURDAY, "sat" to DayOfWeek.SATURDAY,
        "sunday" to DayOfWeek.SUNDAY, "sun" to DayOfWeek.SUNDAY,
    )

    fun parse(input: String, now: LocalDate = today()): Parsed {
        var text = " $input "
        val matches = mutableListOf<String>()
        var date: LocalDate? = null
        var time: LocalTime? = null
        var priority: Int? = null
        val labels = mutableListOf<String>()
        var list: String? = null

        fun take(re: Regex, handle: (MatchResult) -> Boolean) {
            val m = re.find(text) ?: return
            if (handle(m)) {
                matches += m.value.trim()
                text = text.removeRange(m.range).let { it.substring(0, m.range.first) + " " + it.substring(m.range.first) }
            }
        }

        take(Regex("""(?<=\s)#([\p{L}\p{N}_\-/]+)(?=\s)""")) { labels += it.groupValues[1]; true }
        while (Regex("""(?<=\s)#([\p{L}\p{N}_\-/]+)(?=\s)""").containsMatchIn(text)) {
            take(Regex("""(?<=\s)#([\p{L}\p{N}_\-/]+)(?=\s)""")) { labels += it.groupValues[1]; true }
        }
        take(Regex("""(?<=\s)[~^]([\p{L}\p{N}_\-]+)(?=\s)""")) { list = it.groupValues[1]; true }
        take(Regex("""(?<=\s)(!high|!h|!3|!!!|!medium|!m|!2|!!|!low|!l|!1|!none|!0)(?=\s)""", RegexOption.IGNORE_CASE)) {
            priority = when (it.value.lowercase()) {
                "!high", "!h", "!3", "!!!" -> 3
                "!medium", "!m", "!2", "!!" -> 2
                "!low", "!l", "!1" -> 1
                else -> 0
            }
            true
        }
        take(Regex("""(?<=\s)(day after tomorrow)(?=\s)""", RegexOption.IGNORE_CASE)) { date = now.plusDays(2); true }
        take(Regex("""(?<=\s)(today|tod)(?=\s)""", RegexOption.IGNORE_CASE)) { date = now; true }
        take(Regex("""(?<=\s)(tomorrow|tmr|tmrw)(?=\s)""", RegexOption.IGNORE_CASE)) { date = now.plusDays(1); true }
        take(Regex("""(?<=\s)next week(?=\s)""", RegexOption.IGNORE_CASE)) {
            var d = now.plusDays(1)
            while (d.dayOfWeek != DayOfWeek.MONDAY) d = d.plusDays(1)
            date = d; true
        }
        take(Regex("""(?<=\s)in (\d{1,3}) (day|days|week|weeks)(?=\s)""", RegexOption.IGNORE_CASE)) {
            val n = it.groupValues[1].toLong()
            date = if (it.groupValues[2].startsWith("week", true)) now.plusWeeks(n) else now.plusDays(n); true
        }
        if (date == null) {
            take(Regex("""(?<=\s)(next |on )?(monday|tuesday|wednesday|thursday|friday|saturday|sunday|mon|tue|wed|thu|fri|sat|sun)(?=\s)""", RegexOption.IGNORE_CASE)) {
                val dow = weekdays[it.groupValues[2].lowercase()]!!
                var d = now.plusDays(1)
                while (d.dayOfWeek != dow) d = d.plusDays(1)
                date = d; true
            }
        }
        take(Regex("""(?<=\s)(at )?(\d{1,2})(?::(\d{2}))?\s?(am|pm)(?=\s)""", RegexOption.IGNORE_CASE)) {
            var h = it.groupValues[2].toInt()
            val min = it.groupValues[3].ifEmpty { "0" }.toInt()
            if (h !in 1..12 || min > 59) return@take false
            val pm = it.groupValues[4].equals("pm", true)
            if (pm && h != 12) h += 12
            if (!pm && h == 12) h = 0
            time = LocalTime.of(h, min); true
        }
        if (time == null) {
            take(Regex("""(?<=\s)(at )?([01]?\d|2[0-3]):([0-5]\d)(?=\s)""", RegexOption.IGNORE_CASE)) {
                time = LocalTime.of(it.groupValues[2].toInt(), it.groupValues[3].toInt()); true
            }
        }
        if (time != null && date == null) {
            date = if (time!!.isBefore(LocalTime.now())) now.plusDays(1) else now
        }
        val title = text.replace(Regex("\\s+"), " ").trim()
        return Parsed(title.ifEmpty { input.trim() }, date, time, priority, labels, list, matches)
    }
}
