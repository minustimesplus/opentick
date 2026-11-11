package org.opentick.android.ui.tasks

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.List
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.AllInbox
import androidx.compose.material.icons.outlined.CheckBox
import androidx.compose.material.icons.outlined.DateRange
import androidx.compose.material.icons.outlined.Inbox
import androidx.compose.material.icons.outlined.LightMode
import androidx.compose.material.icons.outlined.Sell
import androidx.compose.material.icons.outlined.WbTwilight
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalDrawerSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.AccountMode
import org.opentick.android.data.ProjectEntity
import org.opentick.android.ui.ListSel
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.SmartList
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.parseHex

val LIST_COLORS = listOf("", "e5484d", "f5a623", "f2c94c", "2bb673", "26a69a", "4772fa", "8e5cf7", "e86fb8", "8c8c8c")

/** The list drawer: smart lists, the user's lists and tags, like TickTick's sidebar. */
@Composable
fun ListDrawer(vm: MainViewModel, onPicked: () -> Unit) {
    val data by vm.data.collectAsState()
    val sel by vm.selection.collectAsState()
    val mode by vm.prefs.mode.collectAsState()
    val counts = vm.counts(data)
    var editing by remember { mutableStateOf<ProjectEntity?>(null) }
    var creatingList by remember { mutableStateOf(false) }
    var creatingTag by remember { mutableStateOf(false) }
    var menuFor by remember { mutableStateOf<Any?>(null) }

    fun pick(s: ListSel) {
        vm.selection.value = s
        onPicked()
    }

    ModalDrawerSheet(drawerContainerColor = LocalExtra.current.panel, modifier = Modifier.width(300.dp)) {
        LazyColumn(Modifier.fillMaxHeight().statusBarsPadding()) {
            item {
                Row(Modifier.padding(start = 20.dp, end = 12.dp, top = 16.dp, bottom = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                    val name = vm.prefs.displayName.ifBlank { vm.prefs.username }.ifBlank { "OpenTick" }
                    Box(Modifier.size(36.dp).background(Accent, CircleShape), contentAlignment = Alignment.Center) {
                        Text(name.take(1).uppercase(), color = Color.White, fontWeight = FontWeight.Bold)
                    }
                    Spacer(Modifier.width(12.dp))
                    Column(Modifier.weight(1f)) {
                        Text(name, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Text(
                            if (mode == AccountMode.LOCAL) "On this device only" else vm.prefs.serverUrl.removePrefix("https://").removePrefix("http://"),
                            fontSize = 12.sp, color = LocalExtra.current.textTertiary, maxLines = 1, overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
            }
            val smart = listOf(
                Triple(SmartList.TODAY, Icons.Outlined.LightMode, counts[SmartList.TODAY]),
                Triple(SmartList.TOMORROW, Icons.Outlined.WbTwilight, counts[SmartList.TOMORROW]),
                Triple(SmartList.NEXT7, Icons.Outlined.DateRange, counts[SmartList.NEXT7]),
                Triple(SmartList.INBOX, Icons.Outlined.Inbox, counts[SmartList.INBOX]),
                Triple(SmartList.ALL, Icons.Outlined.AllInbox, counts[SmartList.ALL]),
            )
            items(smart) { (kind, icon, count) ->
                DrawerItem(icon, kind.title, count, sel == ListSel.Smart(kind)) { pick(ListSel.Smart(kind)) }
            }
            item { HorizontalDivider(Modifier.padding(vertical = 8.dp, horizontal = 16.dp), color = LocalExtra.current.divider) }
            item {
                DrawerHeader("Lists") { creatingList = true }
            }
            items(data.projects.filter { !it.isInbox }, key = { "p${it.id}" }) { p ->
                Box {
                    DrawerItem(
                        Icons.AutoMirrored.Outlined.List, p.title, counts[p.id], sel == ListSel.Project(p.id),
                        tint = if (p.hexColor.isNotBlank()) parseHex(p.hexColor) else null,
                        onLongClick = { menuFor = p },
                    ) { pick(ListSel.Project(p.id)) }
                    DropdownMenu(menuFor == p, { menuFor = null }) {
                        DropdownMenuItem(text = { Text("Edit") }, onClick = { editing = p; menuFor = null })
                        DropdownMenuItem(text = { Text("Delete", color = MaterialTheme.colorScheme.error) }, onClick = { vm.deleteProject(p.id); menuFor = null })
                    }
                }
            }
            item {
                HorizontalDivider(Modifier.padding(vertical = 8.dp, horizontal = 16.dp), color = LocalExtra.current.divider)
                DrawerHeader("Tags") { creatingTag = true }
            }
            items(data.labels, key = { "l${it.id}" }) { l ->
                Box {
                    DrawerItem(
                        Icons.Outlined.Sell, l.title, counts["l${l.id}"], sel == ListSel.Label(l.id),
                        tint = if (l.hexColor.isNotBlank()) parseHex(l.hexColor) else null,
                        onLongClick = { menuFor = l },
                    ) { pick(ListSel.Label(l.id)) }
                    DropdownMenu(menuFor == l, { menuFor = null }) {
                        DropdownMenuItem(text = { Text("Delete tag", color = MaterialTheme.colorScheme.error) }, onClick = { vm.deleteLabel(l.id); menuFor = null })
                    }
                }
            }
            item {
                HorizontalDivider(Modifier.padding(vertical = 8.dp, horizontal = 16.dp), color = LocalExtra.current.divider)
                DrawerItem(Icons.Outlined.CheckBox, "Completed", null, sel == ListSel.Smart(SmartList.COMPLETED)) { pick(ListSel.Smart(SmartList.COMPLETED)) }
                Spacer(Modifier.height(24.dp))
            }
        }
    }

    if (creatingList || editing != null) {
        ListDialog(
            initial = editing,
            onSave = { title, color ->
                val e = editing
                if (e != null) vm.updateProject(e.id, title, color) else vm.createProject(title, color)
                creatingList = false; editing = null
                if (e == null) onPicked()
            },
            onDismiss = { creatingList = false; editing = null },
        )
    }
    if (creatingTag) {
        var name by remember { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { creatingTag = false },
            title = { Text("New tag") },
            text = { OutlinedTextField(name, { name = it.replace(" ", "") }, singleLine = true, placeholder = { Text("Tag name") }) },
            confirmButton = { TextButton(enabled = name.isNotBlank(), onClick = { vm.createLabel(name); creatingTag = false }) { Text("Add") } },
            dismissButton = { TextButton(onClick = { creatingTag = false }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun DrawerHeader(title: String, onAdd: () -> Unit) {
    Row(Modifier.fillMaxWidth().padding(start = 20.dp, end = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(title, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = LocalExtra.current.textTertiary, modifier = Modifier.weight(1f))
        IconButton(onClick = onAdd) { Icon(Icons.Outlined.Add, "Add", tint = LocalExtra.current.textTertiary, modifier = Modifier.size(20.dp)) }
    }
}

@OptIn(androidx.compose.foundation.ExperimentalFoundationApi::class)
@Composable
private fun DrawerItem(
    icon: ImageVector,
    title: String,
    count: Int?,
    selected: Boolean,
    tint: Color? = null,
    onLongClick: (() -> Unit)? = null,
    onClick: () -> Unit,
) {
    val extra = LocalExtra.current
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 8.dp, vertical = 1.dp)
            .clip(RoundedCornerShape(8.dp))
            .background(if (selected) extra.selected else Color.Transparent)
            .then(
                if (onLongClick != null) Modifier.combinedClickable(onClick = onClick, onLongClick = onLongClick)
                else Modifier.clickable(onClick = onClick),
            )
            .padding(horizontal = 12.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, null, tint = tint ?: if (selected) Accent else extra.textSecondary, modifier = Modifier.size(20.dp))
        Spacer(Modifier.width(14.dp))
        Text(title, Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis, fontSize = 15.sp,
            color = if (selected) Accent else MaterialTheme.colorScheme.onSurface)
        if (count != null && count > 0) Text("$count", fontSize = 13.sp, color = extra.textTertiary)
    }
}

@Composable
private fun ListDialog(initial: ProjectEntity?, onSave: (String, String) -> Unit, onDismiss: () -> Unit) {
    var title by remember { mutableStateOf(initial?.title ?: "") }
    var color by remember { mutableStateOf(initial?.hexColor ?: "") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(if (initial == null) "Add list" else "Edit list") },
        text = {
            Column {
                OutlinedTextField(title, { title = it }, singleLine = true, placeholder = { Text("Name") }, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.height(16.dp))
                Row(Modifier.fillMaxWidth(), horizontalArrangement = androidx.compose.foundation.layout.Arrangement.SpaceBetween) {
                    for (c in LIST_COLORS) {
                        val col = if (c.isEmpty()) LocalExtra.current.divider else parseHex(c)
                        Box(
                            Modifier.size(24.dp).clip(CircleShape).background(col).clickable { color = c }
                                .then(if (color.removePrefix("#") == c) Modifier.padding(6.dp).clip(CircleShape).background(Color.White) else Modifier),
                        )
                    }
                }
            }
        },
        confirmButton = { TextButton(enabled = title.isNotBlank(), onClick = { onSave(title, color) }) { Text("Save") } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}
