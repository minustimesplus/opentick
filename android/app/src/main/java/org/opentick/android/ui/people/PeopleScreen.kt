package org.opentick.android.ui.people

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.Sort
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material.icons.rounded.Close
import androidx.compose.material.icons.rounded.PersonAdd
import androidx.compose.material.icons.rounded.Search
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.AccountMode
import org.opentick.android.data.ApiContactGroup
import org.opentick.android.data.ApiContactRow
import org.opentick.android.data.ApiContactTag
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.PageHeader
import org.opentick.android.ui.habits.HABIT_COLORS
import org.opentick.android.ui.habits.otFieldColors
import org.opentick.android.ui.tasks.EmptyState
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.ui.theme.parseHex

/** The People tab: your contacts, searchable and filtered by group or tag, and each contact's page. */
@Composable
fun PeopleScreen(vm: MainViewModel) {
    val store = vm.people
    val mode by vm.prefs.mode.collectAsState()
    val openId by store.openId.collectAsState()
    val snackbar = remember { SnackbarHostState() }
    val message by store.message.collectAsState()
    LaunchedEffect(message) { message?.let { snackbar.showSnackbar(it); store.message.value = null } }
    LaunchedEffect(mode) { store.load() }

    Box(Modifier.fillMaxSize()) {
        when {
            mode != AccountMode.SERVER -> Column(Modifier.fillMaxSize()) {
                PageHeader("People")
                EmptyState("Sign in to see your contacts", "People are kept on your OpenTick server. Sign in from Settings → Account.")
            }
            openId != null -> {
                BackHandler { store.closeContact() }
                ContactDetail(vm, openId!!)
            }
            else -> PeopleList(vm)
        }
        SnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
    }
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)
@Composable
private fun PeopleList(vm: MainViewModel) {
    val store = vm.people
    val p = LocalExtra.current
    val contacts by store.contacts.collectAsState()
    val groups by store.groups.collectAsState()
    val tags by store.tags.collectAsState()
    val loaded by store.loaded.collectAsState()
    val loading by store.loading.collectAsState()
    val error by store.error.collectAsState()
    val query by store.query.collectAsState()
    val sort by store.sort.collectAsState()
    val filter by store.filter.collectAsState()
    var sortMenu by remember { mutableStateOf(false) }
    var adding by remember { mutableStateOf(false) }
    var groupSheet by remember { mutableStateOf<Pair<Boolean, ApiContactGroup?>?>(null) }
    var tagActions by remember { mutableStateOf<ApiContactTag?>(null) }

    val tagNames = remember(tags) { tags.associate { it.id to it.name } }
    val groupNames = remember(groups) { groups.associate { it.id to it.name } }
    val shown = remember(contacts, query, filter, sort, tagNames, groupNames) {
        sortContacts(filterContacts(contacts, query, filter, tagNames, groupNames), sort)
    }

    Column(Modifier.fillMaxSize()) {
        PageHeader("People", actions = {
            Box {
                IconButton(onClick = { sortMenu = true }) { Icon(Icons.AutoMirrored.Rounded.Sort, "Sort", tint = p.icon) }
                DropdownMenu(sortMenu, { sortMenu = false }, containerColor = p.card) {
                    for (s in ContactSort.entries) {
                        DropdownMenuItem(
                            text = { Text(s.label, color = p.text) },
                            trailingIcon = { if (s == sort) Icon(Icons.Rounded.Check, null, tint = Accent) },
                            onClick = { store.sort.value = s; sortMenu = false },
                        )
                    }
                }
            }
            IconButton(onClick = { adding = true }) { Icon(Icons.Rounded.PersonAdd, "New contact", tint = p.icon) }
        })
        SearchField(query) { store.query.value = it }
        LazyRow(contentPadding = PaddingValues(horizontal = 12.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.padding(vertical = 8.dp)) {
            item { FilterChip("All", contacts.size.toLong(), null, filter == PeopleFilter.All, onClick = { store.filter.value = PeopleFilter.All }) }
            for (g in groups) item(key = "g${g.id}") {
                FilterChip(
                    g.name, g.members, parseHex(nameColor(g.name, g.hexColor)), filter == PeopleFilter.Group(g.id),
                    onClick = { store.filter.value = if (filter == PeopleFilter.Group(g.id)) PeopleFilter.All else PeopleFilter.Group(g.id) },
                    onLongClick = { groupSheet = true to g },
                )
            }
            item { FilterChip("+ Group", null, null, false, onClick = { groupSheet = true to null }) }
            for (t in tags) item(key = "t${t.id}") {
                FilterChip(
                    "#" + t.name, t.contacts, null, filter == PeopleFilter.Tag(t.id),
                    onClick = { store.filter.value = if (filter == PeopleFilter.Tag(t.id)) PeopleFilter.All else PeopleFilter.Tag(t.id) },
                    onLongClick = { tagActions = t },
                )
            }
        }
        PullToRefreshBox(isRefreshing = loading && loaded, onRefresh = { store.load() }, modifier = Modifier.fillMaxSize()) {
            when {
                !loaded && error != null -> Box(Modifier.fillMaxSize().padding(32.dp), contentAlignment = Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(error!!, color = p.textTertiary, textAlign = TextAlign.Center)
                        TextButton(onClick = { store.load() }) { Text("Try again", color = Accent) }
                    }
                }
                !loaded -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator(color = Accent) }
                contacts.isEmpty() -> EmptyState("No contacts yet", "Add one with the + above, or import your LinkedIn connections on the web.")
                shown.isEmpty() -> EmptyState("No matches", "Nobody matches this search.")
                else -> LazyColumn(contentPadding = PaddingValues(start = 12.dp, end = 12.dp, bottom = 24.dp), modifier = Modifier.fillMaxSize()) {
                    item { Text("${shown.size} ${if (shown.size == 1) "contact" else "contacts"}", fontSize = 13.sp, color = p.textTertiary, modifier = Modifier.padding(start = 6.dp, bottom = 6.dp)) }
                    itemsIndexed(shown, key = { _, c -> c.id }) { i, c ->
                        val shape = RoundedCornerShape(
                            topStart = if (i == 0) 16.dp else 0.dp, topEnd = if (i == 0) 16.dp else 0.dp,
                            bottomStart = if (i == shown.lastIndex) 16.dp else 0.dp, bottomEnd = if (i == shown.lastIndex) 16.dp else 0.dp,
                        )
                        Column(Modifier.clip(shape).background(p.card)) {
                            if (i > 0) Box(Modifier.fillMaxWidth().padding(start = 70.dp).height(0.5.dp).background(p.divider))
                            ContactRow(store, c) { store.openContact(c.id) }
                        }
                    }
                }
            }
        }
    }

    if (adding) NewContactSheet(store) { adding = false }
    groupSheet?.let { (_, g) -> GroupSheet(store, g) { groupSheet = null } }
    tagActions?.let { t ->
        AlertDialog(
            onDismissRequest = { tagActions = null },
            title = { Text("#${t.name}") },
            text = { Text("Delete this tag? It's removed from ${t.contacts} ${if (t.contacts == 1L) "contact" else "contacts"}; the contacts stay.") },
            confirmButton = { TextButton(onClick = { store.deleteTag(t.id); tagActions = null }) { Text("Delete tag", color = PrioHigh) } },
            dismissButton = { TextButton(onClick = { tagActions = null }) { Text("Cancel") } },
            containerColor = p.card,
        )
    }
}

@Composable
private fun SearchField(value: String, onChange: (String) -> Unit) {
    val p = LocalExtra.current
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 12.dp).clip(RoundedCornerShape(12.dp)).background(p.card).padding(horizontal = 12.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(Icons.Rounded.Search, null, tint = p.textTertiary, modifier = Modifier.size(20.dp))
        Spacer(Modifier.width(8.dp))
        Box(Modifier.weight(1f)) {
            if (value.isEmpty()) Text("Search name, company, school, notes…", color = p.textTertiary, fontSize = 15.sp, maxLines = 1)
            BasicTextField(
                value, onChange, singleLine = true, textStyle = TextStyle(color = p.text, fontSize = 15.sp),
                cursorBrush = SolidColor(Accent), modifier = Modifier.fillMaxWidth(),
            )
        }
        if (value.isNotEmpty()) Icon(Icons.Rounded.Close, "Clear search", tint = p.textTertiary, modifier = Modifier.size(20.dp).clip(CircleShape).combinedClickableCompat { onChange("") })
    }
}

@OptIn(ExperimentalFoundationApi::class)
private fun Modifier.combinedClickableCompat(onLongClick: (() -> Unit)? = null, onClick: () -> Unit) = combinedClickable(onClick = onClick, onLongClick = onLongClick)

@Composable
private fun FilterChip(label: String, count: Long?, dot: Color?, selected: Boolean, onClick: () -> Unit, onLongClick: (() -> Unit)? = null) {
    val p = LocalExtra.current
    Row(
        Modifier.clip(RoundedCornerShape(16.dp))
            .background(if (selected) Accent.copy(alpha = 0.14f) else p.card)
            .border(1.dp, if (selected) Accent else Color.Transparent, RoundedCornerShape(16.dp))
            .combinedClickableCompat(onLongClick, onClick).padding(horizontal = 12.dp, vertical = 7.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (dot != null) {
            Box(Modifier.size(8.dp).clip(CircleShape).background(dot))
            Spacer(Modifier.width(6.dp))
        }
        Text(label, fontSize = 14.sp, color = if (selected) Accent else p.text, maxLines = 1)
        if (count != null && count > 0) {
            Spacer(Modifier.width(5.dp))
            Text("$count", fontSize = 12.sp, color = p.textTertiary)
        }
    }
}

@Composable
private fun ContactRow(store: PeopleStore, c: ApiContactRow, onClick: () -> Unit) {
    val p = LocalExtra.current
    Row(Modifier.fillMaxWidth().combinedClickableCompat(onClick = onClick).padding(horizontal = 14.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
        ContactAvatar(store, c.id, c.name, c.hasPhoto, 42)
        Spacer(Modifier.width(14.dp))
        Column(Modifier.weight(1f)) {
            Text(c.name.ifBlank { c.email.ifBlank { "Unnamed" } }, fontSize = 15.sp, color = p.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
            val sub = contactSubtitle(c)
            if (sub.isNotBlank()) Text(sub, fontSize = 13.sp, color = p.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (c.location.isNotBlank()) Text(c.location, fontSize = 12.sp, color = p.textTertiary, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

/** The contact's photo, or their initials on a color picked from the name. */
@Composable
fun ContactAvatar(store: PeopleStore, id: Long, name: String, hasPhoto: Boolean, size: Int) {
    val versions by store.photoVersions.collectAsState()
    val version = versions[id] ?: 0
    val photo by produceState<android.graphics.Bitmap?>(null, id, hasPhoto, version) {
        value = if (hasPhoto) store.photo(id) else null
    }
    Box(Modifier.size(size.dp).clip(CircleShape).background(parseHex(nameColor(name))), contentAlignment = Alignment.Center) {
        val bmp = photo
        if (bmp != null) {
            Image(bmp.asImageBitmap(), name, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
        } else {
            Text(initials(name), color = Color.White, fontWeight = FontWeight.SemiBold, fontSize = (size * 0.38f).sp)
        }
    }
}

// ─── New contact ──────────────────────────────────────────────────────

private val NEW_CONTACT_FIELDS = listOf("full_name", "email", "phone", "title", "company", "location", "linkedin_url")

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun NewContactSheet(store: PeopleStore, onDismiss: () -> Unit) {
    val p = LocalExtra.current
    var values by remember { mutableStateOf(mapOf<String, String>()) }
    var duplicate by remember { mutableStateOf<String?>(null) }
    var saving by remember { mutableStateOf(false) }
    val valid = listOf("full_name", "email", "linkedin_url").any { !values[it].isNullOrBlank() }
    fun save(allowDuplicate: Boolean) {
        saving = true
        store.create(values, allowDuplicate, onDuplicate = { saving = false; duplicate = it }) { c ->
            saving = false
            onDismiss()
            store.openContact(c.id)
        }
    }
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).imePadding().padding(horizontal = 20.dp)) {
            Text("New contact", fontWeight = FontWeight.Bold, fontSize = 18.sp, color = p.text)
            Text("Needs a name, email or LinkedIn URL.", fontSize = 13.sp, color = p.textTertiary)
            Spacer(Modifier.height(8.dp))
            for (key in NEW_CONTACT_FIELDS) {
                val f = CONTACT_FIELDS.first { it.key == key }
                OutlinedTextField(
                    value = values[key].orEmpty(), onValueChange = { values = values + (key to it) },
                    label = { Text(f.label) }, singleLine = true, colors = otFieldColors(), shape = RoundedCornerShape(12.dp),
                    placeholder = if (f.hint.isNotEmpty()) ({ Text(f.hint, color = p.textTertiary) }) else null,
                    modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
                )
            }
            Row(Modifier.fillMaxWidth().padding(top = 14.dp), horizontalArrangement = Arrangement.End) {
                TextButton(onClick = onDismiss) { Text("Cancel", color = p.textSecondary) }
                Spacer(Modifier.width(8.dp))
                Button(onClick = { save(false) }, enabled = valid && !saving, colors = ButtonDefaults.buttonColors(containerColor = Accent), shape = RoundedCornerShape(12.dp)) { Text("Add") }
            }
            Spacer(Modifier.height(24.dp))
        }
    }
    duplicate?.let { msg ->
        AlertDialog(
            onDismissRequest = { duplicate = null },
            title = { Text("Already in your contacts") },
            text = { Text(msg) },
            confirmButton = { TextButton(onClick = { duplicate = null; save(true) }) { Text("Add anyway") } },
            dismissButton = { TextButton(onClick = { duplicate = null }) { Text("Cancel") } },
            containerColor = p.card,
        )
    }
}

// ─── Groups ───────────────────────────────────────────────────────────

/** Creates a group ([group] null) or edits or deletes one. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun GroupSheet(store: PeopleStore, group: ApiContactGroup?, onDismiss: () -> Unit) {
    val p = LocalExtra.current
    var name by remember { mutableStateOf(group?.name ?: "") }
    var color by remember { mutableStateOf(group?.hexColor?.removePrefix("#")?.ifBlank { null } ?: HABIT_COLORS[6]) }
    var description by remember { mutableStateOf(group?.description ?: "") }
    var confirmDelete by remember { mutableStateOf(false) }
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Column(Modifier.fillMaxWidth().imePadding().padding(horizontal = 20.dp)) {
            Text(if (group == null) "New group" else "Edit group", fontWeight = FontWeight.Bold, fontSize = 18.sp, color = p.text)
            Spacer(Modifier.height(10.dp))
            OutlinedTextField(name, { name = it.take(250) }, label = { Text("Name") }, singleLine = true, colors = otFieldColors(), shape = RoundedCornerShape(12.dp), modifier = Modifier.fillMaxWidth())
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(description, { description = it }, label = { Text("Description") }, colors = otFieldColors(), shape = RoundedCornerShape(12.dp), modifier = Modifier.fillMaxWidth())
            Spacer(Modifier.height(12.dp))
            Row {
                for (c in HABIT_COLORS) {
                    Box(
                        Modifier.padding(end = 6.dp).size(26.dp).clip(CircleShape).background(parseHex(c)).combinedClickableCompat { color = c },
                        contentAlignment = Alignment.Center,
                    ) { if (color.equals(c, true)) Icon(Icons.Rounded.Check, null, tint = Color.White, modifier = Modifier.size(16.dp)) }
                }
            }
            Row(Modifier.fillMaxWidth().padding(top = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                if (group != null) TextButton(onClick = { confirmDelete = true }) { Text("Delete", color = PrioHigh) }
                Spacer(Modifier.weight(1f))
                Button(
                    onClick = { store.saveGroup(group?.id, name, color, description); onDismiss() },
                    enabled = name.isNotBlank(), colors = ButtonDefaults.buttonColors(containerColor = Accent), shape = RoundedCornerShape(12.dp),
                ) { Text("Save") }
            }
            Spacer(Modifier.height(24.dp))
        }
    }
    if (confirmDelete && group != null) {
        AlertDialog(
            onDismissRequest = { confirmDelete = false },
            title = { Text("Delete ${group.name}?") },
            text = { Text("The group goes away; its ${group.members} contacts stay.") },
            confirmButton = { TextButton(onClick = { store.deleteGroup(group.id); confirmDelete = false; onDismiss() }) { Text("Delete", color = PrioHigh) } },
            dismissButton = { TextButton(onClick = { confirmDelete = false }) { Text("Cancel") } },
            containerColor = p.card,
        )
    }
}
