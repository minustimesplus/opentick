package org.opentick.android.ui.calendar

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import org.opentick.android.data.AccountMode
import org.opentick.android.data.Api
import org.opentick.android.data.ApiCalendarEvent
import org.opentick.android.data.ApiCalendarSubscription
import org.opentick.android.data.ApiException
import org.opentick.android.data.Prefs
import org.opentick.android.ui.theme.parseHex
import java.time.LocalDateTime
import java.time.OffsetDateTime
import java.time.YearMonth
import java.time.ZoneId
import java.util.concurrent.ConcurrentHashMap

/** Color of subscriptions without one, same as the web app. */
const val DEFAULT_CALENDAR_COLOR = "12B886"

/** A read-only event occurrence from a subscribed calendar, in local time. [end] is exclusive. */
data class ExtEvent(
    val key: String,
    val subscriptionId: Long,
    val calendar: String,
    val title: String,
    val description: String,
    val location: String,
    val url: String,
    val start: LocalDateTime,
    val end: LocalDateTime,
    val allDay: Boolean,
    val recurring: Boolean,
    val hexColor: String,
)

/** Turns an event from the API into local time; null if its dates don't parse. */
fun ApiCalendarEvent.toExtEvent(sub: ApiCalendarSubscription, zone: ZoneId = ZoneId.systemDefault()): ExtEvent? {
    fun local(s: String) = runCatching { OffsetDateTime.parse(s).atZoneSameInstant(zone).toLocalDateTime() }.getOrNull()
    val s = local(start) ?: return null
    val e = local(end)?.takeIf { it >= s } ?: s
    return ExtEvent(
        key = "${sub.id}:$id", subscriptionId = sub.id, calendar = sub.title,
        title = title, description = description, location = location, url = url,
        start = s, end = e, allDay = allDay, recurring = recurring,
        hexColor = sub.hexColor.removePrefix("#").ifBlank { DEFAULT_CALENDAR_COLOR },
    )
}

/**
 * Loads the events of the user's subscribed calendars (web: Calendar → Subscribe)
 * for the months the calendar tab shows. Feeds are read-only and online only:
 * events are kept in memory for a few minutes, the server caches feeds too.
 */
class CalendarFeeds(private val api: Api, private val prefs: Prefs, private val scope: CoroutineScope) {
    private class Cached(val at: Long, val events: List<ExtEvent>)

    private val _subscriptions = MutableStateFlow<List<ApiCalendarSubscription>>(emptyList())
    val subscriptions: StateFlow<List<ApiCalendarSubscription>> = _subscriptions

    /** Why a subscription's feed couldn't be loaded, by subscription id. */
    private val _errors = MutableStateFlow<Map<Long, String>>(emptyMap())
    val errors: StateFlow<Map<Long, String>> = _errors

    /** Events of the visible subscriptions in the months last asked for. */
    private val _events = MutableStateFlow<List<ExtEvent>>(emptyList())
    val events: StateFlow<List<ExtEvent>> = _events

    private val cache = ConcurrentHashMap<String, Cached>()
    private val lock = Mutex()
    @Volatile private var months: Set<YearMonth> = emptySet()
    @Volatile private var subsAt = 0L
    @Volatile private var server = ""

    /** Shows the events of [want], loading what isn't cached yet. */
    fun show(want: Set<YearMonth>) {
        months = want
        if (prefs.mode.value != AccountMode.SERVER) {
            clear()
            return
        }
        if (server != prefs.serverUrl) {
            clear()
            server = prefs.serverUrl
        }
        publish()
        scope.launch(Dispatchers.IO) { lock.withLock { load() } }
    }

    /** Drops the subscription list and cached events, e.g. to pick up a new subscription. */
    fun refresh() {
        subsAt = 0
        cache.clear()
        show(months)
    }

    private fun clear() {
        cache.clear()
        subsAt = 0
        _subscriptions.value = emptyList()
        _errors.value = emptyMap()
        _events.value = emptyList()
    }

    private fun cacheKey(sub: ApiCalendarSubscription, m: YearMonth) = "${sub.id}|${sub.url}|$m"

    private fun load() {
        val now = System.currentTimeMillis()
        if (now - subsAt > TTL_MS) {
            try {
                _subscriptions.value = api.calendarSubscriptions()
                subsAt = now
            } catch (e: Exception) {
                // Offline or signed out: keep what we have.
                return
            }
            publish()
        }
        val zone = ZoneId.systemDefault()
        for (sub in _subscriptions.value) {
            if (sub.id in prefs.hiddenCalendars.value) continue
            for (m in months.sorted()) {
                // The user navigated on; don't load months nobody looks at anymore.
                if (m !in months) continue
                val key = cacheKey(sub, m)
                val hit = cache[key]
                if (hit != null && System.currentTimeMillis() - hit.at < TTL_MS) continue
                try {
                    val from = m.atDay(1).atStartOfDay(zone).toInstant()
                    val to = m.plusMonths(1).atDay(1).atStartOfDay(zone).toInstant()
                    val res = api.calendarEvents(sub.id, from, to, zone.id)
                    cache[key] = Cached(System.currentTimeMillis(), res.events.orEmpty().mapNotNull { it.toExtEvent(sub, zone) })
                    _errors.value = _errors.value - sub.id
                } catch (e: Exception) {
                    _errors.value = _errors.value + (sub.id to errorText(e))
                    break
                }
                publish()
            }
        }
        publish()
    }

    private fun publish() {
        val hidden = prefs.hiddenCalendars.value
        val want = months
        _events.value = _subscriptions.value
            .filter { it.id !in hidden }
            .flatMap { sub -> want.flatMap { m -> cache[cacheKey(sub, m)]?.events.orEmpty() } }
            .distinctBy { it.key }
    }

    private fun errorText(e: Exception): String = when (e) {
        is ApiException -> e.message ?: "Couldn't load this calendar"
        is java.io.IOException -> "Connect to the internet to see this calendar."
        else -> e.message ?: "Couldn't load this calendar"
    }

    companion object {
        /** The server caches feeds for 15 minutes; this only avoids refetching while paging. */
        const val TTL_MS = 5 * 60_000L

        /** The months a calendar view needs, by the days it shows. */
        fun monthsOf(days: Iterable<java.time.LocalDate>): Set<YearMonth> = days.map { YearMonth.from(it) }.toSet()
    }
}

/** The color a subscription's events are drawn in. */
fun calendarColor(hex: String) = parseHex(hex.removePrefix("#").ifBlank { DEFAULT_CALENDAR_COLOR })
