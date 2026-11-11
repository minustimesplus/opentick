package org.opentick.android.ui.focus

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import org.opentick.android.data.Prefs
import org.opentick.android.reminders.FocusAlarm
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.Q4Green

data class FocusState(
    val isBreak: Boolean = false,
    /** Set while running: when the session ends. */
    val endAt: Long? = null,
    /** Set while paused: what's left. */
    val remaining: Long? = null,
)

/** Lives for the whole process, so switching tabs doesn't stop the timer. */
object FocusTimer {
    val state = MutableStateFlow(FocusState())
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FocusScreen(vm: MainViewModel) {
    val prefs: Prefs = vm.prefs
    val context = LocalContext.current
    val s by FocusTimer.state.collectAsState()
    val extra = LocalExtra.current
    val total = (if (s.isBreak) prefs.breakMinutes else prefs.focusMinutes) * 60_000L
    var now by remember { mutableLongStateOf(System.currentTimeMillis()) }

    LaunchedEffect(s.endAt) {
        while (s.endAt != null) {
            now = System.currentTimeMillis()
            if (now >= s.endAt!!) {
                // Session over: switch to the other phase, ready to start.
                FocusTimer.state.value = FocusState(isBreak = !s.isBreak)
                break
            }
            delay(250)
        }
    }

    val left = when {
        s.endAt != null -> (s.endAt!! - now).coerceAtLeast(0)
        s.remaining != null -> s.remaining!!
        else -> total
    }
    val color = if (s.isBreak) Q4Green else Accent

    fun start() {
        val end = System.currentTimeMillis() + left
        FocusTimer.state.value = s.copy(endAt = end, remaining = null)
        FocusAlarm.schedule(context, end, if (s.isBreak) "Break's over. Ready to focus?" else "Focus session done. Time for a break!")
    }

    fun pause() {
        FocusTimer.state.value = s.copy(endAt = null, remaining = left)
        FocusAlarm.cancel(context)
    }

    fun reset(isBreak: Boolean = s.isBreak) {
        FocusTimer.state.value = FocusState(isBreak = isBreak)
        FocusAlarm.cancel(context)
    }

    Column(Modifier.fillMaxSize().background(extra.page)) {
        org.opentick.android.ui.components.PageHeader("Pomodoro")
        Column(
            Modifier.fillMaxSize().padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            SingleChoiceSegmentedButtonRow {
                SegmentedButton(!s.isBreak, { reset(false) }, SegmentedButtonDefaults.itemShape(0, 2), colors = SegmentedButtonDefaults.colors(activeContainerColor = Accent.copy(alpha = 0.12f), activeContentColor = Accent, inactiveContainerColor = extra.card)) { Text("Pomo") }
                SegmentedButton(s.isBreak, { reset(true) }, SegmentedButtonDefaults.itemShape(1, 2), colors = SegmentedButtonDefaults.colors(activeContainerColor = Accent.copy(alpha = 0.12f), activeContentColor = Accent, inactiveContainerColor = extra.card)) { Text("Break") }
            }
            Spacer(Modifier.weight(1f))
            Box(Modifier.size(280.dp), contentAlignment = Alignment.Center) {
                val track = extra.divider
                Canvas(Modifier.size(260.dp)) {
                    val stroke = 10.dp.toPx()
                    val inset = stroke / 2
                    val arcSize = Size(size.width - stroke, size.height - stroke)
                    drawArc(track, 0f, 360f, false, Offset(inset, inset), arcSize, style = Stroke(stroke))
                    val sweep = 360f * (left.toFloat() / total.coerceAtLeast(1))
                    drawArc(color, -90f, sweep, false, Offset(inset, inset), arcSize, style = Stroke(stroke, cap = StrokeCap.Round))
                }
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    val secs = (left + 999) / 1000
                    Text("%02d:%02d".format(secs / 60, secs % 60), fontSize = 52.sp, fontWeight = FontWeight.Light)
                    Text(if (s.isBreak) "Break" else "Focus", color = extra.textTertiary)
                }
            }
            Spacer(Modifier.weight(1f))
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                if (s.endAt != null) {
                    Button(onClick = { pause() }, colors = ButtonDefaults.buttonColors(containerColor = color), shape = RoundedCornerShape(24.dp), modifier = Modifier.width(140.dp).height(48.dp)) { Text("Pause") }
                } else {
                    Button(onClick = { start() }, colors = ButtonDefaults.buttonColors(containerColor = color), shape = RoundedCornerShape(24.dp), modifier = Modifier.width(140.dp).height(48.dp)) {
                        Text(if (s.remaining != null) "Continue" else "Start")
                    }
                }
                if (s.endAt != null || s.remaining != null) {
                    OutlinedButton(onClick = { reset() }, shape = RoundedCornerShape(24.dp), modifier = Modifier.width(110.dp).height(48.dp)) { Text("End") }
                }
            }
            Spacer(Modifier.height(32.dp))
        }
    }
}
