package org.opentick.android.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Flag
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.Inbox
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import org.opentick.android.data.LabelEntity
import org.opentick.android.data.ProjectEntity
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.parseHex
import org.opentick.android.ui.theme.priorityColor

val PRIORITIES = listOf(3 to "High", 2 to "Medium", 1 to "Low", 0 to "None")

@Composable
fun PriorityMenu(expanded: Boolean, current: Int, onPick: (Int) -> Unit, onDismiss: () -> Unit) {
    val none = LocalExtra.current.prioNone
    DropdownMenu(expanded = expanded, onDismissRequest = onDismiss) {
        for ((value, name) in PRIORITIES) {
            DropdownMenuItem(
                text = { Text("$name priority") },
                leadingIcon = { Icon(Icons.Filled.Flag, null, tint = priorityColor(value, none)) },
                trailingIcon = { if (minOf(current, 3) == value) Icon(Icons.Outlined.Check, null, tint = Accent) },
                onClick = { onPick(value); onDismiss() },
            )
        }
    }
}

@Composable
fun ProjectMenu(expanded: Boolean, projects: List<ProjectEntity>, current: Long?, onPick: (Long) -> Unit, onDismiss: () -> Unit) {
    DropdownMenu(expanded = expanded, onDismissRequest = onDismiss) {
        for (p in projects) {
            DropdownMenuItem(
                text = { Text(p.title) },
                leadingIcon = {
                    if (p.isInbox) Icon(Icons.Outlined.Inbox, null)
                    else Box(Modifier.size(10.dp).background(parseHex(p.hexColor), CircleShape))
                },
                trailingIcon = { if (p.id == current) Icon(Icons.Outlined.Check, null, tint = Accent) },
                onClick = { onPick(p.id); onDismiss() },
            )
        }
    }
}

/** Toggle tags on and off; can create a new tag by typing it. */
@Composable
fun LabelMenu(expanded: Boolean, labels: List<LabelEntity>, selected: List<String>, onChange: (List<String>) -> Unit, onDismiss: () -> Unit) {
    var newTag by remember { mutableStateOf("") }
    DropdownMenu(expanded = expanded, onDismissRequest = { newTag = ""; onDismiss() }) {
        OutlinedTextField(
            value = newTag,
            onValueChange = { newTag = it.replace(" ", "") },
            placeholder = { Text("New tag") },
            singleLine = true,
            modifier = Modifier.size(width = 220.dp, height = 56.dp),
            trailingIcon = {
                if (newTag.isNotBlank()) Icon(Icons.Outlined.Add, "Add", Modifier.clickable {
                    onChange((selected + newTag.removePrefix("#")).distinct()); newTag = ""
                })
            },
        )
        if (labels.isNotEmpty()) HorizontalDivider()
        for (l in labels) {
            val on = selected.any { it.equals(l.title, true) }
            DropdownMenuItem(
                text = { Text(l.title) },
                leadingIcon = { Box(Modifier.size(10.dp).background(parseHex(l.hexColor), CircleShape)) },
                trailingIcon = { if (on) Icon(Icons.Outlined.Check, null, tint = Accent) },
                onClick = { onChange(if (on) selected.filterNot { it.equals(l.title, true) } else selected + l.title) },
            )
        }
    }
}
