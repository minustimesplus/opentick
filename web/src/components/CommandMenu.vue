<script setup lang="ts">
import {computed, nextTick, onMounted, ref, watch} from 'vue'
import {Command, Search} from 'lucide-vue-next'
import TaskCheckbox from './TaskCheckbox.vue'
import {useData} from '@/stores/data'
import {t} from '@/lib/i18n'
import {canRun, formatBinding, runShortcut, SHORTCUT_SECTIONS, type ShortcutId} from '@/lib/shortcuts'
import {openTaskDrawer} from '@/lib/ui'

/** Ctrl+K: run any shortcut's action by name, or jump to a task. */
const emit = defineEmits<{close: []}>()
const data = useData()

const query = ref('')
const active = ref(0)
const inputEl = ref<HTMLInputElement | null>(null)
const listEl = ref<HTMLElement | null>(null)

onMounted(() => inputEl.value?.focus())

/** Shortcuts that make no sense from here: this menu itself, Save, and keys that need the list focused. */
const HIDDEN = new Set<ShortcutId>(['commandMenu', 'save', 'addBelow', 'addSubtask'])

type Item =
	| {kind: 'command', key: string, id: ShortcutId, label: string, keys: string}
	| {kind: 'task', key: string, id: number, label: string}

// Read once on open: what the view behind the menu can do.
const commands = SHORTCUT_SECTIONS.flatMap(s => s.rows)
	.filter(r => r.id && !HIDDEN.has(r.id) && canRun(r.id))
	.map(r => ({kind: 'command' as const, key: `c:${r.id}`, id: r.id!, label: t(r.label), keys: formatBinding(r.id!)}))

const items = computed<Item[]>(() => {
	const q = query.value.trim().toLowerCase()
	const cmds = q ? commands.filter(c => c.label.toLowerCase().includes(q)) : commands
	const tasks: Item[] = q
		? data.openTasks
			.filter(task => task.title.toLowerCase().includes(q))
			.slice(0, 8)
			.map(task => ({kind: 'task', key: `t:${task.id}`, id: task.id, label: task.title}))
		: []
	return [...cmds, ...tasks]
})

watch(query, () => { active.value = 0 })
watch(active, i => nextTick(() => {
	listEl.value?.querySelectorAll('.cmd')[i]?.scrollIntoView({block: 'nearest'})
}))

function choose(item: Item | undefined) {
	if (!item) return
	emit('close')
	// After the menu is gone, so the action sees the view, not the menu.
	nextTick(() => {
		if (item.kind === 'task') openTaskDrawer(item.id)
		else runShortcut(item.id)
	})
}

function onKey(e: KeyboardEvent) {
	const n = items.value.length
	if (e.key === 'ArrowDown' && n) {
		e.preventDefault()
		active.value = (active.value + 1) % n
	} else if (e.key === 'ArrowUp' && n) {
		e.preventDefault()
		active.value = (active.value + n - 1) % n
	} else if (e.key === 'Enter') {
		e.preventDefault()
		choose(items.value[active.value])
	} else if (e.key === 'Escape') {
		e.preventDefault()
		e.stopPropagation()
		emit('close')
	}
}

function task(id: number) {
	return data.tasks.get(id)
}
</script>

<template>
	<Teleport to="body">
		<div class="cmd-overlay" @mousedown.self="emit('close')">
			<div class="cmd-box" role="dialog" aria-modal="true" :aria-label="t('scCommandMenu')">
				<div class="cmd-input">
					<Search :size="17" />
					<input ref="inputEl" v-model="query" :placeholder="t('commandPlaceholder')" @keydown="onKey">
				</div>
				<div ref="listEl" class="cmd-list">
					<template v-for="(item, i) in items" :key="item.key">
						<div v-if="item.kind === 'task' && (i === 0 || items[i - 1].kind !== 'task')" class="cmd-label">{{ t('tasks') }}</div>
						<button class="cmd" :class="{active: i === active}" @mouseenter="active = i" @click="choose(item)">
							<template v-if="item.kind === 'command'">
								<Command :size="15" class="ic" />
								<span class="label">{{ item.label }}</span>
								<kbd v-if="item.keys">{{ item.keys }}</kbd>
							</template>
							<template v-else>
								<TaskCheckbox v-if="task(item.id)" :done="task(item.id)!.done" :priority="task(item.id)!.priority" @toggle="data.toggleDone(item.id)" />
								<span class="label">{{ item.label }}</span>
							</template>
						</button>
					</template>
					<div v-if="!items.length" class="cmd-empty">{{ t('nothingFound') }}</div>
				</div>
			</div>
		</div>
	</Teleport>
</template>

<style scoped>
.cmd-overlay {
	position: fixed;
	inset: 0;
	z-index: 950;
	display: flex;
	justify-content: center;
	align-items: flex-start;
	padding-top: 14vh;
	background: rgba(0, 0, 0, 0.35);
}

.cmd-box {
	display: flex;
	flex-direction: column;
	width: min(560px, calc(100vw - 32px));
	max-height: 60vh;
	border-radius: 12px;
	background: var(--bg-elev);
	box-shadow: var(--shadow);
	overflow: hidden;
}

.cmd-input {
	display: flex;
	align-items: center;
	gap: 10px;
	padding: 0 14px;
	border-bottom: 1px solid var(--border);
	color: var(--text-3);
}

.cmd-input input {
	flex: 1;
	height: 48px;
	border: 0;
	outline: none;
	background: transparent;
	font-size: 15px;
	color: var(--text);
}

.cmd-list {
	padding: 6px;
	overflow-y: auto;
}

.cmd-label {
	padding: 8px 10px 4px;
	font-size: 11px;
	color: var(--text-3);
}

.cmd {
	display: flex;
	align-items: center;
	gap: 10px;
	width: 100%;
	min-height: 36px;
	padding: 0 10px;
	border-radius: 7px;
	text-align: left;
}

.cmd.active {
	background: var(--bg-hover);
}

.ic {
	color: var(--text-3);
}

.label {
	flex: 1;
	overflow: hidden;
	white-space: nowrap;
	text-overflow: ellipsis;
}

kbd {
	padding: 1px 6px;
	border-radius: 5px;
	border: 1px solid var(--border-strong);
	font: inherit;
	font-size: 11px;
	color: var(--text-3);
}

.cmd-empty {
	padding: 18px;
	text-align: center;
	color: var(--text-3);
}
</style>
