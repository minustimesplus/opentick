package org.opentick.android.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

// TickTick's palette, as used by the web app (web/src/styles.css).
val Accent = Color(0xFF4772FA)
val PrioHigh = Color(0xFFE5484D)
val PrioMedium = Color(0xFFF5A623)
val PrioLow = Color(0xFF4772FA)
val Q4Green = Color(0xFF2BB673)

@Immutable
data class Extra(
    val textSecondary: Color,
    val textTertiary: Color,
    val divider: Color,
    val panel: Color,
    val prioNone: Color,
    val selected: Color,
)

val LocalExtra = staticCompositionLocalOf {
    Extra(Color.Gray, Color.LightGray, Color.LightGray, Color.White, Color.Gray, Color.LightGray)
}

private val Light = lightColorScheme(
    primary = Accent,
    onPrimary = Color.White,
    primaryContainer = Color(0xFFE9EEFC),
    onPrimaryContainer = Accent,
    secondary = Accent,
    background = Color.White,
    onBackground = Color(0xFF191919),
    surface = Color.White,
    onSurface = Color(0xFF191919),
    surfaceVariant = Color(0xFFF3F3F3),
    onSurfaceVariant = Color(0xFF4A4A4A),
    surfaceContainer = Color(0xFFF7F7F7),
    surfaceContainerLow = Color(0xFFF7F7F7),
    surfaceContainerHigh = Color.White,
    surfaceContainerHighest = Color(0xFFF3F3F3),
    outline = Color(0xFFDCDCDC),
    outlineVariant = Color(0xFFECECEC),
    error = PrioHigh,
)

private val Dark = darkColorScheme(
    primary = Accent,
    onPrimary = Color.White,
    primaryContainer = Color(0xFF243266),
    onPrimaryContainer = Color(0xFFB9C8FF),
    secondary = Accent,
    background = Color(0xFF121212),
    onBackground = Color(0xFFE8E8E8),
    surface = Color(0xFF121212),
    onSurface = Color(0xFFE8E8E8),
    surfaceVariant = Color(0xFF242424),
    onSurfaceVariant = Color(0xFFB8B8B8),
    surfaceContainer = Color(0xFF1B1B1B),
    surfaceContainerLow = Color(0xFF1B1B1B),
    surfaceContainerHigh = Color(0xFF1F1F1F),
    surfaceContainerHighest = Color(0xFF262626),
    outline = Color(0xFF3A3A3A),
    outlineVariant = Color(0xFF2D2D2D),
    error = PrioHigh,
)

@Composable
fun OpenTickTheme(themePref: Int, content: @Composable () -> Unit) {
    val dark = when (themePref) {
        1 -> false
        2 -> true
        else -> isSystemInDarkTheme()
    }
    val extra = if (dark) {
        Extra(Color(0xFFB8B8B8), Color(0xFF868686), Color(0xFF2D2D2D), Color(0xFF1B1B1B), Color(0xFF6E6E6E), Color(0xFF243266))
    } else {
        Extra(Color(0xFF4A4A4A), Color(0xFF8C8C8C), Color(0xFFECECEC), Color(0xFFF7F7F7), Color(0xFFB0B0B0), Color(0xFFE9EEFC))
    }
    val base = Typography()
    val typography = base.copy(
        titleLarge = base.titleLarge.copy(fontWeight = FontWeight.Bold, fontSize = 22.sp),
        bodyLarge = TextStyle(fontSize = 16.sp),
        bodyMedium = TextStyle(fontSize = 14.sp),
    )
    CompositionLocalProvider(LocalExtra provides extra) {
        MaterialTheme(colorScheme = if (dark) Dark else Light, typography = typography, content = content)
    }
}

fun priorityColor(priority: Int, none: Color): Color = when {
    priority >= 3 -> PrioHigh
    priority == 2 -> PrioMedium
    priority == 1 -> PrioLow
    else -> none
}

fun parseHex(hex: String, fallback: Color = Accent): Color = runCatching {
    val h = hex.removePrefix("#")
    if (h.length != 6) fallback else Color(0xFF000000 or h.toLong(16))
}.getOrDefault(fallback)
