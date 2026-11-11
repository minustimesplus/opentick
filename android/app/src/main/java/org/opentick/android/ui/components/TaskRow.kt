package org.opentick.android.ui.components

import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Repeat
import androidx.compose.material.icons.outlined.CloudOff
import androidx.compose.material.icons.outlined.Notes
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxValue
import androidx.compose.material3.Text
import androidx.compose.material3.rememberSwipeToDismissBoxState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.LabelEntity
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.TaskEntity
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.ui.theme.Q4Green
import org.opentick.android.ui.theme.parseHex
import org.opentick.android.ui.theme.priorityColor
import org.opentick.android.util.formatDue
import org.opentick.android.util.isOverdue
import org.opentick.android.util.toLocalDate
import org.opentick.android.util.today

/** TickTick's square checkbox, bordered in the priority color. */
@Composable
fun TaskCheckbox(done: Boolean, priority: Int, onToggle: () -> Unit, modifier: Modifier = Modifier, wontDo: Boolean = false) {
    val extra = LocalExtra.current
    val color = if (done) extra.prioNone else priorityColor(priority, extra.prioNone)
    val fill by animateColorAsState(if (done) color else Color.Transparent, label = "check")
    Box(
        modifier
            .size(40.dp)
            .clip(RoundedCornerShape(8.dp))
            .clickable(onClick = onToggle),
        contentAlignment = Alignment.Center,
    ) {
        Box(
            Modifier
                .size(18.dp)
                .clip(RoundedCornerShape(4.dp))
                .background(if (priority > 0 && !done) color.copy(alpha = 0.12f) else fill)
                .border(1.6.dp, color, RoundedCornerShape(4.dp)),
            contentAlignment = Alignment.Center,
        ) {
            if (done) Icon(if (wontDo) Icons.Filled.Close else Icons.Filled.Check, null, tint = Color.White, modifier = Modifier.size(14.dp))
        }
    }
}

@Composable
fun TaskRow(
    task: TaskEntity,
    project: ProjectEntity?,
    labels: List<LabelEntity>,
    showProject: Boolean,
    onToggle: () -> Unit,
    onClick: () -> Unit,
    onDelete: () -> Unit,
) {
    val state = rememberSwipeToDismissBoxState(
        confirmValueChange = {
            when (it) {
                SwipeToDismissBoxValue.StartToEnd -> { onToggle(); false }
                SwipeToDismissBoxValue.EndToStart -> { onDelete(); true }
                else -> false
            }
        },
        positionalThreshold = { it * 0.35f },
    )
    SwipeToDismissBox(
        state = state,
        backgroundContent = {
            val dir = state.dismissDirection
            val bg = when (dir) {
                SwipeToDismissBoxValue.StartToEnd -> Q4Green
                SwipeToDismissBoxValue.EndToStart -> PrioHigh
                else -> Color.Transparent
            }
            Box(
                Modifier.fillMaxSize().background(bg).padding(horizontal = 20.dp),
                contentAlignment = if (dir == SwipeToDismissBoxValue.StartToEnd) Alignment.CenterStart else Alignment.CenterEnd,
            ) {
                if (dir == SwipeToDismissBoxValue.StartToEnd) Icon(Icons.Filled.Check, "Complete", tint = Color.White)
                if (dir == SwipeToDismissBoxValue.EndToStart) Icon(Icons.Outlined.Delete, "Delete", tint = Color.White)
            }
        },
    ) {
        TaskRowContent(task, project, labels, showProject, onToggle, onClick)
    }
}

@Composable
fun TaskRowContent(
    task: TaskEntity,
    project: ProjectEntity?,
    labels: List<LabelEntity>,
    showProject: Boolean,
    onToggle: () -> Unit,
    onClick: () -> Unit,
) {
    val extra = LocalExtra.current
    Row(
        Modifier
            .fillMaxWidth()
            .background(MaterialTheme.colorScheme.background)
            .clickable(onClick = onClick)
            .padding(start = 6.dp, end = 16.dp, top = 2.dp, bottom = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        TaskCheckbox(task.done, task.priority, onToggle, wontDo = task.wontDo)
        Column(Modifier.weight(1f).padding(vertical = 8.dp)) {
            Text(
                task.title.ifBlank { "Untitled" },
                style = MaterialTheme.typography.bodyLarge,
                color = if (task.done) extra.textTertiary else MaterialTheme.colorScheme.onBackground,
                textDecoration = if (task.done && task.wontDo) TextDecoration.LineThrough else null,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            val hasMeta = labels.isNotEmpty() || task.description.isNotBlank() || task.isRepeating || task.serverId == null || task.dirty
            if (hasMeta && !task.done) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.padding(top = 3.dp)) {
                    if (task.description.isNotBlank()) Icon(Icons.Outlined.Notes, null, Modifier.size(13.dp), tint = extra.textTertiary)
                    if (task.isRepeating) Icon(Icons.Outlined.Repeat, null, Modifier.size(13.dp), tint = extra.textTertiary)
                    for (l in labels.take(3)) LabelChip(l)
                    if (task.dirty) Icon(Icons.Outlined.CloudOff, "Not synced yet", Modifier.size(12.dp), tint = extra.textTertiary)
                }
            }
        }
        Spacer(Modifier.width(8.dp))
        Column(horizontalAlignment = Alignment.End) {
            task.dueDate?.let { due ->
                val overdue = !task.done && isOverdue(due)
                val isToday = due.toLocalDate() == today()
                Text(
                    formatDue(due),
                    fontSize = 12.sp,
                    color = when {
                        task.done -> extra.textTertiary
                        overdue -> PrioHigh
                        isToday -> Accent
                        else -> extra.textTertiary
                    },
                )
            }
            if (showProject && project != null) {
                Text(project.title, fontSize = 11.sp, color = extra.textTertiary, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.width(90.dp), textAlign = androidx.compose.ui.text.style.TextAlign.End)
            }
        }
    }
}

@Composable
fun LabelChip(label: LabelEntity) {
    val c = parseHex(label.hexColor, Accent)
    Text(
        label.title,
        fontSize = 11.sp,
        fontWeight = FontWeight.Medium,
        color = c,
        maxLines = 1,
        modifier = Modifier
            .clip(RoundedCornerShape(10.dp))
            .background(c.copy(alpha = 0.12f))
            .padding(horizontal = 7.dp, vertical = 1.dp),
    )
}
