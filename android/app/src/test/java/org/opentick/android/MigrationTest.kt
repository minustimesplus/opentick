package org.opentick.android

import android.app.Application
import android.database.sqlite.SQLiteDatabase
import androidx.room.Room
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.junit.runner.RunWith
import org.opentick.android.data.AppDatabase
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.io.File

/** A database written by version 1 of the app opens after the upgrade, data intact. */
@RunWith(RobolectricTestRunner::class)
@Config(application = Application::class, sdk = [34])
class MigrationTest {
    @Test
    fun migratesOneToTwo() = runBlocking {
        val ctx = ApplicationProvider.getApplicationContext<Application>()
        val path = ctx.getDatabasePath("migration-test.db").apply { parentFile?.mkdirs(); delete() }

        // Build the version 1 schema from Room's exported description of it.
        val schema = Json.parseToJsonElement(File("schemas/org.opentick.android.data.AppDatabase/1.json").readText()).jsonObject["database"]!!.jsonObject
        SQLiteDatabase.openOrCreateDatabase(path, null).use { db ->
            for (e in schema["entities"]!!.jsonArray) {
                val table = e.jsonObject["tableName"]!!.jsonPrimitive.content
                db.execSQL(e.jsonObject["createSql"]!!.jsonPrimitive.content.replace("\${TABLE_NAME}", table))
                for (i in e.jsonObject["indices"]?.jsonArray.orEmpty()) {
                    db.execSQL(i.jsonObject["createSql"]!!.jsonPrimitive.content.replace("\${TABLE_NAME}", table))
                }
            }
            for (q in schema["setupQueries"]!!.jsonArray) db.execSQL(q.jsonPrimitive.content)
            db.execSQL("INSERT INTO projects (id, serverId, title, hexColor, position, isInbox, isFavorite, dirty, deleted, version) VALUES (1, 7, 'Inbox', '', 0, 1, 0, 0, 0, 0)")
            db.execSQL(
                "INSERT INTO tasks (id, serverId, projectId, title, description, done, wontDo, priority, labelIds, syncedLabelServerIds, repeatAfter, repeatMode, repeatRule, created, updated, dirty, deleted, version) " +
                    "VALUES (1, NULL, 1, 'Written offline', '', 0, 0, 3, '', '', 0, 0, '', 0, 0, 1, 0, 0)",
            )
            db.version = 1
        }

        val room = Room.databaseBuilder(ctx, AppDatabase::class.java, path.absolutePath)
            .addMigrations(AppDatabase.MIGRATION_1_2).allowMainThreadQueries().build()
        val task = room.tasks().get(1)!!
        assertEquals("Written offline", task.title)
        assertEquals(true, task.dirty)
        assertNull(task.startDate)
        assertNull(room.projects().get(1)!!.parentServerId)
        room.close()
    }
}
