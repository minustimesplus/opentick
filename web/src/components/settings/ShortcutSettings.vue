<script setup lang="ts">
import {t, type MessageKey} from '@/lib/i18n'

/** Label, key combo (null: not set) and whether the user may rebind it. */
type Shortcut = [MessageKey, string | null, boolean?]

const sections: {title: MessageKey, desc?: MessageKey, items: Shortcut[]}[] = [
	{title: 'general', items: [
		['save', 'Ctrl+S'],
		['sync', '.', true],
		['cancel', 'Esc'],
		['undo', 'Ctrl+Z'],
		['scRedo', 'Ctrl+Shift+Z'],
		['scPrint', 'Ctrl+P', true],
		['scCommandMenu', 'Ctrl+K', true],
		['shortcutsShort', '?'],
	]},
	{title: 'scTask', items: [
		['addTask', 'Tab+N / N', true],
		['scAddBelow', 'Enter'],
		['scAddSubtask', 'Shift+Enter'],
		['scToggleGroups', null, true],
		['scToggleSubtasks', 'Tab+E', true],
		['scListView', 'V then L'],
		['scKanbanView', 'V then K'],
		['scTimelineView', 'V then T'],
	]},
	{title: 'scQuickAdd', desc: 'scQuickAddDesc', items: [
		['scSetDueDate', '*'],
		['scSetPriority', '!'],
		['scSetTag', '#'],
		['scMoveToList', '~ / ^'],
		['scAssign', '@'],
	]},
	{title: 'scEditTasks', desc: 'scEditTasksDesc', items: [
		['scComplete', 'Tab+M', true],
		['scPin', 'Tab+P'],
		['scDelete', 'Ctrl+Del'],
		['scSetDate', 'Tab+D'],
		['scClearTime', 'Tab+0'],
		['scSetToday', 'Tab+1'],
		['scSetTomorrow', 'Tab+2'],
		['scSetNextWeek', 'Tab+3'],
		['scSetPriorityLevel', 'Alt+0/1/2/3'],
	]},
	{title: 'scNavigation', desc: 'scNavigationDesc', items: [
		['scGoCalendar', null, true],
		['scGoMatrix', null, true],
		['scGoAi', null, true],
		['scSplitView', null, true],
		['scGoSearch', '/', true],
		['scGoSettings', 'G then S'],
		['scGoAll', 'G then A'],
		['scGoToday', 'G then T'],
		['scGoTomorrow', 'G then R'],
		['scGoNext7', 'G then N'],
		['scGoAssigned', 'G then M'],
		['scGoInbox', 'G then I'],
		['scGoCompleted', 'G then C'],
		['scGoWontDo', 'G then W'],
		['scGoSummary', 'G then B'],
		['scGoTrash', 'G then G'],
	]},
	{title: 'featCalendar', items: [
		['scDayView', 'D / 1'],
		['scWeekView', 'W / 2'],
		['scMonthView', 'M / 3'],
		['scYearView', 'Y / 4'],
		['scAgenda', 'A / 5'],
		['scBackToToday', 'T'],
		['scGoToDate', null, true],
	]},
]
</script>

<template>
	<template v-for="section in sections" :key="section.title">
		<h2 class="s-title">{{ t(section.title) }}</h2>
		<p v-if="section.desc" class="section-desc">{{ t(section.desc) }}</p>
		<div class="s-card">
			<div v-for="[label, keys, custom] in section.items" :key="label" class="s-row">
				<span>{{ t(label) }}</span>
				<button class="key" :class="{custom}">{{ keys ?? t('scSetShortcut') }}</button>
			</div>
		</div>
	</template>

	<div class="footer">
		<button class="btn reset">{{ t('resetDefault') }}</button>
	</div>
</template>

<style scoped>
.section-desc {
	margin: -6px 0 10px 15px;
	font-size: 11px;
	color: var(--text-3);
}

.key {
	width: 84px;
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
	cursor: pointer;
}

.key.custom:hover {
	border-color: var(--accent);
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
