package org.opentick.android.ui.people

import android.content.Intent
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Email
import androidx.compose.material.icons.outlined.Lock
import androidx.compose.material.icons.outlined.Phone
import androidx.compose.material.icons.outlined.Sms
import androidx.compose.material.icons.outlined.Work
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material.icons.rounded.Close
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
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import org.opentick.android.data.ApiContact
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.OtCard
import org.opentick.android.ui.components.PageHeader
import org.opentick.android.ui.habits.otFieldColors
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.ui.theme.parseHex
import java.io.ByteArrayOutputStream
import java.time.OffsetDateTime
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/** One editable entry of a contact's history: experiences, educations or notes. */
private data class EntryEdit(val kind: String, val id: Long?, val values: Map<String, String>)

private val EXPERIENCE_FIELDS = listOf("title" to "Title", "company" to "Company", "location" to "Location", "start_date" to "Start (e.g. 2021-09)", "end_date" to "End (empty if current)", "description" to "Description")
private val EDUCATION_FIELDS = listOf("school" to "School", "degree" to "Degree", "field_of_study" to "Field of study", "start_year" to "Start year", "end_year" to "End year")

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ContactDetail(vm: MainViewModel, id: Long) {
    val store = vm.people
    val p = LocalExtra.current
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val contact by store.open.collectAsState()
    val rows by store.contacts.collectAsState()
    val groups by store.groups.collectAsState()
    val tags by store.tags.collectAsState()
    val row = rows.firstOrNull { it.id == id }
    val c = contact?.takeIf { it.id == id }
    var editField by remember { mutableStateOf<ContactField?>(null) }
    var unlockField by remember { mutableStateOf<ContactField?>(null) }
    var entry by remember { mutableStateOf<EntryEdit?>(null) }
    var picking by remember { mutableStateOf(false) }
    var addingTag by remember { mutableStateOf(false) }
    var confirmDelete by remember { mutableStateOf(false) }
    var photoMenu by remember { mutableStateOf(false) }

    val pickPhoto = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        if (uri != null) scope.launch {
            val bytes = withContext(Dispatchers.IO) { runCatching { scaledJpeg(context, uri) }.getOrNull() }
            if (bytes == null) store.message.value = "Couldn't read this image." else store.setPhoto(id, bytes)
        }
    }

    fun open(uri: String, action: String = Intent.ACTION_VIEW) {
        runCatching { context.startActivity(Intent(action, Uri.parse(uri)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
            .onFailure { store.message.value = "No app can open this." }
    }

    val name = c?.name ?: row?.name ?: ""
    Column(Modifier.fillMaxSize()) {
        PageHeader(name.ifBlank { "Contact" }, onBack = store::closeContact, actions = {
            IconButton(onClick = { confirmDelete = true }) { Icon(Icons.Outlined.Delete, "Delete contact", tint = p.icon) }
        })
        if (c == null) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator(color = Accent) }
            return@Column
        }
        LazyColumn(contentPadding = PaddingValues(start = 12.dp, end = 12.dp, bottom = 32.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            item("head") {
                Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
                    Box(Modifier.clip(CircleShape).clickable { photoMenu = true }) { ContactAvatar(store, c.id, c.name, c.hasPhoto, 88) }
                    Spacer(Modifier.height(10.dp))
                    Text(c.name.ifBlank { "Unnamed" }, fontSize = 20.sp, fontWeight = FontWeight.Bold, color = p.text, textAlign = TextAlign.Center)
                    val sub = contactSubtitle(c.toRow())
                    if (sub.isNotBlank()) Text(sub, fontSize = 14.sp, color = p.textSecondary, textAlign = TextAlign.Center)
                    if (c.location.isNotBlank()) Text(c.location, fontSize = 13.sp, color = p.textTertiary, textAlign = TextAlign.Center)
                    Spacer(Modifier.height(14.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        val phone = c.phone.filter { it.isDigit() || it == '+' }
                        QuickAction(Icons.Outlined.Phone, "Call", phone.isNotEmpty()) { open("tel:$phone", Intent.ACTION_DIAL) }
                        QuickAction(Icons.Outlined.Sms, "Message", phone.isNotEmpty()) { open("sms:$phone") }
                        QuickAction(Icons.Outlined.Email, "Email", c.email.isNotBlank()) { open("mailto:${c.email.trim()}", Intent.ACTION_SENDTO) }
                        QuickAction(Icons.Outlined.Work, "LinkedIn", c.linkedinUrl.isNotBlank()) { open(c.linkedinUrl.trim()) }
                    }
                }
            }
            item("chips") {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    for (g in groups.filter { it.id in c.groupIds.orEmpty() }) {
                        Chip(g.name, parseHex(nameColor(g.name, g.hexColor))) { store.setGroup(c.id, g.id.toString(), false) }
                    }
                    for (t in tags.filter { it.id in c.tagIds.orEmpty() }) {
                        Chip("#" + t.name, null) { store.setTag(c.id, t.id.toString(), false) }
                    }
                    AddChip("Group") { picking = true }
                    AddChip("Tag") { addingTag = true }
                }
            }
            item("fields") {
                // Only filled-in fields; the empty ones are one tap away under "Add details".
                val filled = CONTACT_FIELDS.filter { c.field(it.key).isNotBlank() }
                val empty = CONTACT_FIELDS - filled.toSet()
                var addMenu by remember { mutableStateOf(false) }
                OtCard {
                    for ((i, f) in filled.withIndex()) {
                        if (i > 0) Box(Modifier.fillMaxWidth().padding(start = 16.dp).height(0.5.dp).background(p.divider))
                        Row(Modifier.fillMaxWidth().clickable { editField = f }.padding(horizontal = 16.dp, vertical = 11.dp), verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text(f.label, fontSize = 12.sp, color = p.textTertiary)
                                Text(c.field(f.key), fontSize = 15.sp, color = p.text, maxLines = if (f.long) 12 else 2)
                            }
                            if (f.key in c.lockedFields.orEmpty()) {
                                IconButton(onClick = { unlockField = f }) { Icon(Icons.Outlined.Lock, "Edited by hand; tap to unlock", tint = p.textTertiary, modifier = Modifier.size(16.dp)) }
                            }
                        }
                    }
                    if (empty.isNotEmpty()) {
                        if (filled.isNotEmpty()) Box(Modifier.fillMaxWidth().padding(start = 16.dp).height(0.5.dp).background(p.divider))
                        Box {
                            Row(Modifier.fillMaxWidth().clickable { addMenu = true }.padding(horizontal = 16.dp, vertical = 14.dp), verticalAlignment = Alignment.CenterVertically) {
                                Icon(Icons.Rounded.Add, null, tint = Accent, modifier = Modifier.size(18.dp))
                                Spacer(Modifier.width(8.dp))
                                Text("Add details", fontSize = 15.sp, color = Accent)
                            }
                            DropdownMenu(addMenu, { addMenu = false }, containerColor = p.card) {
                                for (f in empty) DropdownMenuItem(text = { Text(f.label, color = p.text) }, onClick = { addMenu = false; editField = f })
                            }
                        }
                    }
                }
            }
            item("experience") {
                Section("Experience", onAdd = { entry = EntryEdit("experiences", null, emptyMap()) }) {
                    val list = c.experiences.orEmpty()
                    if (list.isEmpty()) Empty("No jobs yet")
                    for (e in list) {
                        HistoryRow(
                            e.title.ifBlank { e.company }, if (e.title.isNotBlank()) e.company else "",
                            listOf(span(e.startDate, e.endDate.ifBlank { if (e.startDate.isNotBlank()) "Present" else "" }), e.location).filter { it.isNotBlank() }.joinToString(" · "),
                            e.description,
                        ) {
                            entry = EntryEdit("experiences", e.id, mapOf("title" to e.title, "company" to e.company, "location" to e.location, "start_date" to e.startDate, "end_date" to e.endDate, "description" to e.description))
                        }
                    }
                }
            }
            item("education") {
                Section("Education", onAdd = { entry = EntryEdit("educations", null, emptyMap()) }) {
                    val list = c.educations.orEmpty()
                    if (list.isEmpty()) Empty("No schools yet")
                    for (e in list) {
                        HistoryRow(e.school, listOf(e.degree, e.fieldOfStudy).filter { it.isNotBlank() }.joinToString(", "), span(e.startYear, e.endYear), "") {
                            entry = EntryEdit("educations", e.id, mapOf("school" to e.school, "degree" to e.degree, "field_of_study" to e.fieldOfStudy, "start_year" to e.startYear, "end_year" to e.endYear))
                        }
                    }
                }
            }
            item("notes") {
                Section("Notes", onAdd = { entry = EntryEdit("notes", null, emptyMap()) }) {
                    val list = c.notes.orEmpty()
                    if (list.isEmpty()) Empty("No notes yet")
                    for (n in list) {
                        HistoryRow(n.body, "", fmtStamp(n.created), "") { entry = EntryEdit("notes", n.id, mapOf("body" to n.body)) }
                    }
                }
            }
            if (c.source.isNotBlank() && c.source != "manual") {
                item("source") { Text("Imported from ${c.source}", fontSize = 12.sp, color = p.textTertiary, modifier = Modifier.padding(start = 6.dp)) }
            }
        }
    }

    editField?.let { f -> FieldDialog(f, c?.field(f.key).orEmpty(), { editField = null }) { v -> store.setField(id, f.key, v); editField = null } }
    unlockField?.let { f ->
        AlertDialog(
            onDismissRequest = { unlockField = null },
            title = { Text("Unlock ${f.label}?") },
            text = { Text("You edited this by hand, so imports and enrichment leave it alone. Unlock it to let them update it again.") },
            confirmButton = { TextButton(onClick = { c?.let { store.unlock(it, f.key) }; unlockField = null }) { Text("Unlock") } },
            dismissButton = { TextButton(onClick = { unlockField = null }) { Text("Cancel") } },
            containerColor = p.card,
        )
    }
    entry?.let { e -> EntrySheet(store, id, e) { entry = null } }
    if (picking && c != null) GroupPicker(store, c) { picking = false }
    if (addingTag && c != null) TagDialog(store, c) { addingTag = false }
    if (photoMenu && c != null) {
        AlertDialog(
            onDismissRequest = { photoMenu = false },
            title = { Text("Photo") },
            confirmButton = {
                TextButton(onClick = { photoMenu = false; pickPhoto.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) }) {
                    Text(if (c.hasPhoto) "Change photo" else "Choose photo")
                }
            },
            dismissButton = {
                if (c.hasPhoto) TextButton(onClick = { photoMenu = false; store.setPhoto(id, null) }) { Text("Remove", color = PrioHigh) }
                else TextButton(onClick = { photoMenu = false }) { Text("Cancel") }
            },
            containerColor = p.card,
        )
    }
    if (confirmDelete) {
        AlertDialog(
            onDismissRequest = { confirmDelete = false },
            title = { Text("Delete ${name.ifBlank { "this contact" }}?") },
            text = { Text("Their history and notes are deleted too.") },
            confirmButton = { TextButton(onClick = { confirmDelete = false; store.delete(id) }) { Text("Delete", color = PrioHigh) } },
            dismissButton = { TextButton(onClick = { confirmDelete = false }) { Text("Cancel") } },
            containerColor = p.card,
        )
    }
}

/** "2019 – 2021", or whichever end there is. */
private fun span(from: String, to: String) = listOf(from, to).filter { it.isNotBlank() }.joinToString(" – ")

private fun fmtStamp(s: String): String = runCatching {
    OffsetDateTime.parse(s).atZoneSameInstant(java.time.ZoneId.systemDefault()).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM))
}.getOrDefault("")

/** Reads a picked image, scaled to at most 1024 px and re-encoded as JPEG (the server takes 8 MB, and no HEIC). */
private fun scaledJpeg(context: android.content.Context, uri: Uri): ByteArray? {
    val cr = context.contentResolver
    val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
    cr.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, bounds) }
    var sample = 1
    while (bounds.outWidth / (sample * 2) >= 1024 || bounds.outHeight / (sample * 2) >= 1024) sample *= 2
    val bmp = cr.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, BitmapFactory.Options().apply { inSampleSize = sample }) } ?: return null
    return ByteArrayOutputStream().use { out -> bmp.compress(Bitmap.CompressFormat.JPEG, 90, out); out.toByteArray() }
}

@Composable
private fun QuickAction(icon: ImageVector, label: String, enabled: Boolean, onClick: () -> Unit) {
    val p = LocalExtra.current
    Column(
        Modifier.width(74.dp).clip(RoundedCornerShape(14.dp)).background(p.card).clickable(enabled = enabled, onClick = onClick).padding(vertical = 10.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Icon(icon, null, tint = if (enabled) Accent else p.textTertiary.copy(alpha = 0.4f), modifier = Modifier.size(22.dp))
        Spacer(Modifier.height(4.dp))
        Text(label, fontSize = 12.sp, color = if (enabled) p.text else p.textTertiary.copy(alpha = 0.5f))
    }
}

@Composable
private fun Chip(label: String, dot: Color?, onRemove: () -> Unit) {
    val p = LocalExtra.current
    Row(Modifier.clip(RoundedCornerShape(14.dp)).background(p.card).padding(start = 10.dp, end = 4.dp, top = 4.dp, bottom = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        if (dot != null) {
            Box(Modifier.size(8.dp).clip(CircleShape).background(dot))
            Spacer(Modifier.width(6.dp))
        }
        Text(label, fontSize = 13.sp, color = p.text)
        Icon(Icons.Rounded.Close, "Remove $label", tint = p.textTertiary, modifier = Modifier.padding(start = 2.dp).size(20.dp).clip(CircleShape).clickable(onClick = onRemove).padding(3.dp))
    }
}

@Composable
private fun AddChip(label: String, onClick: () -> Unit) {
    Row(Modifier.clip(RoundedCornerShape(14.dp)).background(Accent.copy(alpha = 0.1f)).clickable(onClick = onClick).padding(horizontal = 10.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
        Icon(Icons.Rounded.Add, null, tint = Accent, modifier = Modifier.size(14.dp))
        Spacer(Modifier.width(3.dp))
        Text(label, fontSize = 13.sp, color = Accent)
    }
}

@Composable
private fun Section(title: String, onAdd: () -> Unit, content: @Composable () -> Unit) {
    val p = LocalExtra.current
    OtCard {
        Row(Modifier.fillMaxWidth().padding(start = 16.dp, end = 4.dp, top = 4.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(title, fontWeight = FontWeight.Bold, fontSize = 15.sp, color = p.text, modifier = Modifier.weight(1f))
            IconButton(onClick = onAdd) { Icon(Icons.Rounded.Add, "Add to $title", tint = Accent) }
        }
        content()
        Spacer(Modifier.height(6.dp))
    }
}

@Composable
private fun Empty(text: String) {
    Text(text, fontSize = 14.sp, color = LocalExtra.current.textTertiary, modifier = Modifier.padding(start = 16.dp, bottom = 8.dp))
}

@Composable
private fun HistoryRow(title: String, subtitle: String, meta: String, body: String, onClick: () -> Unit) {
    val p = LocalExtra.current
    Column(Modifier.fillMaxWidth().clickable(onClick = onClick).padding(horizontal = 16.dp, vertical = 8.dp)) {
        Text(title, fontSize = 15.sp, color = p.text)
        if (subtitle.isNotBlank()) Text(subtitle, fontSize = 13.sp, color = p.textSecondary)
        if (meta.isNotBlank()) Text(meta, fontSize = 12.sp, color = p.textTertiary)
        if (body.isNotBlank()) Text(body, fontSize = 13.sp, color = p.textSecondary, maxLines = 4, modifier = Modifier.padding(top = 2.dp))
    }
}

@Composable
private fun FieldDialog(f: ContactField, initial: String, onDismiss: () -> Unit, onSave: (String) -> Unit) {
    val p = LocalExtra.current
    var v by remember { mutableStateOf(initial) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(f.label) },
        text = {
            OutlinedTextField(
                v, { v = it }, singleLine = !f.long, minLines = if (f.long) 4 else 1, colors = otFieldColors(), shape = RoundedCornerShape(12.dp),
                placeholder = if (f.hint.isNotEmpty()) ({ Text(f.hint, color = p.textTertiary) }) else null,
                modifier = Modifier.fillMaxWidth(),
            )
        },
        confirmButton = { TextButton(onClick = { if (v != initial) onSave(v) else onDismiss() }) { Text("Save") } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
        containerColor = p.card,
    )
}

/** Adds or edits a job, a school or a note. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun EntrySheet(store: PeopleStore, contactId: Long, e: EntryEdit, onDismiss: () -> Unit) {
    val p = LocalExtra.current
    val fields = when (e.kind) {
        "experiences" -> EXPERIENCE_FIELDS
        "educations" -> EDUCATION_FIELDS
        else -> listOf("body" to "Note")
    }
    val title = when (e.kind) {
        "experiences" -> if (e.id == null) "Add job" else "Edit job"
        "educations" -> if (e.id == null) "Add school" else "Edit school"
        else -> if (e.id == null) "Add note" else "Edit note"
    }
    var values by remember { mutableStateOf(e.values) }
    var confirmDelete by remember { mutableStateOf(false) }
    val valid = when (e.kind) {
        "experiences" -> !values["company"].isNullOrBlank() || !values["title"].isNullOrBlank()
        "educations" -> !values["school"].isNullOrBlank()
        else -> !values["body"].isNullOrBlank()
    }
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).imePadding().padding(horizontal = 20.dp)) {
            Text(title, fontWeight = FontWeight.Bold, fontSize = 18.sp, color = p.text)
            Spacer(Modifier.height(8.dp))
            for ((key, label) in fields) {
                val long = key == "description" || key == "body"
                OutlinedTextField(
                    values[key].orEmpty(), { values = values + (key to it) }, label = { Text(label) },
                    singleLine = !long, minLines = if (long) 3 else 1, colors = otFieldColors(), shape = RoundedCornerShape(12.dp),
                    modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
                )
            }
            Row(Modifier.fillMaxWidth().padding(top = 14.dp), verticalAlignment = Alignment.CenterVertically) {
                if (e.id != null) TextButton(onClick = { confirmDelete = true }) { Text("Delete", color = PrioHigh) }
                Spacer(Modifier.weight(1f))
                Button(
                    onClick = {
                        val body = buildJsonObject { for ((k, _) in fields) put(k, values[k].orEmpty().trim()) }
                        store.saveEntry(contactId, e.kind, e.id, body)
                        onDismiss()
                    },
                    enabled = valid, colors = ButtonDefaults.buttonColors(containerColor = Accent), shape = RoundedCornerShape(12.dp),
                ) { Text("Save") }
            }
            Spacer(Modifier.height(24.dp))
        }
    }
    if (confirmDelete && e.id != null) {
        AlertDialog(
            onDismissRequest = { confirmDelete = false },
            title = { Text("Delete this entry?") },
            confirmButton = { TextButton(onClick = { store.deleteEntry(contactId, e.kind, e.id); confirmDelete = false; onDismiss() }) { Text("Delete", color = PrioHigh) } },
            dismissButton = { TextButton(onClick = { confirmDelete = false }) { Text("Cancel") } },
            containerColor = p.card,
        )
    }
}

/** Toggles the contact's groups, or puts it in a new one. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun GroupPicker(store: PeopleStore, c: ApiContact, onDismiss: () -> Unit) {
    val p = LocalExtra.current
    val groups by store.groups.collectAsState()
    val open by store.open.collectAsState()
    val inGroups = (open?.takeIf { it.id == c.id } ?: c).groupIds.orEmpty()
    var newName by remember { mutableStateOf("") }
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).imePadding()) {
            Text("Groups", fontWeight = FontWeight.Bold, fontSize = 18.sp, color = p.text, modifier = Modifier.padding(horizontal = 20.dp))
            Spacer(Modifier.height(6.dp))
            for (g in groups) {
                val on = g.id in inGroups
                Row(Modifier.fillMaxWidth().clickable { store.setGroup(c.id, g.id.toString(), !on) }.padding(horizontal = 20.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                    Box(Modifier.size(10.dp).clip(CircleShape).background(parseHex(nameColor(g.name, g.hexColor))))
                    Spacer(Modifier.width(12.dp))
                    Text(g.name, fontSize = 16.sp, color = p.text, modifier = Modifier.weight(1f))
                    if (on) Icon(Icons.Rounded.Check, "In group", tint = Accent)
                }
            }
            Row(Modifier.padding(horizontal = 20.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(newName, { newName = it.take(250) }, placeholder = { Text("New group", color = p.textTertiary) }, singleLine = true, colors = otFieldColors(), shape = RoundedCornerShape(12.dp), modifier = Modifier.weight(1f))
                Spacer(Modifier.width(8.dp))
                Button(
                    onClick = { store.setGroup(c.id, newName.trim(), true, create = true); newName = "" },
                    enabled = newName.isNotBlank(), colors = ButtonDefaults.buttonColors(containerColor = Accent), shape = RoundedCornerShape(12.dp),
                ) { Text("Add") }
            }
            Spacer(Modifier.height(24.dp))
        }
    }
}

/** Tags the contact with an existing or a new tag. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TagDialog(store: PeopleStore, c: ApiContact, onDismiss: () -> Unit) {
    val p = LocalExtra.current
    val tags by store.tags.collectAsState()
    var name by remember { mutableStateOf("") }
    val q = fold(name)
    val suggestions = tags.filter { it.id !in c.tagIds.orEmpty() && (q.isEmpty() || q in fold(it.name)) }.take(12)
    fun add(tag: String) { if (tag.isNotBlank()) store.setTag(c.id, tag.trim().removePrefix("#"), true); onDismiss() }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Add tag") },
        text = {
            Column {
                OutlinedTextField(name, { name = it.take(250) }, placeholder = { Text("Tag name", color = p.textTertiary) }, singleLine = true, colors = otFieldColors(), shape = RoundedCornerShape(12.dp), modifier = Modifier.fillMaxWidth())
                if (suggestions.isNotEmpty()) {
                    Spacer(Modifier.height(10.dp))
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        for (t in suggestions) AddChip("#" + t.name) { add(t.name) }
                    }
                }
            }
        },
        confirmButton = { TextButton(onClick = { add(name) }, enabled = name.isNotBlank()) { Text("Add") } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
        containerColor = p.card,
    )
}
