package org.opentick.android.data

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.Query
import androidx.room.Update
import androidx.room.Upsert
import kotlinx.coroutines.flow.Flow

@Dao
interface ProjectDao {
    @Query("SELECT * FROM projects WHERE deleted = 0 ORDER BY isInbox DESC, position, id")
    fun observe(): Flow<List<ProjectEntity>>

    @Query("SELECT * FROM projects")
    suspend fun all(): List<ProjectEntity>

    @Query("SELECT * FROM projects WHERE id = :id")
    suspend fun get(id: Long): ProjectEntity?

    @Query("SELECT * FROM projects WHERE serverId = :serverId")
    suspend fun byServerId(serverId: Long): ProjectEntity?

    @Query("SELECT * FROM projects WHERE isInbox = 1 AND deleted = 0 LIMIT 1")
    suspend fun inbox(): ProjectEntity?

    @Insert
    suspend fun insert(p: ProjectEntity): Long

    @Update
    suspend fun update(p: ProjectEntity)

    @Upsert
    suspend fun upsert(p: ProjectEntity): Long

    @Query("DELETE FROM projects WHERE id = :id")
    suspend fun hardDelete(id: Long)

    @Query("UPDATE projects SET dirty = 0 WHERE id = :id AND version = :version")
    suspend fun markClean(id: Long, version: Int)

    @Query("UPDATE projects SET isInbox = (id = :id)")
    suspend fun setInbox(id: Long)
}

@Dao
interface LabelDao {
    @Query("SELECT * FROM labels WHERE deleted = 0 ORDER BY title COLLATE NOCASE")
    fun observe(): Flow<List<LabelEntity>>

    @Query("SELECT * FROM labels")
    suspend fun all(): List<LabelEntity>

    @Query("SELECT * FROM labels WHERE id = :id")
    suspend fun get(id: Long): LabelEntity?

    @Query("SELECT * FROM labels WHERE serverId = :serverId")
    suspend fun byServerId(serverId: Long): LabelEntity?

    @Query("SELECT * FROM labels WHERE title = :title COLLATE NOCASE AND deleted = 0 LIMIT 1")
    suspend fun byTitle(title: String): LabelEntity?

    @Insert
    suspend fun insert(l: LabelEntity): Long

    @Update
    suspend fun update(l: LabelEntity)

    @Query("DELETE FROM labels WHERE id = :id")
    suspend fun hardDelete(id: Long)

    @Query("UPDATE labels SET dirty = 0 WHERE id = :id AND version = :version")
    suspend fun markClean(id: Long, version: Int)
}

@Dao
interface TaskDao {
    @Query("SELECT * FROM tasks WHERE deleted = 0")
    fun observeAll(): Flow<List<TaskEntity>>

    @Query("SELECT * FROM tasks WHERE id = :id")
    fun observe(id: Long): Flow<TaskEntity?>

    @Query("SELECT * FROM tasks")
    suspend fun all(): List<TaskEntity>

    @Query("SELECT * FROM tasks WHERE deleted = 0 AND done = 0")
    suspend fun open(): List<TaskEntity>

    @Query("SELECT * FROM tasks WHERE id = :id")
    suspend fun get(id: Long): TaskEntity?

    @Query("SELECT * FROM tasks WHERE serverId = :serverId")
    suspend fun byServerId(serverId: Long): TaskEntity?

    @Query("SELECT * FROM tasks WHERE dirty = 1 OR deleted = 1 OR serverId IS NULL")
    suspend fun pending(): List<TaskEntity>

    @Query("SELECT COUNT(*) FROM tasks WHERE dirty = 1 OR deleted = 1 OR serverId IS NULL")
    fun observePendingCount(): Flow<Int>

    @Insert
    suspend fun insert(t: TaskEntity): Long

    @Update
    suspend fun update(t: TaskEntity)

    @Query("DELETE FROM tasks WHERE id = :id")
    suspend fun hardDelete(id: Long)

    @Query("UPDATE tasks SET dirty = 0 WHERE id = :id AND version = :version")
    suspend fun markClean(id: Long, version: Int)

    @Query("UPDATE tasks SET deleted = 1, dirty = 1, version = version + 1 WHERE projectId = :projectId")
    suspend fun deleteInProject(projectId: Long)

    @Query("DELETE FROM tasks WHERE projectId = :projectId AND serverId IS NULL")
    suspend fun dropUnsyncedInProject(projectId: Long)

    @Query("DELETE FROM tasks")
    suspend fun clear()
}
