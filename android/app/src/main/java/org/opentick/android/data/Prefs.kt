package org.opentick.android.data

import android.content.Context
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject

/** What the app is signed in to. */
enum class AccountMode { NONE, LOCAL, SERVER }

/**
 * Small key-value settings: the server, tokens and display preferences.
 * Kept in app-private SharedPreferences, which only this app can read.
 */
class Prefs(context: Context) {
    private val sp = context.getSharedPreferences("opentick", Context.MODE_PRIVATE)

    private val _mode = MutableStateFlow(readMode())
    val mode: StateFlow<AccountMode> = _mode

    private fun readMode() = runCatching { AccountMode.valueOf(sp.getString("mode", null) ?: "NONE") }
        .getOrDefault(AccountMode.NONE)

    fun setMode(mode: AccountMode) {
        sp.edit().putString("mode", mode.name).apply()
        _mode.value = mode
    }

    /** Server base URL without trailing slash, e.g. https://opentick.example.com */
    var serverUrl: String
        get() = sp.getString("server_url", "") ?: ""
        set(v) = sp.edit().putString("server_url", v).apply()

    var accessToken: String?
        get() = sp.getString("access_token", null)
        set(v) = sp.edit().putString("access_token", v).apply()

    var refreshToken: String?
        get() = sp.getString("refresh_token", null)
        set(v) = sp.edit().putString("refresh_token", v).apply()

    var username: String
        get() = sp.getString("username", "") ?: ""
        set(v) = sp.edit().putString("username", v).apply()

    var displayName: String
        get() = sp.getString("display_name", "") ?: ""
        set(v) = sp.edit().putString("display_name", v).apply()

    var lastSync: Long
        get() = sp.getLong("last_sync", 0)
        set(v) = sp.edit().putLong("last_sync", v).apply()

    /** 0 = follow system, 1 = light, 2 = dark */
    private val _theme = MutableStateFlow(sp.getInt("theme", 0))
    val theme: StateFlow<Int> = _theme
    fun setTheme(v: Int) {
        sp.edit().putInt("theme", v).apply()
        _theme.value = v
    }

    private val _showCompleted = MutableStateFlow(sp.getBoolean("show_completed", true))
    val showCompleted: StateFlow<Boolean> = _showCompleted
    fun setShowCompleted(v: Boolean) {
        sp.edit().putBoolean("show_completed", v).apply()
        _showCompleted.value = v
    }

    var remindersEnabled: Boolean
        get() = sp.getBoolean("reminders", true)
        set(v) = sp.edit().putBoolean("reminders", v).apply()

    var focusMinutes: Int
        get() = sp.getInt("focus_minutes", 25)
        set(v) = sp.edit().putInt("focus_minutes", v).apply()

    var breakMinutes: Int
        get() = sp.getInt("break_minutes", 5)
        set(v) = sp.edit().putInt("break_minutes", v).apply()

    /** Install app updates in the background when Android allows it (Settings → About). */
    var autoUpdate: Boolean
        get() = sp.getBoolean("auto_update", true)
        set(v) = sp.edit().putBoolean("auto_update", v).apply()

    var lastUpdateCheck: Long
        get() = sp.getLong("last_update_check", 0)
        set(v) = sp.edit().putLong("last_update_check", v).apply()

    /** versionCode of the last "update available" notification, so each version notifies once. */
    var notifiedUpdate: Int
        get() = sp.getInt("notified_update", 0)
        set(v) = sp.edit().putInt("notified_update", v).apply()

    private fun stringFlow(key: String, default: String): MutableStateFlow<String> =
        MutableStateFlow(sp.getString(key, default) ?: default)

    /** Bottom tabs in order, e.g. "TASKS,MATRIX,CALENDAR,POMO" (Settings → Tab Bar). */
    private val _tabs = stringFlow("tabs", "TASKS,MATRIX,CALENDAR,POMO")
    val tabs: StateFlow<String> = _tabs
    fun setTabs(v: String) { sp.edit().putString("tabs", v).apply(); _tabs.value = v }

    /** LIST, YEAR, MONTH, WEEK, THREE_DAY or DAY */
    private val _calendarView = stringFlow("calendar_view", "WEEK")
    val calendarView: StateFlow<String> = _calendarView
    fun setCalendarView(v: String) { sp.edit().putString("calendar_view", v).apply(); _calendarView.value = v }

    /** Subscribed calendars hidden from the calendar tab, by subscription id. */
    private val _hiddenCalendars = MutableStateFlow(parseIds(sp.getString("hidden_calendars", "")))
    val hiddenCalendars: StateFlow<Set<Long>> = _hiddenCalendars
    fun setCalendarHidden(id: Long, hidden: Boolean) {
        val next = if (hidden) _hiddenCalendars.value + id else _hiddenCalendars.value - id
        sp.edit().putString("hidden_calendars", next.joinToString(",")).apply()
        _hiddenCalendars.value = next
    }
    private fun parseIds(s: String?): Set<Long> = s.orEmpty().split(',').mapNotNull { it.trim().toLongOrNull() }.toSet()

    /** 0 = from the locale, otherwise java.time.DayOfWeek value (1 Monday … 7 Sunday). */
    private val _weekStart = MutableStateFlow(sp.getInt("week_start", 0))
    val weekStart: StateFlow<Int> = _weekStart
    fun setWeekStart(v: Int) { sp.edit().putInt("week_start", v).apply(); _weekStart.value = v }

    /** Smart lists shown in the drawer (Settings → General). */
    private val _smartLists = MutableStateFlow(sp.getStringSet("smart_lists", null) ?: DEFAULT_SMART_LISTS)
    val smartLists: StateFlow<Set<String>> = _smartLists
    fun setSmartLists(v: Set<String>) { sp.edit().putStringSet("smart_lists", v).apply(); _smartLists.value = v }

    /** View settings shared with the other devices (see [ViewSync]). */
    private val _views = MutableStateFlow(Views(readJson("views") ?: JsonObject(emptyMap())))
    val views: StateFlow<Views> = _views
    fun setViews(v: Views) { sp.edit().putString("views", v.json.toString()).apply(); _views.value = v }

    /** The server's views as of the last sync, to tell local edits from remote ones. */
    var viewsSnapshot: JsonObject?
        get() = readJson("views_snapshot")
        set(v) = sp.edit().putString("views_snapshot", v?.toString()).apply()

    private fun readJson(key: String): JsonObject? =
        sp.getString(key, null)?.let { runCatching { Json.parseToJsonElement(it) as JsonObject }.getOrNull() }

    /** Ids of reminder alarms currently scheduled, so they can be cancelled. */
    var scheduledReminders: Set<String>
        get() = sp.getStringSet("scheduled_reminders", emptySet()) ?: emptySet()
        set(v) = sp.edit().putStringSet("scheduled_reminders", v).apply()

    companion object {
        val DEFAULT_SMART_LISTS = setOf("TODAY", "INBOX", "TAGS", "COMPLETED", "TRASH")
    }

    fun clearAccount() {
        sp.edit()
            .remove("server_url").remove("access_token").remove("refresh_token")
            .remove("username").remove("display_name").remove("last_sync")
            .remove("views").remove("views_snapshot").remove("hidden_calendars")
            .apply()
        _views.value = Views()
        _hiddenCalendars.value = emptySet()
        setMode(AccountMode.NONE)
    }
}
