package org.opentick.android

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.opentick.android.data.MatrixGroup
import org.opentick.android.data.MatrixSort
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.QuadrantView
import org.opentick.android.data.TaskEntity
import org.opentick.android.data.ViewSync
import org.opentick.android.data.Views
import org.opentick.android.ui.Data
import org.opentick.android.ui.matrix.QUADRANT_ROWS
import org.opentick.android.ui.matrix.matrixGroups
import org.opentick.android.util.allDayMillis
import org.opentick.android.util.today

class ViewSyncTest {
    private fun obj(s: String) = Json.parseToJsonElement(s) as JsonObject

    @Test
    fun freshPhoneTakesTheServerValuesAndPushesNothing() {
        val remote = obj("""{"matrixLists":[7],"matrixShowDone":true,"smartLists":{"today":"show"},"rev":5}""")
        val r = ViewSync.reconcile(JsonObject(emptyMap()), null, remote)
        assertNull(r.patch)
        assertEquals(obj("""{"matrixLists":[7],"matrixShowDone":true,"smartLists":{"today":"show"}}"""), r.values)
    }

    @Test
    fun phoneEditsGoOutAsMergePatchAndKeepRemoteEdits() {
        val snapshot = obj("""{"matrixView":{"1":{"group":"list","sort":"date"}},"matrixShowDone":false,"rev":5}""")
        val local = Views(obj("""{"matrixView":{"1":{"group":"list","sort":"date"}},"matrixShowDone":false}"""))
            .withQuadrant(1, QuadrantView(MatrixGroup.LIST, MatrixSort.TITLE))
        val remote = obj("""{"matrixView":{"1":{"group":"list","sort":"date"},"2":{"group":"none","sort":"date"}},"matrixShowDone":true,"rev":6}""")
        val r = ViewSync.reconcile(local.json, snapshot, remote, now = 3)
        assertEquals(obj("""{"matrixView":{"1":{"sort":"title"}},"rev":7}"""), r.patch)
        assertEquals(
            obj("""{"matrixView":{"1":{"group":"list","sort":"title"},"2":{"group":"none","sort":"date"}},"matrixShowDone":true}"""),
            r.values,
        )
    }

    @Test
    fun resetQuadrantSendsNull() {
        val snapshot = obj("""{"matrixView":{"1":{"group":"list","sort":"date"}},"matrixOrder":{"1":[3,4]},"rev":1}""")
        val local = Views(snapshot).resetQuadrant(1)
        val r = ViewSync.reconcile(local.json, snapshot, snapshot, now = 50)
        assertEquals(obj("""{"matrixView":{"1":null},"matrixOrder":{"1":null},"rev":50}"""), r.patch)
    }

    @Test
    fun staleServerAnswerIsIgnored() {
        val snapshot = obj("""{"matrixShowDone":true,"rev":10}""")
        val r = ViewSync.reconcile(obj("""{"matrixShowDone":true}"""), snapshot, obj("""{"matrixShowDone":false,"rev":9}"""))
        assertNull(r.patch)
        assertEquals(obj("""{"matrixShowDone":true}"""), r.values)
    }

    @Test
    fun localIdsAreSwappedForServerIds() {
        val v = Views().withMatrixLists(listOf(4, -2, -9)).withMatrixOrder(1, listOf(-5, 8))
        val fixed = v.resolveIds(project = { if (it == 2L) 40L else null }, task = { if (it == 5L) 50L else null })
        assertEquals(listOf(4L, 40L), fixed.matrixLists)
        assertEquals(listOf(50L, 8L), fixed.matrixOrder(1))
    }

    @Test
    fun matrixFiltersByListAndGroupsByList() {
        val inbox = ProjectEntity(id = 1, serverId = 11, title = "Inbox", isInbox = true)
        val work = ProjectEntity(id = 2, serverId = 12, title = "Work", position = 1.0)
        val home = ProjectEntity(id = 3, serverId = 13, title = "Home", position = 2.0)
        val due = allDayMillis(today())
        val tasks = listOf(
            TaskEntity(id = 1, serverId = 101, projectId = 2, title = "b", priority = 3, dueDate = due),
            TaskEntity(id = 2, serverId = 102, projectId = 3, title = "a", priority = 5),
            TaskEntity(id = 3, serverId = 103, projectId = 1, title = "c", priority = 3),
            TaskEntity(id = 4, serverId = 104, projectId = 2, title = "low", priority = 1),
        )
        val d = Data(listOf(inbox, work, home), emptyList(), tasks)
        val q1 = QUADRANT_ROWS[0][0]
        val views = Views().withMatrixLists(listOf(12, 13)).withQuadrant(1, QuadrantView(MatrixGroup.LIST, MatrixSort.TITLE))
        val groups = matrixGroups(q1, d, views, today())
        assertEquals(listOf("Work", "Home"), groups.map { it.title })
        assertEquals(listOf(listOf("b"), listOf("a")), groups.map { g -> g.tasks.map { it.title } })
    }

    @Test
    fun customOrderFollowsTheSyncedOrder() {
        val p = ProjectEntity(id = 1, serverId = 11, title = "Inbox", isInbox = true)
        val tasks = (1L..3L).map { TaskEntity(id = it, serverId = 100 + it, projectId = 1, title = "t$it", priority = 2, created = it) }
        val views = Views().withQuadrant(2, QuadrantView(MatrixGroup.NONE, MatrixSort.CUSTOM)).withMatrixOrder(2, listOf(103, 101))
        val q2 = QUADRANT_ROWS[1][0]
        val titles = matrixGroups(q2, Data(listOf(p), emptyList(), tasks), views, today()).single().tasks.map { it.title }
        // t2 isn't placed yet, so it goes on top.
        assertEquals(listOf("t2", "t3", "t1"), titles)
    }
}
