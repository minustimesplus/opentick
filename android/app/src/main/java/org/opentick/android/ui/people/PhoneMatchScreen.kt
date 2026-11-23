package org.opentick.android.ui.people

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ContactPhone
import androidx.compose.material.icons.outlined.Phone
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.RadioButton
import androidx.compose.material3.RadioButtonDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import org.opentick.android.ui.components.OtCard
import org.opentick.android.ui.components.PageHeader
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra

/**
 * Suggests numbers from the phone's contacts for people in People. The
 * phone's contacts are only read on the device; a number is saved only
 * when you add it.
 */
@Composable
fun PhoneMatchScreen(store: PeopleStore) {
    val p = LocalExtra.current
    val context = LocalContext.current
    val suggestions by store.phoneSuggestions.collectAsState()
    val scanning by store.phoneScanning.collectAsState()
    var confirmAll by remember { mutableStateOf(false) }

    fun permitted() = ContextCompat.checkSelfPermission(context, Manifest.permission.READ_CONTACTS) == PackageManager.PERMISSION_GRANTED
    fun scan() = store.scanPhone { readDeviceContacts(context.applicationContext) }
    val askContacts = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) scan() else store.message.value = "Without access to your contacts there's nothing to compare. You can allow it in the system settings."
    }
    // Opened again with access already given: look again right away.
    LaunchedEffect(Unit) { if (suggestions == null && permitted()) scan() }

    Column(Modifier.fillMaxSize()) {
        PageHeader("Phone contacts", onBack = { store.matchingPhone.value = false }, actions = {
            if (suggestions != null) IconButton(onClick = { scan() }, enabled = !scanning) { Icon(Icons.Rounded.Refresh, "Scan again", tint = p.icon) }
        })
        val list = suggestions
        when {
            list == null && scanning -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    CircularProgressIndicator(color = Accent)
                    Spacer(Modifier.height(12.dp))
                    Text("Comparing your phone's contacts…", color = p.textTertiary)
                }
            }
            list == null -> Intro { if (permitted()) scan() else askContacts.launch(Manifest.permission.READ_CONTACTS) }
            list.isEmpty() -> Column(Modifier.fillMaxSize().padding(32.dp), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
                Text("No new numbers", fontSize = 17.sp, fontWeight = FontWeight.SemiBold, color = p.text)
                Spacer(Modifier.height(6.dp))
                Text(
                    "Everyone in People who matches a contact on this phone already has that number.",
                    fontSize = 14.sp, color = p.textTertiary, textAlign = TextAlign.Center,
                )
                if (store.hasIgnoredPhone()) TextButton(onClick = { store.resetIgnoredPhone() }) { Text("Show ignored suggestions", color = Accent) }
            }
            else -> LazyColumn(contentPadding = PaddingValues(start = 12.dp, end = 12.dp, bottom = 32.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                item("head") {
                    Row(Modifier.fillMaxWidth().padding(start = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            "${list.size} ${if (list.size == 1) "suggestion" else "suggestions"}. Nothing is saved until you add it.",
                            fontSize = 13.sp, color = p.textTertiary, modifier = Modifier.weight(1f),
                        )
                        if (list.size > 1) TextButton(onClick = { confirmAll = true }) { Text("Add all", color = Accent) }
                    }
                }
                items(list, key = { it.contact.id }) { s -> SuggestionCard(store, s) }
                if (store.hasIgnoredPhone()) item("ignored") {
                    TextButton(onClick = { store.resetIgnoredPhone() }, modifier = Modifier.fillMaxWidth()) { Text("Show ignored suggestions", color = p.textSecondary) }
                }
            }
        }
    }

    if (confirmAll) {
        val n = suggestions.orEmpty().size
        val replacing = suggestions.orEmpty().count { it.contact.phone.isNotBlank() }
        AlertDialog(
            onDismissRequest = { confirmAll = false },
            title = { Text("Add $n numbers?") },
            text = {
                Text(
                    "Each person gets the first number shown on their card." +
                        if (replacing > 0) " $replacing of them already have a number, which is replaced." else "",
                )
            },
            confirmButton = { TextButton(onClick = { confirmAll = false; store.acceptAllPhone() }) { Text("Add all") } },
            dismissButton = { TextButton(onClick = { confirmAll = false }) { Text("Cancel") } },
            containerColor = p.card,
        )
    }
}

@Composable
private fun Intro(onStart: () -> Unit) {
    val p = LocalExtra.current
    Column(Modifier.fillMaxSize().padding(32.dp), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
        Box(Modifier.size(96.dp).background(Accent.copy(alpha = 0.08f), CircleShape), contentAlignment = Alignment.Center) {
            Icon(Icons.Outlined.ContactPhone, null, tint = Accent, modifier = Modifier.size(44.dp))
        }
        Spacer(Modifier.height(16.dp))
        Text("Find numbers for your people", fontSize = 17.sp, fontWeight = FontWeight.SemiBold, color = p.text, textAlign = TextAlign.Center)
        Spacer(Modifier.height(8.dp))
        Text(
            "OpenTick compares the contacts on this phone with People by name and email and suggests numbers to add. " +
                "Nothing changes until you approve a suggestion, and your phone's contacts are never uploaded.",
            fontSize = 14.sp, color = p.textTertiary, textAlign = TextAlign.Center,
        )
        Spacer(Modifier.height(20.dp))
        Button(onClick = onStart, colors = ButtonDefaults.buttonColors(containerColor = Accent), shape = RoundedCornerShape(12.dp)) { Text("Compare phone contacts") }
    }
}

@Composable
private fun SuggestionCard(store: PeopleStore, s: PhoneSuggestion) {
    val p = LocalExtra.current
    var picked by remember(s.key) { mutableIntStateOf(0) }
    val c = s.contact
    OtCard {
        Row(Modifier.fillMaxWidth().padding(start = 14.dp, end = 14.dp, top = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            ContactAvatar(store, c.id, c.name, c.hasPhoto, 40)
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(c.name.ifBlank { c.email.ifBlank { "Unnamed" } }, fontSize = 15.sp, fontWeight = FontWeight.Medium, color = p.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                val from = s.options.map { it.from }.distinct().joinToString(", ")
                Text("${s.reason.label} “$from” on this phone", fontSize = 12.sp, color = p.textTertiary, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
        }
        if (c.phone.isNotBlank()) {
            Text("Now: ${c.phone}", fontSize = 13.sp, color = p.textSecondary, modifier = Modifier.padding(start = 66.dp, top = 6.dp))
        }
        Spacer(Modifier.height(4.dp))
        for ((i, o) in s.options.withIndex()) {
            Row(
                Modifier.fillMaxWidth().clip(RoundedCornerShape(8.dp)).clickable(enabled = s.options.size > 1) { picked = i }
                    .padding(start = if (s.options.size > 1) 52.dp else 66.dp, end = 14.dp, top = 2.dp, bottom = 2.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                if (s.options.size > 1) {
                    RadioButton(selected = picked == i, onClick = { picked = i }, colors = RadioButtonDefaults.colors(selectedColor = Accent))
                } else {
                    Icon(Icons.Outlined.Phone, null, tint = Accent, modifier = Modifier.size(16.dp))
                    Spacer(Modifier.width(8.dp))
                }
                Text(o.number, fontSize = 15.sp, color = p.text)
                if (o.label.isNotBlank()) Text("  ${o.label}", fontSize = 13.sp, color = p.textTertiary)
            }
        }
        Row(Modifier.fillMaxWidth().padding(start = 14.dp, end = 10.dp, bottom = 8.dp), horizontalArrangement = Arrangement.End, verticalAlignment = Alignment.CenterVertically) {
            TextButton(onClick = { store.ignorePhone(s) }) { Text("Ignore", color = p.textSecondary) }
            Spacer(Modifier.width(6.dp))
            Button(
                onClick = { store.acceptPhone(s, s.options[picked].number) },
                colors = ButtonDefaults.buttonColors(containerColor = Accent), shape = RoundedCornerShape(12.dp),
            ) { Text(if (c.phone.isBlank()) "Add number" else "Replace number") }
        }
    }
}
