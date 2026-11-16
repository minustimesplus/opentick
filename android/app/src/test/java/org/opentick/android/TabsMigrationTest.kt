package org.opentick.android

import android.app.Application
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.opentick.android.data.Prefs
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Habits and People join tab bars set up before they existed. */
@RunWith(RobolectricTestRunner::class)
@Config(application = Application::class, sdk = [34])
class TabsMigrationTest {
    private val ctx get() = ApplicationProvider.getApplicationContext<Application>()

    private fun tabsAfterUpgrade(stored: String?): String {
        val sp = ctx.getSharedPreferences("opentick", 0)
        sp.edit().clear().apply { if (stored != null) putString("tabs", stored) }.commit()
        return Prefs(ctx).tabs.value
    }

    @Test
    fun freshInstallGetsTheNewDefault() {
        assertEquals(Prefs.DEFAULT_TABS, tabsAfterUpgrade(null))
    }

    @Test
    fun theOldDefaultBecomesTheNewOne() {
        assertEquals("TASKS,MATRIX,CALENDAR,HABITS,PEOPLE", tabsAfterUpgrade("TASKS,MATRIX,CALENDAR,POMO"))
    }

    @Test
    fun customBarsGetWhatFits() {
        assertEquals("TASKS,CALENDAR,HABITS,PEOPLE", tabsAfterUpgrade("TASKS,CALENDAR"))
        assertEquals("TASKS,CALENDAR,POMO,SEARCH,HABITS", tabsAfterUpgrade("TASKS,CALENDAR,POMO,SEARCH"))
        assertEquals("TASKS,MATRIX,CALENDAR,POMO,SEARCH", tabsAfterUpgrade("TASKS,MATRIX,CALENDAR,POMO,SEARCH"))
    }

    @Test
    fun runsOnceSoRemovedTabsStayRemoved() {
        tabsAfterUpgrade("TASKS,CALENDAR")
        val prefs = Prefs(ctx)
        prefs.setTabs("TASKS,CALENDAR")
        assertEquals("TASKS,CALENDAR", Prefs(ctx).tabs.value)
    }
}
