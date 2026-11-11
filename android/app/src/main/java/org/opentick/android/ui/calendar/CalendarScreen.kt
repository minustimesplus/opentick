package org.opentick.android.ui.calendar

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CalendarMonth
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.GridOn
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.Today
import androidx.compose.material.icons.outlined.ViewAgenda
import androidx.compose.material.icons.outlined.ViewColumn
import androidx.compose.material.icons.outlined.ViewDay
import androidx.compose.material.icons.outlined.ViewWeek
import androidx.compose.material.icons.outlined.Window
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.DpOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.components.OtFab
import org.opentick.android.ui.components.PageHeader
import org.opentick.android.ui.components.QuickAddSheet
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.util.atTimeMillis
import org.opentick.android.util.today
import java.time.LocalDate
import java.time.LocalTime
import java.time.YearMonth
import java.time.format.TextStyle
import java.util.Locale

private const val PAGES = 20_000
private const val MIDDLE = PAGES / 2

fun viewIcon(v: CalView): ImageVector = when (v) {
    CalView.LIST -> Icons.Outlined.ViewAgenda
    CalView.YEAR -> Icons.Outlined.GridOn
    CalView.MONTH -> Icons.Outlined.Window
    CalView.WEEK -> Icons.Outlined.ViewWeek
    CalView.THREE_DAY -> Icons.Outlined.ViewColumn
    CalView.DAY -> Icons.Outlined.ViewDay
}

/** TickTick's calendar tab: Week by default, five more views from the view menu. */
@Composable
fun CalendarScreen(vm: MainViewModel) {
    val data by vm.data.collectAsState()
    val viewName by vm.prefs.calendarView.collectAsState()
    val weekPref by vm.prefs.weekStart.collectAsState()
    val showCompleted by vm.prefs.showCompleted.collectAsState()
    val view = runCatching { CalView.valueOf(viewName) }.getOrDefault(CalView.WEEK)
    val first = weekStartDay(weekPref)
    val p = LocalExtra.current
    val scope = rememberCoroutineScope()

    // The day each view is anchored on; pages count from it.
    var anchor by rememberSaveable { mutableStateOf(today()) }
    var viewMenu by remember { mutableStateOf(false) }
    var moreMenu by remember { mutableStateOf(false) }
    var addAt by remember { mutableStateOf<Pair<LocalDate, LocalTime?>?>(null) }
    val pager = rememberPagerState(initialPage = MIDDLE) { PAGES }
    LaunchedEffect(view, anchor) { pager.scrollToPage(MIDDLE) }

    fun daysOf(page: Int): List<LocalDate> {
        val off = (page - MIDDLE).toLong()
        return when (view) {
            CalView.WEEK -> startOfWeek(anchor, first).plusWeeks(off).let { s -> (0L until 7L).map { s.plusDays(it) } }
            CalView.THREE_DAY -> anchor.plusDays(off * 3).let { s -> (0L until 3L).map { s.plusDays(it) } }
            CalView.DAY -> listOf(anchor.plusDays(off))
            else -> emptyList()
        }
    }
    fun monthOf(page: Int) = YearMonth.from(anchor).plusMonths((page - MIDDLE).toLong())

    val page = pager.currentPage
    val title = when (view) {
        CalView.YEAR -> "${anchor.year + (page - MIDDLE)}"
        CalView.MONTH -> monthTitle(monthOf(page))
        CalView.LIST -> monthTitle(YearMonth.now())
        else -> monthTitle(YearMonth.from(daysOf(page).first()))
    }

    // Scroll the hours so the current time sits in the upper part, like TickTick.
    val density = LocalDensity.current
    val scroll = rememberScrollState(with(density) { (HOUR_HEIGHT * (java.time.LocalTime.now().hour - 4).coerceIn(0, 16)).roundToPx() })

    fun setView(v: CalView, date: LocalDate = anchor) {
        anchor = date
        vm.prefs.setCalendarView(v.name)
    }

    Box(Modifier.fillMaxSize().background(p.page)) {
        Column(Modifier.fillMaxSize()) {
            PageHeader(title, actions = {
                IconButton(onClick = { anchor = today(); scope.launch { pager.scrollToPage(MIDDLE) } }) {
                    Icon(Icons.Outlined.CalendarMonth, "Go to today", tint = p.icon, modifier = Modifier.size(27.dp))
                }
                Box {
                    IconButton(onClick = { viewMenu = true }) { Icon(viewIcon(view), "Calendar view", tint = p.icon, modifier = Modifier.size(28.dp)) }
                    DropdownMenu(
                        viewMenu, { viewMenu = false },
                        shape = RoundedCornerShape(24.dp), containerColor = p.card, shadowElevation = 10.dp,
                        offset = DpOffset(56.dp, (-48).dp),
                        modifier = Modifier.width(236.dp),
                    ) {
                        for (v in CalView.entries) {
                            val selected = v == view
                            DropdownMenuItem(
                                text = { Text(v.title, fontSize = 16.sp, color = if (selected) Accent else p.text) },
                                leadingIcon = { Icon(viewIcon(v), null, tint = if (selected) Accent else p.icon, modifier = Modifier.size(24.dp)) },
                                trailingIcon = { if (selected) Icon(Icons.Outlined.Check, null, tint = Accent) },
                                onClick = { viewMenu = false; setView(v) },
                                contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 24.dp),
                                modifier = Modifier.height(44.dp),
                            )
                        }
                    }
                }
                Box {
                    IconButton(onClick = { moreMenu = true }) { Icon(Icons.Outlined.MoreVert, "More", tint = p.icon, modifier = Modifier.size(26.dp)) }
                    DropdownMenu(moreMenu, { moreMenu = false }, shape = RoundedCornerShape(20.dp), containerColor = p.card) {
                        DropdownMenuItem(
                            text = { Text(if (showCompleted) "Hide Completed" else "Show Completed") },
                            leadingIcon = { Icon(Icons.Outlined.CheckCircle, null) },
                            onClick = { vm.prefs.setShowCompleted(!showCompleted); moreMenu = false },
                        )
                        DropdownMenuItem(
                            text = { Text("Today") },
                            leadingIcon = { Icon(Icons.Outlined.Today, null) },
                            onClick = { anchor = today(); scope.launch { pager.scrollToPage(MIDDLE) }; moreMenu = false },
                        )
                    }
                }
            })
            Box(Modifier.weight(1f)) {
                when (view) {
                    CalView.WEEK, CalView.THREE_DAY, CalView.DAY -> HorizontalPager(pager, beyondViewportPageCount = 0, key = { it }) { pg ->
                        Timeline(
                            daysOf(pg), data, showCompleted, scroll,
                            onTask = { vm.openTaskId.value = it },
                            onSlot = { d, t -> addAt = d to t },
                            onDay = { d -> if (view != CalView.DAY) setView(CalView.DAY, d) },
                        )
                    }
                    CalView.MONTH -> HorizontalPager(pager, key = { it }) { pg ->
                        MonthGrid(monthOf(pg), first, data, showCompleted, onDay = { d -> setView(CalView.DAY, d) }, onTask = { vm.openTaskId.value = it })
                    }
                    CalView.YEAR -> HorizontalPager(pager, key = { it }) { pg ->
                        YearGrid(anchor.year + (pg - MIDDLE), first, data) { m -> setView(CalView.MONTH, m.atDay(1)) }
                    }
                    CalView.LIST -> AgendaList(data, showCompleted, onToggle = { id -> data.tasks.firstOrNull { it.id == id }?.let(vm::toggleDone) }, onTask = { vm.openTaskId.value = it }, onLongTask = { vm.actionsTaskId.value = it })
                }
            }
        }
        OtFab({ addAt = (if (view == CalView.DAY) daysOf(pager.currentPage).first() else today()) to null }, Modifier.align(Alignment.BottomEnd).padding(end = 22.dp, bottom = 22.dp))
    }

    addAt?.let { (date, time) ->
        QuickAddSheet(
            projects = data.projects, labels = data.labels, defaultProject = null, defaultDate = date, defaultLabel = null,
            defaultDue = time?.let { atTimeMillis(date, it) },
            onAdd = { vm.quickAdd(it.text, it.projectId, it.due, it.datePicked, it.priority, it.labels, it.description) },
            onDismiss = { addAt = null },
        )
    }
}

private fun monthTitle(m: YearMonth): String {
    val name = m.month.getDisplayName(TextStyle.FULL_STANDALONE, Locale.getDefault()).replaceFirstChar { it.uppercase() }
    return if (m.year == today().year) name else "$name ${m.year}"
}
