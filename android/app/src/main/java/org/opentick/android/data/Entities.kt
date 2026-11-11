package org.opentick.android.data

import androidx.room.Entity
import androidx.room.Index
import androidx.room.PrimaryKey

// Every row has a local id (used everywhere in the app) and the server's id
// once it has been synced. Rows created offline have serverId = null until the
// next sync pushes them.
//
// dirty: changed locally and not yet pushed. deleted: deleted locally, the row
// stays until the server delete went through. version increments on every
// local edit so a push only clears `dirty` if nothing changed meanwhile.

@Entity(tableName = "projects", indices = [Index(value = ["serverId"], unique = true)])
data class ProjectEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val serverId: Long? = null,
    val title: String,
    val hexColor: String = "",
    val position: Double = 0.0,
    val isInbox: Boolean = false,
    val isFavorite: Boolean = false,
    /** The parent list on the server; lists with children show as folders. */
    val parentServerId: Long? = null,
    val dirty: Boolean = false,
    val deleted: Boolean = false,
    val version: Int = 0,
)

@Entity(tableName = "labels", indices = [Index(value = ["serverId"], unique = true)])
data class LabelEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val serverId: Long? = null,
    val title: String,
    val hexColor: String = "",
    val dirty: Boolean = false,
    val deleted: Boolean = false,
    val version: Int = 0,
)

@Entity(
    tableName = "tasks",
    indices = [Index(value = ["serverId"], unique = true), Index("projectId"), Index("done")],
)
data class TaskEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val serverId: Long? = null,
    val projectId: Long,
    val title: String,
    val description: String = "",
    val done: Boolean = false,
    val doneAt: Long? = null,
    val wontDo: Boolean = false,
    /** Epoch millis. All-day tasks are due at 23:59 local time, like the web app. */
    val dueDate: Long? = null,
    /** Scheduled time range from the server; the calendar draws these as blocks. */
    val startDate: Long? = null,
    val endDate: Long? = null,
    /** Vikunja 0-5; the app shows 0 none, 1 low, 2 medium, 3+ high. */
    val priority: Int = 0,
    /** Local label ids, comma separated. */
    val labelIds: String = "",
    /** Server label ids the server had at the last sync, for diffing on push. */
    val syncedLabelServerIds: String = "",
    val repeatAfter: Long = 0,
    val repeatMode: Int = 0,
    val repeatRule: String = "",
    val parentServerId: Long? = null,
    val created: Long = System.currentTimeMillis(),
    val updated: Long = System.currentTimeMillis(),
    val dirty: Boolean = false,
    val deleted: Boolean = false,
    val version: Int = 0,
) {
    val labelIdList: List<Long> get() = parseIds(labelIds)
    val isRepeating: Boolean get() = repeatAfter > 0 || repeatRule.isNotBlank() || repeatMode == 1
}

fun parseIds(s: String): List<Long> = s.split(',').mapNotNull { it.trim().toLongOrNull() }
fun joinIds(ids: Collection<Long>): String = ids.distinct().joinToString(",")
