package org.opentick.android.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Flag
import androidx.compose.material.icons.outlined.CalendarToday
import androidx.compose.material.icons.outlined.Flag
import androidx.compose.material.icons.outlined.Inbox
import androidx.compose.material.icons.outlined.Tag
import androidx.compose.material.icons.rounded.ArrowUpward
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
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
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.TextStyle
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
import org.opentick.android.util.formatDueLong
import org.opentick.android.util.splitEmoji
import java.time.LocalDate

data class QuickAddResult(
    val text: String,
    val projectId: Long?,
    val due: Long?,
    val datePicked: Boolean,
    val priority: Int,
    val labels: List<String>,
    val description: String = "",
)

/** TickTick's add-task sheet: title, description, then date, priority, tags and list. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun QuickAddSheet(
    projects: List<ProjectEntity>,
    labels: List<LabelEntity>,
    defaultProject: Long?,
    defaultDate: LocalDate?,
    defaultLabel: String?,
    defaultPriority: Int = 0,
    defaultDue: Long? = null,
    onAdd: (QuickAddResult) -> Unit,
    onDismiss: () -> Unit,
) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var text by remember { mutableStateOf("") }
    var desc by remember { mutableStateOf("") }
    var projectId by remember { mutableStateOf(defaultProject) }
    var due by remember { mutableStateOf(defaultDue ?: defaultDate?.let { allDayMillis(it) }) }
    var datePicked by remember { mutableStateOf(defaultDue != null) }
    var priority by remember { mutableStateOf(defaultPriority) }
    var tags by remember { mutableStateOf(listOfNotNull(defaultLabel)) }
    var showDate by remember { mutableStateOf(false) }
    var showPrio by remember { mutableStateOf(false) }
    var showList by remember { mutableStateOf(false) }
    var showTags by remember { mutableStateOf(false) }
    val focus = remember { FocusRequester() }
    val p = LocalExtra.current

    val parsed = remember(text) { if (text.isBlank()) null else QuickAddParser.parse(text) }
    val effectiveDue = if (datePicked) due else parsed?.date?.let { d -> parsed.time?.let { atTimeMillis(d, it) } ?: allDayMillis(d) } ?: due
    val effectivePrio = if (priority == 0) parsed?.priority ?: 0 else priority

    fun submit() {
        if (text.isBlank()) return
        onAdd(QuickAddResult(text, projectId, due, datePicked, priority, tags, desc.trim()))
        // Stay open for the next task, like TickTick.
        text = ""
        desc = ""
    }

    LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheet,
        dragHandle = null,
        containerColor = p.card,
        shape = RoundedCornerShape(topStart = 20.dp, topEnd = 20.dp),
    ) {
        Column(Modifier.imePadding().padding(top = 18.dp, bottom = 6.dp)) {
            val fieldStyle = TextStyle(fontSize = 17.sp, color = p.text)
            Box(Modifier.padding(horizontal = 20.dp)) {
                if (text.isEmpty()) Text("What would you like to do?", fontSize = 17.sp, color = p.textTertiary)
                BasicTextField(
                    value = text, onValueChange = { text = it }, textStyle = fieldStyle, cursorBrush = SolidColor(Accent),
                    modifier = Modifier.fillMaxWidth().focusRequester(focus),
                    keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences, imeAction = ImeAction.Send),
                    keyboardActions = KeyboardActions(onSend = { submit() }),
                    maxLines = 4,
                )
            }
            Box(Modifier.padding(start = 20.dp, end = 20.dp, top = 10.dp)) {
                if (desc.isEmpty()) Text("Description", fontSize = 14.sp, color = p.textTertiary)
                BasicTextField(
                    value = desc, onValueChange = { desc = it }, textStyle = TextStyle(fontSize = 14.sp, color = p.textSecondary),
                    cursorBrush = SolidColor(Accent), modifier = Modifier.fillMaxWidth(), maxLines = 4,
                )
            }
            val chips = buildList {
                (tags + parsed?.labels.orEmpty()).distinct().forEach { add("#$it" to Accent) }
                parsed?.list?.let { add("~$it" to Accent) }
            }
            if (chips.isNotEmpty()) {
                LazyRow(Modifier.padding(start = 20.dp, end = 20.dp, top = 10.dp), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    items(chips) { (label, color) ->
                        Text(
                            label, fontSize = 12.sp, color = color,
                            modifier = Modifier.clip(RoundedCornerShape(10.dp)).background(color.copy(alpha = 0.12f)).padding(horizontal = 8.dp, vertical = 3.dp),
                        )
                    }
                }
            }
            Row(Modifier.fillMaxWidth().padding(start = 8.dp, end = 12.dp, top = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                Row(
                    Modifier.clip(RoundedCornerShape(8.dp)).clickable { showDate = true }.padding(horizontal = 10.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(Icons.Outlined.CalendarToday, "Date", tint = if (effectiveDue != null) Accent else p.textSecondary, modifier = Modifier.size(22.dp))
                    if (effectiveDue != null) {
                        Spacer(Modifier.width(6.dp))
                        Text(formatDueLong(effectiveDue), fontSize = 14.sp, color = Accent)
                    }
                }
                Box {
                    IconButton(onClick = { showPrio = true }) {
                        Icon(
                            if (effectivePrio > 0) Icons.Filled.Flag else Icons.Outlined.Flag, "Priority",
                            tint = priorityColor(effectivePrio, p.textSecondary), modifier = Modifier.size(23.dp),
                        )
                    }
                    PriorityMenu(showPrio, effectivePrio, { priority = it }, { showPrio = false })
                }
                Box {
                    IconButton(onClick = { showTags = true }) { Icon(Icons.Outlined.Tag, "Tags", tint = if (tags.isNotEmpty()) Accent else p.textSecondary, modifier = Modifier.size(23.dp)) }
                    LabelMenu(showTags, labels, tags, { tags = it }, { showTags = false })
                }
                Box {
                    val proj = projects.firstOrNull { it.id == projectId } ?: projects.firstOrNull { it.isInbox }
                    Row(
                        Modifier.clip(RoundedCornerShape(8.dp)).clickable { showList = true }.padding(horizontal = 10.dp, vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        if (proj == null || proj.isInbox) Icon(Icons.Outlined.Inbox, "List", tint = p.textSecondary, modifier = Modifier.size(22.dp))
                        else ListIcon(proj.title, 20.dp, p.textSecondary)
                        Spacer(Modifier.width(5.dp))
                        Text(parsed?.list ?: splitEmoji(proj?.title ?: "Inbox").second, fontSize = 13.sp, color = p.textSecondary, maxLines = 1)
                    }
                    ProjectMenu(showList, projects, projectId ?: projects.firstOrNull { it.isInbox }?.id, { projectId = it }, { showList = false })
                }
                Spacer(Modifier.weight(1f))
                Box(
                    Modifier.size(36.dp).clip(CircleShape).background(if (text.isNotBlank()) Accent else p.divider).clickable(enabled = text.isNotBlank()) { submit() },
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(Icons.Rounded.ArrowUpward, "Add", tint = Color.White, modifier = Modifier.size(22.dp))
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
