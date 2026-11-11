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
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

// Colors sampled from TickTick's Android app.
val Accent = Color(0xFF4772FA)
val PrioHigh = Color(0xFFDE433C)
val PrioMedium = Color(0xFFF9B00C)
val PrioLow = Color(0xFF4772FA)
val Overdue = Color(0xFFE5483F)
val Orange = Color(0xFFFF5A00)
val Cyan = Color(0xFF12C1EC)
val Green = Color(0xFF0DCD9C)
val Yellow = Color(0xFFFFB001)
val Q1 = Color(0xFFF0656C)
val Q2 = Color(0xFFFFAF00)
val Q3 = Color(0xFF4772FA)
val Q4 = Color(0xFF0BCD9B)
val NowRed = Color(0xFFE5483F)
val MinusRed = Color(0xFFE03D38)
// Kept for older call sites.
val Q4Green = Q4

@Immutable
data class Palette(
    val dark: Boolean,
    /** Page background behind the white cards. */
    val page: Color,
    val card: Color,
    val text: Color,
    val textSecondary: Color,
    val textTertiary: Color,
    val count: Color,
    val divider: Color,
    val selected: Color,
    val prioNone: Color,
    val icon: Color,
    val navIcon: Color,
    val gridBg: Color,
    val gridLine: Color,
    val weekday: Color,
    val drawerTop: Color,
) {
    // Older names.
    val panel get() = page
}

val LightPalette = Palette(
    dark = false,
    page = Color(0xFFF3F4F9), card = Color.White,
    text = Color(0xFF191919), textSecondary = Color(0xFF707175), textTertiary = Color(0xFFA2A2A2), count = Color(0xFFA2A2A2),
    divider = Color(0xFFE8E8E8), selected = Color(0xFFEEF0FC), prioNone = Color(0xFFABABAB),
    icon = Color(0xFF191919), navIcon = Color(0xFF8A8B8F),
    gridBg = Color(0xFFFCFDFF), gridLine = Color(0xFFEEEFF1), weekday = Color(0xFF707175),
    drawerTop = Color(0xFFF0F2FF),
)

val DarkPalette = Palette(
    dark = true,
    page = Color(0xFF121212), card = Color(0xFF1E1E1E),
    text = Color(0xFFE8E8E8), textSecondary = Color(0xFFA0A0A4), textTertiary = Color(0xFF6E6E72), count = Color(0xFF7A7A7E),
    divider = Color(0xFF2C2C2C), selected = Color(0xFF262B3D), prioNone = Color(0xFF6E6E6E),
    icon = Color(0xFFE8E8E8), navIcon = Color(0xFF8A8B8F),
    gridBg = Color(0xFF181818), gridLine = Color(0xFF262626), weekday = Color(0xFFA0A0A4),
    drawerTop = Color(0xFF1A1D2B),
)

val LocalExtra = staticCompositionLocalOf { LightPalette }

@Composable
fun OpenTickTheme(themePref: Int, content: @Composable () -> Unit) {
    val dark = when (themePref) {
        1 -> false
        2 -> true
        else -> isSystemInDarkTheme()
    }
    val p = if (dark) DarkPalette else LightPalette
    val scheme = if (dark) {
        darkColorScheme(
            primary = Accent, onPrimary = Color.White, secondary = Accent,
            background = p.page, onBackground = p.text, surface = p.card, onSurface = p.text,
            surfaceVariant = p.page, onSurfaceVariant = p.textSecondary,
            surfaceContainer = p.card, surfaceContainerLow = p.card, surfaceContainerHigh = p.card, surfaceContainerHighest = p.card,
            outline = p.divider, outlineVariant = p.divider, error = PrioHigh,
        )
    } else {
        lightColorScheme(
            primary = Accent, onPrimary = Color.White, primaryContainer = Color(0xFFE9EEFC), onPrimaryContainer = Accent, secondary = Accent,
            background = p.page, onBackground = p.text, surface = p.card, onSurface = p.text,
            surfaceVariant = p.page, onSurfaceVariant = p.textSecondary,
            surfaceContainer = p.card, surfaceContainerLow = p.card, surfaceContainerHigh = p.card, surfaceContainerHighest = p.card,
            outline = p.divider, outlineVariant = p.divider, error = PrioHigh,
        )
    }
    val base = Typography()
    val typography = base.copy(
        titleLarge = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.Bold),
        bodyLarge = TextStyle(fontSize = 16.sp),
        bodyMedium = TextStyle(fontSize = 14.sp),
    )
    CompositionLocalProvider(LocalExtra provides p) {
        MaterialTheme(colorScheme = scheme, typography = typography, content = content)
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

/** Calendar block colors: a pastel fill and a dark text in the list's hue, like TickTick. */
fun eventColors(base: Color, dark: Boolean): Pair<Color, Color> =
    if (dark) lerp(base, Color.Black, 0.55f) to lerp(base, Color.White, 0.75f)
    else lerp(base, Color.White, 0.55f) to lerp(base, Color.Black, 0.62f)
