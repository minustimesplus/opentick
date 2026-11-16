package org.opentick.android.ui.habits

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.TrendingDown
import androidx.compose.material.icons.automirrored.rounded.TrendingUp
import androidx.compose.material.icons.rounded.Remove
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.data.ApiWeight
import org.opentick.android.ui.components.OtCard
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.Green
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh
import java.time.LocalDate
import java.time.format.DateTimeFormatter

private val dayMonth: DateTimeFormatter
    get() = DateTimeFormatter.ofPattern(android.text.format.DateFormat.getBestDateTimePattern(java.util.Locale.getDefault(), "dMMM"))

/** The latest weight, how it changed over 7 and 30 days, a 30 day chart and a field to log today's. */
@Composable
fun WeightCard(store: HabitsStore, color: Color = Accent, modifier: Modifier = Modifier) {
    val p = LocalExtra.current
    val weights by store.weights.collectAsState()
    val today = store.today()
    val latest = weights.lastOrNull()
    val logged = weights.firstOrNull { it.date == today.toString() }
    OtCard(modifier) {
        Column(Modifier.padding(16.dp)) {
            if (latest != null) {
                Row(verticalAlignment = Alignment.Top) {
                    Column(Modifier.weight(1f)) {
                        Row(verticalAlignment = Alignment.Bottom) {
                            Text(fmtWeight(latest.weight), fontSize = 30.sp, fontWeight = FontWeight.Bold, color = p.text)
                            Text(" kg", fontSize = 15.sp, color = p.textTertiary, modifier = Modifier.padding(bottom = 5.dp))
                        }
                        val sub = if (logged != null) "Today" else runCatching { "Last weigh-in " + LocalDate.parse(latest.date).format(dayMonth) }.getOrDefault("")
                        Text(sub, fontSize = 13.sp, color = p.textTertiary)
                    }
                    Column(horizontalAlignment = Alignment.End, verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        for (n in listOf(7L, 30L)) {
                            val d = weightChange(weights, n, today) ?: continue
                            ChangeChip(d, n)
                        }
                    }
                }
            } else {
                Text("No weigh-ins yet. Log your weight below to start the chart.", fontSize = 14.sp, color = p.textTertiary)
            }
            Spacer(Modifier.height(10.dp))
            WeightChart(weights.filter { it.date >= today.minusDays(30).toString() }, color, Modifier.fillMaxWidth().height(110.dp))
            Spacer(Modifier.height(10.dp))
            WeightInput(store, updating = logged != null)
        }
    }
}

@Composable
private fun ChangeChip(delta: Double, days: Long) {
    val p = LocalExtra.current
    val c = when {
        delta < 0 -> Green
        delta > 0 -> PrioHigh
        else -> p.textTertiary
    }
    Row(
        Modifier.clip(RoundedCornerShape(10.dp)).background(c.copy(alpha = 0.12f)).padding(horizontal = 8.dp, vertical = 3.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            when {
                delta < 0 -> Icons.AutoMirrored.Rounded.TrendingDown
                delta > 0 -> Icons.AutoMirrored.Rounded.TrendingUp
                else -> Icons.Rounded.Remove
            },
            null, tint = c, modifier = Modifier.size(13.dp),
        )
        Spacer(Modifier.width(4.dp))
        Text("${fmtWeight(delta, true)} kg in ${days}d", fontSize = 12.sp, color = c)
    }
}

/** A line through the weigh-ins, spaced by date, with a soft fill below. */
@Composable
fun WeightChart(entries: List<ApiWeight>, color: Color, modifier: Modifier = Modifier) {
    val p = LocalExtra.current
    Canvas(modifier) {
        val pts = entries.mapNotNull { e -> runCatching { LocalDate.parse(e.date).toEpochDay() to e.weight }.getOrNull() }
        // Three faint guide lines.
        for (i in 0..2) {
            val y = size.height * (0.1f + 0.4f * i)
            drawLine(p.divider, Offset(0f, y), Offset(size.width, y), strokeWidth = 1.dp.toPx())
        }
        if (pts.isEmpty()) return@Canvas
        val minX = pts.first().first
        val spanX = (pts.last().first - minX).coerceAtLeast(1)
        val minY = pts.minOf { it.second }
        val maxY = pts.maxOf { it.second }
        val pad = ((maxY - minY) * 0.15).coerceAtLeast(0.5)
        val lo = minY - pad
        val spanY = (maxY + pad) - lo
        fun at(x: Long, w: Double) = Offset(
            if (pts.size == 1) size.width / 2 else (x - minX).toFloat() / spanX * size.width,
            size.height - ((w - lo) / spanY).toFloat() * size.height,
        )
        val line = Path()
        pts.forEachIndexed { i, (x, w) -> val o = at(x, w); if (i == 0) line.moveTo(o.x, o.y) else line.lineTo(o.x, o.y) }
        if (pts.size > 1) {
            val fill = Path().apply {
                addPath(line)
                lineTo(at(pts.last().first, pts.last().second).x, size.height)
                lineTo(at(pts.first().first, pts.first().second).x, size.height)
                close()
            }
            drawPath(fill, Brush.verticalGradient(listOf(color.copy(alpha = 0.25f), color.copy(alpha = 0f))))
            drawPath(line, color, style = Stroke(width = 2.5.dp.toPx(), cap = StrokeCap.Round, join = StrokeJoin.Round))
        }
        val last = pts.last()
        drawCircle(color, radius = 4.dp.toPx(), center = at(last.first, last.second))
    }
}

/** "70.0 kg [Log]": logs today's weight. */
@Composable
fun WeightInput(store: HabitsStore, updating: Boolean, autofocus: Boolean = false, onLogged: () -> Unit = {}) {
    val p = LocalExtra.current
    var text by remember { mutableStateOf("") }
    var saving by remember { mutableStateOf(false) }
    val focus = remember { FocusRequester() }
    val kg = parseWeight(text)
    fun submit() {
        val v = kg ?: return
        if (saving) return
        saving = true
        store.logWeight(v) { ok -> saving = false; if (ok) { text = ""; onLogged() } }
    }
    Row(verticalAlignment = Alignment.CenterVertically) {
        OutlinedTextField(
            value = text,
            onValueChange = { text = it.filter { c -> c.isDigit() || c == '.' || c == ',' }.take(6) },
            placeholder = { Text("70.0", color = p.textTertiary) },
            suffix = { Text("kg", color = p.textTertiary) },
            singleLine = true,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal, imeAction = ImeAction.Done),
            keyboardActions = KeyboardActions(onDone = { submit() }),
            colors = OutlinedTextFieldDefaults.colors(focusedBorderColor = Accent, unfocusedBorderColor = p.divider, focusedTextColor = p.text, unfocusedTextColor = p.text, cursorColor = Accent),
            shape = RoundedCornerShape(12.dp),
            modifier = Modifier.weight(1f).focusRequester(focus),
        )
        Spacer(Modifier.width(10.dp))
        Button(
            onClick = ::submit,
            enabled = kg != null && !saving,
            colors = ButtonDefaults.buttonColors(containerColor = Accent),
            shape = RoundedCornerShape(12.dp),
        ) { Text(if (updating) "Update" else "Log") }
    }
    if (autofocus) androidx.compose.runtime.LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }
}
