package org.opentick.android.ui.habits

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import org.opentick.android.data.AccountMode
import org.opentick.android.data.Api
import org.opentick.android.data.ApiException
import org.opentick.android.data.ApiHabit
import org.opentick.android.data.ApiHabitCheckin
import org.opentick.android.data.ApiHabitStats
import org.opentick.android.data.ApiWeight
import org.opentick.android.data.AuthExpiredException
import org.opentick.android.data.Prefs
import java.io.IOException
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.YearMonth
import kotlin.math.abs
import kotlin.math.roundToInt

/** Color of habits without one, same as the web app. */
const val DEFAULT_HABIT_COLOR = "40C057"
const val ALL_WEEKDAYS = 0b1111111
private const val POSITION_STEP = 65536.0

/** How far back the weight chart and changes look, like the web. */
const val WEIGHT_HISTORY_DAYS = 365L

/** Bit of a day in a habit's weekdays mask: bit 0 is Sunday. */
fun weekdayBit(day: DayOfWeek): Int = 1 shl (day.value % 7)

fun ApiHabit.isDue(day: LocalDate): Boolean = weekdays and weekdayBit(day.dayOfWeek) != 0

/** How much was checked in on [date]; only today and the last 7 days are known without loading stats. */
fun ApiHabit.valueOn(date: LocalDate, today: LocalDate): Int =
    if (date == today) todayValue else recentCheckins.orEmpty().firstOrNull { it.date == date.toString() }?.value ?: 0

/**
 * One tap: a yes/no habit toggles, a counter adds one until its goal is
 * reached, and a reached goal resets. Same as the web.
 */
fun ApiHabit.tapValue(current: Int): Int = when {
    goal <= 1 -> if (current > 0) 0 else 1
    current >= goal -> 0
    else -> current + 1
}

/** The habit as it looks after setting [date]'s value, before the server answers. */
fun ApiHabit.withValue(date: LocalDate, value: Int, today: LocalDate): ApiHabit {
    val key = date.toString()
    val recent = recentCheckins.orEmpty().filter { it.date != key } + if (value > 0) listOf(ApiHabitCheckin(key, value)) else emptyList()
    return copy(todayValue = if (date == today) value else todayValue, recentCheckins = recent)
}

/** Habits in list order: by position, then id. */
fun List<ApiHabit>.sortedHabits(): List<ApiHabit> = sortedWith(compareBy<ApiHabit> { it.position }.thenBy { it.id })

/**
 * The position that puts [h] at [index] among the other active habits (0 is
 * the top), between its new neighbours; null if it doesn't move.
 */
fun movedPosition(active: List<ApiHabit>, h: ApiHabit, index: Int): Double? {
    val others = active.filter { it.id != h.id }
    val i = index.coerceIn(0, others.size)
    val prev = others.getOrNull(i - 1)
    val next = others.getOrNull(i)
    val pos = when {
        prev != null && next != null -> (prev.position + next.position) / 2
        prev != null -> prev.position + POSITION_STEP
        next != null -> next.position / 2
        else -> return null
    }
    return pos.takeIf { it != h.position }
}

/** Latest weight minus the first within the last [days] days; null without two entries. */
fun weightChange(entries: List<ApiWeight>, days: Long, today: LocalDate): Double? {
    val from = today.minusDays(days).toString()
    val recent = entries.filter { it.date >= from }
    if (recent.size < 2) return null
    return ((recent.last().weight - recent.first().weight) * 10).roundToInt() / 10.0
}

/** "72.4", or "+0.3" / "−0.3" with [signed]; "–" for nothing. */
fun fmtWeight(n: Double?, signed: Boolean = false): String {
    if (n == null) return "–"
    val s = String.format(java.util.Locale.ROOT, "%.1f", abs(n))
    return when {
        n < 0 && s != "0.0" -> "−$s"
        signed && n > 0 && s != "0.0" -> "+$s"
        else -> s
    }
}

/** Parses what was typed into the weight field: "72,4" works too. Null unless 1–1000 kg. */
fun parseWeight(text: String): Double? =
    text.trim().replace(',', '.').toDoubleOrNull()?.takeIf { it in 1.0..1000.0 }

/** What a habit editor saves. */
data class HabitInput(
    val title: String,
    val icon: String,
    val hexColor: String,
    val frequency: String,
    val weekdays: Int,
    val goal: Int,
    val unit: String,
    val tracksWeight: Boolean,
) {
    fun toJson(): JsonObject = buildJsonObject {
        put("title", title.trim())
        put("icon", icon.trim())
        put("hex_color", hexColor.removePrefix("#"))
        put("frequency", frequency)
        put("weekdays", if (frequency == "daily") ALL_WEEKDAYS else weekdays)
        put("goal", goal.coerceAtLeast(1))
        put("unit", unit.trim())
        put("tracks_weight", tracksWeight)
    }
}

/**
 * Habits and the weight log, loaded from the server. Online only: they're
 * kept in memory while the app runs and reloaded when the tab opens.
 */
class HabitsStore(private val api: Api, private val prefs: Prefs, private val scope: CoroutineScope) {
    private val _habits = MutableStateFlow<List<ApiHabit>>(emptyList())
    val habits: StateFlow<List<ApiHabit>> = _habits

    private val _weights = MutableStateFlow<List<ApiWeight>>(emptyList())
    val weights: StateFlow<List<ApiWeight>> = _weights

    /** Null once loaded, else why it couldn't be. */
    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error

    private val _loaded = MutableStateFlow(false)
    val loaded: StateFlow<Boolean> = _loaded

    private val _loading = MutableStateFlow(false)
    val loading: StateFlow<Boolean> = _loading

    /** Shown in a snackbar: what went wrong with the last change. */
    val message = MutableStateFlow<String?>(null)

    /** The habit whose detail page is open in the Habits tab. */
    val openId = MutableStateFlow<Long?>(null)

    private var server = ""

    fun today(): LocalDate = LocalDate.now()

    /** The first day of the week from Settings → Date & Time. */
    fun weekStart(): DayOfWeek = org.opentick.android.ui.calendar.weekStartDay(prefs.weekStart.value)

    fun load() {
        if (prefs.mode.value != AccountMode.SERVER) {
            _habits.value = emptyList(); _weights.value = emptyList(); _loaded.value = false
            return
        }
        if (server != prefs.serverUrl) {
            server = prefs.serverUrl
            _habits.value = emptyList(); _weights.value = emptyList(); _loaded.value = false; openId.value = null
        }
        if (_loading.value) return
        _loading.value = true
        scope.launch {
            try {
                val today = today()
                val (h, w) = withContext(Dispatchers.IO) {
                    api.habits(today.toString()) to api.weights(today.minusDays(WEIGHT_HISTORY_DAYS).toString(), today.toString())
                }
                _habits.value = h.sortedHabits()
                _weights.value = w.sortedBy { it.date }
                _loaded.value = true
                _error.value = null
            } catch (e: Exception) {
                _error.value = errorText(e)
            } finally {
                _loading.value = false
            }
        }
    }

    private fun replace(h: ApiHabit) {
        val list = _habits.value
        _habits.value = (if (list.any { it.id == h.id }) list.map { if (it.id == h.id) h else it } else list + h).sortedHabits()
    }

    fun habit(id: Long): ApiHabit? = _habits.value.firstOrNull { it.id == id }

    /** Runs a server call; on failure shows why and returns null. */
    private suspend fun <T> call(block: () -> T): T? = try {
        withContext(Dispatchers.IO) { block() }
    } catch (e: Exception) {
        message.value = errorText(e)
        null
    }

    /** Sets [date]'s value, showing it at once and rolling back if the server refuses. */
    fun checkin(h: ApiHabit, date: LocalDate, value: Int, onDone: () -> Unit = {}) = scope.launch {
        val today = today()
        val before = habit(h.id) ?: h
        replace(before.withValue(date, value, today))
        val saved = call { api.setCheckin(h.id, date.toString(), value.coerceAtLeast(0), today.toString()) }
        replace(saved ?: before)
        onDone()
    }

    fun tap(h: ApiHabit, date: LocalDate = today()) = checkin(h, date, h.tapValue(h.valueOn(date, today())))

    fun create(input: HabitInput, onSaved: (ApiHabit) -> Unit = {}) = scope.launch {
        call { api.createHabit(input.toJson()) }?.let { replace(it); onSaved(it) }
    }

    fun update(id: Long, patch: JsonObject, onSaved: (ApiHabit) -> Unit = {}) = scope.launch {
        call { api.updateHabit(id, patch) }?.let { replace(it); onSaved(it) }
    }

    fun setArchived(h: ApiHabit, archived: Boolean) = update(h.id, buildJsonObject { put("archived", archived) })

    fun delete(id: Long) = scope.launch {
        if (call { api.deleteHabit(id) } != null) {
            _habits.value = _habits.value.filter { it.id != id }
            if (openId.value == id) openId.value = null
        }
    }

    /** Moves an active habit to [index] among the other active ones. */
    fun move(h: ApiHabit, index: Int) {
        val active = _habits.value.filter { !it.archived }
        val pos = movedPosition(active, h, index) ?: return
        replace(h.copy(position = pos))
        scope.launch {
            val saved = call { api.updateHabit(h.id, buildJsonObject { put("position", pos) }) }
            replace(saved ?: h)
        }
    }

    suspend fun stats(id: Long, month: YearMonth): ApiHabitStats? = call { api.habitStats(id, today().toString(), month.toString()) }

    /** Logs [date]'s weight; the server checks off weigh-in habits, so they're reloaded. */
    fun logWeight(kg: Double, date: LocalDate = today(), onDone: (Boolean) -> Unit = {}) = scope.launch {
        val today = today()
        val saved = call { api.setWeight(date.toString(), kg, today.toString()) }
        if (saved != null) {
            _weights.value = (_weights.value.filter { it.date != saved.date } + saved).sortedBy { it.date }
            call { api.habits(today.toString()) }?.let { _habits.value = it.sortedHabits() }
        }
        onDone(saved != null)
    }

    fun deleteWeight(date: String) = scope.launch {
        if (call { api.deleteWeight(date) } != null) _weights.value = _weights.value.filter { it.date != date }
    }

    fun clear() {
        _habits.value = emptyList(); _weights.value = emptyList(); _loaded.value = false; _error.value = null; openId.value = null
    }

    companion object {
        fun errorText(e: Exception): String = when (e) {
            is AuthExpiredException -> e.message ?: "Sign in again."
            is ApiException -> e.message ?: "Something went wrong"
            is IOException -> "Connect to the internet to use this."
            else -> e.message ?: "Something went wrong"
        }
    }
}
