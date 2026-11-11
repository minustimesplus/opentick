package org.opentick.android.ui

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CalendarMonth
import androidx.compose.material.icons.filled.CheckBox
import androidx.compose.material.icons.filled.GridView
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Timer
import androidx.compose.material.icons.outlined.CalendarMonth
import androidx.compose.material.icons.outlined.CheckBox
import androidx.compose.material.icons.outlined.GridView
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.Timer
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.rememberDrawerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import kotlinx.coroutines.launch
import org.opentick.android.ui.auth.LoginScreen
import org.opentick.android.ui.calendar.CalendarScreen
import org.opentick.android.ui.focus.FocusScreen
import org.opentick.android.ui.matrix.MatrixScreen
import org.opentick.android.ui.settings.SettingsScreen
import org.opentick.android.ui.tasks.ListDrawer
import org.opentick.android.ui.tasks.TaskDetailScreen
import org.opentick.android.ui.tasks.TaskListScreen
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra

enum class Tab(val label: String, val icon: ImageVector, val selectedIcon: ImageVector) {
    TASKS("Tasks", Icons.Outlined.CheckBox, Icons.Filled.CheckBox),
    CALENDAR("Calendar", Icons.Outlined.CalendarMonth, Icons.Filled.CalendarMonth),
    MATRIX("Matrix", Icons.Outlined.GridView, Icons.Filled.GridView),
    FOCUS("Focus", Icons.Outlined.Timer, Icons.Filled.Timer),
    SETTINGS("Settings", Icons.Outlined.Settings, Icons.Filled.Settings),
}

@Composable
fun MainScreen(vm: MainViewModel, initialTab: Tab?, oauthError: String?, onOAuthErrorShown: () -> Unit) {
    var tab by rememberSaveable { mutableStateOf(initialTab ?: Tab.TASKS) }
    LaunchedEffect(initialTab) { if (initialTab != null) tab = initialTab }
    val drawer = rememberDrawerState(DrawerValue.Closed)
    val scope = rememberCoroutineScope()
    val snackbar = remember { SnackbarHostState() }
    val openTask by vm.openTaskId.collectAsState()
    val authExpired by vm.authExpired.collectAsState()
    var signingIn by remember { mutableStateOf(false) }
    val extra = LocalExtra.current

    LaunchedEffect(authExpired) {
        if (authExpired) {
            val r = snackbar.showSnackbar("Your session expired. Sign in again to keep syncing.", "Sign in", withDismissAction = true)
            if (r == androidx.compose.material3.SnackbarResult.ActionPerformed) signingIn = true
        }
    }
    // A browser sign in that started from Settings comes back here.
    LaunchedEffect(oauthError) { if (oauthError != null) signingIn = true }

    Box(Modifier.fillMaxSize()) {
        ModalNavigationDrawer(
            drawerState = drawer,
            gesturesEnabled = tab == Tab.TASKS,
            drawerContent = { ListDrawer(vm) { scope.launch { drawer.close() } } },
        ) {
            Scaffold(
                containerColor = MaterialTheme.colorScheme.background,
                snackbarHost = { SnackbarHost(snackbar) },
                bottomBar = {
                    NavigationBar(containerColor = MaterialTheme.colorScheme.background, tonalElevation = androidx.compose.ui.unit.Dp(0f)) {
                        for (t in Tab.entries) {
                            NavigationBarItem(
                                selected = tab == t,
                                onClick = { tab = t },
                                icon = { Icon(if (tab == t) t.selectedIcon else t.icon, t.label) },
                                label = { Text(t.label) },
                                colors = NavigationBarItemDefaults.colors(
                                    selectedIconColor = Accent, selectedTextColor = Accent,
                                    indicatorColor = MaterialTheme.colorScheme.background,
                                    unselectedIconColor = extra.textTertiary, unselectedTextColor = extra.textTertiary,
                                ),
                            )
                        }
                    }
                },
            ) { pad ->
                Box(Modifier.padding(bottom = pad.calculateBottomPadding())) {
                    when (tab) {
                        Tab.TASKS -> TaskListScreen(vm, snackbar) { scope.launch { drawer.open() } }
                        Tab.CALENDAR -> CalendarScreen(vm)
                        Tab.MATRIX -> MatrixScreen(vm)
                        Tab.FOCUS -> FocusScreen(vm)
                        Tab.SETTINGS -> SettingsScreen(vm) { signingIn = true }
                    }
                }
            }
        }

        AnimatedVisibility(
            visible = openTask != null,
            enter = slideInHorizontally { it },
            exit = slideOutHorizontally { it },
        ) {
            val id = remember(openTask) { openTask } ?: return@AnimatedVisibility
            TaskDetailScreen(vm, id) { vm.openTaskId.value = null }
        }

        if (signingIn) {
            androidx.activity.compose.BackHandler { signingIn = false }
            LoginScreen(vm, oauthError, onOAuthErrorShown, allowLocal = false, onCancel = { signingIn = false })
            val mode by vm.prefs.mode.collectAsState()
            val expired by vm.authExpired.collectAsState()
            LaunchedEffect(mode, expired) {
                if (mode == org.opentick.android.data.AccountMode.SERVER && !expired) signingIn = false
            }
        }
    }
}
