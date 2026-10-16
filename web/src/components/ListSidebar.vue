<script setup lang="ts">
import {computed, ref} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {ChevronDown, ChevronRight, Filter, Inbox, Menu, Plus, SquareCheckBig, Tag, Pencil, Trash2, Folder} from 'lucide-vue-next'
import DayIcon from './DayIcon.vue'
import Popover from './Popover.vue'
import ListEditDialog from './ListEditDialog.vue'
import type {Project} from '@/api/types'
import {useData} from '@/stores/data'
import {hasColor, labelColor, projectColor, splitEmoji} from '@/lib/tasks'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {addDays, getSchedule, minutesOfDay, moveSchedule, schedulePatch, startOfDay, withMinutes} from '@/lib/dates'

const data = useData()
const route = useRoute()
const router = useRouter()

const current = computed(() => String(route.params.list ?? 'today'))
const todayNum = new Date().getDate()

interface Node {
	project: Project
	children: Project[]
}

const tree = computed<Node[]>(() => {
	const lists = data.listProjects
	const ids = new Set(lists.map(p => p.id))
	return lists
		.filter(p => !p.parent_project_id || !ids.has(p.parent_project_id))
		.map(p => ({project: p, children: lists.filter(c => c.parent_project_id === p.id)}))
})

function toggle(key: string) {
	prefs.collapsed[key] = !prefs.collapsed[key]
}

function go(key: string) {
	router.push(`/tasks/${key}`)
}

// ---- list context menu ----
const ctx = ref<{project: Project, x: number, y: number} | null>(null)
const editing = ref<Project | null | undefined>(undefined)

async function removeList(p: Project) {
	ctx.value = null
	if (!confirm(t('deleteListConfirm', {name: p.title}))) return
	await data.deleteProject(p.id)
	if (current.value === `p${p.id}`) go('today')
}

// ---- drop targets ----
const dropKey = ref<string | null>(null)

function onDrop(e: DragEvent, key: string) {
	dropKey.value = null
	const id = Number(e.dataTransfer?.getData('application/x-task-id'))
	const task = id ? data.tasks.get(id) : undefined
	if (!task) return
	if (key.startsWith('p')) {
		data.updateTask(id, {project_id: Number(key.slice(1))})
	} else if (key === 'inbox') {
		data.updateTask(id, {project_id: data.inboxId})
	} else if (key === 'today' || key === 'week') {
		const day = key === 'today' ? startOfDay(new Date()) : addDays(startOfDay(new Date()), 1)
		const s = getSchedule(task)
		data.updateTask(id, s && !s.allDay ? moveSchedule(s, withMinutes(day, minutesOfDay(s.start)), false) : schedulePatch(day, true))
	} else if (key.startsWith('l')) {
		const label = data.labels.find(l => l.id === Number(key.slice(1)))
		if (label) data.addLabel(id, label)
	}
}

function dropProps(key: string) {
	return {
		onDragover: (e: DragEvent) => {
			if (e.dataTransfer?.types.includes('application/x-task-id')) {
				e.preventDefault()
				dropKey.value = key
			}
		},
		onDragleave: () => {
			if (dropKey.value === key) dropKey.value = null
		},
		onDrop: (e: DragEvent) => onDrop(e, key),
	}
}
</script>

<template>
	<aside class="sidebar">
		<div class="scroll">
			<button class="item" :class="{active: current === 'today', drop: dropKey === 'today'}" v-bind="dropProps('today')" @click="go('today')">
				<DayIcon :label="todayNum" class="ic" />
				<span class="name">{{ t('today') }}</span>
				<span class="count">{{ data.counts.today || '' }}</span>
			</button>
			<button class="item" :class="{active: current === 'week', drop: dropKey === 'week'}" v-bind="dropProps('week')" @click="go('week')">
				<DayIcon label="7" class="ic" />
				<span class="name">{{ t('next7') }}</span>
				<span class="count">{{ data.counts.week || '' }}</span>
			</button>
			<button class="item" :class="{active: current === 'inbox', drop: dropKey === 'inbox'}" v-bind="dropProps('inbox')" @click="go('inbox')">
				<Inbox :size="18" class="ic" />
				<span class="name">{{ t('inbox') }}</span>
				<span class="count">{{ data.counts.byProject.get(data.inboxId) || '' }}</span>
			</button>

			<div class="sep" />

			<div class="section-head" @click="toggle('lists')">
				<span>{{ t('lists') }}</span>
				<button class="add" :title="t('newList')" @click.stop="editing = null"><Plus :size="15" /></button>
				<component :is="prefs.collapsed.lists ? ChevronRight : ChevronDown" :size="14" class="chev" />
			</div>
			<template v-if="!prefs.collapsed.lists">
				<template v-for="node in tree" :key="node.project.id">
					<button
						class="item"
						:class="{active: current === `p${node.project.id}`, drop: dropKey === `p${node.project.id}`}"
						v-bind="dropProps(`p${node.project.id}`)"
						@click="go(`p${node.project.id}`)"
						@contextmenu.prevent="ctx = {project: node.project, x: $event.clientX, y: $event.clientY}"
					>
						<template v-if="node.children.length">
							<span class="ic folder-toggle" @click.stop="toggle(`p${node.project.id}`)">
								<Folder :size="17" />
							</span>
						</template>
						<span v-else-if="splitEmoji(node.project.title).emoji" class="ic emoji">{{ splitEmoji(node.project.title).emoji }}</span>
						<Menu v-else :size="18" class="ic" />
						<span class="name">{{ splitEmoji(node.project.title).name }}</span>
						<span v-if="hasColor(node.project)" class="cdot" :style="{background: projectColor(node.project)}" />
						<span class="count">{{ data.counts.byProject.get(node.project.id) || '' }}</span>
					</button>
					<template v-if="node.children.length && !prefs.collapsed[`p${node.project.id}`]">
						<button
							v-for="c in node.children"
							:key="c.id"
							class="item child"
							:class="{active: current === `p${c.id}`, drop: dropKey === `p${c.id}`}"
							v-bind="dropProps(`p${c.id}`)"
							@click="go(`p${c.id}`)"
							@contextmenu.prevent="ctx = {project: c, x: $event.clientX, y: $event.clientY}"
						>
							<span v-if="splitEmoji(c.title).emoji" class="ic emoji">{{ splitEmoji(c.title).emoji }}</span>
							<Menu v-else :size="18" class="ic" />
							<span class="name">{{ splitEmoji(c.title).name }}</span>
							<span v-if="hasColor(c)" class="cdot" :style="{background: projectColor(c)}" />
							<span class="count">{{ data.counts.byProject.get(c.id) || '' }}</span>
						</button>
					</template>
				</template>
			</template>

			<template v-if="data.savedFilters.length">
				<div class="section-head" @click="toggle('filters')">
					<span>{{ t('filters') }}</span>
					<component :is="prefs.collapsed.filters ? ChevronRight : ChevronDown" :size="14" class="chev" />
				</div>
				<template v-if="!prefs.collapsed.filters">
					<button v-for="f in data.savedFilters" :key="f.id" class="item" :class="{active: current === `f${-f.id}`}" @click="go(`f${-f.id}`)">
						<Filter :size="17" class="ic" />
						<span class="name">{{ f.title }}</span>
					</button>
				</template>
			</template>

			<div class="section-head" @click="toggle('tags')">
				<span>{{ t('tags') }}</span>
				<component :is="prefs.collapsed.tags ? ChevronRight : ChevronDown" :size="14" class="chev" />
			</div>
			<template v-if="!prefs.collapsed.tags">
				<button
					v-for="l in data.labels"
					:key="l.id"
					class="item"
					:class="{active: current === `l${l.id}`, drop: dropKey === `l${l.id}`}"
					v-bind="dropProps(`l${l.id}`)"
					@click="go(`l${l.id}`)"
				>
					<Tag :size="16" class="ic" :style="{color: labelColor(l.hex_color)}" />
					<span class="name">{{ l.title }}</span>
					<span class="count">{{ data.counts.byLabel.get(l.id) || '' }}</span>
				</button>
			</template>
		</div>

		<div class="bottom">
			<button class="item" :class="{active: current === 'completed'}" @click="go('completed')">
				<SquareCheckBig :size="17" class="ic" />
				<span class="name">{{ t('completed') }}</span>
			</button>
		</div>

		<Popover v-if="ctx" :anchor="{x: ctx.x, y: ctx.y}" @close="ctx = null">
			<div class="menu">
				<button class="menu-item" @click="editing = ctx!.project; ctx = null"><Pencil :size="15" /> {{ t('edit') }}</button>
				<button class="menu-item danger" @click="removeList(ctx!.project)"><Trash2 :size="15" /> {{ t('delete') }}</button>
			</div>
		</Popover>
		<ListEditDialog v-if="editing !== undefined" :project="editing" @close="editing = undefined" @saved="id => go(`p${id}`)" />
	</aside>
</template>

<style scoped>
.sidebar {
	display: flex;
	flex-direction: column;
	width: 260px;
	flex-shrink: 0;
	border-right: 1px solid var(--border);
	background: var(--bg);
}

.scroll {
	flex: 1;
	overflow: auto;
	padding: 14px 10px;
}

.item {
	display: flex;
	align-items: center;
	gap: 12px;
	width: 100%;
	height: 40px;
	padding: 0 12px;
	border-radius: 8px;
	text-align: left;
	color: var(--text);
	font-size: 15px;
}

.item:hover {
	background: var(--bg-hover);
}

.item.active {
	background: var(--bg-active);
}

.item.drop {
	outline: 2px solid var(--accent);
	outline-offset: -2px;
}

.item.child {
	padding-left: 34px;
}

.ic {
	flex-shrink: 0;
	color: var(--text-2);
	width: 18px;
	display: inline-flex;
	justify-content: center;
}

.emoji {
	font-size: 15px;
}

.name {
	flex: 1;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.count {
	font-size: 13px;
	color: var(--text-3);
	min-width: 18px;
	text-align: right;
}

.cdot {
	width: 8px;
	height: 8px;
	border-radius: 50%;
	flex-shrink: 0;
}

.sep {
	height: 1px;
	margin: 10px 8px;
	background: var(--border);
}

.section-head {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 36px;
	padding: 0 12px;
	margin-top: 8px;
	font-size: 13px;
	color: var(--text-3);
	cursor: pointer;
	user-select: none;
}

.section-head span {
	flex: 1;
}

.section-head .add {
	display: none;
	color: var(--text-3);
}

.section-head:hover .add {
	display: flex;
}

.chev {
	color: var(--text-4);
}

.bottom {
	padding: 8px 10px 12px;
	border-top: 1px solid var(--border);
}
</style>
