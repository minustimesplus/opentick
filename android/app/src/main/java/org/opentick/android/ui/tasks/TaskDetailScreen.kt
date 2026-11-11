package org.opentick.android.ui.tasks

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Flag
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.outlined.CalendarToday
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Flag
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.Repeat
import androidx.compose.material.icons.outlined.Sell
import androidx.compose.material.icons.outlined.DoNotDisturbOn
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.flow.debounce
import kotlinx.coroutines.flow.drop
import kotlinx.coroutines.flow.filterNotNull
import kotlinx.coroutines.flow.first
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.DueDateSheet
import org.opentick.android.ui.components.LabelChip
import org.opentick.android.ui.components.LabelMenu
import org.opentick.android.ui.components.PriorityMenu
import org.opentick.android.ui.components.ProjectMenu
import org.opentick.android.ui.components.TaskCheckbox
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.ui.theme.priorityColor
import org.opentick.android.util.formatDue
import org.opentick.android.util.isOverdue

/** Full screen task editor, like TickTick's task detail page. Edits save as you type. */
@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class, FlowPreview::class)
@Composable
fun TaskDetailScreen(vm: MainViewModel, taskId: Long, onClose: () -> Unit) {
    val task by remember(taskId) { vm.repo.task(taskId) }.collectAsState(null)
    val data by vm.data.collectAsState()
    val mode by vm.prefs.mode.collectAsState()
    val extra = LocalExtra.current
    var title by remember { mutableStateOf<String?>(null) }
    var desc by remember { mutableStateOf<String?>(null) }
    var showDate by remember { mutableStateOf(false) }
    var showPrio by remember { mutableStateOf(false) }
    var showList by remember { mutableStateOf(false) }
    var showTags by remember { mutableStateOf(false) }
    var menu by remember { mutableStateOf(false) }

    BackHandler(onBack = onClose)

    // Load the text once, then write edits back after a short pause.
    LaunchedEffect(taskId) {
        val t = vm.repo.task(taskId).filterNotNull().first()
        title = t.title
        desc = t.description
    }
    LaunchedEffect(taskId) {
        androidx.compose.runtime.snapshotFlow { title to desc }.drop(1).debounce(400).collect { (ti, de) ->
            if (ti != null && de != null) vm.update(taskId) { it.copy(title = ti, description = de) }
        }
    }

    val t = task
    if (t == null || t.deleted) {
        // Loaded before and now gone: deleted here or by a sync.
        if (t?.deleted == true || title != null) LaunchedEffect(Unit) { onClose() }
        return
    }
    val project = data.projectMap[t.projectId]
    val tagTitles = t.labelIdList.mapNotNull { data.labelMap[it]?.title }

    Scaffold(
        containerColor = extra.card,
        topBar = {
            TopAppBar(
                colors = TopAppBarDefaults.topAppBarColors(containerColor = extra.card),
                navigationIcon = { IconButton(onClick = onClose) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back") } },
                title = {
                    Box {
                        Row(Modifier.clip(RoundedCornerShape(6.dp)).clickable { showList = true }.padding(6.dp), verticalAlignment = Alignment.CenterVertically) {
                            Text(project?.title ?: "Inbox", fontSize = 16.sp, color = extra.textSecondary, maxLines = 1)
                            Icon(Icons.Filled.KeyboardArrowDown, null, tint = extra.textTertiary)
                        }
                        ProjectMenu(showList, data.projects, t.projectId, { pid -> vm.update(taskId) { it.copy(projectId = pid) } }, { showList = false })
                    }
                },
                actions = {
                    Box {
                        IconButton(onClick = { menu = true }) { Icon(Icons.Outlined.MoreVert, "More") }
                        DropdownMenu(menu, { menu = false }, shape = RoundedCornerShape(20.dp), containerColor = extra.card) {
                            DropdownMenuItem(
                                text = { Text(if (t.wontDo) "Reopen" else "Won't do") },
                                leadingIcon = { Icon(Icons.Outlined.DoNotDisturbOn, null) },
                                onClick = {
                                    menu = false
                                    vm.update(taskId) { if (it.wontDo) it.copy(done = false, wontDo = false, doneAt = null) else it.copy(done = true, wontDo = true, doneAt = System.currentTimeMillis()) }
                                },
                            )
                            DropdownMenuItem(
                                text = { Text("Delete", color = PrioHigh) },
                                leadingIcon = { Icon(Icons.Outlined.Delete, null, tint = PrioHigh) },
                                onClick = { menu = false; vm.delete(t); onClose() },
                            )
                        }
                    }
                },
            )
        },
    ) { pad ->
        Column(Modifier.padding(pad).fillMaxSize().imePadding().verticalScroll(rememberScrollState())) {
            Row(Modifier.fillMaxWidth().padding(horizontal = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                TaskCheckbox(t.done, t.priority, { vm.toggleDone(t) }, wontDo = t.wontDo)
                Row(
                    Modifier.clip(RoundedCornerShape(6.dp)).clickable { showDate = true }.padding(8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    val c = when {
                        t.dueDate == null -> extra.textTertiary
                        !t.done && isOverdue(t.dueDate) -> org.opentick.android.ui.theme.Overdue
                        else -> Accent
                    }
                    Icon(Icons.Outlined.CalendarToday, null, tint = c, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(6.dp))
                    Text(t.dueDate?.let { org.opentick.android.util.formatDueLong(it) } ?: "Date & Reminder", color = c, fontSize = 15.sp)
                    if (t.isRepeating) {
                        Spacer(Modifier.width(6.dp))
                        Icon(Icons.Outlined.Repeat, "Repeats", tint = c, modifier = Modifier.size(16.dp))
                    }
                }
                Spacer(Modifier.weight(1f))
                Box {
                    IconButton(onClick = { showPrio = true }) {
                        Icon(if (t.priority > 0) Icons.Filled.Flag else Icons.Outlined.Flag, "Priority", tint = priorityColor(t.priority, extra.textTertiary))
                    }
                    PriorityMenu(showPrio, t.priority, { p -> vm.update(taskId) { it.copy(priority = p) } }, { showPrio = false })
                }
            }
            HorizontalDivider(color = extra.divider)
            val plain = TextFieldDefaults.colors(
                focusedContainerColor = Color.Transparent, unfocusedContainerColor = Color.Transparent,
                focusedIndicatorColor = Color.Transparent, unfocusedIndicatorColor = Color.Transparent,
            )
            TextField(
                value = title ?: t.title,
                onValueChange = { title = it.replace("\n", " ") },
                textStyle = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.SemiBold, color = if (t.done) extra.textTertiary else MaterialTheme.colorScheme.onBackground),
                placeholder = { Text("What would you like to do?", fontSize = 20.sp) },
                colors = plain,
                modifier = Modifier.fillMaxWidth(),
            )
            TextField(
                value = desc ?: t.description,
                onValueChange = { desc = it },
                placeholder = { Text("Description", color = extra.textTertiary) },
                colors = plain,
                textStyle = TextStyle(fontSize = 15.sp, color = MaterialTheme.colorScheme.onBackground, lineHeight = 22.sp),
                modifier = Modifier.fillMaxWidth(),
                minLines = 4,
            )
            Box(Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.clickable { showTags = true }) {
                    Icon(Icons.Outlined.Sell, "Tags", tint = extra.textTertiary, modifier = Modifier.size(18.dp))
                    if (tagTitles.isEmpty()) Text("Add tags", fontSize = 13.sp, color = extra.textTertiary)
                    for (id in t.labelIdList) data.labelMap[id]?.let { LabelChip(it) }
                }
                LabelMenu(showTags, data.labels, tagTitles, { vm.setLabels(taskId, it) }, { showTags = false })
            }
            if (t.isRepeating) {
                Text(
                    "Repeats. Completing it moves it to the next date once synced.",
                    fontSize = 12.sp, color = extra.textTertiary, modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp),
                )
            }
            Spacer(Modifier.size(24.dp))
            Text(
                when {
                    mode == org.opentick.android.data.AccountMode.LOCAL -> "Saved on this device"
                    t.serverId == null || t.dirty -> "Saved on this device, syncs when online"
                    else -> "Synced"
                },
                fontSize = 12.sp, color = extra.textTertiary,
                modifier = Modifier.padding(horizontal = 16.dp).background(Color.Transparent),
            )
        }
    }

    if (showDate) {
        DueDateSheet(t.dueDate, { d -> vm.update(taskId) { it.copy(dueDate = d) }; showDate = false }, { showDate = false }, repeatLabel = if (t.isRepeating) "Repeats" else null)
    }
}
