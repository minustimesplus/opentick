package org.opentick.android.ui.habits

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.ApiHabit
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.ui.theme.parseHex
import java.time.format.TextStyle
import java.util.Locale

/** Same choices as the web's habit dialog. */
val HABIT_ICONS = listOf("💧", "🏃", "📚", "🧘", "🥗", "😴", "💪", "⚖️", "🚭", "🎸", "✍️", "🌱", "☀️")
val HABIT_COLORS = listOf("E03131", "F76707", "FAB005", "40C057", "12B886", "15AABF", "4772FA", "7048E8", "BE4BDB", "E64980", "868E96")

@Composable
fun otFieldColors() = LocalExtra.current.let { p ->
    OutlinedTextFieldDefaults.colors(
        focusedBorderColor = Accent, unfocusedBorderColor = p.divider, focusedTextColor = p.text, unfocusedTextColor = p.text,
        cursorColor = Accent, focusedLabelColor = Accent, unfocusedLabelColor = p.textTertiary,
    )
}

/** Creates a habit ([habit] null) or edits one; editing also offers Archive and Delete. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HabitEditSheet(store: HabitsStore, habit: ApiHabit?, onDismiss: () -> Unit) {
    val p = LocalExtra.current
    var title by remember { mutableStateOf(habit?.title ?: "") }
    var icon by remember { mutableStateOf(habit?.icon?.ifBlank { null } ?: HABIT_ICONS[0]) }
    var color by remember { mutableStateOf(habit?.hexColor?.removePrefix("#")?.ifBlank { null } ?: DEFAULT_HABIT_COLOR) }
    var daily by remember { mutableStateOf(habit?.frequency != "weekdays") }
    var weekdays by remember { mutableIntStateOf(if (habit?.frequency == "weekdays") habit.weekdays else ALL_WEEKDAYS) }
    var goal by remember { mutableStateOf((habit?.goal ?: 1).toString()) }
    var unit by remember { mutableStateOf(habit?.unit ?: "") }
    var tracksWeight by remember { mutableStateOf(habit?.tracksWeight ?: false) }
    var saving by remember { mutableStateOf(false) }
    var confirmDelete by remember { mutableStateOf(false) }
    val valid = title.isNotBlank() && (daily || weekdays != 0)
    val weekStart = store.weekStart()

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).imePadding().padding(horizontal = 20.dp)) {
            Text(if (habit == null) "New habit" else "Edit habit", fontWeight = FontWeight.Bold, fontSize = 18.sp, color = p.text)
            Spacer(Modifier.height(14.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(48.dp).clip(CircleShape).background(parseHex(color).copy(alpha = 0.18f)), contentAlignment = Alignment.Center) {
                    Text(icon.ifBlank { "•" }, fontSize = 24.sp)
                }
                Spacer(Modifier.width(12.dp))
                OutlinedTextField(
                    value = title, onValueChange = { title = it.take(250) }, singleLine = true,
                    placeholder = { Text("e.g. Drink water", color = p.textTertiary) },
                    colors = otFieldColors(), shape = RoundedCornerShape(12.dp), modifier = Modifier.weight(1f),
                )
            }

            Label("Icon")
            Row(Modifier.horizontalScroll(rememberScrollState()), verticalAlignment = Alignment.CenterVertically) {
                for (i in HABIT_ICONS) {
                    Box(
                        Modifier.padding(end = 6.dp).size(40.dp).clip(RoundedCornerShape(10.dp))
                            .background(if (icon == i) Accent.copy(alpha = 0.15f) else Color.Transparent)
                            .clickable { icon = i },
                        contentAlignment = Alignment.Center,
                    ) { Text(i, fontSize = 20.sp) }
                }
                OutlinedTextField(
                    value = if (icon in HABIT_ICONS) "" else icon, onValueChange = { icon = it.take(8) }, singleLine = true,
                    placeholder = { Text("Other", color = p.textTertiary, fontSize = 13.sp) },
                    colors = otFieldColors(), shape = RoundedCornerShape(10.dp), modifier = Modifier.width(84.dp),
                )
            }

            Label("Color")
            Row(Modifier.horizontalScroll(rememberScrollState())) {
                for (c in HABIT_COLORS) {
                    Box(
                        Modifier.padding(end = 8.dp).size(30.dp).clip(CircleShape).background(parseHex(c)).clickable { color = c },
                        contentAlignment = Alignment.Center,
                    ) { if (color.equals(c, ignoreCase = true)) Icon(Icons.Rounded.Check, null, tint = Color.White, modifier = Modifier.size(18.dp)) }
                }
            }

            Label("Frequency")
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Pill("Every day", daily) { daily = true }
                Pill("Specific days", !daily) { daily = false }
            }
            if (!daily) {
                Spacer(Modifier.height(10.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    for (i in 0L until 7L) {
                        val d = weekStart.plus(i)
                        val bit = weekdayBit(d)
                        val on = weekdays and bit != 0
                        Box(
                            Modifier.size(38.dp).clip(CircleShape)
                                .background(if (on) Accent else Color.Transparent)
                                .border(1.dp, if (on) Accent else p.divider, CircleShape)
                                .clickable { weekdays = weekdays xor bit },
                            contentAlignment = Alignment.Center,
                        ) { Text(d.getDisplayName(TextStyle.NARROW, Locale.getDefault()), color = if (on) Color.White else p.text, fontSize = 13.sp) }
                    }
                }
            }

            Label("Daily goal")
            Row(verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(
                    value = goal, onValueChange = { goal = it.filter(Char::isDigit).take(6) }, singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    colors = otFieldColors(), shape = RoundedCornerShape(12.dp), modifier = Modifier.width(96.dp),
                )
                Spacer(Modifier.width(10.dp))
                OutlinedTextField(
                    value = unit, onValueChange = { unit = it.take(50) }, singleLine = true,
                    placeholder = { Text("Unit, e.g. glasses", color = p.textTertiary) },
                    colors = otFieldColors(), shape = RoundedCornerShape(12.dp), modifier = Modifier.weight(1f),
                )
            }

            Row(Modifier.fillMaxWidth().padding(top = 14.dp).clickable { tracksWeight = !tracksWeight }, verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("Weigh-in habit", fontSize = 15.sp, color = p.text)
                    Text("Checking in asks for your weight; logging a weight checks it off.", fontSize = 12.sp, color = p.textTertiary)
                }
                Switch(tracksWeight, { tracksWeight = it }, colors = SwitchDefaults.colors(checkedTrackColor = Accent))
            }

            Spacer(Modifier.height(18.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (habit != null) {
                    TextButton(onClick = { confirmDelete = true }) { Text("Delete", color = PrioHigh) }
                    TextButton(onClick = { store.setArchived(habit, !habit.archived); onDismiss() }) {
                        Text(if (habit.archived) "Unarchive" else "Archive", color = p.textSecondary)
                    }
                }
                Spacer(Modifier.weight(1f))
                Button(
                    onClick = {
                        if (!valid || saving) return@Button
                        saving = true
                        val input = HabitInput(title, icon, color, if (daily) "daily" else "weekdays", weekdays, goal.toIntOrNull() ?: 1, unit, tracksWeight)
                        val done: (ApiHabit) -> Unit = { onDismiss() }
                        if (habit == null) store.create(input) { done(it); store.openId.value = it.id } else store.update(habit.id, input.toJson(), done)
                        // Errors show in the snackbar; let the user try again.
                        saving = false
                    },
                    enabled = valid && !saving,
                    colors = ButtonDefaults.buttonColors(containerColor = Accent),
                    shape = RoundedCornerShape(12.dp),
                ) { Text("Save") }
            }
            Spacer(Modifier.height(24.dp))
        }
    }
    if (confirmDelete && habit != null) DeleteHabitDialog(habit, { confirmDelete = false }) { store.delete(habit.id); onDismiss() }
}

@Composable
private fun Label(text: String) {
    Text(text, fontSize = 13.sp, color = LocalExtra.current.textTertiary, modifier = Modifier.padding(top = 16.dp, bottom = 6.dp))
}

@Composable
fun Pill(text: String, on: Boolean, onClick: () -> Unit) {
    val p = LocalExtra.current
    Box(
        Modifier.clip(RoundedCornerShape(18.dp))
            .background(if (on) Accent.copy(alpha = 0.14f) else Color.Transparent)
            .border(1.dp, if (on) Accent else p.divider, RoundedCornerShape(18.dp))
            .clickable(onClick = onClick).padding(horizontal = 14.dp, vertical = 7.dp),
    ) { Text(text, color = if (on) Accent else p.text, fontSize = 14.sp) }
}
