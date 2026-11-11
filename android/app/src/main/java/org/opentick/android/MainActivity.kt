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

class MainActivity : ComponentActivity() {
    private val vm: MainViewModel by viewModels()
    private var oauthError by mutableStateOf<String?>(null)
    private var initialTab by mutableStateOf<Tab?>(null)

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
                    MainScreen(vm, initialTab, oauthError) { oauthError = null }
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
        intent.getLongExtra("open_task", 0).takeIf { it != 0L }?.let { vm.openTaskId.value = it; initialTab = Tab.TASKS }
        if (intent.getStringExtra("open_tab") == "focus") initialTab = Tab.POMO
    }
}
