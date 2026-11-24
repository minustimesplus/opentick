<script setup lang="ts">
import {computed} from 'vue'
import {type EarlyEnd, useFocus} from '@/stores/focus'
import {t} from '@/lib/i18n'

/** TickTick's confirmation for ending a pomo before its time; which one depends on how far it got. */
const props = defineProps<{kind: EarlyEnd, compact?: boolean}>()
const emit = defineEmits<{close: []}>()
const focus = useFocus()

const text = computed(() => ({
	complete: {title: t('completeAheadTitle'), body: t('completeAheadBody')},
	saveOrAbandon: {title: t('endEarlyTitle'), body: t('endEarlyBody')},
	abandon: {title: t('abandonFocusTitle'), body: t('abandonFocusBody')},
})[props.kind])

function act(how: 'complete' | 'save' | 'abandon') {
	focus.endEarly(how)
	emit('close')
}
</script>

<template>
	<div class="end-prompt" :class="{compact}">
		<div class="title">{{ text.title }}</div>
		<p v-if="!compact" class="body">{{ text.body }}</p>
		<div class="actions">
			<button class="btn" @click="emit('close')">{{ t('cancel') }}</button>
			<template v-if="kind === 'complete'">
				<button class="btn" @click="act('abandon')">{{ t('abandon') }}</button>
				<button class="btn primary" @click="act('complete')">{{ t('completeAhead') }}</button>
			</template>
			<template v-else-if="kind === 'saveOrAbandon'">
				<button class="btn" @click="act('abandon')">{{ t('abandon') }}</button>
				<button class="btn primary" @click="act('save')">{{ t('save') }}</button>
			</template>
			<button v-else class="btn primary danger" @click="act('abandon')">{{ t('abandon') }}</button>
		</div>
	</div>
</template>

<style scoped>
.title {
	font-size: 16px;
	font-weight: 600;
}

.body {
	margin: 8px 0 0;
	font-size: 13px;
	line-height: 1.5;
	color: var(--text-2);
}

.actions {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
	margin-top: 18px;
}

.btn.primary.danger {
	background: var(--danger);
	border-color: var(--danger);
}

.compact .title {
	font-size: 13px;
	text-align: center;
}

.compact .actions {
	justify-content: center;
	gap: 6px;
	margin-top: 8px;
}

.compact .btn {
	height: 26px;
	padding: 0 10px;
	font-size: 12px;
}
</style>
