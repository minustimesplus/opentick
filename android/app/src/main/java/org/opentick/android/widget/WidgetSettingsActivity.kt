package org.opentick.android.widget

import android.app.Activity
import android.appwidget.AppWidgetManager
import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.RadioButton
import androidx.compose.material3.RadioButtonDefaults
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.datastore.preferences.core.Preferences
import androidx.glance.appwidget.GlanceAppWidgetManager
import androidx.glance.appwidget.state.getAppWidgetState
import androidx.glance.appwidget.state.updateAppWidgetState
import androidx.glance.state.PreferencesGlanceStateDefinition
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.launch
import org.opentick.android.OpenTickApp
import org.opentick.android.ui.Data
import org.opentick.android.ui.ListSel
import org.opentick.android.ui.components.ListIcon
import org.opentick.android.ui.settings.SettingsCard
import org.opentick.android.ui.settings.SettingsPage
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.OpenTickTheme
import org.opentick.android.util.splitEmoji

/**
 * A widget's settings, opened when it's added (before Android 12), from
 * "Edit widget" on a long press, and from its ▾ and ⋮: the list a tasks widget
 * shows, its theme and how see-through it is, like TickTick's widget settings.
 */
class WidgetSettingsActivity : ComponentActivity() {
    private var widgetId = AppWidgetManager.INVALID_APPWIDGET_ID
    private var list by mutableStateOf<String?>(null)
    private var widgetTheme by mutableIntStateOf(THEME_AUTO)
    private var opacity by mutableIntStateOf(DEFAULT_OPACITY)
    private var loaded by mutableStateOf(false)

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        widgetId = intent.getIntExtra(AppWidgetManager.EXTRA_APPWIDGET_ID, AppWidgetManager.INVALID_APPWIDGET_ID)
        // Backing out of the first setup doesn't add the widget.
        setResult(Activity.RESULT_CANCELED, result())
        if (widgetId == AppWidgetManager.INVALID_APPWIDGET_ID) return finish()

        val provider = AppWidgetManager.getInstance(this).getAppWidgetInfo(widgetId)?.provider?.className
        val widget = Widgets.forProvider(provider) ?: return finish()
        val hasList = widget is TasksWidget
        val app = application as OpenTickApp

        lifecycleScope.launch {
            val glanceId = GlanceAppWidgetManager(this@WidgetSettingsActivity).getGlanceIdBy(widgetId)
            val prefs = getAppWidgetState<Preferences>(this@WidgetSettingsActivity, PreferencesGlanceStateDefinition, glanceId)
            list = prefs[WidgetKeys.LIST]
            widgetTheme = prefs[WidgetKeys.THEME] ?: THEME_AUTO
            opacity = prefs[WidgetKeys.OPACITY] ?: DEFAULT_OPACITY
            loaded = true
        }

        fun save() = lifecycleScope.launch {
            val glanceId = GlanceAppWidgetManager(this@WidgetSettingsActivity).getGlanceIdBy(widgetId)
            updateAppWidgetState(this@WidgetSettingsActivity, glanceId) { p ->
                list?.let { p[WidgetKeys.LIST] = it }
                p[WidgetKeys.THEME] = widgetTheme
                p[WidgetKeys.OPACITY] = opacity
                p[WidgetKeys.PAGE] = 0
            }
            widget.update(this@WidgetSettingsActivity, glanceId)
            setResult(Activity.RESULT_OK, result())
            finish()
        }

        val dataFlow = combine(app.repo.allProjects, app.repo.allLabels) { p, l -> Data(p, l) }
        setContent {
            val appTheme by app.prefs.theme.collectAsState()
            val d by dataFlow.collectAsState(Data())
            OpenTickTheme(appTheme) {
                SettingsPage("Widget Settings", onBack = { finish() }) {
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                        TextButton(onClick = { save() }, enabled = loaded) { Text("Save", color = Accent, fontSize = 16.sp) }
                    }
                    if (hasList) {
                        Label("List")
                        val current = resolveWidgetList(parseWidgetKey(list), d).widgetKey()
                        SettingsCard {
                            for (k in WIDGET_SMART_LISTS) Choice(k.title, null, current == ListSel.Smart(k).widgetKey()) { list = ListSel.Smart(k).widgetKey() }
                        }
                        val lists = d.projects.filter { !it.isInbox }
                        if (lists.isNotEmpty()) SettingsCard {
                            for (p in lists) {
                                val key = ListSel.Project(p.id).widgetKey()
                                Choice(splitEmoji(p.title).second, p.title, current == key) { list = key }
                            }
                        }
                        if (d.labels.isNotEmpty()) SettingsCard {
                            for (l in d.labels) {
                                val key = ListSel.Label(l.id).widgetKey()
                                Choice("#" + l.title, null, current == key) { list = key }
                            }
                        }
                    }
                    Label("Theme")
                    ThemePicker(widgetTheme) { widgetTheme = it }
                    Label("Background opacity  $opacity%")
                    SettingsCard {
                        Slider(
                            value = opacity.toFloat(), onValueChange = { opacity = it.toInt() }, valueRange = 0f..100f, steps = 9,
                            colors = SliderDefaults.colors(thumbColor = Accent, activeTrackColor = Accent),
                            modifier = Modifier.padding(horizontal = 18.dp),
                        )
                    }
                }
            }
        }
    }

    private fun result() = Intent().putExtra(AppWidgetManager.EXTRA_APPWIDGET_ID, widgetId)
}

@Composable
private fun Label(text: String) {
    Text(text, fontSize = 14.sp, color = LocalExtra.current.textSecondary, modifier = Modifier.padding(start = 20.dp, top = 14.dp, bottom = 2.dp))
}

/** A list to pick: its emoji (or the list icon) when [iconTitle] is set, then its name. */
@Composable
private fun Choice(title: String, iconTitle: String?, selected: Boolean, onClick: () -> Unit) {
    Row(Modifier.fillMaxWidth().clickable(onClick = onClick).padding(horizontal = 18.dp, vertical = 2.dp), verticalAlignment = Alignment.CenterVertically) {
        if (iconTitle != null) { ListIcon(iconTitle); Spacer(Modifier.width(14.dp)) }
        Text(title, fontSize = 16.sp, color = LocalExtra.current.text, modifier = Modifier.weight(1f), maxLines = 1)
        RadioButton(selected, onClick, colors = RadioButtonDefaults.colors(selectedColor = Accent))
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ThemePicker(theme: Int, onPick: (Int) -> Unit) {
    val options = listOf(THEME_AUTO to "Auto", THEME_LIGHT to "Light", THEME_DARK to "Dark")
    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().padding(horizontal = 4.dp, vertical = 8.dp)) {
        options.forEachIndexed { i, (value, label) ->
            SegmentedButton(
                selected = theme == value, onClick = { onPick(value) },
                shape = SegmentedButtonDefaults.itemShape(i, options.size, RoundedCornerShape(12.dp)),
                colors = SegmentedButtonDefaults.colors(activeContainerColor = Accent.copy(alpha = 0.15f), activeContentColor = Accent),
            ) { Text(label) }
        }
    }
    Text(
        "Auto follows the phone's dark mode.",
        fontSize = 13.sp, color = LocalExtra.current.textTertiary, modifier = Modifier.padding(horizontal = 20.dp),
    )
}
