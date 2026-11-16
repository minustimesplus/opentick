package org.opentick.android.ui.habits

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
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
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowLeft
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.outlined.Archive
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.KeyboardArrowDown
import androidx.compose.material.icons.outlined.MonitorWeight
import androidx.compose.material.icons.outlined.Unarchive
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.ArrowDownward
import androidx.compose.material.icons.rounded.ArrowUpward
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material.icons.rounded.LocalFireDepartment
import androidx.compose.material.icons.rounded.VerticalAlignBottom
import androidx.compose.material.icons.rounded.VerticalAlignTop
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ModalBottomSheet
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
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.AccountMode
import org.opentick.android.data.ApiHabit
import org.opentick.android.data.ApiHabitStats
import org.opentick.android.data.ApiHabitStatsDay
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.calendar.weekStartDay
import org.opentick.android.ui.components.OtCard
import org.opentick.android.ui.components.PageHeader
import org.opentick.android.ui.tasks.EmptyState
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.Orange
import org.opentick.android.ui.theme.PrioHigh
import org.opentick.android.ui.theme.parseHex
import java.time.LocalDate
import java.time.YearMonth
import java.time.format.DateTimeFormatter
import java.time.format.TextStyle
import java.util.Locale

fun habitColor(h: ApiHabit?): Color = parseHex(h?.hexColor?.removePrefix("#")?.ifBlank { null } ?: DEFAULT_HABIT_COLOR)

/** The Habits tab: today's habits with the last 7 days, check-ins, the weight log and each habit's stats. */
@Composable
fun HabitsScreen(vm: MainViewModel) {
    val store = vm.habits
    val mode by vm.prefs.mode.collectAsState()
    val openId by store.openId.collectAsState()
    val snackbar = remember { SnackbarHostState() }
    val message by store.message.collectAsState()
    LaunchedEffect(message) { message?.let { snackbar.showSnackbar(it); store.message.value = null } }
    LaunchedEffect(mode) { store.load() }

    Box(Modifier.fillMaxSize()) {
        if (mode != AccountMode.SERVER) {
            Column(Modifier.fillMaxSize()) {
                PageHeader("Habits")
                EmptyState("Sign in to track habits", "Habits and your weight log are kept on your OpenTick server. Sign in from Settings → Account.")
            }
        } else {
            val habits by store.habits.collectAsState()
            val open = openId?.let { id -> habits.firstOrNull { it.id == id } }
            if (open != null) {
                BackHandler { store.openId.value = null }
                HabitDetail(vm, open) { store.openId.value = null }
            } else {
                HabitList(vm)
            }
        }
        SnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun HabitList(vm: MainViewModel) {
    val store = vm.habits
    val p = LocalExtra.current
    val habits by store.habits.collectAsState()
    val loaded by store.loaded.collectAsState()
    val loading by store.loading.collectAsState()
    val error by store.error.collectAsState()
    val weightOpen by vm.prefs.habitWeightOpen.collectAsState()
    var editing by remember { mutableStateOf<Pair<Boolean, ApiHabit?>?>(null) }
    var actions by remember { mutableStateOf<ApiHabit?>(null) }
    var weighIn by remember { mutableStateOf<ApiHabit?>(null) }
    var showArchived by rememberSaveable { mutableStateOf(false) }
    val today = store.today()
    val week = (6 downTo 0).map { today.minusDays(it.toLong()) }
    val active = habits.filter { !it.archived }
    val archived = habits.filter { it.archived }

    Column(Modifier.fillMaxSize()) {
        PageHeader("Habits", actions = {
            IconButton(onClick = { vm.prefs.setHabitWeightOpen(!weightOpen) }) {
                Icon(Icons.Outlined.MonitorWeight, "Weight", tint = if (weightOpen) Accent else p.icon)
            }
            IconButton(onClick = { editing = true to null }) { Icon(Icons.Rounded.Add, "New habit", tint = p.icon, modifier = Modifier.size(28.dp)) }
        })
        PullToRefreshBox(isRefreshing = loading && loaded, onRefresh = { store.load() }, modifier = Modifier.fillMaxSize()) {
            when {
                !loaded && error != null -> Box(Modifier.fillMaxSize().padding(32.dp), contentAlignment = Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(error!!, color = p.textTertiary, textAlign = TextAlign.Center)
                        TextButton(onClick = { store.load() }) { Text("Try again", color = Accent) }
                    }
                }
                !loaded -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator(color = Accent) }
                else -> LazyColumn(contentPadding = PaddingValues(start = 12.dp, end = 12.dp, bottom = 24.dp), modifier = Modifier.fillMaxSize()) {
                    if (weightOpen) item("weight") { WeightCard(store, modifier = Modifier.padding(bottom = 12.dp)) }
                    if (habits.isEmpty()) {
                        item("empty") {
                            Column(Modifier.fillMaxWidth().padding(top = 48.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                                Text("🌱", fontSize = 44.sp)
                                Spacer(Modifier.height(10.dp))
                                Text("No habits yet", fontWeight = FontWeight.SemiBold, color = p.textSecondary)
                                Text("Build a routine: drink water, read, work out…", fontSize = 13.sp, color = p.textTertiary)
                                TextButton(onClick = { editing = true to null }) { Text("New habit", color = Accent) }
                            }
                        }
                    }
                    if (active.isNotEmpty()) {
                        item("week") { WeekHeader(week, today) }
                        item("active") {
                            OtCard {
                                for ((i, h) in active.withIndex()) {
                                    if (i > 0) Box(Modifier.fillMaxWidth().padding(start = 66.dp).height(0.5.dp).background(p.divider))
                                    HabitRow(
                                        h, week, today, store,
                                        onOpen = { store.openId.value = h.id },
                                        onLongPress = { actions = h },
                                        onCheck = {
                                            if (h.tracksWeight && h.todayValue < h.goal) weighIn = h else store.tap(h)
                                        },
                                    )
                                }
                            }
                        }
                    }
                    if (archived.isNotEmpty()) {
                        item("archived-head") {
                            Row(
                                Modifier.fillMaxWidth().clickable { showArchived = !showArchived }.padding(start = 8.dp, end = 8.dp, top = 18.dp, bottom = 8.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Text("Archived", fontWeight = FontWeight.Bold, fontSize = 15.sp, color = p.textSecondary)
                                Spacer(Modifier.weight(1f))
                                Text("${archived.size}", fontSize = 14.sp, color = p.count)
                                Icon(
                                    if (showArchived) Icons.Outlined.KeyboardArrowDown else Icons.AutoMirrored.Filled.KeyboardArrowRight,
                                    null, tint = p.count, modifier = Modifier.size(20.dp),
                                )
                            }
                        }
                        if (showArchived) {
                            item("archived") {
                                OtCard {
                                    for (h in archived) {
                                        Row(
                                            Modifier.fillMaxWidth().clickable { store.openId.value = h.id }.padding(horizontal = 12.dp, vertical = 10.dp),
                                            verticalAlignment = Alignment.CenterVertically,
                                        ) {
                                            HabitIcon(h, 36)
                                            Spacer(Modifier.width(12.dp))
                                            Text(h.title, color = p.textTertiary, fontSize = 15.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    editing?.let { (_, h) -> HabitEditSheet(store, h) { editing = null } }
    actions?.let { h -> HabitActionsSheet(store, h, active, onEdit = { actions = null; editing = true to h }) { actions = null } }
    weighIn?.let { h ->
        AlertDialog(
            onDismissRequest = { weighIn = null },
            title = { Text("Today's weight") },
            text = {
                Column {
                    Text("Logging it checks off ${h.title}.", fontSize = 14.sp, color = p.textTertiary)
                    Spacer(Modifier.height(12.dp))
                    WeightInput(store, updating = false, autofocus = true) { weighIn = null }
                }
            },
            confirmButton = {},
            dismissButton = { TextButton(onClick = { weighIn = null }) { Text("Cancel") } },
            containerColor = p.card,
        )
    }
}

@Composable
private fun WeekHeader(week: List<LocalDate>, today: LocalDate) {
    val p = LocalExtra.current
    Row(Modifier.fillMaxWidth().padding(start = 12.dp, end = 12.dp, bottom = 6.dp), verticalAlignment = Alignment.CenterVertically) {
        Spacer(Modifier.weight(1f))
        for (d in week) {
            Text(
                d.dayOfWeek.getDisplayName(TextStyle.NARROW, Locale.getDefault()),
                fontSize = 11.sp, textAlign = TextAlign.Center,
                color = if (d == today) Accent else p.textTertiary,
                fontWeight = if (d == today) FontWeight.Bold else FontWeight.Normal,
                modifier = Modifier.width(DOT_CELL.dp),
            )
        }
        Spacer(Modifier.width(CHECK_CELL.dp))
    }
}

private const val DOT_CELL = 22
private const val CHECK_CELL = 48

@Composable
fun HabitIcon(h: ApiHabit, size: Int) {
    Box(Modifier.size(size.dp).clip(CircleShape).background(habitColor(h).copy(alpha = 0.18f)), contentAlignment = Alignment.Center) {
        Text(h.icon.ifBlank { "•" }, fontSize = (size * 0.5f).sp, color = habitColor(h))
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun HabitRow(h: ApiHabit, week: List<LocalDate>, today: LocalDate, store: HabitsStore, onOpen: () -> Unit, onLongPress: () -> Unit, onCheck: () -> Unit) {
    val p = LocalExtra.current
    val color = habitColor(h)
    val weights by store.weights.collectAsState()
    Row(
        Modifier.fillMaxWidth().combinedClickable(onClick = onOpen, onLongClick = onLongPress).padding(start = 12.dp, end = 8.dp, top = 10.dp, bottom = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        HabitIcon(h, 40)
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(h.title, fontSize = 15.sp, color = p.text, maxLines = 2, overflow = TextOverflow.Ellipsis, lineHeight = 18.sp)
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (h.currentStreak > 0) {
                    Icon(Icons.Rounded.LocalFireDepartment, null, tint = Orange, modifier = Modifier.size(13.dp))
                    Text("${h.currentStreak}d", fontSize = 12.sp, color = Orange)
                    Spacer(Modifier.width(6.dp))
                }
                val latest = weights.lastOrNull()
                val sub = when {
                    h.tracksWeight && latest != null -> fmtWeight(latest.weight) + " kg"
                    h.goal > 1 -> "${h.todayValue}/${h.goal} ${h.unit}".trim()
                    !h.isDue(today) -> "Not due today"
                    else -> ""
                }
                if (sub.isNotEmpty()) Text(sub, fontSize = 12.sp, color = p.textTertiary, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        for (d in week) {
            val v = h.valueOn(d, today)
            val due = h.isDue(d)
            Box(
                Modifier.width(DOT_CELL.dp).height(32.dp).clickable { store.tap(h, d) }
                    .semantics { contentDescription = "${h.title} $d" },
                contentAlignment = Alignment.Center,
            ) {
                val m = Modifier.size(12.dp).clip(CircleShape)
                when {
                    v >= h.goal -> Box(m.background(color))
                    v > 0 -> Box(m.background(color.copy(alpha = 0.4f)))
                    due -> Box(m.border(1.5.dp, p.divider, CircleShape))
                    else -> Box(Modifier.size(4.dp).clip(CircleShape).background(p.divider))
                }
            }
        }
        Box(Modifier.width(CHECK_CELL.dp), contentAlignment = Alignment.Center) {
            CheckButton(h, onCheck)
        }
    }
}

/** Filled with a check when done; for counters, a ring that fills towards the goal. */
@Composable
private fun CheckButton(h: ApiHabit, onClick: () -> Unit) {
    val p = LocalExtra.current
    val color = habitColor(h)
    val done = h.todayValue >= h.goal
    Box(
        Modifier.size(34.dp).clip(CircleShape).clickable(onClick = onClick)
            .semantics { contentDescription = if (done) "Undo check-in ${h.title}" else "Check in ${h.title}" },
        contentAlignment = Alignment.Center,
    ) {
        if (done) {
            Box(Modifier.size(30.dp).clip(CircleShape).background(color), contentAlignment = Alignment.Center) {
                Icon(Icons.Rounded.Check, null, tint = Color.White, modifier = Modifier.size(20.dp))
            }
        } else {
            val progress = if (h.goal > 0) h.todayValue.toFloat() / h.goal else 0f
            androidx.compose.foundation.Canvas(Modifier.size(30.dp)) {
                val w = 2.5.dp.toPx()
                drawCircle(p.divider, radius = size.minDimension / 2 - w / 2, style = Stroke(w))
                if (progress > 0f) drawArc(color, -90f, 360f * progress, false, style = Stroke(w, cap = StrokeCap.Round),
                    topLeft = androidx.compose.ui.geometry.Offset(w / 2, w / 2),
                    size = androidx.compose.ui.geometry.Size(size.width - w, size.height - w))
            }
            if (h.goal > 1) Icon(Icons.Rounded.Add, null, tint = color, modifier = Modifier.size(16.dp))
        }
    }
}

// ─── Detail ───────────────────────────────────────────────────────────

@Composable
private fun HabitDetail(vm: MainViewModel, h: ApiHabit, onBack: () -> Unit) {
    val store = vm.habits
    val p = LocalExtra.current
    val color = habitColor(h)
    var editing by remember { mutableStateOf(false) }
    var month by remember(h.id) { mutableStateOf(YearMonth.now()) }
    var stats by remember(h.id) { mutableStateOf<ApiHabitStats?>(null) }
    var reload by remember { mutableStateOf(0) }
    // Check-ins from the list or the calendar change the stats; refetch on every change.
    LaunchedEffect(h.id, month, h.todayValue, h.recentCheckins, reload) { store.stats(h.id, month)?.let { stats = it } }
    val today = store.today()
    val weekStart = weekStartDay(vm.prefs.weekStart.collectAsState().value)

    Column(Modifier.fillMaxSize()) {
        PageHeader(h.title, onBack = onBack, actions = {
            IconButton(onClick = { editing = true }) { Icon(Icons.Outlined.Edit, "Edit habit", tint = p.icon) }
        })
        LazyColumn(contentPadding = PaddingValues(start = 12.dp, end = 12.dp, bottom = 24.dp)) {
            item {
                Row(Modifier.padding(start = 6.dp, bottom = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                    HabitIcon(h, 44)
                    Spacer(Modifier.width(12.dp))
                    Column {
                        Text(frequencyLabel(h, weekStart), fontSize = 14.sp, color = p.textSecondary)
                        if (h.goal > 1) Text("Goal: ${h.goal} ${h.unit} a day".replace("  ", " "), fontSize = 13.sp, color = p.textTertiary)
                        if (h.archived) Text("Archived", fontSize = 13.sp, color = p.textTertiary)
                    }
                }
            }
            if (h.tracksWeight) item { WeightCard(store, color, Modifier.padding(bottom = 12.dp)) }
            item {
                val s = stats
                Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        StatCard("${s?.currentStreak ?: h.currentStreak}", "Current streak", color, Modifier.weight(1f))
                        StatCard(s?.longestStreak?.toString() ?: "–", "Longest streak", p.text, Modifier.weight(1f))
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        StatCard(s?.let { "${Math.round(it.completionRate * 100)}%" } ?: "–", "Completion rate", p.text, Modifier.weight(1f))
                        StatCard(s?.totalCheckins?.toString() ?: "–", "Total check-ins", p.text, Modifier.weight(1f))
                    }
                }
                Spacer(Modifier.height(12.dp))
            }
            item {
                OtCard {
                    Column(Modifier.padding(12.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                month.atDay(1).format(DateTimeFormatter.ofPattern("LLLL yyyy")),
                                fontWeight = FontWeight.Bold, fontSize = 16.sp, color = p.text, modifier = Modifier.weight(1f).padding(start = 6.dp),
                            )
                            IconButton(onClick = { month = month.minusMonths(1) }) { Icon(Icons.AutoMirrored.Filled.KeyboardArrowLeft, "Previous month", tint = p.icon) }
                            TextButton(onClick = { month = YearMonth.now() }) { Text("Today", color = Accent) }
                            IconButton(onClick = { month = month.plusMonths(1) }) { Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, "Next month", tint = p.icon) }
                        }
                        MonthGrid(month, stats?.takeIf { it.month == month.toString() }?.days.orEmpty(), h, today, weekStart, color) { d ->
                            store.checkin(h, LocalDate.parse(d.date), if (d.completed) 0 else h.goal) { reload++ }
                        }
                        Text(
                            "Tap a day to check it in or undo it. Faded days aren't due.",
                            fontSize = 12.sp, color = p.textTertiary, modifier = Modifier.padding(start = 6.dp, top = 8.dp),
                        )
                    }
                }
            }
        }
    }
    if (editing) HabitEditSheet(store, h) { editing = false }
}

private fun frequencyLabel(h: ApiHabit, weekStart: java.time.DayOfWeek): String {
    if (h.frequency == "daily") return "Every day"
    return (0L until 7L).map { weekStart.plus(it) }
        .filter { h.weekdays and weekdayBit(it) != 0 }
        .joinToString(", ") { it.getDisplayName(TextStyle.SHORT, Locale.getDefault()) }
}

@Composable
private fun StatCard(value: String, label: String, color: Color, modifier: Modifier) {
    val p = LocalExtra.current
    Column(modifier.clip(RoundedCornerShape(16.dp)).background(p.card).padding(14.dp)) {
        Text(value, fontSize = 24.sp, fontWeight = FontWeight.Bold, color = color)
        Text(label, fontSize = 13.sp, color = p.textTertiary)
    }
}

@Composable
private fun MonthGrid(month: YearMonth, days: List<ApiHabitStatsDay>, h: ApiHabit, today: LocalDate, weekStart: java.time.DayOfWeek, color: Color, onTap: (ApiHabitStatsDay) -> Unit) {
    val p = LocalExtra.current
    val byDate = days.associateBy { it.date }
    val first = month.atDay(1)
    val lead = ((first.dayOfWeek.value - weekStart.value) + 7) % 7
    val cells: List<LocalDate?> = List(lead) { null } + (1..month.lengthOfMonth()).map { month.atDay(it) }
    Row(Modifier.fillMaxWidth()) {
        for (i in 0L until 7L) {
            Text(
                weekStart.plus(i).getDisplayName(TextStyle.NARROW, Locale.getDefault()),
                fontSize = 11.sp, color = p.textTertiary, textAlign = TextAlign.Center, modifier = Modifier.weight(1f),
            )
        }
    }
    for (week in cells.chunked(7)) {
        Row(Modifier.fillMaxWidth().padding(vertical = 2.dp)) {
            for (i in 0 until 7) {
                val d = week.getOrNull(i)
                Box(Modifier.weight(1f).aspectRatio(1f).padding(3.dp), contentAlignment = Alignment.Center) {
                    if (d == null) return@Box
                    val s = byDate[d.toString()]
                    val future = d > today
                    val due = s?.due ?: h.isDue(d)
                    val bg = when {
                        s?.completed == true -> color
                        (s?.value ?: 0) > 0 -> color.copy(alpha = 0.35f)
                        else -> Color.Transparent
                    }
                    Box(
                        Modifier.fillMaxSize().clip(CircleShape).background(bg)
                            .then(if (d == today && s?.completed != true) Modifier.border(1.5.dp, color, CircleShape) else Modifier)
                            .then(if (!future && s != null) Modifier.clickable { onTap(s) } else Modifier),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            "${d.dayOfMonth}", fontSize = 13.sp,
                            color = when {
                                s?.completed == true -> Color.White
                                future || !due -> p.textTertiary.copy(alpha = 0.5f)
                                else -> p.text
                            },
                        )
                    }
                }
            }
        }
    }
}

// ─── Long press actions ───────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun HabitActionsSheet(store: HabitsStore, h: ApiHabit, active: List<ApiHabit>, onEdit: () -> Unit, onDismiss: () -> Unit) {
    val p = LocalExtra.current
    val index = active.indexOfFirst { it.id == h.id }
    var confirmDelete by remember { mutableStateOf(false) }
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = p.card) {
        Row(Modifier.padding(horizontal = 20.dp, vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
            HabitIcon(h, 32)
            Spacer(Modifier.width(12.dp))
            Text(h.title, fontWeight = FontWeight.SemiBold, fontSize = 16.sp, color = p.text)
        }
        Spacer(Modifier.height(6.dp))
        ActionRow(Icons.Rounded.VerticalAlignTop, "Move to top", index > 0) { store.move(h, 0); onDismiss() }
        ActionRow(Icons.Rounded.ArrowUpward, "Move up", index > 0) { store.move(h, index - 1); onDismiss() }
        ActionRow(Icons.Rounded.ArrowDownward, "Move down", index in 0 until active.lastIndex) { store.move(h, index + 1); onDismiss() }
        ActionRow(Icons.Rounded.VerticalAlignBottom, "Move to bottom", index in 0 until active.lastIndex) { store.move(h, active.size); onDismiss() }
        ActionRow(Icons.Outlined.Edit, "Edit", true, onClick = onEdit)
        ActionRow(if (h.archived) Icons.Outlined.Unarchive else Icons.Outlined.Archive, if (h.archived) "Unarchive" else "Archive", true) {
            store.setArchived(h, !h.archived); onDismiss()
        }
        ActionRow(Icons.Outlined.Delete, "Delete", true, PrioHigh) { confirmDelete = true }
        Spacer(Modifier.height(24.dp))
    }
    if (confirmDelete) DeleteHabitDialog(h, { confirmDelete = false }) { store.delete(h.id); onDismiss() }
}

@Composable
fun ActionRow(icon: androidx.compose.ui.graphics.vector.ImageVector, label: String, enabled: Boolean, tint: Color? = null, onClick: () -> Unit) {
    val p = LocalExtra.current
    val c = if (!enabled) p.textTertiary.copy(alpha = 0.5f) else tint ?: p.text
    Row(
        Modifier.fillMaxWidth().clickable(enabled = enabled, onClick = onClick).padding(horizontal = 20.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, null, tint = if (!enabled) c else tint ?: p.icon, modifier = Modifier.size(22.dp))
        Spacer(Modifier.width(16.dp))
        Text(label, fontSize = 16.sp, color = c)
    }
}

@Composable
fun DeleteHabitDialog(h: ApiHabit, onDismiss: () -> Unit, onConfirm: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Delete ${h.title}?") },
        text = { Text("Its check-ins are deleted too. To keep its history, archive it instead.") },
        confirmButton = { TextButton(onClick = { onDismiss(); onConfirm() }) { Text("Delete", color = PrioHigh) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
        containerColor = LocalExtra.current.card,
    )
}
