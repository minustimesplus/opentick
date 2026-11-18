package org.opentick.android

import android.Manifest
import android.content.Intent
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import kotlinx.coroutines.launch
import androidx.lifecycle.lifecycleScope
import org.opentick.android.data.AccountMode
import org.opentick.android.ui.MainScreen
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.Tab
import org.opentick.android.ui.auth.LoginScreen
import org.opentick.android.ui.theme.OpenTickTheme
import org.opentick.android.widget.parseWidgetKey

class MainActivity : ComponentActivity() {
    private val vm: MainViewModel by viewModels()
    private var oauthError by mutableStateOf<String?>(null)
    /** The tab to switch to, with a counter so asking for the same tab again switches back to it. */
    private var tabRequest by mutableStateOf<Pair<Tab, Int>?>(null)

    private fun requestTab(tab: Tab) { tabRequest = tab to (tabRequest?.second ?: 0) + 1 }

    private val notificationPermission = registerForActivityResult(ActivityResultContracts.RequestPermission()) {}

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        if (savedInstanceState == null) handleIntent(intent)

        setContent {
            val theme by vm.prefs.theme.collectAsState()
            val mode by vm.prefs.mode.collectAsState()
            OpenTickTheme(theme) {
                if (mode == AccountMode.NONE) {
                    LoginScreen(vm, oauthError, { oauthError = null })
                } else {
                    MainScreen(vm, tabRequest, oauthError) { oauthError = null }
                }
            }
        }

        if (Build.VERSION.SDK_INT >= 33) notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleIntent(intent)
    }

    private fun handleIntent(intent: Intent?) {
        intent ?: return
        val data = intent.data
        if (intent.action == Intent.ACTION_VIEW && data?.scheme == "vikunja-opentick") {
            vm.handleOAuthCallback(data) { err -> oauthError = err }
            return
        }
        if (intent.action == Intent.ACTION_SEND) {
            val text = intent.getStringExtra(Intent.EXTRA_TEXT)?.trim()
            val subject = intent.getStringExtra(Intent.EXTRA_SUBJECT)?.trim()
            if (!text.isNullOrBlank() && vm.prefs.mode.value != AccountMode.NONE) {
                lifecycleScope.launch {
                    val title = subject?.takeIf { it.isNotBlank() } ?: text.lineSequence().first().take(200)
                    val desc = if (subject.isNullOrBlank() && text.lines().size == 1 && text.length <= 200) "" else text
                    val id = vm.repo.createTask(title, null, description = desc)
                    vm.openTaskId.value = id
                }
            }
            return
        }
        if (intent.getBooleanExtra(EXTRA_INSTALL_UPDATE, false)) {
            intent.removeExtra(EXTRA_INSTALL_UPDATE)
            vm.installUpdate()
        }
        intent.getLongExtra("open_task", 0).takeIf { it != 0L }?.let { vm.openTaskId.value = it; requestTab(Tab.TASKS) }
        // From a home screen widget: a list (maybe with quick add), a tab or a habit.
        val quickAdd = intent.getBooleanExtra(EXTRA_QUICK_ADD, false)
        intent.getStringExtra(EXTRA_WIDGET_LIST)?.let { key ->
            vm.openTaskId.value = null
            vm.selection.value = parseWidgetKey(key)
            requestTab(Tab.TASKS)
            if (quickAdd) vm.quickAddTab.value = Tab.TASKS
        }
        val tab = when (intent.getStringExtra("open_tab")) {
            "focus" -> Tab.POMO
            "tasks" -> Tab.TASKS
            "matrix" -> Tab.MATRIX
            "calendar" -> Tab.CALENDAR
            "habits" -> Tab.HABITS
            else -> null
        }
        if (tab != null) {
            if (intent.getBooleanExtra(EXTRA_FROM_WIDGET, false)) vm.openTaskId.value = null
            requestTab(tab)
            if (quickAdd) vm.quickAddTab.value = tab
        }
        intent.getLongExtra(EXTRA_OPEN_HABIT, 0).takeIf { it != 0L }?.let { vm.habits.openId.value = it }
    }

    companion object {
        /** From the "update available" notification: download and install it. */
        const val EXTRA_INSTALL_UPDATE = "install_update"
        const val EXTRA_FROM_WIDGET = "from_widget"
        /** A widget's list (see parseWidgetKey) to show in the Tasks tab. */
        const val EXTRA_WIDGET_LIST = "widget_list"
        /** Opens quick add in the tab or list the intent opens. */
        const val EXTRA_QUICK_ADD = "quick_add"
        const val EXTRA_OPEN_HABIT = "open_habit"
    }
}
