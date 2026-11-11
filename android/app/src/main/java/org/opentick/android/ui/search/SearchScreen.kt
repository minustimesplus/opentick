package org.opentick.android.ui.search

import androidx.compose.foundation.background
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
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.TaskRowContent
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.util.splitEmoji

/** Search across titles, descriptions, tags and lists. Works offline. */
@Composable
fun SearchScreen(vm: MainViewModel) {
    val p = LocalExtra.current
    val data by vm.data.collectAsState()
    var q by rememberSaveable { mutableStateOf("") }
    val focus = remember { FocusRequester() }
    LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }

    Column(Modifier.fillMaxSize().background(p.page)) {
        Row(Modifier.statusBarsPadding().height(64.dp).padding(start = 6.dp, end = 16.dp), verticalAlignment = Alignment.CenterVertically) {
            IconButton(onClick = vm::back) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back", tint = p.icon) }
            Row(
                Modifier.weight(1f).height(42.dp).clip(RoundedCornerShape(21.dp)).background(p.card).padding(horizontal = 14.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(Icons.Outlined.Search, null, tint = p.textTertiary, modifier = Modifier.size(20.dp))
                Spacer(Modifier.width(8.dp))
                Box(Modifier.weight(1f)) {
                    if (q.isEmpty()) Text("Search tasks", fontSize = 15.sp, color = p.textTertiary)
                    BasicTextField(q, { q = it }, singleLine = true, textStyle = TextStyle(fontSize = 15.sp, color = p.text), cursorBrush = SolidColor(Accent), modifier = Modifier.fillMaxWidth().focusRequester(focus))
                }
            }
        }
        val results = if (q.isBlank()) emptyList() else data.tasks.filter { t ->
            t.title.contains(q, true) || t.description.contains(q, true) ||
                t.labelIdList.any { data.labelMap[it]?.title?.contains(q.removePrefix("#"), true) == true } ||
                data.projectMap[t.projectId]?.title?.contains(q, true) == true
        }.sortedWith(compareBy({ it.done }, { it.dueDate ?: Long.MAX_VALUE }))
        if (q.isNotBlank() && results.isEmpty()) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { Text("No results", color = p.textTertiary) }
        }
        LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            for ((pid, list) in results.groupBy { it.projectId }) {
                item(key = pid) {
                    Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(p.card).padding(bottom = 6.dp)) {
                        Text(splitEmoji(data.projectMap[pid]?.title ?: "Inbox").let { (e, n) -> listOfNotNull(e, n).joinToString(" ") }, fontSize = 16.sp, fontWeight = FontWeight.Bold, color = p.text, modifier = Modifier.padding(start = 18.dp, top = 15.dp, bottom = 6.dp))
                        for (t in list) TaskRowContent(t, data.projectMap[t.projectId], t.labelIdList.mapNotNull { data.labelMap[it] }, false, { vm.toggleDone(t) }, { vm.openTaskId.value = t.id }, { vm.actionsTaskId.value = t.id })
                    }
                }
            }
        }
    }
}
