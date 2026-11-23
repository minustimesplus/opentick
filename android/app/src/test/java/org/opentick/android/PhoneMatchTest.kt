package org.opentick.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.opentick.android.data.ApiContactRow
import org.opentick.android.ui.people.DeviceContact
import org.opentick.android.ui.people.DeviceNumber
import org.opentick.android.ui.people.MatchReason
import org.opentick.android.ui.people.comparableNumber
import org.opentick.android.ui.people.matchPhoneContacts
import org.opentick.android.ui.people.matchReason

class PhoneMatchTest {
    private fun person(id: Long, name: String, email: String = "", phone: String = "") = ApiContactRow(id = id, name = name, email = email, phone = phone)
    private fun phone(name: String, vararg numbers: String, emails: List<String> = emptyList(), label: String = "Mobile") =
        DeviceContact(name, emails, numbers.map { DeviceNumber(it, label) })

    @Test
    fun numbersCompareWithoutCountryOrTrunkPrefix() {
        assertEquals(comparableNumber("+49 170 1234567"), comparableNumber("0170 / 123 45 67"))
        assertEquals(comparableNumber("+1 (555) 123-4567"), comparableNumber("555.123.4567"))
        assertEquals("", comparableNumber("112"))
    }

    @Test
    fun matchesByEmailThenNameThenSimilarName() {
        val ada = person(1, "Ada Lovelace", email = "Ada@Example.com")
        assertEquals(MatchReason.EMAIL, matchReason(ada, phone("Mum's friend", "1", emails = listOf("ada@example.com "))))
        assertEquals(MatchReason.NAME, matchReason(ada, phone("ada lovelace", "1")))
        assertEquals(MatchReason.NAME, matchReason(person(2, "José Müller"), phone("jose muller", "1")))
        assertEquals(MatchReason.SIMILAR_NAME, matchReason(person(3, "Anna Maria Schmidt"), phone("Anna Schmidt", "1")))
        assertEquals(MatchReason.SIMILAR_NAME, matchReason(person(3, "Anna Schmidt"), phone("Schmidt, Anna", "1")))
        // One word is too little to go on.
        assertNull(matchReason(person(4, "Anna Schmidt"), phone("Anna", "1")))
        assertNull(matchReason(person(5, "Anna Schmidt"), phone("Anna Meier", "1")))
    }

    @Test
    fun suggestsNumbersOnlyForMatchesThatLackThem() {
        val contacts = listOf(
            person(1, "Ada Lovelace"),
            person(2, "Alan Turing", phone = "+44 20 7946 0001"),
            person(3, "Grace Hopper", phone = "+1 555 000 0000"),
            person(4, "Nobody Here"),
        )
        val device = listOf(
            phone("Ada Lovelace", "+44 7700 900123"),
            phone("Alan Turing", "020 7946 0001"), // already has it
            phone("Grace Hopper", "+1 555 999 9999"),
            phone("Someone Else", "+44 7700 900999"),
        )
        val s = matchPhoneContacts(contacts, device)
        assertEquals(listOf(1L, 3L), s.map { it.contact.id }) // without a number first
        assertEquals("+44 7700 900123", s[0].options.single().number)
        assertEquals("+1 555 999 9999", s[1].options.single().number)
    }

    @Test
    fun prefersTheStrongestMatchAndMobileNumbers() {
        val ada = person(1, "Ada Lovelace", email = "ada@example.com")
        val device = listOf(
            phone("Ada L.", "+44 20 1111 1111", emails = listOf("ada@example.com"), label = "Work"),
            DeviceContact("Ada Lovelace", emptyList(), listOf(DeviceNumber("+44 7700 900123", "Mobile"))),
        )
        val s = matchPhoneContacts(listOf(ada), device).single()
        assertEquals(MatchReason.EMAIL, s.reason)
        assertEquals(listOf("+44 20 1111 1111"), s.options.map { it.number })

        val two = DeviceContact("Grace Hopper", emptyList(), listOf(DeviceNumber("+1 555 222 2222", "Work"), DeviceNumber("+1 555 333 3333", "Mobile"), DeviceNumber("1 555 333 3333", "Other")))
        val g = matchPhoneContacts(listOf(person(2, "Grace Hopper")), listOf(two)).single()
        assertEquals(listOf("+1 555 333 3333", "+1 555 222 2222"), g.options.map { it.number })
    }

    @Test
    fun ignoredSuggestionsStayHiddenUntilANewNumberShowsUp() {
        val contacts = listOf(person(1, "Ada Lovelace"))
        val first = matchPhoneContacts(contacts, listOf(phone("Ada Lovelace", "+44 7700 900123"))).single()
        assertTrue(matchPhoneContacts(contacts, listOf(phone("Ada Lovelace", "+44 7700 900123")), setOf(first.key)).isEmpty())
        assertEquals(1, matchPhoneContacts(contacts, listOf(phone("Ada Lovelace", "+44 7700 900456")), setOf(first.key)).size)
    }
}
