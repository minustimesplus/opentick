package org.opentick.android.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.FloatingActionButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.TextMeasurer
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.drawText
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.util.splitEmoji
import org.opentick.android.util.today

/** White rounded card on the grey page, TickTick's basic surface. */
@Composable
fun OtCard(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(LocalExtra.current.card),
        content = content,
    )
}

/** Large bold page title row ("Today", "November", "Eisenhower Matrix"). */
@Composable
fun PageHeader(
    title: String,
    onMenu: (() -> Unit)? = null,
    onBack: (() -> Unit)? = null,
    titleExtra: @Composable RowScope.() -> Unit = {},
    actions: @Composable RowScope.() -> Unit = {},
) {
    val p = LocalExtra.current
    Row(
        Modifier.fillMaxWidth().statusBarsPadding().height(64.dp).padding(start = if (onMenu != null || onBack != null) 6.dp else 18.dp, end = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (onMenu != null) IconButton(onClick = onMenu) { Icon(Icons.Rounded.Menu, "Open lists", tint = p.icon, modifier = Modifier.size(26.dp)) }
        if (onBack != null) IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back", tint = p.icon) }
        if (onMenu != null || onBack != null) Spacer(Modifier.width(8.dp))
        Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically) {
            Text(title, fontSize = 20.sp, fontWeight = FontWeight.Bold, color = p.text, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
            titleExtra()
        }
        actions()
    }
}

/** TickTick's big round blue add button. */
@Composable
fun OtFab(onClick: () -> Unit, modifier: Modifier = Modifier) {
    FloatingActionButton(
        onClick = onClick,
        shape = CircleShape,
        containerColor = Accent,
        contentColor = Color.White,
        elevation = FloatingActionButtonDefaults.elevation(6.dp, 6.dp, 6.dp, 6.dp),
        modifier = modifier.size(60.dp).shadow(14.dp, CircleShape, ambientColor = Accent, spotColor = Accent),
    ) { Icon(Icons.Filled.Add, "Add task", modifier = Modifier.size(32.dp)) }
}

/** The icon of a list: its leading emoji, or TickTick's three lines. */
@Composable
fun ListIcon(title: String, size: Dp = 22.dp, tint: Color = LocalExtra.current.icon) {
    val (emoji, _) = splitEmoji(title)
    Box(Modifier.size(size), contentAlignment = Alignment.Center) {
        if (emoji != null) Text(emoji, fontSize = (size.value * 0.82f).sp)
        else Icon(Icons.Rounded.Menu, null, tint = tint, modifier = Modifier.size(size))
    }
}

/** A row in a settings card: colored icon, title, optional value and chevron. */
@Composable
fun SettingsRow(icon: ImageVector?, iconTint: Color, title: String, value: String? = null, subtitle: String? = null, titleColor: Color? = null, chevron: Boolean = true, trailing: (@Composable () -> Unit)? = null, onClick: (() -> Unit)?) {
    val p = LocalExtra.current
    Row(
        Modifier.fillMaxWidth().then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier).padding(horizontal = 18.dp, vertical = if (subtitle != null) 12.dp else 15.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (icon != null) {
            Icon(icon, null, tint = iconTint, modifier = Modifier.size(24.dp))
            Spacer(Modifier.width(16.dp))
        }
        Column(Modifier.weight(1f)) {
            Text(title, fontSize = 16.sp, color = titleColor ?: p.text)
            if (subtitle != null) Text(subtitle, fontSize = 13.sp, color = p.textTertiary)
        }
        if (value != null) Text(value, fontSize = 14.sp, color = p.textTertiary, maxLines = 1)
        trailing?.invoke()
        if (chevron && onClick != null) Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, null, tint = p.count, modifier = Modifier.size(22.dp))
    }
}

// ─── Tab bar icons, drawn like TickTick's ───────────────────────────────

/** Filled rounded square with a check: the Task tab. */
@Composable
fun TaskTabIcon(color: Color, modifier: Modifier = Modifier) {
    Canvas(modifier.size(26.dp)) {
        val s = size.minDimension
        val inset = s * 0.06f
        drawRoundRect(color, Offset(inset, inset), Size(s - 2 * inset, s - 2 * inset), CornerRadius(s * 0.16f))
        val stroke = Stroke(width = s * 0.1f, cap = StrokeCap.Round, join = StrokeJoin.Round)
        val path = androidx.compose.ui.graphics.Path().apply {
            moveTo(s * 0.28f, s * 0.52f); lineTo(s * 0.44f, s * 0.68f); lineTo(s * 0.73f, s * 0.36f)
        }
        drawPath(path, Color.White, style = stroke)
    }
}

/** Four rounded petals: the Eisenhower Matrix tab. */
@Composable
fun MatrixTabIcon(color: Color, modifier: Modifier = Modifier) {
    Canvas(modifier.size(26.dp)) {
        val s = size.minDimension
        val gap = s * 0.08f
        val cell = (s - gap) / 2
        for (r in 0..1) for (c in 0..1) {
            drawRoundRect(color, Offset(c * (cell + gap), r * (cell + gap)), Size(cell, cell), CornerRadius(cell * 0.46f))
        }
    }
}

/** A calendar page with today's day number: the Calendar tab and the Today list. */
@Composable
fun CalendarDayIcon(color: Color, modifier: Modifier = Modifier, size: Dp = 26.dp, day: Int = today().dayOfMonth) {
    val measurer = rememberTextMeasurer()
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        drawRoundRect(color, Offset(s * 0.04f, s * 0.12f), Size(s * 0.92f, s * 0.84f), CornerRadius(s * 0.16f))
        // Binder rings.
        for (x in listOf(0.27f, 0.73f)) {
            drawRoundRect(color, Offset(s * x - s * 0.06f, 0f), Size(s * 0.12f, s * 0.24f), CornerRadius(s * 0.06f))
        }
        drawCenteredText(measurer, "$day", TextStyle(color = Color.White, fontSize = (s * 0.42f / density).sp, fontWeight = FontWeight.Bold), Offset(s / 2, s * 0.56f))
    }
}

/** Ring with a dot: the Pomodoro tab. */
@Composable
fun PomoTabIcon(color: Color, modifier: Modifier = Modifier) {
    Canvas(modifier.size(26.dp)) {
        val s = size.minDimension
        drawCircle(color, radius = s * 0.42f, style = Stroke(width = s * 0.11f))
        drawCircle(color, radius = s * 0.17f)
    }
}

private fun androidx.compose.ui.graphics.drawscope.DrawScope.drawCenteredText(measurer: TextMeasurer, text: String, style: TextStyle, center: Offset) {
    val layout = measurer.measure(text, style)
    drawText(layout, topLeft = Offset(center.x - layout.size.width / 2f, center.y - layout.size.height / 2f))
}

/** Little roman numeral badge of a matrix quadrant. */
@Composable
fun RomanBadge(text: String, color: Color) {
    Box(Modifier.size(19.dp).clip(CircleShape).background(color), contentAlignment = Alignment.Center) {
        Text(
            text, color = Color.White, fontSize = if (text.length > 2) 9.sp else 11.sp, fontWeight = FontWeight.Bold,
            fontFamily = androidx.compose.ui.text.font.FontFamily.Serif, maxLines = 1,
        )
    }
}

/** A circle with the user's initial, standing in for the avatar. */
@Composable
fun Avatar(name: String, size: Dp) {
    Box(Modifier.size(size).clip(CircleShape).background(Accent), contentAlignment = Alignment.Center) {
        Text(name.take(1).uppercase().ifEmpty { "O" }, color = Color.White, fontWeight = FontWeight.Bold, fontSize = (size.value * 0.42f).sp)
    }
}
