<script setup lang="ts">
import {computed, reactive, ref} from 'vue'
import {ChevronLeft} from 'lucide-vue-next'
import {addMonths, fmtWeekdayLong, fmtWeekdayNarrow, startOfWeek} from '@/lib/dates'
import {t} from '@/lib/i18n'
import {describeRepeat, emptySpec, unitLabel, weekdayOrdinal, type RepeatFreq, type RepeatSpec} from '@/lib/repeat'

const props = defineProps<{spec: RepeatSpec | null, anchor: Date}>()
const emit = defineEmits<{save: [RepeatSpec], cancel: []}>()

const draft = reactive<RepeatSpec>(props.spec && !props.spec.raw
	? {...props.spec, weekdays: [...props.spec.weekdays]}
	: {...emptySpec(props.spec?.freq ?? 'WEEKLY'), afterCompletion: props.spec?.afterCompletion ?? false})
if (draft.freq === 'WEEKLY' && !draft.weekdays.length) draft.weekdays = [props.anchor.getDay()]

type EndKind = 'never' | 'until' | 'count'
const endKind = ref<EndKind>(draft.count ? 'count' : draft.until ? 'until' : 'never')
const toInput = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const untilInput = ref(toInput(draft.until ?? addMonths(props.anchor, 1)))
const countInput = ref(draft.count ?? 10)

const FREQS: RepeatFreq[] = ['DAILY', 'WEEKLY', 'MONTHLY', 'YEARLY']
// Weekday chips in the user's week order.
const weekdays = computed(() => {
	const first = startOfWeek(props.anchor)
	return Array.from({length: 7}, (_, i) => {
		const d = new Date(first)
		d.setDate(first.getDate() + i)
		return {day: d.getDay(), label: fmtWeekdayNarrow(d)}
	})
})

function toggleDay(d: number) {
	const i = draft.weekdays.indexOf(d)
	if (i >= 0) {
		if (draft.weekdays.length > 1) draft.weekdays.splice(i, 1)
	} else {
		draft.weekdays.push(d)
	}
}

const nthLabel = computed(() => {
	const n = weekdayOrdinal(props.anchor)
	return t('repOnNth', {nth: t(n === -1 ? 'ordLast' : (['ord1', 'ord2', 'ord3', 'ord4'] as const)[n - 1]), day: fmtWeekdayLong(props.anchor)})
})

function result(): RepeatSpec {
	const interval = Math.min(999, Math.max(1, Math.round(Number(draft.interval)) || 1))
	const until = endKind.value === 'until' && untilInput.value ? new Date(`${untilInput.value}T00:00:00`) : null
	const count = endKind.value === 'count' ? Math.min(99999, Math.max(1, Math.round(Number(countInput.value)) || 1)) : null
	return {
		...draft,
		interval,
		weekdays: draft.freq === 'WEEKLY' ? [...draft.weekdays] : [],
		monthMode: draft.freq === 'MONTHLY' ? draft.monthMode : 'day',
		until,
		count,
		raw: null,
	}
}

const summary = computed(() => describeRepeat(result(), props.anchor))
</script>

<template>
	<div class="re" @keydown.enter.prevent.stop="emit('save', result())">
		<div class="head">
			<button class="icon-btn sm" :title="t('back')" @click="emit('cancel')"><ChevronLeft :size="16" /></button>
			<span class="title">{{ t('repCustom') }}</span>
		</div>

		<div class="line">
			<span>{{ t('repEvery') }}</span>
			<input v-model.number="draft.interval" type="number" min="1" max="999" class="num">
			<select v-model="draft.freq" class="sel">
				<option v-for="f in FREQS" :key="f" :value="f">{{ unitLabel(f, draft.interval) }}</option>
			</select>
		</div>

		<template v-if="!draft.afterCompletion">
			<div v-if="draft.freq === 'WEEKLY'" class="days">
				<button
					v-for="w in weekdays"
					:key="w.day"
					class="day"
					:class="{on: draft.weekdays.includes(w.day)}"
					@click="toggleDay(w.day)"
				>{{ w.label }}</button>
			</div>
			<div v-if="draft.freq === 'MONTHLY'" class="opts">
				<label><input v-model="draft.monthMode" type="radio" value="day"> {{ t('repOnDay', {day: anchor.getDate()}) }}</label>
				<label><input v-model="draft.monthMode" type="radio" value="nth"> {{ nthLabel }}</label>
				<label><input v-model="draft.monthMode" type="radio" value="last"> {{ t('repLastDay') }}</label>
			</div>
		</template>

		<label class="check">
			<input v-model="draft.afterCompletion" type="checkbox">
			<span>{{ t('repAfterCompletion') }}</span>
		</label>
		<p v-if="draft.afterCompletion" class="hint">{{ t('repAfterCompletionHint') }}</p>

		<div class="line">
			<span>{{ t('repEnds') }}</span>
			<select v-model="endKind" class="sel">
				<option value="never">{{ t('repEndNever') }}</option>
				<option value="until">{{ t('repEndOn') }}</option>
				<option value="count">{{ t('repEndAfter') }}</option>
			</select>
		</div>
		<div v-if="endKind === 'until'" class="line indent">
			<input v-model="untilInput" type="date" class="sel">
		</div>
		<div v-if="endKind === 'count'" class="line indent">
			<input v-model.number="countInput" type="number" min="1" max="99999" class="num">
			<span>{{ t('repOccurrences') }}</span>
		</div>

		<p class="summary">{{ summary }}</p>

		<div class="actions">
			<button class="btn" @click="emit('cancel')">{{ t('cancel') }}</button>
			<button class="btn primary" @click="emit('save', result())">{{ t('ok') }}</button>
		</div>
	</div>
</template>

<style scoped>
.re {
	display: flex;
	flex-direction: column;
	gap: 8px;
}

.head {
	display: flex;
	align-items: center;
	gap: 6px;
	margin-bottom: 2px;
}

.title {
	font-weight: 600;
}

.icon-btn.sm {
	width: 24px;
	height: 24px;
}

.line {
	display: flex;
	align-items: center;
	gap: 8px;
	color: var(--text-2);
}

.line.indent {
	padding-left: 12px;
}

.num,
.sel {
	height: 28px;
	padding: 0 6px;
	border-radius: 5px;
	border: 1px solid var(--border-strong);
	background: var(--bg-input);
	color: var(--text);
	outline: none;
}

.num {
	width: 64px;
}

.sel {
	flex: 1;
	min-width: 0;
}

.days {
	display: grid;
	grid-template-columns: repeat(7, 1fr);
	gap: 4px;
}

.day {
	height: 30px;
	border-radius: 6px;
	background: var(--bg-input);
	color: var(--text-2);
	font-size: 12px;
	text-transform: uppercase;
}

.day.on {
	background: var(--accent);
	color: #fff;
}

.opts {
	display: flex;
	flex-direction: column;
	gap: 6px;
	color: var(--text-2);
}

.opts label,
.check {
	display: flex;
	align-items: center;
	gap: 8px;
	color: var(--text-2);
}

.hint {
	margin: -4px 0 0 24px;
	font-size: 12px;
	color: var(--text-3);
}

.summary {
	margin: 4px 0 0;
	padding: 8px;
	border-radius: 6px;
	background: var(--bg-input);
	color: var(--text-2);
	font-size: 12px;
}

.actions {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
}

.actions .btn {
	height: 30px;
	padding: 0 18px;
}
</style>
