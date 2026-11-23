package org.opentick.android.ui.people

import android.content.Context
import android.provider.ContactsContract
import org.opentick.android.data.ApiContactRow

/** A number from the phone's address book, with its label ("Mobile", "Work", …). */
data class DeviceNumber(val number: String, val label: String)

/** A contact from the phone's address book. Read on the device only, never uploaded. */
data class DeviceContact(val name: String, val emails: List<String> = emptyList(), val numbers: List<DeviceNumber> = emptyList())

/** Why a phone contact was matched to a person, strongest first. */
enum class MatchReason(val label: String) {
    EMAIL("Same email as"),
    NAME("Same name as"),
    SIMILAR_NAME("Similar name to"),
}

/** A number on the phone that could go on a person, and which phone contact it's from. */
data class NumberOption(val number: String, val label: String, val from: String)

/** A suggestion to add a number from the phone to a person in People. */
data class PhoneSuggestion(
    val contact: ApiContactRow,
    val reason: MatchReason,
    val options: List<NumberOption>,
) {
    /** What "Ignore" remembers: the person and the numbers offered, so a new number is suggested again. */
    val key: String get() = "${contact.id}:" + options.map { comparableNumber(it.number) }.sorted().joinToString(",")
}

/**
 * The digits of a number that identify it, so "+49 170 1234567" and
 * "0170 1234567" compare equal: the last nine digits, which drops country
 * and trunk prefixes. Empty for anything too short to be a phone number.
 */
fun comparableNumber(number: String): String {
    val digits = number.filter { it in '0'..'9' }
    return if (digits.length < 6) "" else digits.takeLast(9)
}

private val NAME_SPLIT = Regex("[\\s,]+")

/** The folded words of a name, in order. */
private fun nameTokens(name: String): List<String> = name.split(NAME_SPLIT).map(::fold).filter { it.isNotEmpty() }

/** How well a phone contact matches a person, or null if it doesn't. */
fun matchReason(contact: ApiContactRow, device: DeviceContact): MatchReason? {
    val email = contact.email.trim().lowercase()
    if (email.isNotEmpty() && device.emails.any { it.trim().lowercase() == email }) return MatchReason.EMAIL
    val name = fold(contact.name)
    if (name.isEmpty()) return null
    if (name == fold(device.name)) return MatchReason.NAME
    // "Anna Maria Schmidt" and "Anna Schmidt", or "Schmidt, Anna": same first
    // and last name, or the same words in another order. Needs two words on
    // both sides; a single "Anna" is too weak to suggest.
    val a = nameTokens(contact.name)
    val b = nameTokens(device.name)
    if (a.size < 2 || b.size < 2) return null
    if (a.first() == b.first() && a.last() == b.last()) return MatchReason.SIMILAR_NAME
    if (a.sorted() == b.sorted()) return MatchReason.SIMILAR_NAME
    return null
}

/**
 * Suggestions for numbers from the phone's contacts to add to people in
 * People. Each person gets at most one suggestion, from the phone contacts
 * that match them best; it lists every number those have that the person
 * doesn't. People whose number is already on a matching phone contact, and
 * suggestions in [ignored] (by [PhoneSuggestion.key]), are left out. People
 * without a number come first.
 */
fun matchPhoneContacts(contacts: List<ApiContactRow>, device: List<DeviceContact>, ignored: Set<String> = emptySet()): List<PhoneSuggestion> {
    val withNumbers = device.filter { d -> d.numbers.any { comparableNumber(it.number).isNotEmpty() } }
    val out = mutableListOf<PhoneSuggestion>()
    for (c in contacts) {
        val matches = withNumbers.mapNotNull { d -> matchReason(c, d)?.let { d to it } }
        if (matches.isEmpty()) continue
        val best = matches.minOf { it.second }
        val from = matches.filter { it.second == best }.map { it.first }
        val current = comparableNumber(c.phone)
        if (current.isNotEmpty() && from.any { d -> d.numbers.any { comparableNumber(it.number) == current } }) continue
        val seen = mutableSetOf<String>()
        val options = from.flatMap { d ->
            d.numbers.mapNotNull { n ->
                val cmp = comparableNumber(n.number)
                if (cmp.isEmpty() || !seen.add(cmp)) null else NumberOption(n.number.trim(), n.label, d.name)
            }
        }.sortedBy { if (it.label.equals("Mobile", ignoreCase = true)) 0 else 1 }
        if (options.isEmpty()) continue
        val s = PhoneSuggestion(c, best, options)
        if (s.key !in ignored) out += s
    }
    return out.sortedWith(compareBy<PhoneSuggestion> { it.contact.phone.isNotBlank() }.thenBy { it.reason }.thenBy { it.contact.name.lowercase() })
}

/** Reads the phone's contacts that have a number. Needs READ_CONTACTS. */
fun readDeviceContacts(context: Context): List<DeviceContact> {
    data class Acc(var name: String = "", val emails: MutableList<String> = mutableListOf(), val numbers: MutableList<DeviceNumber> = mutableListOf())
    val byId = LinkedHashMap<Long, Acc>()
    val phoneType = ContactsContract.CommonDataKinds.Phone.CONTENT_ITEM_TYPE
    val emailType = ContactsContract.CommonDataKinds.Email.CONTENT_ITEM_TYPE
    context.contentResolver.query(
        ContactsContract.Data.CONTENT_URI,
        arrayOf(
            ContactsContract.Data.CONTACT_ID,
            ContactsContract.Data.DISPLAY_NAME_PRIMARY,
            ContactsContract.Data.MIMETYPE,
            ContactsContract.Data.DATA1,
            ContactsContract.Data.DATA2,
            ContactsContract.Data.DATA3,
        ),
        "${ContactsContract.Data.MIMETYPE} IN (?, ?)",
        arrayOf(phoneType, emailType),
        null,
    )?.use { cur ->
        while (cur.moveToNext()) {
            val value = cur.getString(3)?.trim().orEmpty()
            if (value.isEmpty()) continue
            val acc = byId.getOrPut(cur.getLong(0)) { Acc() }
            if (acc.name.isEmpty()) acc.name = cur.getString(1).orEmpty()
            if (cur.getString(2) == phoneType) {
                val label = ContactsContract.CommonDataKinds.Phone.getTypeLabel(context.resources, cur.getInt(4), cur.getString(5)).toString()
                acc.numbers += DeviceNumber(value, label)
            } else {
                acc.emails += value
            }
        }
    }
    return byId.values.filter { it.numbers.isNotEmpty() && it.name.isNotBlank() }.map { DeviceContact(it.name, it.emails, it.numbers) }
}
