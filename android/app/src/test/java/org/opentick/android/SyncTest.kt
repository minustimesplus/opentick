package org.opentick.android

import android.app.Application
import androidx.room.Room
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.opentick.android.data.AccountMode
import org.opentick.android.data.Api
import org.opentick.android.data.AppDatabase
import org.opentick.android.data.Prefs
import org.opentick.android.data.Repository
import org.opentick.android.data.SyncState
import org.opentick.android.util.allDayMillis
import org.opentick.android.util.today
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * End-to-end sync against a running OpenTick server with local accounts:
 *   OPENTICK_TEST_SERVER=http://localhost:3456 ./gradlew testDebugUnitTest
 * Skipped when the variable isn't set.
 */
@RunWith(RobolectricTestRunner::class)
@Config(application = Application::class, sdk = [34])
class SyncTest {
    private val server = System.getenv("OPENTICK_TEST_SERVER")
    private lateinit var db: AppDatabase
    private lateinit var prefs: Prefs
    private lateinit var api: Api
    private lateinit var repo: Repository

    @Before
    fun setUp() {
        assumeTrue("OPENTICK_TEST_SERVER not set", !server.isNullOrBlank())
        val ctx = ApplicationProvider.getApplicationContext<Application>()
        ctx.getSharedPreferences("opentick", 0).edit().clear().commit()
        db = Room.inMemoryDatabaseBuilder(ctx, AppDatabase::class.java).build()
        prefs = Prefs(ctx)
        api = Api(prefs)
        repo = Repository(db, api, prefs) {}
    }

    @After
    fun tearDown() {
        if (::db.isInitialized) db.close()
    }

    private fun signIn(): String {
        val user = "droidtest" + System.nanoTime().toString().takeLast(9)
        api.register(server!!, user, "$user@example.com", "password12345")
        val (token, refresh) = api.login(server, user, "password12345", null)
        assertNotNull("refresh token cookie", refresh)
        prefs.serverUrl = server
        prefs.accessToken = token
        prefs.refreshToken = refresh
        prefs.setMode(AccountMode.SERVER)
        return user
    }

    @Test
    fun offlineWorkUploadsAfterSignIn() = runBlocking {
        // Used without an account first: everything is local.
        prefs.setMode(AccountMode.LOCAL)
        val inboxTask = repo.createTask("Offline inbox task", null, dueDate = allDayMillis(today()), priority = 3, labelTitles = listOf("urgent"))
        val projectId = repo.createProject("Groceries", "2bb673")
        repo.createTask("Milk", projectId)
        assertTrue(repo.sync()) // no-op in local mode

        signIn()
        assertTrue("first sync", repo.sync())

        val remote = api.tasks("done = false")
        val t = remote.single { it.title == "Offline inbox task" }
        assertEquals(3, t.priority)
        assertEquals(listOf("urgent"), t.labels?.map { it.title })
        assertNotNull(t.dueDate)
        val groceries = api.projects().single { it.title == "Groceries" }
        assertEquals(groceries.id, remote.single { it.title == "Milk" }.projectId)
        // The local inbox merged into the server's: still exactly one inbox.
        assertEquals(1, db.projects().all().count { it.isInbox })
        assertEquals(t.id, db.tasks().get(inboxTask)!!.serverId)
        assertEquals(0, repo.pendingCount.first())
    }

    @Test
    fun editsDeletesAndServerChangesSync() = runBlocking {
        signIn()
        assertTrue(repo.sync())
        val a = repo.createTask("First", null)
        val b = repo.createTask("Second", null)
        assertTrue(repo.sync())

        // Local edits go up.
        repo.updateTask(a) { it.copy(title = "First, renamed", priority = 2) }
        repo.setDone(b, true)
        assertTrue(repo.sync())
        val aRemote = api.tasks("done = false").single { it.id == db.tasks().get(a)!!.serverId }
        assertEquals("First, renamed", aRemote.title)
        assertEquals(2, aRemote.priority)
        assertTrue(api.tasks("done = true").any { it.title == "Second" })

        // Server changes come down.
        val inboxServerId = db.projects().inbox()!!.serverId!!
        val created = api.createTask(inboxServerId, buildJsonObject { put("title", "From the web") })
        api.updateTask(aRemote.id, buildJsonObject { put("title", "Edited on the web") })
        assertTrue(repo.sync())
        assertNotNull(db.tasks().byServerId(created.id))
        assertEquals("Edited on the web", db.tasks().get(a)!!.title)

        // Deletes go up; server deletes come down.
        repo.deleteTask(a)
        api.deleteTask(created.id)
        assertTrue(repo.sync())
        assertNull(db.tasks().get(a))
        assertNull(db.tasks().byServerId(created.id))
        assertFalse(api.tasks("done = false").any { it.id == aRemote.id })
    }

    @Test
    fun queuesWhileOfflineAndRefreshesExpiredToken() = runBlocking {
        signIn()
        assertTrue(repo.sync())

        // Offline: nothing listens on this port.
        prefs.serverUrl = "http://127.0.0.1:9"
        val id = repo.createTask("Written offline", null)
        assertFalse(repo.sync())
        assertTrue((repo.syncState.value as SyncState.Failed).offline)
        assertEquals(1, repo.pendingCount.first())

        // Back online with an expired access token: the refresh token renews it.
        prefs.serverUrl = server!!
        prefs.accessToken = "expired.invalid.token"
        assertTrue(repo.sync())
        assertFalse(repo.authExpired.value)
        assertTrue(prefs.accessToken != "expired.invalid.token")
        assertNotNull(db.tasks().get(id)!!.serverId)
        assertTrue(api.tasks("done = false").any { it.title == "Written offline" })
    }

    @Test
    fun revokedSessionAsksToSignInAgain() = runBlocking {
        signIn()
        prefs.accessToken = "expired.invalid.token"
        prefs.refreshToken = "revoked"
        assertFalse(repo.sync())
        assertTrue(repo.authExpired.value)
    }
}
