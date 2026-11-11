package org.opentick.android.ui.tasks

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
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
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.MoreHoriz
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.SyncState
import org.opentick.android.ui.Data
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.TaskRow
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra

/**
 * The kanban layout of a list (web KanbanBoard.vue): one column per section,
 * side by side and scrolled sideways, like TickTick's Android kanban.
 * Sections live on the server, so the board needs a connection; the tasks
 * themselves come from the local database.
 */
@Composable
fun KanbanBoard(vm: MainViewModel, projectId: Long, data: Data, showCompleted: Boolean, addingSection: Boolean, onSectionDialogDone: () -> Unit) {
    val state by vm.kanban.collectAsState()
    val sync by vm.syncState.collectAsState()
    var syncing by remember { mutableStateOf(false) }
    LaunchedEffect(projectId) { vm.loadKanban(projectId) }
    // A finished sync may have moved tasks on another device.
    LaunchedEffect(sync) {
        if (sync is SyncState.Running) syncing = true
        else if (syncing) {
            syncing = false
            vm.loadKanban(projectId)
        }
    }
    val k = state?.takeIf { it.projectId == projectId }

    if (addingSection) {
        TextDialog("Add Section", "Section name", "", "Add", onDismiss = onSectionDialogDone) { vm.addSection(it); onSectionDialogDone() }
    }

    when {
        k == null || (k.loading && k.buckets.isEmpty()) -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            CircularProgressIndicator(color = Accent)
        }
        k.buckets.isEmpty() -> Column(Modifier.fillMaxSize(), horizontalAlignment = Alignment.CenterHorizontally) {
            Box(Modifier.weight(1f)) { EmptyState("No sections", k.error ?: "Add a section from the ⋮ menu.") }
            TextButton(onClick = { vm.loadKanban(projectId) }, modifier = Modifier.padding(bottom = 120.dp)) { Text("Retry", color = Accent) }
        }
        else -> {
            val tasks = data.tasks.filter { it.projectId == projectId && (showCompleted || !it.done) }
            val columns = kanbanColumns(tasks, k.buckets, k.placement, k.defaultBucket, k.doneBucket)
            BoxWithConstraints(Modifier.fillMaxSize()) {
                val width = maxWidth * 0.84f
                LazyRow(
                    Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 96.dp),
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    items(columns, key = { it.bucket.id }) { col -> SectionColumn(vm, data, col, columns, width) }
                }
            }
        }
    }
}

@Composable
private fun SectionColumn(vm: MainViewModel, data: Data, col: KanbanColumn, all: List<KanbanColumn>, width: androidx.compose.ui.unit.Dp) {
    val p = LocalExtra.current
    var menu by remember { mutableStateOf(false) }
    var renaming by remember { mutableStateOf(false) }
    var deleting by remember { mutableStateOf(false) }
    var adding by remember { mutableStateOf(false) }
    var cardMenu by remember { mutableStateOf<Long?>(null) }

    Column(Modifier.width(width).fillMaxHeight().clip(RoundedCornerShape(16.dp)).background(p.card)) {
        Row(Modifier.fillMaxWidth().padding(start = 18.dp, end = 4.dp, top = 6.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(col.bucket.title, fontSize = 16.sp, fontWeight = FontWeight.Bold, color = p.text, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
            Spacer(Modifier.width(8.dp))
            Text("${col.tasks.size}", fontSize = 14.sp, color = p.count)
            Spacer(Modifier.weight(1f))
            IconButton(onClick = { adding = true }) { Icon(Icons.Outlined.Add, "Add task", tint = p.icon, modifier = Modifier.size(22.dp)) }
            Box {
                IconButton(onClick = { menu = true }) { Icon(Icons.Outlined.MoreHoriz, "Section menu", tint = p.icon, modifier = Modifier.size(22.dp)) }
                DropdownMenu(menu, { menu = false }, shape = RoundedCornerShape(16.dp), containerColor = p.card) {
                    DropdownMenuItem(text = { Text("Rename") }, onClick = { menu = false; renaming = true })
                    if (all.size > 1) DropdownMenuItem(text = { Text("Delete", color = MaterialTheme.colorScheme.error) }, onClick = { menu = false; deleting = true })
                }
            }
        }
        LazyColumn(Modifier.fillMaxWidth().weight(1f), contentPadding = PaddingValues(bottom = 8.dp)) {
            items(col.tasks, key = { it.id }) { t ->
                Box {
                    TaskRow(
                        task = t,
                        project = data.projectMap[t.projectId],
                        labels = t.labelIdList.mapNotNull { data.labelMap[it] },
                        showProject = false,
                        onToggle = { vm.toggleDone(t) },
                        onClick = { vm.openTaskId.value = t.id },
                        onDelete = { vm.delete(t) },
                        onLongClick = { cardMenu = t.id },
                    )
                    DropdownMenu(cardMenu == t.id, { cardMenu = null }, shape = RoundedCornerShape(16.dp), containerColor = p.card) {
                        Text("Move to section", fontSize = 13.sp, color = p.textTertiary, modifier = Modifier.padding(horizontal = 16.dp, vertical = 6.dp))
                        for (other in all) if (other.bucket.id != col.bucket.id) {
                            DropdownMenuItem(text = { Text(other.bucket.title) }, onClick = { cardMenu = null; vm.moveToBucket(t, other.bucket.id, other.tasks) })
                        }
                        HorizontalDivider(color = p.divider)
                        DropdownMenuItem(text = { Text("More actions") }, onClick = { cardMenu = null; vm.actionsTaskId.value = t.id })
                    }
                }
            }
            item {
                Row(
                    Modifier.fillMaxWidth().clickable { adding = true }.padding(horizontal = 18.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(Icons.Outlined.Add, null, tint = p.textTertiary, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(8.dp))
                    Text("Add Task", fontSize = 14.sp, color = p.textTertiary)
                }
            }
        }
    }

    if (adding) {
        TextDialog("Add Task", "Add task to \"${col.bucket.title}\"", "", "Add", onDismiss = { adding = false }) {
            vm.addToBucket(it, col.bucket.id, col.tasks)
            adding = false
        }
    }
    if (renaming) {
        TextDialog("Rename Section", "Section name", col.bucket.title, "Save", onDismiss = { renaming = false }) {
            vm.renameSection(col.bucket, it)
            renaming = false
        }
    }
    if (deleting) {
        AlertDialog(
            onDismissRequest = { deleting = false },
            title = { Text("Delete section?") },
            text = { Text("\"${col.bucket.title}\" will be deleted. Its tasks move to the default section.") },
            confirmButton = { TextButton(onClick = { vm.deleteSection(col.bucket); deleting = false }) { Text("Delete", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton(onClick = { deleting = false }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun TextDialog(title: String, placeholder: String, initial: String, confirm: String, onDismiss: () -> Unit, onConfirm: (String) -> Unit) {
    var text by remember { mutableStateOf(initial) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = { OutlinedTextField(text, { text = it }, singleLine = true, placeholder = { Text(placeholder) }, modifier = Modifier.fillMaxWidth()) },
        confirmButton = { TextButton(enabled = text.isNotBlank(), onClick = { onConfirm(text.trim()) }) { Text(confirm) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}
