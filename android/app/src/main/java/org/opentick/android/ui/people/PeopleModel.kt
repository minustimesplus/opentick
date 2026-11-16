package org.opentick.android.ui.people

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import org.opentick.android.data.AccountMode
import org.opentick.android.data.Api
import org.opentick.android.data.ApiContact
import org.opentick.android.data.ApiContactGroup
import org.opentick.android.data.ApiContactRow
import org.opentick.android.data.ApiContactTag
import org.opentick.android.data.ApiException
import org.opentick.android.data.Prefs
import org.opentick.android.ui.habits.HabitsStore
import java.text.Normalizer
import java.util.concurrent.ConcurrentHashMap

private val FOLD = mapOf('ß' to "ss", 'æ' to "ae", 'œ' to "oe", 'ø' to "o", 'đ' to "d", 'ł' to "l")
private val MARKS = Regex("\\p{M}+")
private val NOT_ALNUM = Regex("[^\\p{L}\\p{N}]")

/**
 * Lowercases text, strips accents and keeps only letters and digits, so that
 * "Sciences Po", "SciencesPo" and "sciences-po" compare equal. Mirrors the
 * web and the server.
 */
fun fold(text: String): String {
    val lower = buildString { for (ch in text.lowercase()) append(FOLD[ch] ?: ch) }
    return Normalizer.normalize(lower, Normalizer.Form.NFD).replace(MARKS, "").replace(NOT_ALNUM, "")
}

fun initials(name: String): String {
    val parts = name.trim().split(Regex("\\s+")).filter { it.isNotEmpty() }
    if (parts.isEmpty()) return "?"
    fun first(s: String) = String(Character.toChars(s.codePointAt(0)))
    if (parts.size == 1) {
        val cps = parts[0].codePoints().limit(2).toArray()
        return String(cps, 0, cps.size).uppercase()
    }
    return (first(parts.first()) + first(parts.last())).uppercase()
}

private val AVATAR_COLORS = listOf("E03131", "F76707", "F59F00", "40C057", "12B886", "15AABF", "4772FA", "7048E8", "BE4BDB", "E64980")

/** A stable color for a name, for avatars without a photo and chips without a color. Same hash as the web. */
fun nameColor(name: String, hex: String? = null): String {
    val given = hex?.removePrefix("#")
    if (!given.isNullOrBlank()) return given
    var hash = 0L
    name.codePoints().forEach { hash = (hash * 31 + it) and 0xFFFFFFFFL }
    return AVATAR_COLORS[(hash % AVATAR_COLORS.size).toInt()]
}

enum class ContactSort(val label: String) { NAME("Name"), COMPANY("Company"), LOCATION("Location"), RECENT("Recently updated") }

/** What the list is narrowed to besides the search. */
sealed interface PeopleFilter {
    data object All : PeopleFilter
    data class Group(val id: Long) : PeopleFilter
    data class Tag(val id: Long) : PeopleFilter
}

fun haystack(c: ApiContactRow, tagNames: Map<Long, String>, groupNames: Map<Long, String>): String = fold(
    (listOf(c.name, c.headline, c.company, c.title, c.location, c.email, c.phone, c.searchText) +
        c.schools.orEmpty() + c.tagIds.orEmpty().map { tagNames[it].orEmpty() } + c.groupIds.orEmpty().map { groupNames[it].orEmpty() })
        .joinToString(" "),
)

/** Every word of the query has to appear somewhere on the contact. */
fun filterContacts(
    contacts: List<ApiContactRow>,
    query: String,
    filter: PeopleFilter,
    tagNames: Map<Long, String>,
    groupNames: Map<Long, String>,
): List<ApiContactRow> {
    val words = query.split(Regex("\\s+")).map(::fold).filter { it.isNotEmpty() }
    return contacts.filter { c ->
        when (filter) {
            is PeopleFilter.Group -> filter.id in c.groupIds.orEmpty()
            is PeopleFilter.Tag -> filter.id in c.tagIds.orEmpty()
            PeopleFilter.All -> true
        } && (words.isEmpty() || haystack(c, tagNames, groupNames).let { h -> words.all { it in h } })
    }
}

private val collator = java.text.Collator.getInstance().apply { strength = java.text.Collator.PRIMARY }

/** Contacts without the sorted field go last. */
private fun compareText(a: String, b: String): Int = when {
    a.isEmpty() || b.isEmpty() -> if (a.isNotEmpty()) -1 else if (b.isNotEmpty()) 1 else 0
    else -> collator.compare(a, b)
}

fun sortContacts(contacts: List<ApiContactRow>, sort: ContactSort): List<ApiContactRow> {
    val byName = Comparator<ApiContactRow> { a, b -> compareText(a.name, b.name) }
    return when (sort) {
        ContactSort.COMPANY -> contacts.sortedWith(Comparator<ApiContactRow> { a, b -> compareText(a.company, b.company) }.then(byName))
        ContactSort.LOCATION -> contacts.sortedWith(Comparator<ApiContactRow> { a, b -> compareText(a.location, b.location) }.then(byName))
        ContactSort.RECENT -> contacts.sortedWith(compareByDescending<ApiContactRow> { it.updated }.then(byName))
        ContactSort.NAME -> contacts.sortedWith(byName)
    }
}

/** "Consultant · BCG", or whichever of the two there is, else the headline. */
fun contactSubtitle(c: ApiContactRow): String = listOf(c.title, c.company).filter { it.isNotBlank() }.joinToString(" · ").ifEmpty { c.headline }

/** An editable contact field: API name, label, and whether it's long text. */
data class ContactField(val key: String, val label: String, val long: Boolean = false, val hint: String = "")

val CONTACT_FIELDS = listOf(
    ContactField("full_name", "Name"),
    ContactField("headline", "Headline"),
    ContactField("title", "Job title"),
    ContactField("company", "Company"),
    ContactField("location", "Location"),
    ContactField("email", "Email"),
    ContactField("phone", "Phone"),
    ContactField("linkedin_url", "LinkedIn", hint = "https://www.linkedin.com/in/…"),
    ContactField("birthday", "Birthday", hint = "YYYY-MM-DD or MM-DD"),
    ContactField("about", "About", long = true),
)

/**
 * Contacts, groups and tags from the server. Online only, like habits: kept
 * in memory and reloaded when the tab opens.
 */
class PeopleStore(private val api: Api, private val prefs: Prefs, private val scope: CoroutineScope) {
    private val _contacts = MutableStateFlow<List<ApiContactRow>>(emptyList())
    val contacts: StateFlow<List<ApiContactRow>> = _contacts
    private val _groups = MutableStateFlow<List<ApiContactGroup>>(emptyList())
    val groups: StateFlow<List<ApiContactGroup>> = _groups
    private val _tags = MutableStateFlow<List<ApiContactTag>>(emptyList())
    val tags: StateFlow<List<ApiContactTag>> = _tags

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error
    private val _loaded = MutableStateFlow(false)
    val loaded: StateFlow<Boolean> = _loaded
    private val _loading = MutableStateFlow(false)
    val loading: StateFlow<Boolean> = _loading

    val message = MutableStateFlow<String?>(null)

    // What the tab showed, kept here so it's the same when you come back.
    val query = MutableStateFlow("")
    val sort = MutableStateFlow(ContactSort.NAME)
    val filter = MutableStateFlow<PeopleFilter>(PeopleFilter.All)
    val openId = MutableStateFlow<Long?>(null)

    /** The open contact, read in full. */
    private val _open = MutableStateFlow<ApiContact?>(null)
    val open: StateFlow<ApiContact?> = _open

    /** Bumped when a contact's photo changes, so it's fetched again. */
    private val _photoVersions = MutableStateFlow<Map<Long, Int>>(emptyMap())
    val photoVersions: StateFlow<Map<Long, Int>> = _photoVersions
    private val photos = ConcurrentHashMap<String, Bitmap>()

    private var server = ""

    fun load() {
        if (prefs.mode.value != AccountMode.SERVER) {
            clear()
            return
        }
        if (server != prefs.serverUrl) {
            server = prefs.serverUrl
            clear()
        }
        if (_loading.value) return
        _loading.value = true
        scope.launch {
            try {
                val o = withContext(Dispatchers.IO) { api.contactOverview() }
                _contacts.value = o.contacts.orEmpty()
                _groups.value = o.groups.orEmpty().sortedWith(compareBy(collator) { it.name })
                _tags.value = o.tags.orEmpty().sortedWith(compareBy(collator) { it.name })
                _loaded.value = true
                _error.value = null
                // A group or tag that was deleted elsewhere.
                when (val f = filter.value) {
                    is PeopleFilter.Group -> if (_groups.value.none { it.id == f.id }) filter.value = PeopleFilter.All
                    is PeopleFilter.Tag -> if (_tags.value.none { it.id == f.id }) filter.value = PeopleFilter.All
                    PeopleFilter.All -> {}
                }
            } catch (e: Exception) {
                _error.value = HabitsStore.errorText(e)
            } finally {
                _loading.value = false
            }
        }
    }

    fun clear() {
        _contacts.value = emptyList(); _groups.value = emptyList(); _tags.value = emptyList()
        _loaded.value = false; _error.value = null; openId.value = null; _open.value = null
        photos.clear()
    }

    private suspend fun <T> call(block: () -> T): T? = try {
        withContext(Dispatchers.IO) { block() }
    } catch (e: Exception) {
        message.value = HabitsStore.errorText(e)
        null
    }

    private fun applyToRow(c: ApiContact) {
        val row = c.toRow()
        val list = _contacts.value
        _contacts.value = if (list.any { it.id == c.id }) list.map { if (it.id == c.id) row else it } else list + row
        if (openId.value == c.id) _open.value = c
    }

    /** Opens a contact's detail page and reads it in full. */
    fun openContact(id: Long) {
        openId.value = id
        if (_open.value?.id != id) _open.value = null
        scope.launch { call { api.contact(id) }?.let { if (openId.value == id) applyToRow(it) } }
    }

    fun closeContact() {
        openId.value = null
        _open.value = null
    }

    /** Creates a contact; a 409 means one with the same email or LinkedIn exists, which [onDuplicate] can override. */
    fun create(fields: Map<String, String>, allowDuplicate: Boolean, onDuplicate: (String) -> Unit, onCreated: (ApiContact) -> Unit) = scope.launch {
        val body = buildJsonObject { for ((k, v) in fields) if (v.isNotBlank()) put(k, v.trim()) }
        try {
            val c = withContext(Dispatchers.IO) { api.createContact(body, allowDuplicate) }
            applyToRow(c)
            onCreated(c)
        } catch (e: ApiException) {
            if (e.status == 409) onDuplicate(e.message ?: "This contact already exists.") else message.value = HabitsStore.errorText(e)
        } catch (e: Exception) {
            message.value = HabitsStore.errorText(e)
        }
    }

    /** Sets one field; the server locks it so imports don't overwrite it. */
    fun setField(id: Long, key: String, value: String) = patch(id, buildJsonObject { put(key, value.trim()) })

    /** Unlocks a field so imports and enrichment may update it again. */
    fun unlock(c: ApiContact, key: String) =
        patch(c.id, buildJsonObject { put("locked_fields", JsonArray(c.lockedFields.orEmpty().filter { it != key }.map { JsonPrimitive(it) })) })

    private fun patch(id: Long, body: JsonObject) = scope.launch { call { api.updateContact(id, body) }?.let(::applyToRow) }

    fun delete(id: Long) = scope.launch {
        if (call { api.deleteContact(id) } != null) {
            _contacts.value = _contacts.value.filter { it.id != id }
            if (openId.value == id) closeContact()
            load() // member counts changed
        }
    }

    /** Runs a change to the contact's groups, tags, history, notes or photo, then rereads it. */
    private fun change(id: Long, reloadLists: Boolean = false, block: () -> Unit) = scope.launch {
        if (call(block) == null) return@launch
        call { api.contact(id) }?.let(::applyToRow)
        if (reloadLists) load()
    }

    fun setGroup(id: Long, group: String, on: Boolean, create: Boolean = false) = change(id, reloadLists = true) { api.setContactGroup(group, listOf(id), on, create) }
    fun setTag(id: Long, tag: String, on: Boolean) = change(id, reloadLists = true) { api.setContactTag(tag, listOf(id), on) }

    /** [kind] is experiences, educations or notes; [entry] null adds one. */
    fun saveEntry(id: Long, kind: String, entry: Long?, fields: JsonObject) = change(id) { api.saveContactEntry(id, kind, entry, fields) }
    fun deleteEntry(id: Long, kind: String, entry: Long) = change(id) { api.deleteContactEntry(id, kind, entry) }

    fun setPhoto(id: Long, bytes: ByteArray?, mime: String = "image/jpeg") = scope.launch {
        val ok = call { if (bytes != null) api.uploadContactPhoto(id, bytes, mime) else api.deleteContactPhoto(id) }
        if (ok == null) return@launch
        _photoVersions.value = _photoVersions.value + (id to (_photoVersions.value[id] ?: 0) + 1)
        call { api.contact(id) }?.let(::applyToRow)
    }

    /** The contact's photo, cached per version; null if it has none or it can't be loaded. */
    suspend fun photo(id: Long): Bitmap? {
        val key = "$id:${_photoVersions.value[id] ?: 0}"
        photos[key]?.let { return it }
        return try {
            withContext(Dispatchers.IO) {
                val bytes = api.contactPhoto(id)
                BitmapFactory.decodeByteArray(bytes, 0, bytes.size)
            }?.also { photos[key] = it }
        } catch (e: Exception) {
            null
        }
    }

    fun saveGroup(id: Long?, name: String, hexColor: String, description: String) = scope.launch {
        call { api.saveContactGroup(id, name.trim(), hexColor.removePrefix("#"), description.trim()) }?.let { load() }
    }

    fun deleteGroup(id: Long) = scope.launch {
        if (call { api.deleteContactGroup(id) } != null) {
            if (filter.value == PeopleFilter.Group(id)) filter.value = PeopleFilter.All
            load()
        }
    }

    fun deleteTag(id: Long) = scope.launch {
        if (call { api.deleteContactTag(id) } != null) {
            if (filter.value == PeopleFilter.Tag(id)) filter.value = PeopleFilter.All
            load()
        }
    }
}
