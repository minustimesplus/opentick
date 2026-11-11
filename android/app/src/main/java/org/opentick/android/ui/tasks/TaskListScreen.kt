package org.opentick.android.ui.tasks

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.Sort
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.CloudOff
import androidx.compose.material.icons.outlined.Menu
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Sync
import androidx.compose.material.icons.outlined.TaskAlt
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import org.opentick.android.data.SyncState
import org.opentick.android.data.TaskEntity
import org.opentick.android.ui.Data
import org.opentick.android.ui.ListSel
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.SmartList
import org.opentick.android.ui.components.QuickAddSheet
import org.opentick.android.ui.components.TaskRow
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.util.Section
import org.opentick.android.util.sectionOf

enum class SortMode(val title: String) { DATE("Date"), PRIORITY("Priority"), TITLE("Title") }

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TaskListScreen(vm: MainViewModel, snackbar: SnackbarHostState, onMenu: () -> Unit) {
    val data by vm.data.collectAsState()
    val sel by vm.selection.collectAsState()
    val sync by vm.syncState.collectAsState()
    val showCompleted by vm.prefs.showCompleted.collectAsState()
    var adding by remember { mutableStateOf(false) }
    var searching by rememberSaveable { mutableStateOf(false) }
    var query by rememberSaveable { mutableStateOf("") }
    var sort by rememberSaveable { mutableStateOf(SortMode.DATE) }
    var menu by remember { mutableStateOf(false) }
    val collapsed = remember { mutableStateMapOf<String, Boolean>() }
    val scope = rememberCoroutineScope()

    val all = vm.tasksFor(sel, data)
    val visible = if (searching && query.isNotBlank()) {
        data.tasks.filter { it.title.contains(query, true) || it.description.contains(query, true) }
    } else all
    val showProject = sel !is ListSel.Project
    val isCompletedList = sel == ListSel.Smart(SmartList.COMPLETED)

    fun delete(t: TaskEntity) {
        vm.delete(t)
        scope.launch {
            val r = snackbar.showSnackbar("Task deleted", "Undo", duration = androidx.compose.material3.SnackbarDuration.Short)
            if (r == androidx.compose.material3.SnackbarResult.ActionPerformed) vm.undoDelete()
        }
    }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            if (searching) {
                TopAppBar(
                    navigationIcon = { IconButton(onClick = { searching = false; query = "" }) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back") } },
                    title = {
                        TextField(
                            query, { query = it }, placeholder = { Text("Search tasks") }, singleLine = true,
                            colors = TextFieldDefaults.colors(
                                focusedContainerColor = Color.Transparent, unfocusedContainerColor = Color.Transparent,
                                focusedIndicatorColor = Color.Transparent, unfocusedIndicatorColor = Color.Transparent,
                            ),
                        )
                    },
                )
            } else {
                TopAppBar(
                    colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background),
                    navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Outlined.Menu, "Lists") } },
                    title = {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(vm.title(sel, data), fontWeight = FontWeight.Bold, fontSize = 20.sp)
                            Spacer(Modifier.size(8.dp))
                            SyncIndicator(sync)
                        }
                    },
                    actions = {
                        IconButton(onClick = { searching = true }) { Icon(Icons.Outlined.Search, "Search") }
                        Box {
                            IconButton(onClick = { menu = true }) { Icon(Icons.Outlined.MoreVert, "More") }
                            DropdownMenu(menu, { menu = false }) {
                                DropdownMenuItem(
                                    text = { Text(if (showCompleted) "Hide completed" else "Show completed") },
                                    leadingIcon = { Icon(Icons.Outlined.CheckCircle, null) },
                                    onClick = { vm.prefs.setShowCompleted(!showCompleted); menu = false },
                                )
                                for (m in SortMode.entries) {
                                    DropdownMenuItem(
                                        text = { Text("Sort by ${m.title}") },
                                        leadingIcon = { Icon(Icons.AutoMirrored.Outlined.Sort, null, tint = if (sort == m) Accent else LocalExtra.current.textSecondary) },
                                        onClick = { sort = m; menu = false },
                                    )
                                }
                                DropdownMenuItem(
                                    text = { Text("Sync now") },
                                    leadingIcon = { Icon(Icons.Outlined.Sync, null) },
                                    onClick = { vm.sync(); menu = false },
                                )
                            }
                        }
                    },
                )
            }
        },
        floatingActionButton = {
            if (!isCompletedList) {
                FloatingActionButton(onClick = { adding = true }, containerColor = Accent, contentColor = Color.White, shape = CircleShape) {
                    Icon(Icons.Filled.Add, "Add task")
                }
            }
        },
    ) { pad ->
        PullToRefreshBox(
            isRefreshing = sync is SyncState.Running,
            onRefresh = { vm.sync() },
            modifier = Modifier.padding(pad).fillMaxSize(),
        ) {
            val open = visible.filter { !it.done }
            val done = visible.filter { it.done }.sortedByDescending { it.doneAt ?: it.updated }
            if (visible.isEmpty()) {
                EmptyState(if (searching) "No matching tasks" else "No tasks here", if (searching) "" else "Tap + to add one. Try \"Call mom tomorrow 5pm #family !high\".")
            } else {
                LazyColumn(Modifier.fillMaxSize(), contentPadding = androidx.compose.foundation.layout.PaddingValues(bottom = 96.dp)) {
                    if (isCompletedList) {
                        items(done, key = { it.id }) { t -> TaskItem(vm, data, t, showProject, ::delete) }
                    } else {
                        val groups: List<Pair<String, List<TaskEntity>>> = when (sort) {
                            SortMode.DATE -> open.groupBy { sectionOf(it.dueDate) }.toSortedMap().map { (s, l) -> s.title to sortTasks(l, sort) }
                            SortMode.PRIORITY -> open.groupBy { minOf(it.priority, 3) }.toSortedMap(compareByDescending { it })
                                .map { (p, l) -> listOf("No Priority", "Low Priority", "Medium Priority", "High Priority")[p] to sortTasks(l, sort) }
                            SortMode.TITLE -> listOf("" to sortTasks(open, sort))
                        }
                        val single = groups.size == 1 && (sel as? ListSel.Smart)?.kind in setOf(SmartList.TODAY, SmartList.TOMORROW)
                        for ((title, list) in groups) {
                            val isCollapsed = collapsed[title] == true
                            if (title.isNotEmpty() && !single) {
                                item(key = "h-$title") {
                                    SectionHeader(title, list.size, isCollapsed, color = if (title == Section.OVERDUE.title) PrioHigh else null) {
                                        collapsed[title] = !isCollapsed
                                    }
                                }
                            }
                            if (!isCollapsed) items(list, key = { it.id }) { t -> TaskItem(vm, data, t, showProject, ::delete) }
                        }
                        if (showCompleted && done.isNotEmpty()) {
                            val isCollapsed = collapsed["done"] != false
                            item(key = "h-done") { SectionHeader("Completed", done.size, isCollapsed) { collapsed["done"] = !isCollapsed } }
                            if (!isCollapsed) items(done, key = { it.id }) { t -> TaskItem(vm, data, t, showProject, ::delete) }
                        }
                    }
                }
            }
        }
    }

    if (adding) {
        val (proj, date, label) = vm.defaultsFor(sel)
        QuickAddSheet(
            projects = data.projects,
            labels = data.labels,
            defaultProject = proj,
            defaultDate = date,
            defaultLabel = label?.let { data.labelMap[it]?.title },
            onAdd = { vm.quickAdd(it.text, it.projectId, it.due, it.datePicked, it.priority, it.labels) },
            onDismiss = { adding = false },
        )
    }
}

private fun sortTasks(list: List<TaskEntity>, sort: SortMode): List<TaskEntity> = when (sort) {
    SortMode.DATE -> list.sortedWith(compareBy<TaskEntity> { it.dueDate ?: Long.MAX_VALUE }.thenByDescending { it.priority }.thenBy { it.created })
    SortMode.PRIORITY -> list.sortedWith(compareByDescending<TaskEntity> { it.priority }.thenBy { it.dueDate ?: Long.MAX_VALUE })
    SortMode.TITLE -> list.sortedBy { it.title.lowercase() }
}

@Composable
private fun TaskItem(vm: MainViewModel, data: Data, t: TaskEntity, showProject: Boolean, onDelete: (TaskEntity) -> Unit) {
    TaskRow(
        task = t,
        project = data.projectMap[t.projectId],
        labels = t.labelIdList.mapNotNull { data.labelMap[it] },
        showProject = showProject,
        onToggle = { vm.toggleDone(t) },
        onClick = { vm.openTaskId.value = t.id },
        onDelete = { onDelete(t) },
    )
}

@Composable
fun SectionHeader(title: String, count: Int, collapsed: Boolean, color: Color? = null, onClick: () -> Unit) {
    val extra = LocalExtra.current
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onClick).padding(start = 16.dp, end = 16.dp, top = 14.dp, bottom = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(title, fontWeight = FontWeight.SemiBold, fontSize = 14.sp, color = color ?: MaterialTheme.colorScheme.onBackground)
        Spacer(Modifier.size(6.dp))
        Text("$count", fontSize = 13.sp, color = extra.textTertiary)
        Spacer(Modifier.weight(1f))
        Icon(if (collapsed) Icons.Filled.KeyboardArrowRight else Icons.Filled.KeyboardArrowDown, null, tint = extra.textTertiary, modifier = Modifier.size(20.dp))
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
    Column(Modifier.fillMaxSize().padding(32.dp), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
        Box(Modifier.size(96.dp).background(Accent.copy(alpha = 0.08f), CircleShape), contentAlignment = Alignment.Center) {
            Icon(Icons.Outlined.TaskAlt, null, tint = Accent.copy(alpha = 0.7f), modifier = Modifier.size(48.dp))
        }
        Spacer(Modifier.height(16.dp))
        Text(title, fontWeight = FontWeight.SemiBold, color = LocalExtra.current.textSecondary)
        if (hint.isNotEmpty()) {
            Spacer(Modifier.height(6.dp))
            Text(hint, fontSize = 13.sp, color = LocalExtra.current.textTertiary, textAlign = androidx.compose.ui.text.style.TextAlign.Center)
        }
    }
}

