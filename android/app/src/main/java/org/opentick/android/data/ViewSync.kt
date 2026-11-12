package org.opentick.android.data

import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.longOrNull

/**
 * View settings shared with the web and desktop apps (the Eisenhower matrix
 * setup, …), stored on the server in frontend_settings.views. Same rules as
 * web/src/lib/viewSync.ts: values this device changed since the last sync are
 * sent as a JSON merge patch (RFC 7396), everything else follows the server.
 *
 * Unlike the web app, the phone only knows the keys it got from the server or
 * changed itself, so a fresh install never overwrites what the desktop set up.
 */
object ViewSync {
    const val REV = "rev"

    data class Result(
        /** What this device should use now. */
        val values: JsonObject,
        /** Merge patch for frontend_settings.views, or null if the server is up to date. */
        val patch: JsonObject?,
        /** The server state once the patch went through; the next snapshot. */
        val server: JsonObject,
    )

    fun reconcile(local: JsonObject, snapshot: JsonObject?, remote: JsonObject, now: Long = System.currentTimeMillis()): Result {
        // A response fetched before our last push went through: ignore what it says.
        val base = if (snapshot != null && rev(remote) < rev(snapshot)) snapshot else remote
        val patch = LinkedHashMap<String, JsonElement>()
        for ((key, value) in local) {
            if (key == REV) continue
            if (key !in base) {
                patch[key] = value
            } else if (snapshot != null && key in snapshot && snapshot[key] != value) {
                val d = diff(snapshot[key]!!, value)
                if (d !is JsonObject || d.isNotEmpty()) patch[key] = d
            }
        }
        if (patch.isEmpty()) return Result(withLocalOnly(base, local), null, base)
        patch[REV] = JsonPrimitive(maxOf(now, rev(base) + 1))
        val p = JsonObject(patch)
        val server = apply(base, p) as JsonObject
        return Result(withLocalOnly(server, local), p, server)
    }

    private fun withLocalOnly(server: JsonObject, local: JsonObject): JsonObject =
        JsonObject(local.filterKeys { it != REV && it !in server } + server.filterKeys { it != REV })

    private fun rev(o: JsonObject) = (o[REV] as? JsonPrimitive)?.longOrNull ?: 0L

    /** RFC 7396: objects merge key by key, null removes a key, anything else replaces. */
    fun apply(target: JsonElement?, patch: JsonElement): JsonElement {
        if (patch !is JsonObject) return patch
        val out = LinkedHashMap((target as? JsonObject).orEmpty())
        for ((k, v) in patch) {
            if (v is JsonNull) out.remove(k) else out[k] = apply(out[k], v)
        }
        return JsonObject(out)
    }

    /** The merge patch that turns [from] into [to]. */
    fun diff(from: JsonElement, to: JsonElement): JsonElement {
        if (from !is JsonObject || to !is JsonObject) return to
        val out = LinkedHashMap<String, JsonElement>()
        for (k in from.keys) if (k !in to) out[k] = JsonNull
        for ((k, v) in to) if (from[k] != v) out[k] = from[k]?.let { diff(it, v) } ?: v
        return JsonObject(out)
    }
}

// ─── Typed access to the matrix settings ───────────────────────────────

enum class MatrixGroup(val key: String, val label: String) {
    PRIORITY("priority", "Priority"), LIST("list", "List"), DATE("date", "Date"), NONE("none", "None");
    companion object { fun of(s: String?) = entries.firstOrNull { it.key == s } ?: PRIORITY }
}

enum class MatrixSort(val key: String, val label: String) {
    DATE("date", "Date"), PRIORITY("priority", "Priority"), TITLE("title", "Title"), CREATED("created", "Created time"), CUSTOM("custom", "Custom");
    companion object { fun of(s: String?) = entries.firstOrNull { it.key == s } ?: DATE }
}

data class QuadrantView(val group: MatrixGroup = MatrixGroup.PRIORITY, val sort: MatrixSort = MatrixSort.DATE)

// ─── Per-list settings, keyed like the web app's /tasks/:list route ────

/** Sort of a task list (web prefs.listSort). */
enum class ListSort(val key: String, val label: String) {
    DATE("date", "Date"), PRIORITY("priority", "Priority"), TITLE("title", "Title"), CREATED("created", "Created time"), CUSTOM("custom", "Custom");
    companion object { fun of(s: String?) = entries.firstOrNull { it.key == s } ?: DATE }
}

/** Layout of a list (web prefs.listView): plain list, kanban columns or timeline. */
enum class ListLayout(val key: String, val label: String) {
    LIST("list", "List"), KANBAN("kanban", "Kanban"), TIMELINE("timeline", "Timeline");
    companion object { fun of(s: String?) = entries.firstOrNull { it.key == s } ?: LIST }
}

/** Zoom of the timeline view; [dayWidth] is the width of one day in dp, as on the web. */
enum class TimelineScale(val key: String, val label: String, val dayWidth: Int) {
    DAY("day", "Day", 56), WEEK("week", "Week", 22), MONTH("month", "Month", 7);
    companion object { fun of(s: String?) = entries.firstOrNull { it.key == s } ?: WEEK }
}

/** Settings > Smart List on the web: show, hide, or show if not empty. */
enum class Visibility(val key: String, val label: String) {
    SHOW("show", "Show"), HIDE("hide", "Hide"), AUTO("auto", "Show if not empty");
    companion object { fun of(s: String?) = entries.firstOrNull { it.key == s } }
}

/** The web app's smart lists (web lib/smartLists.ts), in its order, with TickTick's defaults. */
object SmartLists {
    val DEFAULTS: Map<String, Visibility> = linkedMapOf(
        "all" to Visibility.HIDE,
        "today" to Visibility.SHOW,
        "tomorrow" to Visibility.HIDE,
        "next7" to Visibility.SHOW,
        "assigned" to Visibility.AUTO,
        "inbox" to Visibility.SHOW,
        "summary" to Visibility.HIDE,
        "tags" to Visibility.SHOW,
        "filters" to Visibility.SHOW,
        "completed" to Visibility.SHOW,
        "wontDo" to Visibility.AUTO,
        "trash" to Visibility.SHOW,
    )

    /** Summary is a report, never empty, so it has no "auto". */
    fun modes(key: String): List<Visibility> = if (key == "summary") listOf(Visibility.SHOW, Visibility.HIDE) else Visibility.entries

    /** Names the phone used before smart lists were synced (Prefs.smartLists), by web key. */
    val LEGACY = mapOf(
        "today" to "TODAY", "tomorrow" to "TOMORROW", "next7" to "NEXT7", "inbox" to "INBOX",
        "all" to "ALL", "tags" to "TAGS", "completed" to "COMPLETED", "trash" to "TRASH",
    )
}

/**
 * The synced view settings as the app uses them. Project and task ids are the
 * server's; something not synced yet is stored as its negated local id.
 */
data class Views(val json: JsonObject = JsonObject(emptyMap())) {
    val matrixLists: List<Long> get() = longs(json["matrixLists"])
    val matrixShowDone: Boolean get() = (json["matrixShowDone"] as? JsonPrimitive)?.content == "true"

    fun quadrant(q: Int): QuadrantView {
        val o = (json["matrixView"] as? JsonObject)?.get(q.toString()) as? JsonObject ?: return QuadrantView()
        return QuadrantView(MatrixGroup.of(str(o["group"])), MatrixSort.of(str(o["sort"])))
    }

    fun matrixOrder(q: Int): List<Long> = longs((json["matrixOrder"] as? JsonObject)?.get(q.toString()))

    fun isDefault(q: Int) = quadrant(q) == QuadrantView() && (json["matrixOrder"] as? JsonObject)?.get(q.toString()) == null

    // Per-list settings. [key] is the web route's list segment: "today", "inbox", "p12", "l3", …

    fun listSort(key: String): ListSort = ListSort.of(str(obj("listSort")[key]))
    /** Missing means shown, as on the web. */
    /** Completed tasks are hidden unless the list was set to show them, like the web app. */
    fun listShowCompleted(key: String): Boolean = str(obj("listShowCompleted")[key]) == "true"
    fun listLayout(key: String): ListLayout = ListLayout.of(str(obj("listView")[key]))
    val timelineScale: TimelineScale get() = TimelineScale.of(str(json["timelineScale"]))

    fun withListSort(key: String, v: ListSort) = withEntry("listSort", key, JsonPrimitive(v.key))
    fun withListShowCompleted(key: String, on: Boolean) = withEntry("listShowCompleted", key, JsonPrimitive(on))
    fun withListLayout(key: String, v: ListLayout) = withEntry("listView", key, JsonPrimitive(v.key))
    fun withTimelineScale(v: TimelineScale) = with("timelineScale", JsonPrimitive(v.key))

    /**
     * Visibility of each smart list. Before anything is synced, the phone's own
     * old switches apply ([legacy], Prefs.smartLists) and the web's defaults fill the rest.
     */
    fun smartLists(legacy: Set<String>): Map<String, Visibility> {
        val synced = json["smartLists"] as? JsonObject
        return SmartLists.DEFAULTS.mapValues { (key, def) ->
            if (synced != null) Visibility.of(str(synced[key])) ?: def
            else SmartLists.LEGACY[key]?.let { if (it in legacy) Visibility.SHOW else Visibility.HIDE } ?: def
        }
    }

    /** Sets one smart list. Writes every key, like the web app, so no device ends up with a partial set. */
    fun withSmartList(key: String, v: Visibility, legacy: Set<String>): Views {
        val current = (json["smartLists"] as? JsonObject).orEmpty()
        val all = smartLists(legacy).mapValues { JsonPrimitive(it.value.key) as JsonElement }
        // Keys only a newer web app knows stay as they are.
        return with("smartLists", JsonObject(current + all + (key to JsonPrimitive(v.key))))
    }

    private fun obj(key: String): Map<String, JsonElement> = (json[key] as? JsonObject).orEmpty()

    private fun withEntry(key: String, entry: String, value: JsonElement) = with(key, JsonObject(obj(key) + (entry to value)))

    /** Settings > Features on the web; missing means on. */
    fun feature(name: String): Boolean = ((json["features"] as? JsonObject)?.get(name) as? JsonPrimitive)?.content != "false"

    fun with(key: String, value: JsonElement?): Views =
        Views(JsonObject(if (value == null) json - key else json + (key to value)))

    fun withMatrixLists(ids: List<Long>) = with("matrixLists", JsonArray(ids.map { JsonPrimitive(it) }))
    fun withMatrixShowDone(on: Boolean) = with("matrixShowDone", JsonPrimitive(on))

    fun withQuadrant(q: Int, v: QuadrantView): Views {
        val views = (json["matrixView"] as? JsonObject).orEmpty()
        val entry = JsonObject(mapOf("group" to JsonPrimitive(v.group.key), "sort" to JsonPrimitive(v.sort.key)))
        return with("matrixView", JsonObject(views + (q.toString() to entry)))
    }

    fun withMatrixOrder(q: Int, ids: List<Long>): Views {
        val orders = (json["matrixOrder"] as? JsonObject).orEmpty()
        return with("matrixOrder", JsonObject(orders + (q.toString() to JsonArray(ids.map { JsonPrimitive(it) }))))
    }

    fun resetQuadrant(q: Int): Views {
        val k = q.toString()
        var v = this
        (json["matrixView"] as? JsonObject)?.let { v = v.with("matrixView", JsonObject(it - k)) }
        (json["matrixOrder"] as? JsonObject)?.let { v = v.with("matrixOrder", JsonObject(it - k)) }
        return v
    }

    /** Swaps negated local ids for server ids once known; drops ids of things that are gone. */
    fun resolveIds(project: (Long) -> Long?, task: (Long) -> Long?, label: (Long) -> Long? = { null }): Views {
        fun fix(ids: List<Long>, lookup: (Long) -> Long?) = ids.mapNotNull { if (it < 0) lookup(-it) else it }
        var v = this
        // Per-list settings of lists made offline are keyed "p-<local id>" until the list is on the server.
        fun fixKey(k: String): String? {
            val lookup = when (k.firstOrNull()) { 'p' -> project; 'l' -> label; else -> return k }
            val id = k.drop(1).toLongOrNull() ?: return k
            return if (id < 0) lookup(-id)?.let { "${k[0]}$it" } else k
        }
        for (key in LIST_KEYS) {
            val o = json[key] as? JsonObject ?: continue
            val fixed = LinkedHashMap<String, JsonElement>()
            for ((k, value) in o) fixKey(k)?.let { fixed[it] = value }
            if (fixed != o) v = v.with(key, JsonObject(fixed))
        }
        if ("matrixLists" in json) {
            val fixed = fix(matrixLists, project)
            if (fixed != matrixLists) v = v.withMatrixLists(fixed)
        }
        (json["matrixOrder"] as? JsonObject)?.let { orders ->
            val fixed = orders.mapValues { (_, ids) -> JsonArray(fix(longs(ids), task).map { JsonPrimitive(it) }) }
            if (fixed != orders) v = v.with("matrixOrder", JsonObject(fixed))
        }
        return v
    }

    companion object {
        /** Settings stored per list key. */
        val LIST_KEYS = listOf("listSort", "listShowCompleted", "listView")
    }

    private fun str(e: JsonElement?) = (e as? JsonPrimitive)?.content
    private fun longs(e: JsonElement?) = (e as? JsonArray).orEmpty().mapNotNull { (it as? JsonPrimitive)?.longOrNull }
}

/** How views refer to a list or task: the server id, or the negated local id before the first sync. */
fun ProjectEntity.viewKey(): Long = serverId ?: -id
fun TaskEntity.viewKey(): Long = serverId ?: -id
fun LabelEntity.viewKey(): Long = serverId ?: -id
