package org.opentick.android

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.opentick.android.data.LabelEntity
import org.opentick.android.data.ListLayout
import org.opentick.android.data.ListSort
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.SmartLists
import org.opentick.android.data.TaskEntity
import org.opentick.android.data.TimelineScale
import org.opentick.android.data.ViewSync
import org.opentick.android.data.Views
import org.opentick.android.data.Visibility
import org.opentick.android.ui.Data
import org.opentick.android.ui.ListSel
import org.opentick.android.ui.SmartList
import org.opentick.android.ui.tasks.KanbanBucket
import org.opentick.android.ui.tasks.Placement
import org.opentick.android.ui.tasks.kanbanColumns
import org.opentick.android.ui.tasks.listKey
import org.opentick.android.ui.tasks.sortTasks
import org.opentick.android.ui.tasks.topPosition

/** Per-list settings and smart lists shared with the web app (web prefs.listSort, listView, …). */
class ListSettingsTest {
    private fun obj(s: String) = Json.parseToJsonElement(s) as JsonObject

    private val data = Data(
        projects = listOf(
            ProjectEntity(id = 1, serverId = 10, title = "Inbox", isInbox = true),
            ProjectEntity(id = 2, serverId = 20, title = "Work"),
            ProjectEntity(id = 3, serverId = null, title = "Offline"),
        ),
        labels = listOf(LabelEntity(id = 4, serverId = 40, title = "home"), LabelEntity(id = 5, title = "new")),
    )

    @Test
    fun listKeysMatchTheWebRoutes() {
        assertEquals("today", listKey(ListSel.Smart(SmartList.TODAY), data))
        assertEquals("week", listKey(ListSel.Smart(SmartList.NEXT7), data))
        assertEquals("wontdo", listKey(ListSel.Smart(SmartList.WONT_DO), data))
        assertEquals("inbox", listKey(ListSel.Smart(SmartList.INBOX), data))
        assertEquals("inbox", listKey(ListSel.Project(1), data))
        assertEquals("p20", listKey(ListSel.Project(2), data))
        assertEquals("p-3", listKey(ListSel.Project(3), data))
        assertEquals("l40", listKey(ListSel.Label(4), data))
        assertEquals("l-5", listKey(ListSel.Label(5), data))
    }

    @Test
    fun readsAndWritesTheWebValues() {
        val v = Views(obj("""{"listSort":{"p20":"priority"},"listShowCompleted":{"p20":false},"listView":{"p20":"kanban"},"timelineScale":"month"}"""))
        assertEquals(ListSort.PRIORITY, v.listSort("p20"))
        assertEquals(ListSort.DATE, v.listSort("today"))
        assertFalse(v.listShowCompleted("p20"))
        assertTrue(v.listShowCompleted("today"))
        assertEquals(ListLayout.KANBAN, v.listLayout("p20"))
        assertEquals(ListLayout.LIST, v.listLayout("inbox"))
        assertEquals(TimelineScale.MONTH, v.timelineScale)
        assertEquals(TimelineScale.WEEK, Views().timelineScale)

        val edited = v.withListSort("today", ListSort.TITLE).withListShowCompleted("p20", true).withListLayout("p20", ListLayout.TIMELINE)
        assertEquals(
            obj("""{"listSort":{"p20":"priority","today":"title"},"listShowCompleted":{"p20":true},"listView":{"p20":"timeline"},"timelineScale":"month"}"""),
            edited.json,
        )
    }

    @Test
    fun onlyTheChangedListGoesToTheServer() {
        val snapshot = obj("""{"listSort":{"p20":"priority","today":"date"},"rev":5}""")
        val local = Views(obj("""{"listSort":{"p20":"priority","today":"date"}}""")).withListSort("today", ListSort.CUSTOM)
        val r = ViewSync.reconcile(local.json, snapshot, snapshot, now = 9)
        assertEquals(obj("""{"listSort":{"today":"custom"},"rev":9}"""), r.patch)
    }

    @Test
    fun offlineListKeysTakeTheServerIdOnceSynced() {
        val v = Views(obj("""{"listSort":{"p-3":"title","p20":"date","l-5":"priority","p-9":"title"},"listView":{"p-3":"kanban"}}"""))
        val fixed = v.resolveIds(project = { if (it == 3L) 30L else null }, task = { null }, label = { if (it == 5L) 50L else null })
        // p-9 is a list deleted before it ever synced.
        assertEquals(obj("""{"listSort":{"p30":"title","p20":"date","l50":"priority"},"listView":{"p30":"kanban"}}"""), fixed.json)
    }

    @Test
    fun smartListsFollowTheServerElseThePhonesOldSwitches() {
        val legacy = setOf("TODAY", "INBOX", "TRASH")
        val before = Views().smartLists(legacy)
        assertEquals(Visibility.SHOW, before["today"])
        assertEquals(Visibility.HIDE, before["next7"]) // switched off on the phone
        assertEquals(Visibility.AUTO, before["wontDo"]) // not on the phone before: web default
        assertEquals(SmartLists.DEFAULTS.keys, before.keys)

        val synced = Views(obj("""{"smartLists":{"today":"auto","next7":"show","summary":"show"}}""")).smartLists(legacy)
        assertEquals(Visibility.AUTO, synced["today"])
        assertEquals(Visibility.SHOW, synced["next7"])
        assertEquals(Visibility.SHOW, synced["summary"])
        assertEquals(Visibility.SHOW, synced["trash"]) // missing key: web default
    }

    @Test
    fun settingASmartListWritesTheWholeSet() {
        val v = Views(obj("""{"smartLists":{"today":"auto","summary":"show","future":"show"}}""")).withSmartList("inbox", Visibility.HIDE, emptySet())
        val o = v.json["smartLists"] as JsonObject
        assertEquals(SmartLists.DEFAULTS.keys + "future", o.keys)
        assertEquals("hide", (o["inbox"] as kotlinx.serialization.json.JsonPrimitive).content)
        assertEquals("auto", (o["today"] as kotlinx.serialization.json.JsonPrimitive).content)
        assertEquals("show", (o["summary"] as kotlinx.serialization.json.JsonPrimitive).content)
    }

    // ─── Sorting ───────────────────────────────────────────────────────

    private fun task(id: Long, title: String, due: Long? = null, priority: Int = 0, created: Long = 0) =
        TaskEntity(id = id, serverId = id + 100, projectId = 2, title = title, dueDate = due, priority = priority, created = created)

    private val tasks = listOf(
        task(1, "banana", due = 3000, priority = 1, created = 10),
        task(2, "Apple", due = 1000, priority = 0, created = 30),
        task(3, "🍒 cherry", due = null, priority = 3, created = 20),
        task(4, "date", due = 1000, priority = 3, created = 40),
    )
    private val work = ListSel.Project(2)

    private fun ids(l: List<TaskEntity>) = l.map { it.id }

    @Test
    fun sortsLikeTheWebApp() {
        // Date: earliest first, higher priority first on the same date, undated last.
        assertEquals(listOf(4L, 2L, 1L, 3L), ids(sortTasks(tasks, ListSort.DATE, work)))
        assertEquals(listOf(4L, 3L, 1L, 2L), ids(sortTasks(tasks, ListSort.PRIORITY, work)))
        // Title ignores case and a leading emoji.
        assertEquals(listOf(2L, 1L, 3L, 4L), ids(sortTasks(tasks, ListSort.TITLE, work)))
        assertEquals(listOf(4L, 2L, 3L, 1L), ids(sortTasks(tasks, ListSort.CREATED, work)))
        // Custom: the list's manual order; unplaced tasks at the end.
        val positions = mapOf(103L to 1.0, 101L to 2.0, 104L to 3.0)
        assertEquals(listOf(3L, 1L, 4L, 2L), ids(sortTasks(tasks, ListSort.CUSTOM, work, positions)))
    }

    // ─── Kanban ────────────────────────────────────────────────────────

    @Test
    fun kanbanPutsTasksInTheirSections() {
        val buckets = listOf(KanbanBucket(1, "To Do", 1.0), KanbanBucket(2, "Doing", 2.0), KanbanBucket(3, "Done", 3.0))
        val ts = listOf(
            task(1, "a"), task(2, "b"), task(3, "c").copy(done = true), task(4, "d"),
            TaskEntity(id = 5, projectId = 2, title = "made offline"),
        )
        val placement = mapOf(101L to Placement(2, 20.0), 102L to Placement(2, 10.0), 104L to Placement(3, 5.0))
        val cols = kanbanColumns(ts, buckets, placement, defaultBucket = 1, doneBucket = 3)
        // Unplaced and offline tasks go to the default section; an open task can't sit in the done section.
        assertEquals(listOf(4L, 5L), ids(cols[0].tasks))
        assertEquals(listOf(2L, 1L), ids(cols[1].tasks))
        assertEquals(listOf(3L), ids(cols[2].tasks))
        assertEquals(5.0, topPosition(cols[1].tasks, placement), 0.0)
        assertEquals(65536.0, topPosition(emptyList(), placement), 0.0)
    }
}
