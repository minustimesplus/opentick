package org.opentick.android.data

import android.content.Context
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

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

    /** Ids of reminder alarms currently scheduled, so they can be cancelled. */
    var scheduledReminders: Set<String>
        get() = sp.getStringSet("scheduled_reminders", emptySet()) ?: emptySet()
        set(v) = sp.edit().putStringSet("scheduled_reminders", v).apply()

    fun clearAccount() {
        sp.edit()
            .remove("server_url").remove("access_token").remove("refresh_token")
            .remove("username").remove("display_name").remove("last_sync")
            .apply()
        setMode(AccountMode.NONE)
    }
}
