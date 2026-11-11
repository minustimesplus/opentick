package org.opentick.android.ui.tasks

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.Sort
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.CloudOff
import androidx.compose.material.icons.outlined.KeyboardArrowDown
import androidx.compose.material.icons.outlined.KeyboardArrowRight
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.Sync
import androidx.compose.material.icons.outlined.TaskAlt
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.SnackbarDuration
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.SnackbarResult
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import org.opentick.android.data.SyncState
import org.opentick.android.data.TaskEntity
import org.opentick.android.ui.Data
import org.opentick.android.ui.GroupMode
import org.opentick.android.ui.ListSel
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.SmartList
import org.opentick.android.ui.components.OtFab
import org.opentick.android.ui.components.PageHeader
import org.opentick.android.ui.components.QuickAddSheet
import org.opentick.android.ui.components.TaskRow
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.Overdue
import org.opentick.android.ui.theme.parseHex
import org.opentick.android.util.Section
import org.opentick.android.util.formatLongDay
import org.opentick.android.util.sectionOf
import org.opentick.android.util.splitEmoji
import org.opentick.android.util.toLocalDate

/** One card of tasks: a list, a date section, a priority, or everything. */
private data class Group(val key: String, val title: String?, val emoji: String?, val color: Color?, val titleColor: Color?, val tasks: List<TaskEntity>)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TaskListScreen(vm: MainViewModel, snackbar: SnackbarHostState, onMenu: () -> Unit) {
    val data by vm.data.collectAsState()
    val sel by vm.selection.collectAsState()
    val sync by vm.syncState.collectAsState()
    val showCompleted by vm.prefs.showCompleted.collectAsState()
    var adding by remember { mutableStateOf(false) }
    var groupOverride by rememberSaveable { mutableStateOf<GroupMode?>(null) }
    var menu by remember { mutableStateOf(false) }
    val collapsed = remember { mutableStateMapOf<String, Boolean>() }
    val scope = rememberCoroutineScope()
    val p = LocalExtra.current

    val all = vm.tasksFor(sel, data)
    val mode = groupOverride ?: vm.groupMode(sel)
    val isCompletedList = sel == ListSel.Smart(SmartList.COMPLETED)

    fun delete(t: TaskEntity) {
        vm.delete(t)
        scope.launch {
            if (snackbar.showSnackbar("Task deleted", "Undo", duration = SnackbarDuration.Short) == SnackbarResult.ActionPerformed) vm.undoDelete()
        }
    }

    Box(Modifier.fillMaxSize().background(p.page)) {
        Column(Modifier.fillMaxSize()) {
            PageHeader(
                vm.title(sel, data),
                onMenu = onMenu,
                titleExtra = { Spacer(Modifier.width(8.dp)); SyncIndicator(sync) },
                actions = {
                    Box {
                        IconButton(onClick = { menu = true }) { Icon(Icons.Outlined.MoreVert, "More", tint = p.icon, modifier = Modifier.size(26.dp)) }
                        DropdownMenu(menu, { menu = false }, shape = RoundedCornerShape(20.dp), containerColor = p.card) {
                            DropdownMenuItem(
                                text = { Text(if (showCompleted) "Hide Completed" else "Show Completed") },
                                leadingIcon = { Icon(Icons.Outlined.CheckCircle, null) },
                                onClick = { vm.prefs.setShowCompleted(!showCompleted); menu = false },
                            )
                            for ((m, name) in listOf(GroupMode.LIST to "Group by List", GroupMode.DATE to "Group by Date", GroupMode.PRIORITY to "Group by Priority", GroupMode.NONE to "No Grouping")) {
                                DropdownMenuItem(
                                    text = { Text(name, color = if (mode == m) Accent else p.text) },
                                    leadingIcon = { Icon(Icons.AutoMirrored.Outlined.Sort, null, tint = if (mode == m) Accent else p.textSecondary) },
                                    onClick = { groupOverride = m; menu = false },
                                )
                            }
                            DropdownMenuItem(text = { Text("Sync") }, leadingIcon = { Icon(Icons.Outlined.Sync, null) }, onClick = { vm.sync(); menu = false })
                        }
                    }
                },
            )
            PullToRefreshBox(isRefreshing = sync is SyncState.Running, onRefresh = { vm.sync() }, modifier = Modifier.fillMaxSize()) {
                val open = all.filter { !it.done }
                val done = all.filter { it.done }.sortedByDescending { it.doneAt ?: it.updated }
                if (all.isEmpty() || (open.isEmpty() && !showCompleted && !isCompletedList)) {
                    EmptyState("No tasks", "Tap + to add one. Try \"Call mom tomorrow 5pm #family !high\".")
                } else {
                    val groups = if (isCompletedList) groupTasks(done, GroupMode.DATE, data, completed = true) else groupTasks(sortFor(sel, open), mode, data)
                    LazyColumn(
                        Modifier.fillMaxSize(),
                        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 110.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        for (g in groups) taskCard(g, collapsed, vm, data, sel, ::delete)
                        if (!isCompletedList && showCompleted && done.isNotEmpty()) {
                            taskCard(Group("done", "Completed", null, null, null, done), collapsed, vm, data, sel, ::delete, defaultCollapsed = true)
                        }
                    }
                }
            }
        }
        if (!isCompletedList) OtFab({ adding = true }, Modifier.align(Alignment.BottomEnd).padding(end = 22.dp, bottom = 22.dp))
    }

    if (adding) {
        val (proj, date, label) = vm.defaultsFor(sel)
        QuickAddSheet(
            projects = data.projects,
            labels = data.labels,
            defaultProject = proj,
            defaultDate = date,
            defaultLabel = label?.let { data.labelMap[it]?.title },
            onAdd = { vm.quickAdd(it.text, it.projectId, it.due, it.datePicked, it.priority, it.labels, it.description) },
            onDismiss = { adding = false },
        )
    }
}

/** Today in TickTick lists the most recent dates first; other lists go by date. */
private fun sortFor(sel: ListSel, tasks: List<TaskEntity>): List<TaskEntity> =
    if (sel == ListSel.Smart(SmartList.TODAY)) {
        tasks.sortedWith(compareByDescending<TaskEntity> { it.dueDate ?: 0 }.thenBy { it.created }.thenBy { it.id })
    } else {
        tasks.sortedWith(compareBy<TaskEntity> { it.dueDate ?: Long.MAX_VALUE }.thenByDescending { it.priority }.thenBy { it.created })
    }

private fun groupTasks(tasks: List<TaskEntity>, mode: GroupMode, d: Data, completed: Boolean = false): List<Group> = when (mode) {
    GroupMode.NONE -> listOf(Group("all", null, null, null, null, tasks))
    GroupMode.LIST -> tasks.groupBy { it.projectId }
        .entries.sortedBy { (pid, _) -> d.projectMap[pid]?.let { if (it.isInbox) -1.0 else it.position } ?: Double.MAX_VALUE }
        .map { (pid, list) ->
            val proj = d.projectMap[pid]
            val (emoji, name) = splitEmoji(proj?.title ?: "Inbox")
            Group("p$pid", name, emoji, proj?.hexColor?.takeIf { it.isNotBlank() }?.let { parseHex(it) }, null, list)
        }
    GroupMode.DATE -> if (completed) {
        tasks.groupBy { (it.doneAt ?: it.updated).toLocalDate() }.entries.sortedByDescending { it.key }
            .map { (day, list) -> Group("d$day", formatLongDay(day), null, null, null, list) }
    } else {
        tasks.groupBy { sectionOf(it.dueDate) }.toSortedMap().map { (s, list) ->
            Group("s${s.name}", s.title, null, null, if (s == Section.OVERDUE) Overdue else null, list)
        }
    }
    GroupMode.PRIORITY -> tasks.groupBy { minOf(it.priority, 3) }.toSortedMap(compareByDescending { it }).map { (pr, list) ->
        Group("pr$pr", listOf("No Priority", "Low", "Medium", "High")[pr], null, null, null, list)
    }
}

private fun LazyListScope.taskCard(
    g: Group,
    collapsed: MutableMap<String, Boolean>,
    vm: MainViewModel,
    data: Data,
    sel: ListSel,
    onDelete: (TaskEntity) -> Unit,
    defaultCollapsed: Boolean = false,
) {
    item(key = "card-${g.key}") {
        val p = LocalExtra.current
        val isCollapsed = collapsed[g.key] ?: defaultCollapsed
        Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(p.card)) {
            if (g.title != null) {
                Row(
                    Modifier.fillMaxWidth().clickable { collapsed[g.key] = !isCollapsed }.padding(start = 18.dp, end = 14.dp, top = 15.dp, bottom = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    if (g.emoji != null) {
                        Text(g.emoji, fontSize = 15.sp)
                        Spacer(Modifier.width(6.dp))
                    }
                    Text(g.title, fontSize = 16.sp, fontWeight = FontWeight.Bold, color = g.titleColor ?: p.text, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                    Text("${g.tasks.size}", fontSize = 14.sp, color = p.count)
                    Spacer(Modifier.width(4.dp))
                    Icon(if (isCollapsed) Icons.Outlined.KeyboardArrowRight else Icons.Outlined.KeyboardArrowDown, null, tint = p.count, modifier = Modifier.size(20.dp))
                }
            } else {
                Spacer(Modifier.height(4.dp))
            }
            if (!isCollapsed) {
                Box {
                    Column(Modifier.padding(bottom = 6.dp)) {
                        for (t in g.tasks) {
                            androidx.compose.runtime.key(t.id) {
                                TaskRow(
                                    task = t,
                                    project = data.projectMap[t.projectId],
                                    labels = t.labelIdList.mapNotNull { data.labelMap[it] },
                                    showProject = sel !is ListSel.Project && g.key.startsWith("p").not(),
                                    onToggle = { vm.toggleDone(t) },
                                    onClick = { vm.openTaskId.value = t.id },
                                    onDelete = { onDelete(t) },
                                )
                            }
                        }
                    }
                    // The list's color runs down the left edge of its card.
                    if (g.color != null) Box(Modifier.matchParentSize().padding(end = 0.dp)) {
                        Box(Modifier.width(3.dp).fillMaxHeight().background(g.color))
                    }
                }
            }
        }
    }
}

@Composable
fun SyncIndicator(state: SyncState) {
    when (state) {
        is SyncState.Running -> CircularProgressIndicator(Modifier.size(14.dp), strokeWidth = 2.dp, color = LocalExtra.current.textTertiary)
        is SyncState.Failed -> Icon(Icons.Outlined.CloudOff, state.message, Modifier.size(16.dp), tint = LocalExtra.current.textTertiary)
        else -> {}
    }
}

@Composable
fun EmptyState(title: String, hint: String) {
    val p = LocalExtra.current
    Column(Modifier.fillMaxSize().padding(32.dp), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
        Box(Modifier.size(110.dp).background(Accent.copy(alpha = 0.08f), CircleShape), contentAlignment = Alignment.Center) {
            Icon(Icons.Outlined.TaskAlt, null, tint = Accent.copy(alpha = 0.6f), modifier = Modifier.size(52.dp))
        }
        Spacer(Modifier.height(18.dp))
        Text(title, fontWeight = FontWeight.SemiBold, fontSize = 16.sp, color = p.textSecondary)
        if (hint.isNotEmpty()) {
            Spacer(Modifier.height(6.dp))
            Text(hint, fontSize = 13.sp, color = p.textTertiary, textAlign = TextAlign.Center)
        }
    }
}

@Composable
fun SectionHeader(title: String, count: Int, collapsed: Boolean, color: Color? = null, onClick: () -> Unit) {
    val p = LocalExtra.current
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onClick).padding(start = 18.dp, end = 14.dp, top = 14.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(title, fontWeight = FontWeight.Bold, fontSize = 16.sp, color = color ?: p.text)
        Spacer(Modifier.weight(1f))
        Text("$count", fontSize = 14.sp, color = p.count)
        Icon(if (collapsed) Icons.Outlined.KeyboardArrowRight else Icons.Outlined.KeyboardArrowDown, null, tint = p.count, modifier = Modifier.size(20.dp))
    }
}
