package org.opentick.android.ui.tasks

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.CheckBox
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.DisabledByDefault
import androidx.compose.material.icons.filled.Folder
import androidx.compose.material.icons.filled.Inbox
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.LocalOffer
import androidx.compose.material.icons.filled.WbSunny
import androidx.compose.material.icons.outlined.AddBox
import androidx.compose.material.icons.outlined.AllInbox
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.Tune
import androidx.compose.material.icons.outlined.ViewWeek
import androidx.compose.material3.AlertDialog
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
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.ProjectEntity
import org.opentick.android.ui.ListSel
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.Page
import org.opentick.android.ui.SmartList
import org.opentick.android.ui.components.Avatar
import org.opentick.android.ui.components.CalendarDayIcon
import org.opentick.android.ui.components.ListIcon
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.Cyan
import org.opentick.android.ui.theme.Green
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.Orange
import org.opentick.android.ui.theme.parseHex
import org.opentick.android.util.splitEmoji

val LIST_COLORS = listOf("", "e5484d", "f5a623", "f2c94c", "2bb673", "26a69a", "4772fa", "8e5cf7", "e86fb8", "8c8c8c")

/** TickTick's sidebar: profile, smart lists, tags, lists and folders, Completed and Trash. */
@Composable
fun ListDrawer(vm: MainViewModel, onPicked: () -> Unit) {
    val data by vm.data.collectAsState()
    val sel by vm.selection.collectAsState()
    // Re-read when the synced or the phone's own smart list settings change.
    val views by vm.views.collectAsState()
    val legacy by vm.prefs.smartLists.collectAsState()
    val trashFull by vm.trashNonEmpty.collectAsState()
    val p = LocalExtra.current
    val counts = vm.counts(data)
    val smart = remember(views, legacy, data, counts, trashFull) { vm.visibleSmartLists(data, counts, trashFull) }
    LaunchedEffect(views, legacy) { vm.probeTrash() }
    var editing by remember { mutableStateOf<ProjectEntity?>(null) }
    var creatingList by remember { mutableStateOf(false) }
    var creatingTag by remember { mutableStateOf(false) }
    var addMenu by remember { mutableStateOf(false) }
    var menuFor by remember { mutableStateOf<Any?>(null) }
    var tagsOpen by rememberSaveable { mutableStateOf(false) }
    val openFolders = remember { androidx.compose.runtime.mutableStateMapOf<Long, Boolean>() }

    fun pick(s: ListSel) {
        vm.selection.value = s
        onPicked()
    }

    fun page(page: Page) {
        onPicked()
        vm.open(page)
    }

    Box(
        Modifier.fillMaxHeight().width(331.dp)
            .background(Brush.verticalGradient(0f to p.drawerTop, 0.16f to p.card, 1f to p.card)),
    ) {
        Column(Modifier.fillMaxSize()) {
            // Profile row with search, notifications and settings.
            Row(Modifier.statusBarsPadding().padding(start = 22.dp, end = 10.dp, top = 22.dp, bottom = 18.dp), verticalAlignment = Alignment.CenterVertically) {
                val name = vm.prefs.displayName.ifBlank { vm.prefs.username }.ifBlank { "OpenTick" }
                Avatar(name, 40.dp)
                Spacer(Modifier.width(22.dp))
                Text(name, fontSize = 16.sp, fontWeight = FontWeight.Bold, color = p.text, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                IconButton(onClick = { page(Page.SEARCH) }) { Icon(Icons.Outlined.Search, "Search", tint = p.icon, modifier = Modifier.size(25.dp)) }
                IconButton(onClick = { page(Page.SETTINGS) }) { Icon(Icons.Outlined.Settings, "Settings", tint = p.icon, modifier = Modifier.size(25.dp)) }
            }

            LazyColumn(Modifier.weight(1f)) {
                if ("today" in smart) item {
                    DrawerItem({ CalendarDayIcon(Orange, size = 22.dp) }, "Today", counts[SmartList.TODAY], sel == ListSel.Smart(SmartList.TODAY)) { pick(ListSel.Smart(SmartList.TODAY)) }
                }
                if ("tomorrow" in smart) item {
                    DrawerItem({ Icon(Icons.Filled.WbSunny, null, tint = Orange, modifier = Modifier.size(23.dp)) }, "Tomorrow", counts[SmartList.TOMORROW], sel == ListSel.Smart(SmartList.TOMORROW)) { pick(ListSel.Smart(SmartList.TOMORROW)) }
                }
                if ("next7" in smart) item {
                    DrawerItem({ Icon(Icons.Outlined.ViewWeek, null, tint = Color(0xFF9B6CFF), modifier = Modifier.size(23.dp)) }, "Next 7 Days", counts[SmartList.NEXT7], sel == ListSel.Smart(SmartList.NEXT7)) { pick(ListSel.Smart(SmartList.NEXT7)) }
                }
                if ("inbox" in smart) item {
                    DrawerItem({ Icon(Icons.Filled.Inbox, null, tint = Orange, modifier = Modifier.size(24.dp)) }, "Inbox", counts[SmartList.INBOX], sel == ListSel.Smart(SmartList.INBOX)) { pick(ListSel.Smart(SmartList.INBOX)) }
                }
                if ("all" in smart) item {
                    DrawerItem({ Icon(Icons.Outlined.AllInbox, null, tint = Accent, modifier = Modifier.size(23.dp)) }, "All", counts[SmartList.ALL], sel == ListSel.Smart(SmartList.ALL)) { pick(ListSel.Smart(SmartList.ALL)) }
                }
                if ("tags" in smart) {
                    item {
                        DrawerItem(
                            { Icon(Icons.Filled.LocalOffer, null, tint = Cyan, modifier = Modifier.size(23.dp)) }, "Tags", null, false,
                            trailing = { Chevron(tagsOpen) },
                            onLongClick = { creatingTag = true },
                        ) { tagsOpen = !tagsOpen }
                    }
                    if (tagsOpen) {
                        items(data.labels, key = { "l${it.id}" }) { l ->
                            Box {
                                DrawerItem(
                                    { Icon(Icons.Filled.LocalOffer, null, tint = if (l.hexColor.isNotBlank()) parseHex(l.hexColor) else p.count, modifier = Modifier.size(18.dp)) },
                                    l.title, counts["l${l.id}"], sel == ListSel.Label(l.id), indent = true, onLongClick = { menuFor = l },
                                ) { pick(ListSel.Label(l.id)) }
                                DropdownMenu(menuFor == l, { menuFor = null }) {
                                    DropdownMenuItem(text = { Text("Delete tag", color = MaterialTheme.colorScheme.error) }, onClick = { vm.deleteLabel(l.id); menuFor = null })
                                }
                            }
                        }
                        item { DrawerItem({ Icon(Icons.Outlined.AddBox, null, tint = p.count, modifier = Modifier.size(18.dp)) }, "Add Tag", null, false, indent = true, muted = true) { creatingTag = true } }
                    }
                }
                item { DrawerDivider() }
                for (proj in data.topLevel) {
                    val kids = data.children[proj.id].orEmpty()
                    if (kids.isNotEmpty()) {
                        val open = openFolders[proj.id] == true
                        item(key = "f${proj.id}") {
                            DrawerItem(
                                { Icon(Icons.Filled.Folder, null, tint = p.icon.copy(alpha = 0.85f), modifier = Modifier.size(24.dp)) },
                                splitEmoji(proj.title).second, counts[proj.id], sel == ListSel.Project(proj.id),
                                trailing = { Chevron(open) },
                            ) { openFolders[proj.id] = !open }
                        }
                        if (open) items(kids, key = { "p${it.id}" }) { kid -> ProjectItem(kid, counts, sel, true, menuFor, { menuFor = it }, { editing = it }, vm, ::pick) }
                    } else {
                        item(key = "p${proj.id}") { ProjectItem(proj, counts, sel, false, menuFor, { menuFor = it }, { editing = it }, vm, ::pick) }
                    }
                }
                if ("completed" in smart || "wontDo" in smart || "trash" in smart) item { DrawerDivider() }
                if ("completed" in smart) item {
                    DrawerItem({ Icon(Icons.Filled.CheckBox, null, tint = Green, modifier = Modifier.size(25.dp)) }, "Completed", null, sel == ListSel.Smart(SmartList.COMPLETED)) { pick(ListSel.Smart(SmartList.COMPLETED)) }
                }
                if ("wontDo" in smart) item {
                    DrawerItem({ Icon(Icons.Filled.DisabledByDefault, null, tint = p.count, modifier = Modifier.size(25.dp)) }, "Won't Do", null, sel == ListSel.Smart(SmartList.WONT_DO)) { pick(ListSel.Smart(SmartList.WONT_DO)) }
                }
                if ("trash" in smart) item {
                    DrawerItem({ Icon(Icons.Filled.Delete, null, tint = Accent, modifier = Modifier.size(25.dp)) }, "Trash", null, false) { page(Page.TRASH) }
                }
                item { Spacer(Modifier.height(12.dp)) }
            }

            Row(Modifier.fillMaxWidth().navigationBarsPadding().padding(start = 8.dp, end = 10.dp, top = 4.dp, bottom = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                Box {
                    Row(
                        Modifier.clip(RoundedCornerShape(12.dp)).clickable { addMenu = true }.padding(horizontal = 20.dp, vertical = 12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(Icons.Outlined.AddBox, null, tint = p.icon, modifier = Modifier.size(24.dp))
                        Spacer(Modifier.width(16.dp))
                        Text("Add", fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = p.text)
                    }
                    DropdownMenu(addMenu, { addMenu = false }, shape = RoundedCornerShape(16.dp), containerColor = p.card) {
                        DropdownMenuItem(text = { Text("List") }, onClick = { addMenu = false; creatingList = true })
                        DropdownMenuItem(text = { Text("Tag") }, onClick = { addMenu = false; creatingTag = true })
                    }
                }
                Spacer(Modifier.weight(1f))
                IconButton(onClick = { page(Page.GENERAL) }) { Icon(Icons.Outlined.Tune, "Manage lists", tint = p.icon) }
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
            title = { Text("Add Tag") },
            text = { OutlinedTextField(name, { name = it.replace(" ", "") }, singleLine = true, placeholder = { Text("Tag name") }) },
            confirmButton = { TextButton(enabled = name.isNotBlank(), onClick = { vm.createLabel(name); creatingTag = false; tagsOpen = true }) { Text("Add") } },
            dismissButton = { TextButton(onClick = { creatingTag = false }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun ProjectItem(
    proj: ProjectEntity,
    counts: Map<Any, Int>,
    sel: ListSel,
    indent: Boolean,
    menuFor: Any?,
    setMenu: (Any?) -> Unit,
    edit: (ProjectEntity) -> Unit,
    vm: MainViewModel,
    pick: (ListSel) -> Unit,
) {
    val p = LocalExtra.current
    Box {
        DrawerItem(
            { ListIcon(proj.title, tint = if (proj.hexColor.isNotBlank()) parseHex(proj.hexColor) else p.icon.copy(alpha = 0.8f)) },
            splitEmoji(proj.title).second, counts[proj.id], sel == ListSel.Project(proj.id), indent = indent,
            onLongClick = { setMenu(proj) },
        ) { pick(ListSel.Project(proj.id)) }
        DropdownMenu(menuFor == proj, { setMenu(null) }) {
            DropdownMenuItem(text = { Text("Edit") }, onClick = { edit(proj); setMenu(null) })
            DropdownMenuItem(text = { Text("Delete", color = MaterialTheme.colorScheme.error) }, onClick = { vm.deleteProject(proj.id); setMenu(null) })
        }
    }
}

@Composable
private fun Chevron(open: Boolean) {
    Icon(
        if (open) Icons.Filled.KeyboardArrowDown else Icons.AutoMirrored.Filled.KeyboardArrowRight, null,
        tint = LocalExtra.current.count, modifier = Modifier.size(24.dp),
    )
}

@Composable
private fun DrawerDivider() {
    HorizontalDivider(Modifier.padding(horizontal = 30.dp, vertical = 10.dp), color = LocalExtra.current.divider)
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun DrawerItem(
    icon: @Composable () -> Unit,
    title: String,
    count: Int?,
    selected: Boolean,
    indent: Boolean = false,
    muted: Boolean = false,
    trailing: (@Composable () -> Unit)? = null,
    onLongClick: (() -> Unit)? = null,
    onClick: () -> Unit,
) {
    val p = LocalExtra.current
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 13.dp)
            .height(50.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(if (selected) p.selected else Color.Transparent)
            .combinedClickable(onClick = onClick, onLongClick = onLongClick)
            .padding(start = if (indent) 44.dp else 15.dp, end = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(24.dp).graphicsLayer(scaleX = 0.9f, scaleY = 0.9f), contentAlignment = Alignment.Center) { icon() }
        Spacer(Modifier.width(15.dp))
        Text(
            title, Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis, fontSize = 14.sp,
            fontWeight = FontWeight.SemiBold, color = if (muted) p.textTertiary else p.text,
        )
        if (count != null && count > 0) Text("$count", fontSize = 14.sp, color = p.count)
        if (trailing != null) {
            if (count != null && count > 0) Spacer(Modifier.width(6.dp))
            trailing()
        }
    }
}

@Composable
private fun ListDialog(initial: ProjectEntity?, onSave: (String, String) -> Unit, onDismiss: () -> Unit) {
    var title by remember { mutableStateOf(initial?.title ?: "") }
    var color by remember { mutableStateOf(initial?.hexColor ?: "") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(if (initial == null) "Add List" else "Edit List") },
        text = {
            Column {
                OutlinedTextField(title, { title = it }, singleLine = true, placeholder = { Text("Name (start with an emoji for an icon)") }, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.height(16.dp))
                Row(Modifier.fillMaxWidth(), horizontalArrangement = androidx.compose.foundation.layout.Arrangement.SpaceBetween) {
                    for (c in LIST_COLORS) {
                        val col = if (c.isEmpty()) LocalExtra.current.divider else parseHex(c)
                        Box(
                            Modifier.size(24.dp).clip(CircleShape).background(col).clickable { color = c }
                                .then(if (color.removePrefix("#") == c) Modifier.padding(7.dp).clip(CircleShape).background(Color.White) else Modifier),
                        )
                    }
                }
            }
        },
        confirmButton = { TextButton(enabled = title.isNotBlank(), onClick = { onSave(title, color) }) { Text("Save") } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}
