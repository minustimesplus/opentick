package org.opentick.android.util

/**
 * Splits a leading emoji off a list title ("🏔️ University" -> icon + name),
 * like the web app. TickTick shows that emoji as the list's icon.
 */
fun splitEmoji(title: String): Pair<String?, String> {
    fun pictographic(cp: Int) =
        cp in 0x1F000..0x1FAFF || cp in 0x2600..0x27BF || cp in 0x2300..0x23FF || cp in 0x2B00..0x2BFF
    fun modifier(cp: Int) = cp == 0xFE0F || cp in 0x1F3FB..0x1F3FF || cp in 0xE0020..0xE007F

    if (title.isEmpty() || !pictographic(title.codePointAt(0))) return null to title
    var i = Character.charCount(title.codePointAt(0))
    while (i < title.length) {
        val cp = title.codePointAt(i)
        when {
            modifier(cp) -> i += Character.charCount(cp)
            // Zero width joiner: the next pictograph belongs to the same emoji.
            cp == 0x200D && i + 1 < title.length -> {
                i += 1
                i += Character.charCount(title.codePointAt(i))
            }
            // Flags are two regional indicators.
            cp in 0x1F1E6..0x1F1FF && title.codePointAt(0) in 0x1F1E6..0x1F1FF && i == 2 -> i += 2
            else -> break
        }
    }
    val rest = title.substring(i).trim()
    return title.substring(0, i) to rest.ifEmpty { title }
}
