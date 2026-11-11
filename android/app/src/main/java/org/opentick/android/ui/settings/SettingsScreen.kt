package org.opentick.android.ui.settings

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.FormatListBulleted
import androidx.compose.material.icons.automirrored.filled.Help
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.automirrored.outlined.Logout
import androidx.compose.material.icons.filled.CloudSync
import androidx.compose.material.icons.filled.FormatPaint
import androidx.compose.material.icons.filled.GridView
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.MusicNote
import androidx.compose.material.icons.filled.Schedule
import androidx.compose.material.icons.filled.Timer
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.DragHandle
import androidx.compose.material.icons.rounded.Remove
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.RadioButton
import androidx.compose.material3.RadioButtonDefaults
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.BuildConfig
import org.opentick.android.data.AccountMode
import org.opentick.android.data.SyncState
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.Page
import org.opentick.android.ui.Tab
import org.opentick.android.ui.TabIcon
import org.opentick.android.ui.components.Avatar
import org.opentick.android.ui.components.PageHeader
import org.opentick.android.ui.components.SettingsRow
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.Green
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.MinusRed
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.ui.theme.Yellow
import java.text.DateFormat
import java.util.Date
import kotlin.math.roundToInt

/** Grey page with a back arrow and a title, holding white setting cards. */
@Composable
fun SettingsPage(title: String, onBack: () -> Unit, content: @Composable ColumnScope.() -> Unit) {
    val p = LocalExtra.current
    Column(Modifier.fillMaxSize().background(p.page)) {
        PageHeader(title, onBack = onBack)
        Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp).navigationBarsPadding(), content = {
            content()
            Spacer(Modifier.height(32.dp))
        })
    }
}

@Composable
fun SettingsCard(content: @Composable ColumnScope.() -> Unit) {
    Column(
        Modifier.fillMaxWidth().padding(vertical = 8.dp).clip(RoundedCornerShape(16.dp)).background(LocalExtra.current.card).padding(vertical = 4.dp),
        content = content,
    )
}

@Composable
fun SettingsScreen(vm: MainViewModel, onSignIn: () -> Unit) {
    val mode by vm.prefs.mode.collectAsState()
    val p = LocalExtra.current
    val context = LocalContext.current
    SettingsPage("Settings", vm::back) {
        val name = vm.prefs.displayName.ifBlank { vm.prefs.username }.ifBlank { if (mode == AccountMode.LOCAL) "Local account" else "OpenTick" }
        Row(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).clickable { if (mode == AccountMode.SERVER) vm.open(Page.ACCOUNT) else onSignIn() }.padding(vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Avatar(name, 72.dp)
            Spacer(Modifier.width(18.dp))
            Column(Modifier.weight(1f)) {
                Text(name, fontSize = 18.sp, fontWeight = FontWeight.Bold, color = p.text)
                Spacer(Modifier.height(6.dp))
                Text(
                    if (mode == AccountMode.SERVER) vm.prefs.serverUrl.removePrefix("https://").removePrefix("http://") else "Sign in to sync",
                    fontSize = 13.sp, color = if (mode == AccountMode.SERVER) p.textTertiary else Accent,
                    modifier = Modifier.clip(RoundedCornerShape(10.dp)).background(if (mode == AccountMode.SERVER) p.page else Accent.copy(alpha = 0.1f)).padding(horizontal = 10.dp, vertical = 3.dp),
                )
            }
            Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, null, tint = p.count)
        }
        SettingsCard { SettingsRow(Icons.Filled.GridView, Accent, "Tab Bar") { vm.open(Page.TAB_BAR) } }
        SettingsCard {
            SettingsRow(Icons.Filled.FormatPaint, Accent, "Appearance") { vm.open(Page.APPEARANCE) }
            SettingsRow(Icons.Filled.Schedule, Accent, "Date & Time") { vm.open(Page.DATE_TIME) }
            SettingsRow(Icons.Filled.MusicNote, Accent, "Sounds & Notifications") { vm.open(Page.NOTIFICATIONS) }
            SettingsRow(Icons.Filled.Timer, Accent, "Pomodoro") { vm.open(Page.POMO_SETTINGS) }
            SettingsRow(Icons.AutoMirrored.Filled.FormatListBulleted, Accent, "General") { vm.open(Page.GENERAL) }
        }
        SettingsCard {
            SettingsRow(Icons.Filled.CloudSync, Green, if (mode == AccountMode.SERVER) "Account & Sync" else "Sign In & Sync") {
                if (mode == AccountMode.SERVER) vm.open(Page.ACCOUNT) else onSignIn()
            }
        }
        SettingsCard {
            SettingsRow(Icons.AutoMirrored.Filled.Help, Yellow, "Help & Feedback") {
                context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse("https://github.com/minustimesplus/opentick/issues")).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            }
            SettingsRow(Icons.Filled.Info, Accent, "About") { vm.open(Page.ABOUT) }
        }
    }
}

// ─── Tab Bar ──────────────────────────────────────────────────────────

@Composable
fun TabBarSettings(vm: MainViewModel) {
    val p = LocalExtra.current
    val tabsPref by vm.prefs.tabs.collectAsState()
    val enabled = Tab.parse(tabsPref)
    val disabled = Tab.entries.filter { it !in enabled }
    fun save(list: List<Tab>) = vm.prefs.setTabs(list.joinToString(",") { it.name })

    SettingsPage("Tab Bar", vm::back) {
        SettingsCard {
            // Drag the handle to reorder.
            var dragging by remember { mutableStateOf<Tab?>(null) }
            var dragY by remember { mutableFloatStateOf(0f) }
            val rowPx = with(LocalDensity.current) { 74.dp.toPx() }
            for ((i, t) in enabled.withIndex()) {
                val isDragged = dragging == t
                Box(
                    Modifier.then(if (isDragged) Modifier.offset { IntOffset(0, dragY.roundToInt()) }.shadow(6.dp, RoundedCornerShape(12.dp)).background(p.card) else Modifier),
                ) {
                    TabRow(
                        t,
                        action = {
                            val canRemove = t != Tab.TASKS
                            Box(
                                Modifier.size(22.dp).clip(CircleShape).background(if (canRemove) MinusRed else p.divider)
                                    .clickable(enabled = canRemove) { save(enabled - t) },
                                contentAlignment = Alignment.Center,
                            ) { Icon(Icons.Rounded.Remove, "Remove", tint = Color.White, modifier = Modifier.size(16.dp)) }
                        },
                        handle = Modifier.pointerInput(enabled) {
                            detectDragGestures(
                                onDragStart = { dragging = t; dragY = 0f },
                                onDragEnd = {
                                    val target = (i + (dragY / rowPx).roundToInt()).coerceIn(0, enabled.lastIndex)
                                    if (target != i) save(enabled.toMutableList().apply { removeAt(i); add(target, t) })
                                    dragging = null; dragY = 0f
                                },
                                onDragCancel = { dragging = null; dragY = 0f },
                            ) { change, amount -> change.consume(); dragY += amount.y }
                        },
                    )
                }
            }
        }
        if (disabled.isNotEmpty()) {
            Text("Disabled", fontSize = 16.sp, color = p.textSecondary, modifier = Modifier.padding(start = 26.dp, top = 18.dp, bottom = 4.dp))
            SettingsCard {
                for (t in disabled) {
                    TabRow(t, action = {
                        val full = enabled.size >= 5
                        Box(
                            Modifier.size(22.dp).clip(CircleShape).background(if (full) p.divider else Green).clickable(enabled = !full) { save(enabled + t) },
                            contentAlignment = Alignment.Center,
                        ) { Icon(Icons.Rounded.Add, "Add", tint = Color.White, modifier = Modifier.size(16.dp)) }
                    }, handle = Modifier)
                }
            }
            Text("Up to 5 tabs. Drag ≡ to reorder.", fontSize = 13.sp, color = p.textTertiary, modifier = Modifier.padding(start = 26.dp, top = 4.dp))
        }
    }
}

@Composable
private fun TabRow(tab: Tab, action: @Composable () -> Unit, handle: Modifier) {
    val p = LocalExtra.current
    Row(Modifier.fillMaxWidth().height(74.dp).padding(start = 16.dp, end = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        action()
        Spacer(Modifier.width(12.dp))
        TabIcon(tab, Color(0xFF838383), Modifier.size(22.dp))
        Spacer(Modifier.width(14.dp))
        Column(Modifier.weight(1f)) {
            Text(tab.title, fontSize = 16.sp, color = p.text)
            Text(tab.description, fontSize = 13.sp, color = p.textTertiary, lineHeight = 17.sp)
        }
        Icon(Icons.Rounded.DragHandle, "Reorder", tint = p.count.copy(alpha = 0.7f), modifier = handle.padding(10.dp).size(22.dp))
    }
}

// ─── Other pages ──────────────────────────────────────────────────────

@Composable
fun AppearanceSettings(vm: MainViewModel) {
    val theme by vm.prefs.theme.collectAsState()
    SettingsPage("Appearance", vm::back) {
        SettingsCard {
            listOf("System Default", "Light", "Dark").forEachIndexed { i, name -> RadioRow(name, theme == i) { vm.prefs.setTheme(i) } }
        }
    }
}

@Composable
fun DateTimeSettings(vm: MainViewModel) {
    val week by vm.prefs.weekStart.collectAsState()
    SettingsPage("Date & Time", vm::back) {
        Label("Start week on")
        SettingsCard {
            for ((v, name) in listOf(0 to "System Default", 6 to "Saturday", 7 to "Sunday", 1 to "Monday")) RadioRow(name, week == v) { vm.prefs.setWeekStart(v) }
        }
    }
}

@Composable
fun NotificationSettings(vm: MainViewModel) {
    var reminders by remember { mutableStateOf(vm.prefs.remindersEnabled) }
    SettingsPage("Sounds & Notifications", vm::back) {
        SettingsCard {
            SwitchRow("Task reminders", "Notify at a task's due time. All-day tasks remind at 9:00.", reminders) {
                reminders = it; vm.prefs.remindersEnabled = it; vm.sync()
            }
        }
    }
}

@Composable
fun PomoSettings(vm: MainViewModel) {
    var focus by remember { mutableIntStateOf(vm.prefs.focusMinutes) }
    var brk by remember { mutableIntStateOf(vm.prefs.breakMinutes) }
    SettingsPage("Pomodoro", vm::back) {
        SettingsCard {
            Stepper("Pomo duration", focus, 5..120, 5) { focus = it; vm.prefs.focusMinutes = it }
            Stepper("Break duration", brk, 1..60, 1) { brk = it; vm.prefs.breakMinutes = it }
        }
    }
}

@Composable
fun GeneralSettings(vm: MainViewModel) {
    val smart by vm.prefs.smartLists.collectAsState()
    val showCompleted by vm.prefs.showCompleted.collectAsState()
    SettingsPage("General", vm::back) {
        Label("Smart Lists")
        SettingsCard {
            for ((key, name) in listOf("TODAY" to "Today", "TOMORROW" to "Tomorrow", "NEXT7" to "Next 7 Days", "INBOX" to "Inbox", "ALL" to "All", "TAGS" to "Tags", "COMPLETED" to "Completed", "TRASH" to "Trash")) {
                SwitchRow(name, null, key in smart) { on -> vm.prefs.setSmartLists(if (on) smart + key else smart - key) }
            }
        }
        Label("Lists")
        SettingsCard { SwitchRow("Show Completed", "Show completed tasks at the bottom of lists", showCompleted) { vm.prefs.setShowCompleted(it) } }
    }
}

@Composable
fun AccountSettings(vm: MainViewModel) {
    val sync by vm.syncState.collectAsState()
    val pending by vm.pendingCount.collectAsState()
    var confirm by remember { mutableStateOf(false) }
    SettingsPage("Account & Sync", vm::back) {
        SettingsCard {
            SettingsRow(null, Accent, "Account", value = vm.prefs.displayName.ifBlank { vm.prefs.username }, onClick = null)
            SettingsRow(null, Accent, "Server", value = vm.prefs.serverUrl.removePrefix("https://").removePrefix("http://"), onClick = null)
        }
        SettingsCard {
            val last = vm.prefs.lastSync
            SettingsRow(
                Icons.Filled.CloudSync, Green, "Sync Now",
                subtitle = when (val s = sync) {
                    is SyncState.Running -> "Syncing…"
                    is SyncState.Failed -> s.message
                    else -> (if (last > 0) "Last synced " + DateFormat.getDateTimeInstance(DateFormat.MEDIUM, DateFormat.SHORT).format(Date(last)) else "Not synced yet") +
                        if (pending > 0) " · $pending waiting" else ""
                },
                chevron = false,
            ) { vm.sync() }
        }
        SettingsCard { SettingsRow(Icons.AutoMirrored.Outlined.Logout, PrioHigh, "Sign Out", titleColor = PrioHigh, chevron = false) { confirm = true } }
    }
    if (confirm) SignOutDialog(vm, pending) { confirm = false }
}

@Composable
fun AboutPage(vm: MainViewModel) {
    val mode by vm.prefs.mode.collectAsState()
    var confirm by remember { mutableStateOf(false) }
    SettingsPage("About", vm::back) {
        SettingsCard {
            SettingsRow(null, Accent, "Version", value = BuildConfig.VERSION_NAME, onClick = null)
            SettingsRow(null, Accent, "Works with", value = "OpenTick Cloud and self-hosted", onClick = null)
        }
        if (mode == AccountMode.LOCAL) {
            SettingsCard { SettingsRow(Icons.AutoMirrored.Outlined.Logout, PrioHigh, "Delete Local Data", titleColor = PrioHigh, chevron = false) { confirm = true } }
        }
    }
    if (confirm) SignOutDialog(vm, 0) { confirm = false }
}

@Composable
private fun SignOutDialog(vm: MainViewModel, pending: Int, onDismiss: () -> Unit) {
    val mode by vm.prefs.mode.collectAsState()
    val server = mode == AccountMode.SERVER
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(if (server) "Sign out?" else "Delete local data?") },
        text = {
            Text(
                if (server) { if (pending > 0) "$pending changes haven't synced yet and will be lost." else "Your tasks stay on the server." }
                else "All tasks on this device will be deleted.",
            )
        },
        confirmButton = { TextButton(onClick = { onDismiss(); vm.signOut() }) { Text(if (server) "Sign Out" else "Delete", color = PrioHigh) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
        containerColor = LocalExtra.current.card,
    )
}

@Composable
private fun Label(text: String) {
    Text(text, fontSize = 14.sp, color = LocalExtra.current.textSecondary, modifier = Modifier.padding(start = 20.dp, top = 14.dp, bottom = 2.dp))
}

@Composable
private fun RadioRow(title: String, selected: Boolean, onClick: () -> Unit) {
    Row(Modifier.fillMaxWidth().clickable(onClick = onClick).padding(horizontal = 18.dp, vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(title, fontSize = 16.sp, color = LocalExtra.current.text, modifier = Modifier.weight(1f))
        RadioButton(selected, onClick, colors = RadioButtonDefaults.colors(selectedColor = Accent))
    }
}

@Composable
private fun SwitchRow(title: String, subtitle: String?, checked: Boolean, onChange: (Boolean) -> Unit) {
    val p = LocalExtra.current
    Row(Modifier.fillMaxWidth().clickable { onChange(!checked) }.padding(horizontal = 18.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Text(title, fontSize = 16.sp, color = p.text)
            if (subtitle != null) Text(subtitle, fontSize = 13.sp, color = p.textTertiary)
        }
        Switch(checked, onChange, colors = SwitchDefaults.colors(checkedTrackColor = Accent))
    }
}

@Composable
private fun Stepper(title: String, value: Int, range: IntRange, step: Int, onChange: (Int) -> Unit) {
    val p = LocalExtra.current
    Row(Modifier.fillMaxWidth().padding(start = 18.dp, end = 8.dp, top = 4.dp, bottom = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(title, Modifier.weight(1f), fontSize = 16.sp, color = p.text)
        TextButton(onClick = { onChange((value - step).coerceIn(range)) }) { Text("−", fontSize = 18.sp, color = Accent) }
        Text("$value min", color = p.text)
        TextButton(onClick = { onChange((value + step).coerceIn(range)) }) { Text("+", fontSize = 18.sp, color = Accent) }
    }
}

