package org.opentick.android.ui

import android.app.Activity
import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Search
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.SnackbarResult
import androidx.compose.material3.rememberDrawerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.core.view.WindowCompat
import kotlinx.coroutines.launch
import org.opentick.android.data.AccountMode
import org.opentick.android.ui.auth.LoginScreen
import org.opentick.android.ui.calendar.CalendarScreen
import org.opentick.android.ui.components.CalendarDayIcon
import org.opentick.android.ui.components.MatrixTabIcon
import org.opentick.android.ui.components.PomoTabIcon
import org.opentick.android.ui.components.TaskTabIcon
import org.opentick.android.ui.focus.FocusScreen
import org.opentick.android.ui.matrix.MatrixScreen
import org.opentick.android.ui.search.SearchScreen
import org.opentick.android.ui.settings.AboutPage
import org.opentick.android.ui.settings.AccountSettings
import org.opentick.android.ui.settings.AppearanceSettings
import org.opentick.android.ui.settings.DateTimeSettings
import org.opentick.android.ui.settings.GeneralSettings
import org.opentick.android.ui.settings.NotificationSettings
import org.opentick.android.ui.settings.PomoSettings
import org.opentick.android.ui.settings.SettingsScreen
import org.opentick.android.ui.settings.TabBarSettings
import org.opentick.android.ui.tasks.ListDrawer
import org.opentick.android.ui.tasks.TaskDetailScreen
import org.opentick.android.ui.tasks.TaskListScreen
import org.opentick.android.ui.tasks.TrashScreen
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra

/** Bottom tabs; Settings → Tab Bar picks which ones show and in what order. */
enum class Tab(val title: String, val description: String) {
    TASKS("Task", "Manage your task with lists and filters."),
    MATRIX("Eisenhower Matrix", "Focus on what's important and urgent."),
    CALENDAR("Calendar", "Manage your task with six calendar views."),
    POMO("Pomodoro", "Use the Pomo timer to keep focus."),
    SEARCH("Search", "Do a quick search easily.");

    companion object {
        fun parse(pref: String): List<Tab> {
            val tabs = pref.split(',').mapNotNull { runCatching { valueOf(it.trim()) }.getOrNull() }.distinct()
            return if (TASKS in tabs) tabs else listOf(TASKS) + tabs
        }
    }
}

@Composable
fun TabIcon(tab: Tab, color: Color, modifier: Modifier = Modifier) = when (tab) {
    Tab.TASKS -> TaskTabIcon(color, modifier)
    Tab.MATRIX -> MatrixTabIcon(color, modifier)
    Tab.CALENDAR -> CalendarDayIcon(color, modifier)
    Tab.POMO -> PomoTabIcon(color, modifier)
    Tab.SEARCH -> Icon(Icons.Rounded.Search, null, tint = color, modifier = modifier.size(30.dp))
}

@Composable
fun MainScreen(vm: MainViewModel, initialTab: Tab?, oauthError: String?, onOAuthErrorShown: () -> Unit) {
    val tabsPref by vm.prefs.tabs.collectAsState()
    val tabs = Tab.parse(tabsPref)
    var tab by rememberSaveable { mutableStateOf(initialTab ?: Tab.TASKS) }
    LaunchedEffect(initialTab) { if (initialTab != null) tab = initialTab }
    if (tab !in tabs && tab != Tab.POMO) tab = Tab.TASKS
    val drawer = rememberDrawerState(DrawerValue.Closed)
    val scope = rememberCoroutineScope()
    val snackbar = remember { SnackbarHostState() }
    val openTask by vm.openTaskId.collectAsState()
    val authExpired by vm.authExpired.collectAsState()
    val message by vm.message.collectAsState()
    var signingIn by remember { mutableStateOf(false) }
    val p = LocalExtra.current

    // Dark status bar icons on the light grey page, light ones in dark mode.
    val view = LocalView.current
    SideEffect {
        (view.context as? Activity)?.window?.let { w ->
            WindowCompat.getInsetsController(w, view).apply {
                isAppearanceLightStatusBars = !p.dark
                isAppearanceLightNavigationBars = !p.dark
            }
        }
    }

    LaunchedEffect(authExpired) {
        if (authExpired) {
            val r = snackbar.showSnackbar("Your session expired. Sign in again to keep syncing.", "Sign in", withDismissAction = true)
            if (r == SnackbarResult.ActionPerformed) signingIn = true
        }
    }
    LaunchedEffect(message) { message?.let { snackbar.showSnackbar(it); vm.message.value = null } }
    // A browser sign in that started from Settings comes back here.
    LaunchedEffect(oauthError) { if (oauthError != null) signingIn = true }

    BackHandler(enabled = drawer.isOpen) { scope.launch { drawer.close() } }

    Box(Modifier.fillMaxSize().background(p.page)) {
        ModalNavigationDrawer(
            drawerState = drawer,
            gesturesEnabled = tab == Tab.TASKS || drawer.isOpen,
            scrimColor = Color.Black.copy(alpha = 0.55f),
            drawerContent = { ListDrawer(vm) { scope.launch { drawer.close() } } },
        ) {
            Column(Modifier.fillMaxSize().background(p.page)) {
                Box(Modifier.weight(1f)) {
                    when (tab) {
                        Tab.TASKS -> TaskListScreen(vm, snackbar) { scope.launch { drawer.open() } }
                        Tab.CALENDAR -> CalendarScreen(vm)
                        Tab.MATRIX -> MatrixScreen(vm)
                        Tab.POMO -> FocusScreen(vm)
                        Tab.SEARCH -> SearchTab(vm)
                    }
                }
                BottomBar(tabs, tab) { tab = it }
            }
        }

        // Pages (Settings, Search, Trash) slide over the tabs.
        val page = vm.pages.lastOrNull()
        if (page != null) {
            BackHandler { vm.back() }
            Box(Modifier.fillMaxSize().background(p.page)) {
                when (page) {
                    Page.SETTINGS -> SettingsScreen(vm) { signingIn = true }
                    Page.TAB_BAR -> TabBarSettings(vm)
                    Page.APPEARANCE -> AppearanceSettings(vm)
                    Page.DATE_TIME -> DateTimeSettings(vm)
                    Page.NOTIFICATIONS -> NotificationSettings(vm)
                    Page.POMO_SETTINGS -> PomoSettings(vm)
                    Page.GENERAL -> GeneralSettings(vm)
                    Page.ACCOUNT -> AccountSettings(vm)
                    Page.ABOUT -> AboutPage(vm)
                    Page.SEARCH -> SearchScreen(vm)
                    Page.TRASH -> TrashScreen(vm)
                }
            }
        }

        AnimatedVisibility(visible = openTask != null, enter = slideInHorizontally { it }, exit = slideOutHorizontally { it }) {
            val id = remember(openTask) { openTask } ?: return@AnimatedVisibility
            TaskDetailScreen(vm, id) { vm.openTaskId.value = null }
        }

        SnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter).navigationBarsPadding().padding(bottom = 70.dp))

        if (signingIn) {
            BackHandler { signingIn = false }
            LoginScreen(vm, oauthError, onOAuthErrorShown, allowLocal = false, onCancel = { signingIn = false })
            val mode by vm.prefs.mode.collectAsState()
            val expired by vm.authExpired.collectAsState()
            LaunchedEffect(mode, expired) { if (mode == AccountMode.SERVER && !expired) signingIn = false }
        }
    }
}

/** Icons only, evenly spread, blue when selected, like TickTick. */
@Composable
private fun BottomBar(tabs: List<Tab>, selected: Tab, onSelect: (Tab) -> Unit) {
    val p = LocalExtra.current
    Row(Modifier.fillMaxWidth().background(p.page).navigationBarsPadding().height(62.dp), verticalAlignment = Alignment.CenterVertically) {
        for (t in tabs) {
            Box(
                Modifier.weight(1f).fillMaxSize()
                    .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) { onSelect(t) }
                    .semantics { contentDescription = t.title },
                contentAlignment = Alignment.Center,
            ) {
                TabIcon(t, if (t == selected) Accent else p.navIcon)
            }
        }
    }
}

@Composable
private fun SearchTab(vm: MainViewModel) {
    // The search tab is the search page without the back arrow.
    SearchScreen(vm)
}
