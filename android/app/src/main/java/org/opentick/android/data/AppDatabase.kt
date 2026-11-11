package org.opentick.android.data

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase

@Database(
    entities = [ProjectEntity::class, LabelEntity::class, TaskEntity::class],
    version = 2,
    exportSchema = true,
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun projects(): ProjectDao
    abstract fun labels(): LabelDao
    abstract fun tasks(): TaskDao

    companion object {
        fun create(context: Context): AppDatabase =
            Room.databaseBuilder(context, AppDatabase::class.java, "opentick.db")
                .addMigrations(MIGRATION_1_2)
                .build()

        /** Folders (parent lists) and task time ranges for the calendar. */
        val MIGRATION_1_2 = object : Migration(1, 2) {
            override fun migrate(db: SupportSQLiteDatabase) {
                db.execSQL("ALTER TABLE projects ADD COLUMN parentServerId INTEGER")
                db.execSQL("ALTER TABLE tasks ADD COLUMN startDate INTEGER")
                db.execSQL("ALTER TABLE tasks ADD COLUMN endDate INTEGER")
            }
        }
    }
}
