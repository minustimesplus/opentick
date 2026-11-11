package org.opentick.android.ui.tasks

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.DeleteForever
import androidx.compose.material.icons.outlined.RestoreFromTrash
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.PageHeader
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh

/** Deleted tasks kept by the server; restore them or delete them for good. */
@Composable
fun TrashScreen(vm: MainViewModel) {
    val p = LocalExtra.current
    val state by vm.trash.collectAsState()
    var confirmEmpty by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) { vm.loadTrash() }
    Column(Modifier.fillMaxSize().background(p.page)) {
        PageHeader("Trash", onBack = vm::back, actions = {
            if ((state as? MainViewModel.TrashState.Loaded)?.tasks?.isNotEmpty() == true) {
                TextButton(onClick = { confirmEmpty = true }) { Text("Empty", color = PrioHigh) }
            }
        })
        when (val s = state) {
            is MainViewModel.TrashState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator(color = Accent) }
            is MainViewModel.TrashState.Failed -> Box(Modifier.fillMaxSize().padding(32.dp), contentAlignment = Alignment.Center) { Text(s.message, color = p.textTertiary, textAlign = TextAlign.Center) }
            is MainViewModel.TrashState.Loaded -> if (s.tasks.isEmpty()) {
                EmptyState("Trash is empty", "")
            } else {
                LazyColumn(contentPadding = PaddingValues(16.dp)) {
                    item {
                        Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(p.card).padding(vertical = 6.dp)) {
                            for (t in s.tasks) {
                                Row(Modifier.fillMaxWidth().padding(start = 18.dp, end = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                                    Text(t.title, fontSize = 16.sp, color = p.text, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                                    IconButton(onClick = { vm.restoreFromTrash(t.id) }) { Icon(Icons.Outlined.RestoreFromTrash, "Restore", tint = Accent) }
                                    IconButton(onClick = { vm.deleteForever(t.id) }) { Icon(Icons.Outlined.DeleteForever, "Delete forever", tint = PrioHigh) }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    if (confirmEmpty) {
        AlertDialog(
            onDismissRequest = { confirmEmpty = false },
            title = { Text("Empty trash?") },
            text = { Text("These tasks will be deleted for good.") },
            confirmButton = { TextButton(onClick = { confirmEmpty = false; vm.emptyTrash() }) { Text("Empty", color = PrioHigh) } },
            dismissButton = { TextButton(onClick = { confirmEmpty = false }) { Text("Cancel") } },
            containerColor = p.card,
        )
    }
}

