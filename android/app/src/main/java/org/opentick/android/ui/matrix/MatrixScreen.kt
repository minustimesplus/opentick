package org.opentick.android.ui.matrix

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.TaskEntity
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.QuickAddSheet
import org.opentick.android.ui.components.TaskCheckbox
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.ui.theme.PrioLow
import org.opentick.android.ui.theme.PrioMedium
import org.opentick.android.ui.theme.Q4Green
import org.opentick.android.util.formatDue
import org.opentick.android.util.isOverdue

private data class Quadrant(val roman: String, val title: String, val color: Color, val priority: Int)

// Same mapping as the web app: the quadrant follows the task's priority.
private val QUADRANTS = listOf(
    Quadrant("I", "Urgent & Important", PrioHigh, 3),
    Quadrant("II", "Not Urgent & Important", PrioMedium, 2),
    Quadrant("III", "Urgent & Unimportant", PrioLow, 1),
    Quadrant("IV", "Not Urgent & Unimportant", Q4Green, 0),
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MatrixScreen(vm: MainViewModel) {
    val data by vm.data.collectAsState()
    var addingTo by remember { mutableStateOf<Quadrant?>(null) }
    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background),
                title = { Text("Eisenhower Matrix", fontWeight = FontWeight.Bold) },
            )
        },
    ) { pad ->
        Column(Modifier.padding(pad).fillMaxSize().padding(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            for (row in QUADRANTS.chunked(2)) {
                Row(Modifier.weight(1f), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    for (q in row) {
                        val tasks = data.open.filter { minOf(it.priority, 3) == q.priority }
                            .sortedWith(compareBy<TaskEntity> { it.dueDate ?: Long.MAX_VALUE }.thenBy { it.created })
                        QuadrantCard(q, tasks, vm, Modifier.weight(1f)) { addingTo = q }
                    }
                }
            }
        }
    }
    addingTo?.let { q ->
        QuickAddSheet(
            projects = data.projects, labels = data.labels, defaultProject = null, defaultDate = null, defaultLabel = null,
            defaultPriority = q.priority,
            onAdd = { vm.quickAdd(it.text, it.projectId, it.due, it.datePicked, it.priority, it.labels) },
            onDismiss = { addingTo = null },
        )
    }
}

@Composable
private fun QuadrantCard(q: Quadrant, tasks: List<TaskEntity>, vm: MainViewModel, modifier: Modifier, onAdd: () -> Unit) {
    val extra = LocalExtra.current
    Column(
        modifier.fillMaxHeight().clip(RoundedCornerShape(12.dp)).background(extra.panel),
    ) {
        Row(Modifier.fillMaxWidth().padding(start = 12.dp, top = 4.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(q.roman, color = Color.White, fontSize = 11.sp, fontWeight = FontWeight.Bold,
                modifier = Modifier.clip(RoundedCornerShape(4.dp)).background(q.color).padding(horizontal = 5.dp, vertical = 1.dp))
            Spacer(Modifier.size(6.dp))
            Text(q.title, color = q.color, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
            IconButton(onClick = onAdd, modifier = Modifier.size(36.dp)) { Icon(Icons.Outlined.Add, "Add", tint = extra.textTertiary, modifier = Modifier.size(18.dp)) }
        }
        if (tasks.isEmpty()) {
            Text("No tasks", fontSize = 12.sp, color = extra.textTertiary, modifier = Modifier.padding(12.dp))
        }
        LazyColumn {
            items(tasks, key = { it.id }) { t ->
                Row(
                    Modifier.fillMaxWidth().padding(end = 8.dp).clickable { vm.openTaskId.value = t.id },
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    TaskCheckbox(t.done, t.priority, { vm.toggleDone(t) }, Modifier.size(34.dp))
                    Column(Modifier.weight(1f)) {
                        Text(t.title, fontSize = 13.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
                        t.dueDate?.let {
                            Text(formatDue(it), fontSize = 11.sp, color = if (isOverdue(it)) PrioHigh else extra.textTertiary)
                        }
                    }
                }
            }
        }
    }
}
