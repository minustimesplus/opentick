package org.opentick.android.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.longOrNull
import kotlinx.serialization.json.put
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import java.io.IOException
import java.time.Instant
import java.util.Base64
import java.util.concurrent.TimeUnit

@Serializable
data class ApiUser(
    val id: Long = 0,
    val username: String = "",
    val name: String = "",
    val settings: ApiUserSettings? = null,
)

@Serializable
data class ApiUserSettings(
    @SerialName("default_project_id") val defaultProjectId: Long = 0,
    @SerialName("week_start") val weekStart: Int = 0,
    @SerialName("frontend_settings") val frontendSettings: JsonElement? = null,
) {
    /** frontend_settings.views, shared with the web app; empty if not set up yet. */
    val views: JsonObject get() = ((frontendSettings as? JsonObject)?.get("views") as? JsonObject) ?: JsonObject(emptyMap())
}

@Serializable
data class AgentConfig(
    @SerialName("claude_fire_url") val claudeFireUrl: String = "",
    @SerialName("claude_token_set") val claudeTokenSet: Boolean = false,
    @SerialName("instinct_email") val instinctEmail: String = "",
    @SerialName("mailer_enabled") val mailerEnabled: Boolean = false,
) {
    fun ready(agent: String) = if (agent == "claude") claudeFireUrl.isNotBlank() && claudeTokenSet else instinctEmail.isNotBlank() && mailerEnabled
}

@Serializable
data class AgentDispatch(
    val id: Long = 0,
    val agent: String = "",
    val status: String = "",
    @SerialName("session_url") val sessionUrl: String = "",
    val error: String = "",
)

@Serializable
data class AgentPrompt(val prompt: String = "")

@Serializable
data class ApiProject(
    val id: Long,
    val title: String = "",
    @SerialName("hex_color") val hexColor: String = "",
    val position: Double = 0.0,
    @SerialName("is_archived") val isArchived: Boolean = false,
    @SerialName("is_favorite") val isFavorite: Boolean = false,
    @SerialName("parent_project_id") val parentProjectId: Long = 0,
)

@Serializable
data class ApiProjectView(
    val id: Long,
    val title: String = "",
    @SerialName("view_kind") val viewKind: String = "",
    val position: Double = 0.0,
    @SerialName("bucket_configuration_mode") val bucketMode: String = "",
    @SerialName("default_bucket_id") val defaultBucketId: Long = 0,
    @SerialName("done_bucket_id") val doneBucketId: Long = 0,
)

/** A kanban column ("section" in TickTick). */
@Serializable
data class ApiBucket(
    val id: Long,
    val title: String = "",
    val position: Double = 0.0,
    val count: Long = 0,
    val tasks: List<ApiTask>? = null,
)

@Serializable
data class ApiLabel(
    val id: Long,
    val title: String = "",
    @SerialName("hex_color") val hexColor: String = "",
)

@Serializable
data class ApiTask(
    val id: Long,
    val title: String = "",
    val description: String = "",
    val done: Boolean = false,
    @SerialName("done_at") val doneAt: String? = null,
    @SerialName("wont_do") val wontDo: Boolean = false,
    @SerialName("due_date") val dueDate: String? = null,
    @SerialName("start_date") val startDate: String? = null,
    @SerialName("end_date") val endDate: String? = null,
    val priority: Int = 0,
    @SerialName("project_id") val projectId: Long = 0,
    val labels: List<ApiLabel>? = null,
    @SerialName("repeat_after") val repeatAfter: Long = 0,
    @SerialName("repeat_mode") val repeatMode: Int = 0,
    @SerialName("repeat_rule") val repeatRule: String = "",
    @SerialName("related_tasks") val relatedTasks: Map<String, List<ApiRelated>?>? = null,
    /** Position in the view it was fetched through; 0 elsewhere. */
    val position: Double = 0.0,
    val created: String? = null,
    val updated: String? = null,
)

@Serializable
data class ApiRelated(val id: Long)

/** A subscribed ICS calendar (web: Calendar → Subscribe). Read-only on the phone. */
@Serializable
data class ApiCalendarSubscription(
    val id: Long = 0,
    val title: String = "",
    val url: String = "",
    @SerialName("hex_color") val hexColor: String = "",
)

/** One occurrence of an event from a subscribed calendar. [end] is exclusive. */
@Serializable
data class ApiCalendarEvent(
    val id: String = "",
    val title: String = "",
    val description: String = "",
    val location: String = "",
    val url: String = "",
    val start: String = "",
    val end: String = "",
    @SerialName("all_day") val allDay: Boolean = false,
    val recurring: Boolean = false,
)

@Serializable
data class ApiCalendarEvents(
    @SerialName("calendar_name") val calendarName: String = "",
    val events: List<ApiCalendarEvent>? = null,
)

@Serializable
data class Paginated<T>(
    val items: List<T>? = null,
    val page: Int = 1,
    @SerialName("total_pages") val totalPages: Int = 1,
)

@Serializable
data class AuthInfo(
    val local: LocalAuth = LocalAuth(),
    val clerk: ClerkAuth? = null,
    @SerialName("openid_connect") val openid: OpenIdAuth? = null,
)

@Serializable
data class LocalAuth(
    val enabled: Boolean = false,
    @SerialName("registration_enabled") val registrationEnabled: Boolean = false,
)

@Serializable
data class ClerkAuth(val enabled: Boolean = false)

@Serializable
data class OpenIdAuth(val enabled: Boolean = false)

@Serializable
data class ServerInfo(val version: String = "", val auth: AuthInfo = AuthInfo())

@Serializable
data class TokenResponse(
    @SerialName("access_token") val accessToken: String = "",
    @SerialName("refresh_token") val refreshToken: String = "",
    val token: String = "",
)

/** The server answered with an error status. */
class ApiException(val status: Int, message: String) : IOException(message)

/** The session is gone (refresh failed); the user has to sign in again. */
class AuthExpiredException : IOException("Your session expired. Please sign in again.")

/** Parses server timestamps; Vikunja sends 0001-01-01 for "no date". */
fun parseServerDate(s: String?): Long? {
    if (s.isNullOrBlank() || s.startsWith("0001-")) return null
    return runCatching { Instant.parse(s).toEpochMilli() }.getOrNull()
}

fun formatServerDate(ms: Long?): JsonElement =
    if (ms == null) JsonNull else JsonPrimitive(Instant.ofEpochMilli(ms).toString())

/**
 * Talks to the OpenTick /api/v2. Works with self-hosted and hosted servers:
 * both speak the same API; they only differ in how you sign in.
 */
class Api(private val prefs: Prefs) {
    val json = Json { ignoreUnknownKeys = true; explicitNulls = false; coerceInputValues = true }

    private val http = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .followRedirects(true)
        .build()

    private val jsonType = "application/json".toMediaType()
    private val mergeType = "application/merge-patch+json".toMediaType()
    private val refreshLock = Any()

    private fun base(server: String = prefs.serverUrl) = "$server/api/v2"

    // ─── Unauthenticated ───────────────────────────────────────────────

    /** Normalizes what the user typed into a server URL. */
    fun normalizeServerUrl(input: String): String {
        var s = input.trim().trimEnd('/')
        if (!s.startsWith("http://") && !s.startsWith("https://")) s = "https://$s"
        // People paste the API URL or a page URL; keep just the origin (+ subpath).
        s = s.replace(Regex("/api/v[12].*$"), "")
        return s.trimEnd('/')
    }

    fun info(server: String): ServerInfo {
        val req = Request.Builder().url("${base(server)}/info").header("Accept", "application/json").build()
        http.newCall(req).execute().use { res ->
            if (!res.isSuccessful) throw ApiException(res.code, "This doesn't look like an OpenTick server (${res.code}).")
            return json.decodeFromString(ServerInfo.serializer(), res.body!!.string())
        }
    }

    /** Password sign in. Returns the access token and the refresh token from the cookie. */
    fun login(server: String, username: String, password: String, totp: String?): Pair<String, String?> {
        val body = buildJsonObject {
            put("username", username)
            put("password", password)
            put("long_token", true)
            if (!totp.isNullOrBlank()) put("totp_passcode", totp)
        }
        val req = Request.Builder().url("${base(server)}/login")
            .post(body.toString().toRequestBody(jsonType))
            .header("Accept", "application/json").build()
        http.newCall(req).execute().use { res ->
            val text = res.body?.string().orEmpty()
            if (!res.isSuccessful) throw ApiException(res.code, errorMessage(text, res.code))
            val token = json.decodeFromString(TokenResponse.serializer(), text).token
            val refresh = res.headers("Set-Cookie")
                .firstOrNull { it.startsWith("vikunja_refresh_token=") }
                ?.substringAfter("=")?.substringBefore(";")
            return token to refresh
        }
    }

    fun register(server: String, username: String, email: String, password: String) {
        val body = buildJsonObject {
            put("username", username); put("email", email); put("password", password)
        }
        val req = Request.Builder().url("${base(server)}/register")
            .post(body.toString().toRequestBody(jsonType)).header("Accept", "application/json").build()
        http.newCall(req).execute().use { res ->
            if (!res.isSuccessful) throw ApiException(res.code, errorMessage(res.body?.string().orEmpty(), res.code))
        }
    }

    /** OAuth authorization code + PKCE exchange (browser sign in). */
    fun exchangeCode(server: String, clientId: String, code: String, redirectUri: String, verifier: String): TokenResponse {
        val body = buildJsonObject {
            put("grant_type", "authorization_code")
            put("code", code)
            put("client_id", clientId)
            put("redirect_uri", redirectUri)
            put("code_verifier", verifier)
        }
        return tokenRequest(server, body)
    }

    private fun tokenRequest(server: String, body: JsonObject): TokenResponse {
        val req = Request.Builder().url("${base(server)}/oauth/token")
            .post(body.toString().toRequestBody(jsonType)).header("Accept", "application/json").build()
        http.newCall(req).execute().use { res ->
            val text = res.body?.string().orEmpty()
            if (!res.isSuccessful) throw ApiException(res.code, errorMessage(text, res.code))
            return json.decodeFromString(TokenResponse.serializer(), text)
        }
    }

    /**
     * Renews the access token with the stored refresh token. Session refresh
     * tokens (password sign in) and OAuth grants both go through /oauth/token.
     * Returns false when the session is no longer valid.
     */
    private fun refresh(staleToken: String?): Boolean = synchronized(refreshLock) {
        // Another thread already refreshed while we waited.
        if (prefs.accessToken != staleToken && prefs.accessToken != null) return true
        val rt = prefs.refreshToken ?: return false
        val res = try {
            tokenRequest(prefs.serverUrl, buildJsonObject {
                put("grant_type", "refresh_token")
                put("refresh_token", rt)
                put("client_id", OAUTH_CLIENT_ID)
            })
        } catch (e: ApiException) {
            if (e.status in 400..499) return false
            throw e
        }
        if (res.accessToken.isBlank()) return false
        prefs.accessToken = res.accessToken
        if (res.refreshToken.isNotBlank()) prefs.refreshToken = res.refreshToken
        true
    }

    private fun tokenExpiresSoon(token: String): Boolean = runCatching {
        val payload = String(Base64.getUrlDecoder().decode(token.split('.')[1].padEnd((token.split('.')[1].length + 3) / 4 * 4, '=')))
        val exp = json.parseToJsonElement(payload).jsonObject["exp"]?.jsonPrimitive?.longOrNull ?: return false
        exp * 1000 < System.currentTimeMillis() + 60_000
    }.getOrDefault(false)

    // ─── Authenticated ─────────────────────────────────────────────────

    private fun send(method: String, path: String, body: JsonElement?, merge: Boolean): String {
        var token = prefs.accessToken ?: throw AuthExpiredException()
        if (tokenExpiresSoon(token) && prefs.refreshToken != null) {
            if (refresh(token)) token = prefs.accessToken!!
        }
        var res = execute(method, path, body, merge, token)
        if (res.code == 401) {
            res.close()
            if (!refresh(token)) throw AuthExpiredException()
            res = execute(method, path, body, merge, prefs.accessToken!!)
            if (res.code == 401) {
                res.close()
                throw AuthExpiredException()
            }
        }
        res.use {
            val text = it.body?.string().orEmpty()
            if (!it.isSuccessful) throw ApiException(it.code, errorMessage(text, it.code))
            return text
        }
    }

    private fun execute(method: String, path: String, body: JsonElement?, merge: Boolean, token: String): Response {
        val rb = body?.toString()?.toRequestBody(if (merge) mergeType else jsonType)
            ?: if (method == "POST" || method == "PUT" || method == "PATCH") ByteArray(0).toRequestBody(null) else null
        val req = Request.Builder().url(base() + path)
            .method(method, rb)
            .header("Authorization", "Bearer $token")
            .header("Accept", "application/json")
            .build()
        return http.newCall(req).execute()
    }

    private fun errorMessage(text: String, code: Int): String = runCatching {
        val o = json.parseToJsonElement(text).jsonObject
        (o["detail"] ?: o["message"] ?: o["title"])?.jsonPrimitive?.content
    }.getOrNull() ?: "Request failed ($code)"

    private inline fun <reified T> getAll(path: String, query: String = "", maxPages: Int = 50): List<T> {
        val out = mutableListOf<T>()
        var page = 1
        while (true) {
            val sep = if (query.isEmpty()) "" else "&"
            val text = send("GET", "$path?$query${sep}page=$page&per_page=50", null, false)
            val res = json.decodeFromString<Paginated<T>>(text)
            val items = res.items.orEmpty()
            out += items
            if (page >= res.totalPages || page >= maxPages || items.isEmpty()) break
            page++
        }
        return out
    }

    fun user(): ApiUser = json.decodeFromString(send("GET", "/user", null, false))

    fun projects(): List<ApiProject> = getAll("/projects", "is_archived=false")

    fun labels(): List<ApiLabel> = getAll("/labels")

    fun tasks(filter: String, maxPages: Int = 50): List<ApiTask> =
        getAll("/tasks", "filter=" + java.net.URLEncoder.encode(filter, "UTF-8") + "&format=markdown", maxPages)

    fun createProject(title: String, hexColor: String): ApiProject = json.decodeFromString(
        send("POST", "/projects", buildJsonObject { put("title", title); put("hex_color", hexColor) }, false),
    )

    fun updateProject(id: Long, title: String, hexColor: String): ApiProject = json.decodeFromString(
        send("PATCH", "/projects/$id", buildJsonObject { put("title", title); put("hex_color", hexColor) }, true),
    )

    fun deleteProject(id: Long) {
        send("DELETE", "/projects/$id", null, false)
    }

    fun createLabel(title: String, hexColor: String): ApiLabel = json.decodeFromString(
        send("POST", "/labels", buildJsonObject { put("title", title); put("hex_color", hexColor) }, false),
    )

    fun deleteLabel(id: Long) {
        send("DELETE", "/labels/$id", null, false)
    }

    fun createTask(projectId: Long, fields: JsonObject): ApiTask = json.decodeFromString(
        send("POST", "/projects/$projectId/tasks?format=markdown", fields, false),
    )

    fun updateTask(id: Long, fields: JsonObject): ApiTask = json.decodeFromString(
        send("PATCH", "/tasks/$id?format=markdown", fields, true),
    )

    fun deleteTask(id: Long) {
        send("DELETE", "/tasks/$id", null, false)
    }

    fun addLabel(taskId: Long, labelId: Long) {
        send("POST", "/tasks/$taskId/labels", buildJsonObject { put("label_id", labelId) }, false)
    }

    fun removeLabel(taskId: Long, labelId: Long) {
        send("DELETE", "/tasks/$taskId/labels/$labelId", null, false)
    }

    /** Deleted tasks the server keeps for a while (online only). */
    fun trash(): List<ApiTask> = getAll("/tasks/trash", "format=markdown", maxPages = 10)

    fun restoreFromTrash(id: Long) {
        send("POST", "/tasks/trash/$id/restore", null, false)
    }

    fun deleteForever(id: Long) {
        send("DELETE", "/tasks/trash/$id", null, false)
    }

    fun emptyTrash() {
        send("POST", "/tasks/trash/empty", null, false)
    }

    /** Whether the trash has anything in it, for a smart list set to "show if not empty". */
    fun trashNonEmpty(): Boolean =
        json.decodeFromString<Paginated<ApiTask>>(send("GET", "/tasks/trash?page=1&per_page=1", null, false)).items.orEmpty().isNotEmpty()

    // ─── Project views: manual order and kanban ────────────────────────

    fun projectViews(projectId: Long): List<ApiProjectView> = getAll("/projects/$projectId/views")

    fun projectView(projectId: Long, viewId: Long): ApiProjectView =
        json.decodeFromString(send("GET", "/projects/$projectId/views/$viewId", null, false))

    /** Tasks of a view with their position in it. */
    fun viewTasks(projectId: Long, viewId: Long, filter: String): List<ApiTask> =
        getAll("/projects/$projectId/views/$viewId/tasks", "filter=" + java.net.URLEncoder.encode(filter, "UTF-8"), maxPages = 10)

    /** A kanban view's buckets with their tasks. Pages apply to the tasks inside each bucket. */
    fun bucketsWithTasks(projectId: Long, viewId: Long): List<ApiBucket> {
        val byId = LinkedHashMap<Long, ApiBucket>()
        for (page in 1..20) {
            val text = send("GET", "/projects/$projectId/views/$viewId/buckets/tasks?page=$page", null, false)
            val items = json.decodeFromString<Paginated<ApiBucket>>(text).items.orEmpty()
            for (b in items) {
                val have = byId[b.id]
                byId[b.id] = if (have == null) b.copy(tasks = b.tasks.orEmpty()) else have.copy(tasks = have.tasks.orEmpty() + b.tasks.orEmpty())
            }
            if (items.none { b -> b.tasks.orEmpty().isNotEmpty() && byId[b.id]!!.tasks!!.size < b.count }) break
        }
        return byId.values.sortedBy { it.position }
    }

    fun moveToBucket(projectId: Long, viewId: Long, bucketId: Long, taskId: Long) {
        send("PUT", "/projects/$projectId/views/$viewId/buckets/$bucketId/tasks", buildJsonObject { put("task_id", taskId) }, false)
    }

    fun setPosition(taskId: Long, viewId: Long, position: Double) {
        send("PUT", "/tasks/$taskId/position", buildJsonObject { put("position", position); put("project_view_id", viewId) }, false)
    }

    fun createBucket(projectId: Long, viewId: Long, title: String, position: Double): ApiBucket = json.decodeFromString(
        send("POST", "/projects/$projectId/views/$viewId/buckets", buildJsonObject { put("title", title); put("position", position) }, false),
    )

    fun updateBucket(projectId: Long, viewId: Long, bucketId: Long, title: String, position: Double) {
        send("PUT", "/projects/$projectId/views/$viewId/buckets/$bucketId", buildJsonObject { put("title", title); put("position", position) }, false)
    }

    fun deleteBucket(projectId: Long, viewId: Long, bucketId: Long) {
        send("DELETE", "/projects/$projectId/views/$viewId/buckets/$bucketId", null, false)
    }

    /** Merges into frontend_settings.views (JSON merge patch). */
    fun patchViews(patch: JsonObject) {
        send("PATCH", "/user/settings/general", buildJsonObject { put("frontend_settings", buildJsonObject { put("views", patch) }) }, true)
    }

    // ─── Subscribed calendars ──────────────────────────────────────────

    fun calendarSubscriptions(): List<ApiCalendarSubscription> = getAll("/calendar-subscriptions")

    /** Event occurrences overlapping [from, to); all-day events are placed in [timezone]. */
    fun calendarEvents(id: Long, from: Instant, to: Instant, timezone: String): ApiCalendarEvents {
        fun enc(v: String) = java.net.URLEncoder.encode(v, "UTF-8")
        return json.decodeFromString(send("GET", "/calendar-subscriptions/$id/events?from=${enc(from.toString())}&to=${enc(to.toString())}&timezone=${enc(timezone)}", null, false))
    }

    // ─── AI agents (Settings > AI features on the web) ─────────────────

    fun agentConfig(): AgentConfig = json.decodeFromString(send("GET", "/agents/config", null, false))

    fun agentPrompt(taskId: Long, extra: String): String {
        val q = if (extra.isBlank()) "" else "?extra=" + java.net.URLEncoder.encode(extra, "UTF-8")
        return json.decodeFromString<AgentPrompt>(send("GET", "/tasks/$taskId/agent-prompt$q", null, false)).prompt
    }

    fun sendToAgent(taskId: Long, agent: String, extra: String): AgentDispatch = json.decodeFromString(
        send("POST", "/tasks/$taskId/agent-dispatches", buildJsonObject { put("agent", agent); put("extra_prompt", extra) }, false),
    )

    fun logout() {
        runCatching { send("POST", "/logout", null, false) }
    }

    companion object {
        /** First-party OAuth client id of the Android app. */
        const val OAUTH_CLIENT_ID = "opentick-android"
        const val OAUTH_REDIRECT_URI = "vikunja-opentick://oauth/callback"
    }
}

/** The fields the app edits, as sent on create and update. */
fun TaskEntity.toApiFields(projectServerId: Long): JsonObject = buildJsonObject {
    put("title", title)
    put("description", description)
    put("done", done)
    put("wont_do", done && wontDo)
    put("due_date", formatServerDate(dueDate))
    // The timeline edits date ranges; the calendar shows them as blocks.
    put("start_date", formatServerDate(startDate))
    put("end_date", formatServerDate(endDate))
    put("priority", priority)
    put("project_id", projectServerId)
}
