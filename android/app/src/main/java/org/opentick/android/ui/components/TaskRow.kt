package org.opentick.android.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.outlined.Article
import androidx.compose.material.icons.outlined.CloudOff
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Repeat
import androidx.compose.material3.Icon
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxValue
import androidx.compose.material3.Text
import androidx.compose.material3.rememberSwipeToDismissBoxState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.LabelEntity
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.TaskEntity
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.Green
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.Overdue
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.ui.theme.parseHex
import org.opentick.android.ui.theme.priorityColor
import org.opentick.android.util.formatDue
import org.opentick.android.util.isOverdue

/** TickTick's square checkbox: an outline in the priority color, grey and filled when done. */
@Composable
fun TaskCheckbox(done: Boolean, priority: Int, onToggle: () -> Unit, modifier: Modifier = Modifier, wontDo: Boolean = false, boxSize: Dp = 18.dp, touch: Dp = 22.dp) {
    val p = LocalExtra.current
    val color = if (done) p.prioNone else priorityColor(priority, p.prioNone)
    Box(
        modifier
            .size(boxSize + touch)
            .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null, onClick = onToggle),
        contentAlignment = Alignment.Center,
    ) {
        Box(
            Modifier
                .size(boxSize)
                .clip(RoundedCornerShape(boxSize * 0.2f))
                .background(if (done) color else Color.Transparent)
                .border(boxSize * 0.095f, color, RoundedCornerShape(boxSize * 0.2f)),
            contentAlignment = Alignment.Center,
        ) {
            if (done) Icon(if (wontDo) Icons.Filled.Close else Icons.Filled.Check, null, tint = Color.White, modifier = Modifier.size(boxSize * 0.8f))
        }
    }
}

/** Date color in rows: red when overdue, blue otherwise. */
@Composable
fun dueColor(task: TaskEntity): Color = when {
    task.done -> LocalExtra.current.textTertiary
    task.dueDate != null && isOverdue(task.dueDate) -> Overdue
    else -> Accent
}

/** A row inside a list card, with swipe right to complete and left to delete. */
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
                SwipeToDismissBoxValue.StartToEnd -> Green
                SwipeToDismissBoxValue.EndToStart -> PrioHigh
                else -> Color.Transparent
            }
            Box(
                Modifier.fillMaxSize().background(bg).padding(horizontal = 22.dp),
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
    val p = LocalExtra.current
    Row(
        Modifier
            .fillMaxWidth()
            .background(p.card)
            .clickable(onClick = onClick)
            .heightIn(min = 48.dp)
            .padding(start = 8.dp, end = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        TaskCheckbox(task.done, task.priority, onToggle, wontDo = task.wontDo)
        Spacer(Modifier.width(3.dp))
        Column(Modifier.weight(1f).padding(vertical = 10.dp)) {
            Text(
                task.title.ifBlank { "Untitled" },
                fontSize = 16.sp,
                color = if (task.done) p.textTertiary else p.text,
                textDecoration = if (task.done && task.wontDo) TextDecoration.LineThrough else null,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (labels.isNotEmpty() && !task.done) {
                Row(horizontalArrangement = Arrangement.spacedBy(5.dp), modifier = Modifier.padding(top = 4.dp)) {
                    for (l in labels.take(3)) LabelChip(l)
                }
            }
        }
        Spacer(Modifier.width(8.dp))
        Column(horizontalAlignment = Alignment.End, verticalArrangement = Arrangement.Center) {
            task.dueDate?.let { due -> Text(formatDue(due), fontSize = 13.sp, color = dueColor(task), maxLines = 1) }
            val icons = buildList {
                if (task.isRepeating) add(Icons.Outlined.Repeat)
                if (task.description.isNotBlank()) add(Icons.Outlined.Article)
                if (task.dirty && task.serverId != null) add(Icons.Outlined.CloudOff)
            }
            if (icons.isNotEmpty() && !task.done) {
                Row(horizontalArrangement = Arrangement.spacedBy(4.dp), modifier = Modifier.padding(top = 3.dp)) {
                    for (i in icons) Icon(i, null, tint = p.count, modifier = Modifier.size(15.dp))
                }
            }
            if (showProject && project != null && task.dueDate == null) {
                Text(project.title, fontSize = 12.sp, color = p.textTertiary, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.width(96.dp), textAlign = androidx.compose.ui.text.style.TextAlign.End)
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
