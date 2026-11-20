<script setup lang="ts">
import {onBeforeUnmount, ref} from 'vue'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'
import {
	binding, formatBinding, formatSequence, keyToken, rebind, resetShortcuts, SHORTCUT_SECTIONS, isDefault,
	type ShortcutId,
} from '@/lib/shortcuts'
import {prefs} from '@/lib/prefs'

/** The shortcut waiting for new keys, and the keys pressed so far. */
const recording = ref<ShortcutId | null>(null)
const draft = ref<string[]>([])
let timer: number | undefined

const labels = new Map(SHORTCUT_SECTIONS.flatMap(s => s.rows).filter(r => r.id).map(r => [r.id!, r.label]))

function start(id: ShortcutId) {
	stop()
	recording.value = id
	draft.value = []
	window.addEventListener('keydown', onKey, true)
}

function stop() {
	clearTimeout(timer)
	recording.value = null
	draft.value = []
	window.removeEventListener('keydown', onKey, true)
}

function commit() {
	const id = recording.value
	const seq = draft.value.join(' ')
	stop()
	if (!id || !seq) return
	const taken = rebind(id, seq)
	if (taken) toast(t('scTakenFrom', {keys: formatSequence(seq), name: t(labels.get(taken) ?? 'shortcutsShort')}))
}

// A plain key may start a two-key sequence (G then C, Tab+M): wait briefly for a second one.
function onKey(e: KeyboardEvent) {
	const token = keyToken(e)
	if (!token) return
	e.preventDefault()
	e.stopPropagation()
	if (!draft.value.length) {
		if (token === 'esc') return stop()
		if (token === 'backspace' || token === 'delete') {
			const id = recording.value!
			stop()
			rebind(id, null)
			return
		}
	}
	draft.value = [...draft.value, token]
	clearTimeout(timer)
	if (draft.value.length >= 2 || token.includes('+')) commit()
	else timer = window.setTimeout(commit, 900)
}

onBeforeUnmount(stop)

function reset() {
	stop()
	resetShortcuts()
}
</script>

<template>
	<template v-for="section in SHORTCUT_SECTIONS" :key="section.title">
		<h2 class="s-title">{{ t(section.title) }}</h2>
		<p v-if="section.desc" class="section-desc">{{ t(section.desc) }}</p>
		<div class="s-card">
			<div v-for="row in section.rows" :key="row.label" class="s-row">
				<span class="s-label">{{ t(row.label) }}</span>
				<span v-if="row.keys" class="key">{{ row.keys }}</span>
				<button
					v-else-if="row.id"
					class="key custom"
					:class="{recording: recording === row.id, changed: !isDefault(row.id)}"
					:title="recording === row.id ? t('scRecordHint') : t('scClickToChange')"
					@click="recording === row.id ? stop() : start(row.id)"
				>
					<template v-if="recording === row.id">{{ draft.length ? draft.map(formatSequence).join(' ') + '…' : t('scPressKeys') }}</template>
					<template v-else>{{ binding(row.id).length ? formatBinding(row.id) : t('scSetShortcut') }}</template>
				</button>
			</div>
		</div>
	</template>

	<div class="footer">
		<button class="btn reset" :disabled="!Object.keys(prefs.shortcuts).length" @click="reset">{{ t('resetDefault') }}</button>
	</div>
</template>

<style scoped>
.section-desc {
	margin: -6px 0 10px 15px;
	font-size: 11px;
	color: var(--text-3);
}

.key {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	min-width: 84px;
	max-width: 160px;
	height: 18px;
	flex-shrink: 0;
	padding: 0 6px;
	overflow: hidden;
	border-radius: 6px;
	border: 1px solid var(--border-strong);
	font-size: 11px;
	white-space: nowrap;
	text-overflow: ellipsis;
	color: var(--text-3);
	cursor: default;
}

.key.custom {
	color: var(--text-2);
}

.key.custom {
	cursor: pointer;
}

.key.custom:hover,
.key.recording {
	border-color: var(--accent);
}

.key.recording {
	color: var(--accent);
}

.key.changed {
	color: var(--text);
}

.footer {
	position: sticky;
	bottom: -20px;
	display: flex;
	justify-content: flex-end;
	margin: 0 -20px -20px;
	padding: 10px 20px;
	background: var(--bg);
}

.reset {
	height: 28px;
	padding: 0 10px;
	font-size: 12px;
	color: var(--text-2);
}
</style>
