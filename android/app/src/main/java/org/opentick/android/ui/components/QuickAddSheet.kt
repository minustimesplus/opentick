package org.opentick.android.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Flag
import androidx.compose.material.icons.outlined.CalendarToday
import androidx.compose.material.icons.outlined.Flag
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material.icons.outlined.Sell
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.LabelEntity
import org.opentick.android.data.ProjectEntity
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.priorityColor
import org.opentick.android.util.QuickAddParser
import org.opentick.android.util.allDayMillis
import org.opentick.android.util.atTimeMillis
import org.opentick.android.util.formatDue
import java.time.LocalDate

data class QuickAddResult(
    val text: String,
    val projectId: Long?,
    val due: Long?,
    val datePicked: Boolean,
    val priority: Int,
    val labels: List<String>,
)

/** The add-task bar that slides up from the bottom, like TickTick's. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun QuickAddSheet(
    projects: List<ProjectEntity>,
    labels: List<LabelEntity>,
    defaultProject: Long?,
    defaultDate: LocalDate?,
    defaultLabel: String?,
    defaultPriority: Int = 0,
    onAdd: (QuickAddResult) -> Unit,
    onDismiss: () -> Unit,
) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var text by remember { mutableStateOf("") }
    var projectId by remember { mutableStateOf(defaultProject) }
    var due by remember { mutableStateOf(defaultDate?.let { allDayMillis(it) }) }
    var datePicked by remember { mutableStateOf(false) }
    var priority by remember { mutableStateOf(defaultPriority) }
    var tags by remember { mutableStateOf(listOfNotNull(defaultLabel)) }
    var showDate by remember { mutableStateOf(false) }
    var showPrio by remember { mutableStateOf(false) }
    var showList by remember { mutableStateOf(false) }
    var showTags by remember { mutableStateOf(false) }
    val focus = remember { FocusRequester() }
    val extra = LocalExtra.current

    val parsed = remember(text) { if (text.isBlank()) null else QuickAddParser.parse(text) }
    val effectiveDue = if (datePicked) due else parsed?.date?.let { d -> parsed.time?.let { atTimeMillis(d, it) } ?: allDayMillis(d) } ?: due
    val effectivePrio = if (priority == 0) parsed?.priority ?: 0 else priority

    fun submit() {
        if (text.isBlank()) return
        onAdd(QuickAddResult(text, projectId, due, datePicked, priority, tags))
        // Stay open for the next task, like TickTick.
        text = ""
    }

    LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheet,
        dragHandle = null,
        containerColor = MaterialTheme.colorScheme.surface,
        shape = RoundedCornerShape(topStart = 16.dp, topEnd = 16.dp),
    ) {
        Column(Modifier.imePadding().padding(top = 8.dp, bottom = 4.dp)) {
            TextField(
                value = text,
                onValueChange = { text = it },
                placeholder = { Text("What would you like to do?", color = extra.textTertiary) },
                modifier = Modifier.fillMaxWidth().focusRequester(focus),
                colors = TextFieldDefaults.colors(
                    focusedContainerColor = Color.Transparent,
                    unfocusedContainerColor = Color.Transparent,
                    focusedIndicatorColor = Color.Transparent,
                    unfocusedIndicatorColor = Color.Transparent,
                ),
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences, imeAction = ImeAction.Send),
                keyboardActions = KeyboardActions(onSend = { submit() }),
                maxLines = 4,
            )
            val chips = buildList {
                effectiveDue?.let { add(formatDue(it) to Accent) }
                if (effectivePrio > 0) add(PRIORITIES.first { it.first == minOf(effectivePrio, 3) }.second to priorityColor(effectivePrio, extra.prioNone))
                (tags + parsed?.labels.orEmpty()).distinct().forEach { add("#$it" to Accent) }
                parsed?.list?.let { add("~$it" to Accent) }
            }
            if (chips.isNotEmpty()) {
                LazyRow(Modifier.padding(horizontal = 16.dp), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    items(chips) { (label, color) ->
                        Text(
                            label, fontSize = 12.sp, color = color,
                            modifier = Modifier.clip(RoundedCornerShape(10.dp)).background(color.copy(alpha = 0.12f)).padding(horizontal = 8.dp, vertical = 3.dp),
                        )
                    }
                }
            }
            Row(Modifier.fillMaxWidth().padding(horizontal = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                IconButton(onClick = { showDate = true }) {
                    Icon(Icons.Outlined.CalendarToday, "Date", tint = if (effectiveDue != null) Accent else extra.textSecondary)
                }
                Box {
                    IconButton(onClick = { showPrio = true }) {
                        Icon(
                            if (effectivePrio > 0) Icons.Filled.Flag else Icons.Outlined.Flag, "Priority",
                            tint = priorityColor(effectivePrio, extra.textSecondary),
                        )
                    }
                    PriorityMenu(showPrio, effectivePrio, { priority = it }, { showPrio = false })
                }
                Box {
                    IconButton(onClick = { showTags = true }) { Icon(Icons.Outlined.Sell, "Tags", tint = if (tags.isNotEmpty()) Accent else extra.textSecondary) }
                    LabelMenu(showTags, labels, tags, { tags = it }, { showTags = false })
                }
                Box {
                    IconButton(onClick = { showList = true }) { Icon(Icons.Outlined.Folder, "List", tint = extra.textSecondary) }
                    ProjectMenu(showList, projects, projectId ?: projects.firstOrNull { it.isInbox }?.id, { projectId = it }, { showList = false })
                }
                val listName = projects.firstOrNull { it.id == projectId }?.title ?: projects.firstOrNull { it.isInbox }?.title ?: "Inbox"
                Text(parsed?.list ?: listName, fontSize = 12.sp, color = extra.textTertiary, modifier = Modifier.weight(1f), maxLines = 1)
                Spacer(Modifier.size(4.dp))
                IconButton(
                    onClick = { submit() },
                    enabled = text.isNotBlank(),
                    modifier = Modifier.padding(end = 4.dp).size(40.dp).clip(CircleShape)
                        .background(if (text.isNotBlank()) Accent else extra.divider),
                ) {
                    Icon(Icons.AutoMirrored.Filled.Send, "Add", tint = Color.White, modifier = Modifier.size(20.dp))
                }
            }
        }
    }

    if (showDate) {
        DueDateSheet(
            current = effectiveDue,
            onPick = { due = it; datePicked = true; showDate = false },
            onDismiss = { showDate = false },
        )
    }
}
