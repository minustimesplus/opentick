package org.opentick.android.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

// API shapes of Habits, weights and People, the features the web app shows
// under Habits and People. They live on the server only; the app reads and
// writes them online, like the kanban board and subscribed calendars.

@Serializable
data class ApiHabit(
    val id: Long = 0,
    val title: String = "",
    val icon: String = "",
    @SerialName("hex_color") val hexColor: String = "",
    /** daily or weekdays */
    val frequency: String = "daily",
    /** Bit 0 is Sunday, bit 6 Saturday. */
    val weekdays: Int = 127,
    val goal: Int = 1,
    val unit: String = "",
    val archived: Boolean = false,
    @SerialName("tracks_weight") val tracksWeight: Boolean = false,
    val position: Double = 0.0,
    @SerialName("today_value") val todayValue: Int = 0,
    @SerialName("current_streak") val currentStreak: Int = 0,
    @SerialName("recent_checkins") val recentCheckins: List<ApiHabitCheckin>? = null,
)

@Serializable
data class ApiHabitCheckin(
    val date: String = "",
    val value: Int = 0,
)

@Serializable
data class ApiHabitStats(
    @SerialName("current_streak") val currentStreak: Int = 0,
    @SerialName("longest_streak") val longestStreak: Int = 0,
    @SerialName("completed_days") val completedDays: Int = 0,
    @SerialName("due_days") val dueDays: Int = 0,
    @SerialName("completion_rate") val completionRate: Double = 0.0,
    @SerialName("total_checkins") val totalCheckins: Int = 0,
    @SerialName("total_value") val totalValue: Int = 0,
    val month: String = "",
    val days: List<ApiHabitStatsDay>? = null,
)

@Serializable
data class ApiHabitStatsDay(
    val date: String = "",
    val value: Int = 0,
    val due: Boolean = false,
    val completed: Boolean = false,
)

/** A day's body weight in kg. */
@Serializable
data class ApiWeight(
    val date: String = "",
    val weight: Double = 0.0,
)

/** A contact as the People list shows it. */
@Serializable
data class ApiContactRow(
    val id: Long = 0,
    val name: String = "",
    val headline: String = "",
    val company: String = "",
    val title: String = "",
    val location: String = "",
    val email: String = "",
    val phone: String = "",
    @SerialName("linkedin_url") val linkedinUrl: String = "",
    val birthday: String = "",
    @SerialName("has_photo") val hasPhoto: Boolean = false,
    @SerialName("tag_ids") val tagIds: List<Long>? = null,
    @SerialName("group_ids") val groupIds: List<Long>? = null,
    val schools: List<String>? = null,
    @SerialName("search_text") val searchText: String = "",
    val updated: String = "",
)

@Serializable
data class ApiContactGroup(
    val id: Long = 0,
    val name: String = "",
    @SerialName("hex_color") val hexColor: String = "",
    val description: String = "",
    val members: Long = 0,
)

@Serializable
data class ApiContactTag(
    val id: Long = 0,
    val name: String = "",
    @SerialName("hex_color") val hexColor: String = "",
    val contacts: Long = 0,
)

@Serializable
data class ApiContactOverview(
    val contacts: List<ApiContactRow>? = null,
    val groups: List<ApiContactGroup>? = null,
    val tags: List<ApiContactTag>? = null,
)

@Serializable
data class ApiContactExperience(
    val id: Long = 0,
    val company: String = "",
    val title: String = "",
    val location: String = "",
    @SerialName("start_date") val startDate: String = "",
    @SerialName("end_date") val endDate: String = "",
    val description: String = "",
)

@Serializable
data class ApiContactEducation(
    val id: Long = 0,
    val school: String = "",
    val degree: String = "",
    @SerialName("field_of_study") val fieldOfStudy: String = "",
    @SerialName("start_year") val startYear: String = "",
    @SerialName("end_year") val endYear: String = "",
)

@Serializable
data class ApiContactNote(
    val id: Long = 0,
    val body: String = "",
    val created: String = "",
    val updated: String = "",
)

/** A contact with its history and notes. */
@Serializable
data class ApiContact(
    val id: Long = 0,
    val name: String = "",
    @SerialName("full_name") val fullName: String = "",
    val headline: String = "",
    val company: String = "",
    val title: String = "",
    val location: String = "",
    val email: String = "",
    val phone: String = "",
    @SerialName("linkedin_url") val linkedinUrl: String = "",
    val birthday: String = "",
    val about: String = "",
    val source: String = "",
    @SerialName("locked_fields") val lockedFields: List<String>? = null,
    @SerialName("has_photo") val hasPhoto: Boolean = false,
    val experiences: List<ApiContactExperience>? = null,
    val educations: List<ApiContactEducation>? = null,
    val notes: List<ApiContactNote>? = null,
    @SerialName("tag_ids") val tagIds: List<Long>? = null,
    @SerialName("group_ids") val groupIds: List<Long>? = null,
    val updated: String = "",
) {
    /** The value of an editable field by its API name. */
    fun field(key: String): String = when (key) {
        "full_name" -> fullName
        "headline" -> headline
        "company" -> company
        "title" -> title
        "location" -> location
        "email" -> email
        "phone" -> phone
        "linkedin_url" -> linkedinUrl
        "birthday" -> birthday
        "about" -> about
        else -> ""
    }

    /** What the list shows, after reading or editing the full contact. */
    fun toRow(): ApiContactRow = ApiContactRow(
        id = id, name = name, headline = headline, company = company, title = title, location = location,
        email = email, phone = phone, linkedinUrl = linkedinUrl, birthday = birthday, hasPhoto = hasPhoto,
        tagIds = tagIds.orEmpty(), groupIds = groupIds.orEmpty(),
        schools = educations.orEmpty().map { it.school }.filter { it.isNotBlank() }.distinct(),
        searchText = (listOf(about) + educations.orEmpty().flatMap { listOf(it.degree, it.fieldOfStudy) } +
            experiences.orEmpty().flatMap { listOf(it.company, it.title) }).filter { it.isNotBlank() }.joinToString(" "),
        updated = updated,
    )
}
