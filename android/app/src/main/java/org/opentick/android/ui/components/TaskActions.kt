package org.opentick.android.ui.components

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.DriveFileMove
import androidx.compose.material.icons.automirrored.outlined.Send
import androidx.compose.material.icons.filled.Flag
import androidx.compose.material.icons.outlined.CalendarMonth
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.DoNotDisturbOn
import androidx.compose.material.icons.outlined.EventBusy
import androidx.compose.material.icons.outlined.Link
import androidx.compose.material.icons.outlined.Replay
import androidx.compose.material.icons.outlined.SmartToy
import androidx.compose.material.icons.outlined.WbSunny
import androidx.compose.material.icons.outlined.WbTwilight
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import org.opentick.android.data.AgentConfig
import org.opentick.android.data.TaskEntity
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.Snack
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.ui.theme.priorityColor
import org.opentick.android.util.allDayMillis
import org.opentick.android.util.isAllDay
import org.opentick.android.util.startOfWeek
import org.opentick.android.util.toLocalDateTime
import org.opentick.android.util.toMillis
import org.opentick.android.util.today
import java.time.LocalDate

val AGENTS = listOf("claude", "instinct")
fun agentName(agent: String) = if (agent == "claude") "Claude" else "Instinct"

/** Moves the due date to another day, keeping the time of a timed task. */
fun dueOn(task: TaskEntity, day: LocalDate): Long {
    val due = task.dueDate
    return if (due == null || isAllDay(due)) allDayMillis(day) else day.atTime(due.toLocalDateTime().toLocalTime()).toMillis()
}

/**
 * What you can do with a task, opened by a long press on it. Same actions as
 * the web app's right-click menu: date, priority, move, won't do, duplicate,
 * send to an AI agent, copy link and delete.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TaskActionsSheet(vm: MainViewModel, taskId: Long, onDismiss: () -> Unit) {
    val data by vm.data.collectAsState()
    val task = data.tasks.firstOrNull { it.id == taskId && !it.deleted }
    if (task == null) {
        LaunchedEffect(Unit) { onDismiss() }
        return
    }
    val p = LocalExtra.current
    val clipboard = LocalClipboardManager.current
    var pickDate by remember { mutableStateOf(false) }
    var moving by remember { mutableStateOf(false) }
    val t0 = today()

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Column(Modifier.navigationBarsPadding().verticalScroll(rememberScrollState()).padding(bottom = 12.dp)) {
            Text(
                task.title.ifBlank { "Untitled" }, fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = p.text,
                maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(horizontal = 22.dp),
            )
            SectionLabel("Date")
            Row(Modifier.fillMaxWidth().padding(horizontal = 14.dp), horizontalArrangement = Arrangement.SpaceBetween) {
                IconAction(Icons.Outlined.WbSunny, "Today") { vm.update(task.id) { it.copy(dueDate = dueOn(it, t0)) }; onDismiss() }
                IconAction(Icons.Outlined.WbTwilight, "Tomorrow") { vm.update(task.id) { it.copy(dueDate = dueOn(it, t0.plusDays(1))) }; onDismiss() }
                IconAction(null, "Next week", text = "+7") { vm.update(task.id) { it.copy(dueDate = dueOn(it, startOfWeek(t0).plusDays(7))) }; onDismiss() }
                IconAction(Icons.Outlined.CalendarMonth, "Pick date") { pickDate = true }
                IconAction(Icons.Outlined.EventBusy, "Clear") { vm.update(task.id) { it.copy(dueDate = null) }; onDismiss() }
            }
            SectionLabel("Priority")
            Row(Modifier.fillMaxWidth().padding(horizontal = 14.dp), horizontalArrangement = Arrangement.SpaceBetween) {
                for ((value, name) in PRIORITIES) {
                    IconAction(Icons.Filled.Flag, name, tint = priorityColor(value, p.prioNone), selected = minOf(task.priority, 3) == value) {
                        vm.update(task.id) { it.copy(priority = value) }; onDismiss()
                    }
                }
            }
            HorizontalDivider(color = p.divider, modifier = Modifier.padding(vertical = 8.dp))
            Box {
                ActionRow(Icons.AutoMirrored.Outlined.DriveFileMove, "Move to") { moving = true }
                ProjectMenu(moving, data.projects, task.projectId, { pid -> vm.update(task.id) { it.copy(projectId = pid) }; onDismiss() }, { moving = false })
            }
            if (task.done && task.wontDo) ActionRow(Icons.Outlined.Replay, "Reopen") { vm.toggleWontDo(task); onDismiss() }
            else ActionRow(Icons.Outlined.DoNotDisturbOn, "Won't do") { vm.toggleWontDo(task); onDismiss() }
            ActionRow(Icons.Outlined.ContentCopy, "Duplicate") { vm.duplicate(task); onDismiss() }
            if (vm.agentsAvailable()) {
                HorizontalDivider(color = p.divider, modifier = Modifier.padding(vertical = 8.dp))
                for (a in AGENTS) ActionRow(Icons.Outlined.SmartToy, "Send to ${agentName(a)}") {
                    onDismiss()
                    vm.agentSend.value = task.id to a
                }
            }
            vm.webLink(task)?.let { link ->
                ActionRow(Icons.Outlined.Link, "Copy link") {
                    clipboard.setText(AnnotatedString(link))
                    vm.message.value = "Link copied"
                    onDismiss()
                }
            }
            HorizontalDivider(color = p.divider, modifier = Modifier.padding(vertical = 8.dp))
            ActionRow(Icons.Outlined.Delete, "Delete", color = PrioHigh) {
                vm.delete(task)
                vm.snack.value = Snack("Deleted \"${task.title}\"", "Undo") { vm.undoDelete() }
                onDismiss()
            }
        }
    }
    if (pickDate) {
        DueDateSheet(task.dueDate, { d -> vm.update(task.id) { it.copy(dueDate = d) }; pickDate = false; onDismiss() }, { pickDate = false }, repeatLabel = if (task.isRepeating) "Repeats" else null)
    }
}

@Composable
private fun SectionLabel(text: String) {
    Text(text, fontSize = 13.sp, color = LocalExtra.current.textTertiary, modifier = Modifier.padding(start = 22.dp, top = 14.dp, bottom = 4.dp))
}

@Composable
private fun IconAction(icon: ImageVector?, label: String, tint: Color? = null, selected: Boolean = false, text: String? = null, onClick: () -> Unit) {
    val p = LocalExtra.current
    Column(
        Modifier.width(62.dp).clip(RoundedCornerShape(12.dp)).background(if (selected) p.selected else Color.Transparent)
            .clickable(onClick = onClick).padding(vertical = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(Modifier.size(24.dp), contentAlignment = Alignment.Center) {
            if (icon != null) Icon(icon, null, tint = tint ?: p.icon)
            else Text(text.orEmpty(), fontSize = 13.sp, fontWeight = FontWeight.Bold, color = p.icon)
        }
        Spacer(Modifier.height(4.dp))
        Text(label, fontSize = 11.sp, color = p.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
fun ActionRow(icon: ImageVector, label: String, color: Color? = null, trailing: String? = null, onClick: () -> Unit) {
    val p = LocalExtra.current
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onClick).heightIn(min = 50.dp).padding(horizontal = 22.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, null, tint = color ?: p.icon, modifier = Modifier.size(22.dp))
        Spacer(Modifier.width(18.dp))
        Text(label, fontSize = 16.sp, color = color ?: p.text, modifier = Modifier.weight(1f))
        if (trailing != null) Text(trailing, fontSize = 14.sp, color = p.textTertiary)
    }
}

/**
 * Hands a task to Claude or Instinct, like the web app's Send dialog: pick the
 * agent, add instructions, optionally preview the prompt, send.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AgentSendSheet(vm: MainViewModel, taskId: Long, initialAgent: String, onDismiss: () -> Unit) {
    val data by vm.data.collectAsState()
    val task = data.tasks.firstOrNull { it.id == taskId }
    val p = LocalExtra.current
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var agent by remember { mutableStateOf(initialAgent) }
    var extra by remember { mutableStateOf("") }
    var config by remember { mutableStateOf<AgentConfig?>(null) }
    var configError by remember { mutableStateOf<String?>(null) }
    var showPreview by remember { mutableStateOf(false) }
    var preview by remember { mutableStateOf<String?>(null) }
    var sending by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        runCatching { vm.agentConfig() }
            .onSuccess { config = it }
            .onFailure { configError = "Connect to the internet to send tasks to an agent." }
    }
    LaunchedEffect(showPreview, extra) {
        if (!showPreview) return@LaunchedEffect
        delay(400)
        preview = runCatching { vm.agentPrompt(taskId, extra.trim()) }.getOrElse { "Couldn't load the prompt: ${it.message}" }
    }

    val ready = config?.ready(agent) == true
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Column(Modifier.navigationBarsPadding().verticalScroll(rememberScrollState()).padding(horizontal = 22.dp).padding(bottom = 16.dp)) {
            Text("Send to an agent", fontSize = 18.sp, fontWeight = FontWeight.SemiBold, color = p.text)
            Text(task?.title.orEmpty(), fontSize = 13.sp, color = p.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 2.dp, bottom = 12.dp))
            Row(Modifier.fillMaxWidth().clip(RoundedCornerShape(10.dp)).background(p.page).padding(3.dp)) {
                for (a in AGENTS) {
                    Box(
                        Modifier.weight(1f).clip(RoundedCornerShape(8.dp)).background(if (a == agent) p.card else Color.Transparent)
                            .clickable { agent = a }.padding(vertical = 8.dp),
                        contentAlignment = Alignment.Center,
                    ) { Text(agentName(a), fontSize = 14.sp, color = if (a == agent) p.text else p.textSecondary, fontWeight = if (a == agent) FontWeight.SemiBold else null) }
                }
            }
            val note = when {
                configError != null -> configError
                config == null -> null
                ready -> null
                agent == "instinct" && config!!.instinctEmail.isNotBlank() && !config!!.mailerEnabled -> "The server can't send email, so tasks can't reach Instinct."
                else -> "${agentName(agent)} isn't set up yet. Set it up on the web app under Settings → AI features."
            }
            if (note != null) Text(note, fontSize = 13.sp, color = PrioHigh, modifier = Modifier.padding(top = 10.dp))
            OutlinedTextField(
                value = extra, onValueChange = { extra = it },
                label = { Text("Extra instructions (optional)") },
                minLines = 3, modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
            )
            TextButton(onClick = { showPreview = !showPreview }, contentPadding = androidx.compose.foundation.layout.PaddingValues(0.dp)) {
                Text(if (showPreview) "Hide prompt" else "Preview prompt", color = Accent)
            }
            if (showPreview) {
                Text(
                    preview ?: "…", fontSize = 12.sp, fontFamily = FontFamily.Monospace, color = p.textSecondary,
                    modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(8.dp)).background(p.page).padding(10.dp).heightIn(max = 260.dp).verticalScroll(rememberScrollState()),
                )
            }
            Row(Modifier.fillMaxWidth().padding(top = 12.dp), horizontalArrangement = Arrangement.End, verticalAlignment = Alignment.CenterVertically) {
                TextButton(onClick = onDismiss) { Text("Cancel", color = p.textSecondary) }
                Spacer(Modifier.width(8.dp))
                Button(
                    enabled = ready && !sending,
                    colors = ButtonDefaults.buttonColors(containerColor = Accent),
                    onClick = {
                        sending = true
                        scope.launch {
                            try {
                                val d = vm.sendToAgent(taskId, agent, extra.trim())
                                if (d.status == "failed") {
                                    vm.message.value = "Couldn't send to ${agentName(agent)}: ${d.error}"
                                } else {
                                    val url = d.sessionUrl
                                    vm.snack.value = if (url.isBlank()) Snack("Sent to ${agentName(agent)}")
                                    else Snack("Sent to ${agentName(agent)}", "Open session") {
                                        runCatching { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url))) }
                                    }
                                }
                                onDismiss()
                            } catch (e: Exception) {
                                vm.message.value = e.message ?: "Sending failed"
                            } finally {
                                sending = false
                            }
                        }
                    },
                ) {
                    Icon(Icons.AutoMirrored.Outlined.Send, null, modifier = Modifier.size(16.dp))
                    Spacer(Modifier.width(6.dp))
                    Text("Send to ${agentName(agent)}")
                }
            }
        }
    }
}
