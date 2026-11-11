package org.opentick.android.ui.matrix

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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.TaskEntity
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.OtFab
import org.opentick.android.ui.components.PageHeader
import org.opentick.android.ui.components.QuickAddSheet
import org.opentick.android.ui.components.RomanBadge
import org.opentick.android.ui.components.TaskCheckbox
import org.opentick.android.ui.components.dueColor
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.Q1
import org.opentick.android.ui.theme.Q2
import org.opentick.android.ui.theme.Q3
import org.opentick.android.ui.theme.Q4
import org.opentick.android.util.formatDay
import org.opentick.android.util.toLocalDate

private data class Quadrant(val roman: String, val title: String, val color: Color, val priority: Int)

// TickTick's layout: I and III on top, II and IV below. The quadrant follows
// the task's priority, like the web app.
private val ROWS = listOf(
    listOf(Quadrant("I", "Urgent & Important", Q1, 3), Quadrant("III", "Urgent & Unimportant", Q3, 1)),
    listOf(Quadrant("II", "Not Urgent & Important", Q2, 2), Quadrant("IV", "Not Urgent & Unimportant", Q4, 0)),
)

@Composable
fun MatrixScreen(vm: MainViewModel) {
    val data by vm.data.collectAsState()
    val showCompleted by vm.prefs.showCompleted.collectAsState()
    var adding by remember { mutableStateOf(false) }
    var menu by remember { mutableStateOf(false) }
    val p = LocalExtra.current
    Box(Modifier.fillMaxSize().background(p.page)) {
        Column(Modifier.fillMaxSize()) {
            PageHeader("Eisenhower Matrix", actions = {
                Box {
                    IconButton(onClick = { menu = true }) { Icon(Icons.Outlined.MoreVert, "More", tint = p.icon, modifier = Modifier.size(26.dp)) }
                    DropdownMenu(menu, { menu = false }, shape = RoundedCornerShape(20.dp), containerColor = p.card) {
                        DropdownMenuItem(
                            text = { Text(if (showCompleted) "Hide Completed" else "Show Completed") },
                            leadingIcon = { Icon(Icons.Outlined.CheckCircle, null) },
                            onClick = { vm.prefs.setShowCompleted(!showCompleted); menu = false },
                        )
                    }
                }
            })
            Column(Modifier.fillMaxSize().padding(start = 11.dp, end = 11.dp, top = 4.dp, bottom = 11.dp), verticalArrangement = Arrangement.spacedBy(11.dp)) {
                for (row in ROWS) {
                    Row(Modifier.weight(1f), horizontalArrangement = Arrangement.spacedBy(11.dp)) {
                        for (q in row) {
                            val tasks = data.tasks.filter { minOf(it.priority, 3) == q.priority && (!it.done || showCompleted && it.doneAt?.toLocalDate() == org.opentick.android.util.today()) }
                                .sortedWith(compareBy<TaskEntity> { it.done }.thenBy { it.dueDate ?: Long.MAX_VALUE }.thenBy { it.created })
                            QuadrantCard(q, tasks, vm, Modifier.weight(1f))
                        }
                    }
                }
            }
        }
        OtFab({ adding = true }, Modifier.align(Alignment.BottomEnd).padding(end = 22.dp, bottom = 22.dp))
    }
    if (adding) {
        QuickAddSheet(
            projects = data.projects, labels = data.labels, defaultProject = null, defaultDate = null, defaultLabel = null,
            onAdd = { vm.quickAdd(it.text, it.projectId, it.due, it.datePicked, it.priority, it.labels, it.description) },
            onDismiss = { adding = false },
        )
    }
}

@Composable
private fun QuadrantCard(q: Quadrant, tasks: List<TaskEntity>, vm: MainViewModel, modifier: Modifier) {
    val p = LocalExtra.current
    Column(modifier.fillMaxHeight().clip(RoundedCornerShape(20.dp)).background(p.card)) {
        Row(Modifier.fillMaxWidth().padding(start = 11.dp, end = 10.dp, top = 13.dp, bottom = 3.dp), verticalAlignment = Alignment.CenterVertically) {
            RomanBadge(q.roman, q.color)
            Spacer(Modifier.width(5.dp))
            Text(q.title, color = q.color, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        LazyColumn(contentPadding = PaddingValues(bottom = 8.dp)) {
            items(tasks, key = { it.id }) { t ->
                Row(
                    Modifier.fillMaxWidth().clickable { vm.openTaskId.value = t.id }.padding(end = 8.dp),
                    verticalAlignment = Alignment.Top,
                ) {
                    TaskCheckbox(t.done, t.priority, { vm.toggleDone(t) }, Modifier.padding(start = 3.dp), boxSize = 17.dp, touch = 14.dp)
                    Column(Modifier.weight(1f).padding(top = 6.dp, bottom = 5.dp)) {
                        Text(t.title, fontSize = 13.sp, lineHeight = 16.sp, color = if (t.done) p.textTertiary else p.text, maxLines = 2, overflow = TextOverflow.Ellipsis)
                        t.dueDate?.let { Text(formatDay(it.toLocalDate()), fontSize = 10.sp, lineHeight = 13.sp, color = dueColor(t)) }
                    }
                }
            }
        }
    }
}
