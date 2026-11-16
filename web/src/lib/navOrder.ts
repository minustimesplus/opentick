/**
 * Applies the saved nav rail order (prefs.navOrder) to the default list of
 * view keys. Saved keys that no longer exist are dropped; keys missing from
 * the saved order (views added later) go right after the view they follow by
 * default, or first if that one is missing too.
 */
export function orderKeys(defaults: readonly string[], saved: readonly string[]): string[] {
	const known = new Set(defaults)
	const out = [...new Set(saved.filter(k => known.has(k)))]
	defaults.forEach((key, i) => {
		if (out.includes(key)) return
		const prev = i > 0 ? out.indexOf(defaults[i - 1]) : -1
		out.splice(prev + 1, 0, key)
	})
	return out
}

/**
 * Moves `key` to where `target` is in `order`: before it when moving up,
 * after it when moving down. Works on the full order, so hidden views keep
 * their places.
 */
export function moveKey(order: readonly string[], key: string, target: string): string[] {
	const from = order.indexOf(key)
	const to = order.indexOf(target)
	if (from < 0 || to < 0 || from === to) return [...order]
	const out = order.filter(k => k !== key)
	const at = out.indexOf(target)
	out.splice(from < to ? at + 1 : at, 0, key)
	return out
}
