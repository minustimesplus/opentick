package org.opentick.android.ui.matrix

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.Inbox
import androidx.compose.material.icons.outlined.KeyboardArrowDown
import androidx.compose.material.icons.outlined.KeyboardArrowRight
import androidx.compose.material.icons.outlined.MoreHoriz
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.RestartAlt
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CheckboxDefaults
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
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
import org.opentick.android.data.MatrixGroup
import org.opentick.android.data.MatrixSort
import org.opentick.android.data.ProjectEntity
import org.opentick.android.data.Views
import org.opentick.android.data.viewKey
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.OtFab
import org.opentick.android.ui.components.PageHeader
import org.opentick.android.ui.components.QuickAddSheet
import org.opentick.android.ui.components.RomanBadge
import org.opentick.android.ui.components.TaskCheckbox
import org.opentick.android.ui.components.dueColor
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.Q1
import org.opentick.android.ui.theme.Q2
import org.opentick.android.ui.theme.Q3
import org.opentick.android.ui.theme.Q4
import org.opentick.android.ui.theme.parseHex
import org.opentick.android.util.formatDay
import org.opentick.android.util.toLocalDate
import org.opentick.android.util.today

private fun colorOf(q: Quadrant) = when (q.id) { 1 -> Q1; 2 -> Q2; 3 -> Q3; else -> Q4 }

/**
 * The Eisenhower matrix. Which lists it shows, completed tasks and each
 * quadrant's grouping and sorting are synced with the web and desktop apps.
 */
@Composable
fun MatrixScreen(vm: MainViewModel) {
    val data by vm.data.collectAsState()
    val views by vm.views.collectAsState()
    var adding by remember { mutableStateOf<Quadrant?>(null) }
    var menu by remember { mutableStateOf(false) }
    var editing by remember { mutableStateOf(false) }
    val p = LocalExtra.current
    val today = today()
    Box(Modifier.fillMaxSize().background(p.page)) {
        Column(Modifier.fillMaxSize()) {
            PageHeader("Eisenhower Matrix", actions = {
                IconButton(onClick = { editing = true }) { Icon(Icons.Outlined.Edit, "Edit matrix", tint = p.icon, modifier = Modifier.size(24.dp)) }
                Box {
                    IconButton(onClick = { menu = true }) { Icon(Icons.Outlined.MoreVert, "More", tint = p.icon, modifier = Modifier.size(26.dp)) }
                    DropdownMenu(menu, { menu = false }, shape = RoundedCornerShape(20.dp), containerColor = p.card) {
                        DropdownMenuItem(
                            text = { Text(if (views.matrixShowDone) "Hide Completed" else "Show Completed") },
                            leadingIcon = { Icon(Icons.Outlined.CheckCircle, null) },
                            onClick = { vm.updateViews { it.withMatrixShowDone(!it.matrixShowDone) }; menu = false },
                        )
                        DropdownMenuItem(
                            text = { Text("Edit") },
                            leadingIcon = { Icon(Icons.Outlined.Edit, null) },
                            onClick = { editing = true; menu = false },
                        )
                    }
                }
            })
            Column(Modifier.fillMaxSize().padding(start = 11.dp, end = 11.dp, top = 4.dp, bottom = 11.dp), verticalArrangement = Arrangement.spacedBy(11.dp)) {
                for (row in QUADRANT_ROWS) {
                    Row(Modifier.weight(1f), horizontalArrangement = Arrangement.spacedBy(11.dp)) {
                        for (q in row) {
                            QuadrantCard(q, matrixGroups(q, data, views, today), views, vm, onAdd = { adding = q }, modifier = Modifier.weight(1f))
                        }
                    }
                }
            }
        }
        OtFab({ adding = QUADRANT_ROWS[0][0] }, Modifier.align(Alignment.BottomEnd).padding(end = 22.dp, bottom = 22.dp))
    }
    adding?.let { q ->
        // With exactly one list picked, new tasks go there, like on the web.
        val only = views.matrixLists.singleOrNull()?.let { key -> data.projects.firstOrNull { it.viewKey() == key }?.id }
        QuickAddSheet(
            projects = data.projects, labels = data.labels, defaultProject = only, defaultDate = null, defaultLabel = null,
            defaultPriority = q.priority,
            onAdd = { vm.quickAdd(it.text, it.projectId, it.due, it.datePicked, it.priority, it.labels, it.description) },
            onDismiss = { adding = null },
        )
    }
    if (editing) MatrixEditSheet(vm, views, data.projects) { editing = false }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun QuadrantCard(q: Quadrant, groups: List<MatrixTaskGroup>, views: Views, vm: MainViewModel, onAdd: () -> Unit, modifier: Modifier) {
    val p = LocalExtra.current
    val color = colorOf(q)
    val collapsed = remember { mutableStateMapOf<String, Boolean>() }
    var menu by remember { mutableStateOf(false) }
    val showList = views.quadrant(q.id).group != MatrixGroup.LIST
    val projects = vm.data.collectAsState().value.projectMap
    Column(modifier.fillMaxHeight().clip(RoundedCornerShape(20.dp)).background(p.card)) {
        Row(Modifier.fillMaxWidth().padding(start = 11.dp, end = 2.dp, top = 6.dp, bottom = 0.dp), verticalAlignment = Alignment.CenterVertically) {
            RomanBadge(q.roman, color)
            Spacer(Modifier.width(5.dp))
            Text(q.title, color = color, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
            Box(Modifier.size(30.dp).clip(CircleShape).clickable(onClick = onAdd), contentAlignment = Alignment.Center) {
                Icon(Icons.Outlined.Add, "Add to quadrant ${q.roman}", tint = p.textTertiary, modifier = Modifier.size(18.dp))
            }
            Box {
                Box(Modifier.size(30.dp).clip(CircleShape).clickable { menu = true }, contentAlignment = Alignment.Center) {
                    Icon(Icons.Outlined.MoreHoriz, "Quadrant ${q.roman} options", tint = p.textTertiary, modifier = Modifier.size(18.dp))
                }
                QuadrantMenu(menu, q, views, { menu = false }) { edit -> vm.updateViews(edit) }
            }
        }
        LazyColumn(contentPadding = PaddingValues(bottom = 8.dp)) {
            if (groups.isEmpty()) item { Text("—", color = p.textTertiary, fontSize = 13.sp, modifier = Modifier.padding(start = 14.dp, top = 4.dp)) }
            for (g in groups) {
                val key = g.key
                val closed = g.header && collapsed[key] == true
                if (g.header) item(key = "h$key") {
                    Row(
                        Modifier.fillMaxWidth().clickable { collapsed[key] = !closed }.padding(start = 8.dp, top = 6.dp, bottom = 2.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(if (closed) Icons.Outlined.KeyboardArrowRight else Icons.Outlined.KeyboardArrowDown, null, tint = p.textTertiary, modifier = Modifier.size(14.dp))
                        Spacer(Modifier.width(3.dp))
                        Text(g.title, fontSize = 12.sp, fontWeight = FontWeight.SemiBold, color = p.text, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                        Spacer(Modifier.width(5.dp))
                        Text("${g.tasks.size}", fontSize = 11.sp, color = p.textTertiary)
                    }
                }
                if (!closed) items(g.tasks, key = { "t${it.id}" }) { t ->
                    Row(
                        Modifier.fillMaxWidth()
                            .combinedClickable(onClick = { vm.openTaskId.value = t.id }, onLongClick = { vm.actionsTaskId.value = t.id })
                            .padding(end = 8.dp),
                        verticalAlignment = Alignment.Top,
                    ) {
                        TaskCheckbox(t.done, t.priority, { vm.toggleDone(t) }, Modifier.padding(start = 3.dp), wontDo = t.wontDo, boxSize = 17.dp, touch = 14.dp)
                        Column(Modifier.weight(1f).padding(top = 6.dp, bottom = 5.dp)) {
                            Text(t.title, fontSize = 13.sp, lineHeight = 16.sp, color = if (t.done) p.textTertiary else p.text, maxLines = 2, overflow = TextOverflow.Ellipsis)
                            val list = projects[t.projectId]?.takeIf { showList }?.let { if (it.isInbox) "Inbox" else it.title }
                            val date = t.dueDate?.let { formatDay(it.toLocalDate()) }
                            if (date != null || list != null) Row {
                                if (date != null) Text(date, fontSize = 10.sp, lineHeight = 13.sp, color = dueColor(t))
                                if (date != null && list != null) Spacer(Modifier.width(6.dp))
                                if (list != null) Text(list, fontSize = 10.sp, lineHeight = 13.sp, color = p.textTertiary, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            }
                        }
                    }
                }
            }
        }
    }
}

/** Group by, sort by and reset for one quadrant: the web app's quadrant "…" menu. */
@Composable
private fun QuadrantMenu(expanded: Boolean, q: Quadrant, views: Views, onDismiss: () -> Unit, onChange: ((Views) -> Views) -> Unit) {
    val p = LocalExtra.current
    val view = views.quadrant(q.id)
    DropdownMenu(expanded, onDismiss, shape = RoundedCornerShape(20.dp), containerColor = p.card) {
        MenuLabel("Group by")
        for (g in MatrixGroup.entries) DropdownMenuItem(
            text = { Text(g.label) },
            trailingIcon = { if (view.group == g) Icon(Icons.Outlined.Check, null, tint = Accent) },
            onClick = { onChange { it.withQuadrant(q.id, view.copy(group = g)) }; onDismiss() },
        )
        HorizontalDivider(color = p.divider)
        MenuLabel("Sort by")
        for (s in MatrixSort.entries) DropdownMenuItem(
            text = { Text(s.label) },
            trailingIcon = { if (view.sort == s) Icon(Icons.Outlined.Check, null, tint = Accent) },
            onClick = { onChange { it.withQuadrant(q.id, view.copy(sort = s)) }; onDismiss() },
        )
        HorizontalDivider(color = p.divider)
        DropdownMenuItem(
            text = { Text("Reset to default order") },
            leadingIcon = { Icon(Icons.Outlined.RestartAlt, null) },
            enabled = !views.isDefault(q.id),
            onClick = { onChange { it.resetQuadrant(q.id) }; onDismiss() },
        )
    }
}

@Composable
private fun MenuLabel(text: String) {
    Text(text, fontSize = 12.sp, color = LocalExtra.current.textTertiary, modifier = Modifier.padding(horizontal = 16.dp, vertical = 6.dp))
}

/**
 * Everything the web app's matrix menus offer, in one sheet: completed tasks,
 * which lists to include and how each quadrant groups and sorts.
 */
@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
private fun MatrixEditSheet(vm: MainViewModel, views: Views, projects: List<ProjectEntity>, onDismiss: () -> Unit) {
    val p = LocalExtra.current
    val selected = views.matrixLists.toSet()
    fun setLists(keys: List<Long>) = vm.updateViews { it.withMatrixLists(keys) }
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Column(Modifier.navigationBarsPadding().verticalScroll(rememberScrollState()).padding(bottom = 16.dp)) {
            Row(Modifier.fillMaxWidth().padding(horizontal = 22.dp), verticalAlignment = Alignment.CenterVertically) {
                Text("Edit Matrix", fontSize = 18.sp, fontWeight = FontWeight.SemiBold, color = p.text, modifier = Modifier.weight(1f))
                TextButton(onClick = onDismiss) { Text("Done", color = Accent) }
            }
            Row(Modifier.fillMaxWidth().clickable { vm.updateViews { it.withMatrixShowDone(!it.matrixShowDone) } }.padding(horizontal = 22.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                Text("Show completed", fontSize = 16.sp, color = p.text, modifier = Modifier.weight(1f))
                Switch(views.matrixShowDone, { on -> vm.updateViews { it.withMatrixShowDone(on) } }, colors = SwitchDefaults.colors(checkedTrackColor = Accent))
            }

            SheetLabel("Lists")
            ListOption("All lists", null, selected.isEmpty()) { setLists(emptyList()) }
            for (proj in projects) {
                val key = proj.viewKey()
                ListOption(if (proj.isInbox) "Inbox" else proj.title, proj, key in selected) {
                    setLists(if (key in selected) views.matrixLists - key else views.matrixLists + key)
                }
            }

            for (q in QUADRANT_ROWS.flatten().sortedBy { it.id }) {
                val view = views.quadrant(q.id)
                Row(Modifier.fillMaxWidth().padding(start = 22.dp, end = 10.dp, top = 18.dp), verticalAlignment = Alignment.CenterVertically) {
                    RomanBadge(q.roman, colorOf(q))
                    Spacer(Modifier.width(6.dp))
                    Text(q.title, fontSize = 15.sp, fontWeight = FontWeight.Medium, color = colorOf(q), modifier = Modifier.weight(1f))
                    if (!views.isDefault(q.id)) TextButton(onClick = { vm.updateViews { it.resetQuadrant(q.id) } }) { Text("Reset", color = Accent) }
                }
                ChipRow("Group by", MatrixGroup.entries, view.group, { it.label }) { g -> vm.updateViews { it.withQuadrant(q.id, view.copy(group = g)) } }
                ChipRow("Sort by", MatrixSort.entries, view.sort, { it.label }) { s -> vm.updateViews { it.withQuadrant(q.id, view.copy(sort = s)) } }
            }
            Text(
                "These settings sync with OpenTick on your other devices.",
                fontSize = 12.sp, color = p.textTertiary, modifier = Modifier.padding(start = 22.dp, end = 22.dp, top = 18.dp),
            )
        }
    }
}

@Composable
private fun SheetLabel(text: String) {
    Text(text, fontSize = 13.sp, color = LocalExtra.current.textTertiary, modifier = Modifier.padding(start = 22.dp, top = 16.dp, bottom = 2.dp))
}

@Composable
private fun ListOption(title: String, project: ProjectEntity?, checked: Boolean, onClick: () -> Unit) {
    val p = LocalExtra.current
    Row(Modifier.fillMaxWidth().clickable(onClick = onClick).heightIn(min = 46.dp).padding(horizontal = 22.dp), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(22.dp), contentAlignment = Alignment.Center) {
            when {
                project == null -> {}
                project.isInbox -> Icon(Icons.Outlined.Inbox, null, tint = p.icon, modifier = Modifier.size(20.dp))
                else -> Box(Modifier.size(10.dp).background(parseHex(project.hexColor, p.textTertiary), CircleShape))
            }
        }
        Spacer(Modifier.width(14.dp))
        Text(title, fontSize = 16.sp, color = p.text, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
        Checkbox(checked, { onClick() }, colors = CheckboxDefaults.colors(checkedColor = Accent))
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun <T> ChipRow(label: String, options: List<T>, current: T, name: (T) -> String, onPick: (T) -> Unit) {
    val p = LocalExtra.current
    Row(Modifier.fillMaxWidth().padding(start = 22.dp, end = 16.dp, top = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(label, fontSize = 13.sp, color = p.textSecondary, modifier = Modifier.width(64.dp))
        FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            for (o in options) FilterChip(
                selected = o == current,
                onClick = { onPick(o) },
                label = { Text(name(o), fontSize = 12.sp) },
                colors = FilterChipDefaults.filterChipColors(selectedContainerColor = Accent.copy(alpha = 0.15f), selectedLabelColor = Accent),
            )
        }
    }
}
