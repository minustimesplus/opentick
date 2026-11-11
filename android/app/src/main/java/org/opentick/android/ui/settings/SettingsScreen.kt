package org.opentick.android.ui.settings

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.Logout
import androidx.compose.material.icons.outlined.AccountCircle
import androidx.compose.material.icons.outlined.CloudSync
import androidx.compose.material.icons.outlined.DarkMode
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.Notifications
import androidx.compose.material.icons.outlined.Timer
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.BuildConfig
import org.opentick.android.data.AccountMode
import org.opentick.android.data.SyncState
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh
import java.text.DateFormat
import java.util.Date

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(vm: MainViewModel, onSignIn: () -> Unit) {
    val mode by vm.prefs.mode.collectAsState()
    val theme by vm.prefs.theme.collectAsState()
    val sync by vm.syncState.collectAsState()
    val pending by vm.pendingCount.collectAsState()
    var reminders by remember { mutableStateOf(vm.prefs.remindersEnabled) }
    var focus by remember { mutableIntStateOf(vm.prefs.focusMinutes) }
    var brk by remember { mutableIntStateOf(vm.prefs.breakMinutes) }
    var confirmSignOut by remember { mutableStateOf(false) }
    var themeDialog by remember { mutableStateOf(false) }
    val extra = LocalExtra.current

    Scaffold(
        containerColor = extra.panel,
        topBar = {
            TopAppBar(
                colors = TopAppBarDefaults.topAppBarColors(containerColor = extra.panel),
                title = { Text("Settings", fontWeight = FontWeight.Bold) },
            )
        },
    ) { pad ->
        Column(Modifier.padding(pad).fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp)) {
            Group {
                if (mode == AccountMode.SERVER) {
                    Item(Icons.Outlined.AccountCircle, vm.prefs.displayName.ifBlank { vm.prefs.username }, vm.prefs.serverUrl)
                    val last = vm.prefs.lastSync
                    Item(
                        Icons.Outlined.CloudSync, "Sync now",
                        when (val s = sync) {
                            is SyncState.Running -> "Syncing…"
                            is SyncState.Failed -> s.message
                            else -> (if (last > 0) "Last synced " + DateFormat.getDateTimeInstance(DateFormat.MEDIUM, DateFormat.SHORT).format(Date(last)) else "Not synced yet") +
                                if (pending > 0) " · $pending waiting to sync" else ""
                        },
                    ) { vm.sync() }
                } else {
                    Item(Icons.Outlined.AccountCircle, "Sign in to sync", "Your tasks are only on this device. Signing in uploads them to your account.", onClick = onSignIn)
                }
            }
            Group {
                Item(Icons.Outlined.DarkMode, "Theme", listOf("System default", "Light", "Dark")[theme]) { themeDialog = true }
                Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                    Icon(Icons.Outlined.Notifications, null, tint = extra.textSecondary)
                    Spacer(Modifier.size(16.dp))
                    Column(Modifier.weight(1f)) {
                        Text("Reminders")
                        Text("Notify at a task's due time (all-day tasks at 9:00)", fontSize = 12.sp, color = extra.textTertiary)
                    }
                    Switch(reminders, { reminders = it; vm.prefs.remindersEnabled = it; vm.sync() })
                }
            }
            Group {
                Stepper(Icons.Outlined.Timer, "Pomo length", focus, 5..120, 5) { focus = it; vm.prefs.focusMinutes = it }
                Stepper(Icons.Outlined.Timer, "Break length", brk, 1..60, 1) { brk = it; vm.prefs.breakMinutes = it }
            }
            Group {
                Item(Icons.Outlined.Info, "About", "OpenTick for Android ${BuildConfig.VERSION_NAME}. Works with OpenTick Cloud and self-hosted servers.")
            }
            Group {
                Item(
                    Icons.AutoMirrored.Outlined.Logout,
                    if (mode == AccountMode.SERVER) "Sign out" else "Delete local data",
                    null, tint = PrioHigh,
                ) { confirmSignOut = true }
            }
            Spacer(Modifier.size(32.dp))
        }
    }

    if (themeDialog) {
        AlertDialog(
            onDismissRequest = { themeDialog = false },
            title = { Text("Theme") },
            text = {
                Column {
                    listOf("System default", "Light", "Dark").forEachIndexed { i, name ->
                        Row(Modifier.fillMaxWidth().clickable { vm.prefs.setTheme(i); themeDialog = false }.padding(vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                            RadioButton(theme == i, { vm.prefs.setTheme(i); themeDialog = false })
                            Text(name)
                        }
                    }
                }
            },
            confirmButton = {},
        )
    }
    if (confirmSignOut) {
        AlertDialog(
            onDismissRequest = { confirmSignOut = false },
            title = { Text(if (mode == AccountMode.SERVER) "Sign out?" else "Delete local data?") },
            text = {
                Text(
                    if (mode == AccountMode.SERVER) {
                        if (pending > 0) "$pending changes haven't synced yet and will be lost." else "Your tasks stay on the server."
                    } else "All tasks on this device will be deleted.",
                )
            },
            confirmButton = { TextButton(onClick = { confirmSignOut = false; vm.signOut() }) { Text("Sign out", color = PrioHigh) } },
            dismissButton = { TextButton(onClick = { confirmSignOut = false }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun Group(content: @Composable () -> Unit) {
    Surface(shape = RoundedCornerShape(12.dp), color = MaterialTheme.colorScheme.background, modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
        Column(Modifier.padding(vertical = 4.dp)) { content() }
    }
}

@Composable
private fun Item(icon: ImageVector, title: String, subtitle: String?, tint: androidx.compose.ui.graphics.Color? = null, onClick: (() -> Unit)? = null) {
    val extra = LocalExtra.current
    Row(
        Modifier.fillMaxWidth().then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier).padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, null, tint = tint ?: extra.textSecondary)
        Spacer(Modifier.size(16.dp))
        Column(Modifier.weight(1f)) {
            Text(title, color = tint ?: MaterialTheme.colorScheme.onBackground)
            if (!subtitle.isNullOrBlank()) Text(subtitle, fontSize = 12.sp, color = extra.textTertiary)
        }
    }
}

@Composable
private fun Stepper(icon: ImageVector, title: String, value: Int, range: IntRange, step: Int, onChange: (Int) -> Unit) {
    val extra = LocalExtra.current
    Row(Modifier.fillMaxWidth().padding(start = 16.dp, end = 8.dp, top = 4.dp, bottom = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        Icon(icon, null, tint = extra.textSecondary)
        Spacer(Modifier.size(16.dp))
        Text(title, Modifier.weight(1f))
        TextButton(onClick = { onChange((value - step).coerceIn(range)) }) { Text("−", fontSize = 18.sp) }
        Text("$value min")
        TextButton(onClick = { onChange((value + step).coerceIn(range)) }) { Text("+", fontSize = 18.sp) }
    }
}
