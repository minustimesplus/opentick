<script setup lang="ts">
// One field to log today's weight; prefilled with today's or the last one.
import {onMounted, ref, watch} from 'vue'
import {useWeights} from '@/stores/weights'
import {t} from '@/lib/i18n'

const props = withDefaults(defineProps<{compact?: boolean, autofocus?: boolean}>(), {compact: false, autofocus: false})
const emit = defineEmits<{logged: []}>()
const weights = useWeights()
const value = ref((weights.today ?? weights.latest)?.weight.toFixed(1) ?? '')
const input = ref<HTMLInputElement | null>(null)
const saving = ref(false)

// Follow weights logged elsewhere (the habit row, the panel, another device).
watch(() => (weights.today ?? weights.latest)?.weight, kg => {
	if (kg !== undefined) value.value = kg.toFixed(1)
})

onMounted(() => {
	if (props.autofocus) input.value?.select()
})

async function submit() {
	const kg = Number(String(value.value).replace(',', '.'))
	if (!kg || saving.value) return
	saving.value = true
	try {
		if (await weights.log(kg)) emit('logged')
	} finally {
		saving.value = false
	}
}
</script>

<template>
	<form class="weight-input" :class="{compact}" @submit.prevent="submit" @click.stop>
		<div class="field">
			<input
				ref="input"
				v-model="value"
				class="input"
				inputmode="decimal"
				:aria-label="t('todaysWeight')"
				placeholder="70.0"
				@keydown.esc="emit('logged')"
			>
			<span class="unit">kg</span>
		</div>
		<button class="btn primary" type="submit" :disabled="saving">{{ weights.today ? t('updateWeight') : t('logWeight') }}</button>
	</form>
</template>

<style scoped>
.weight-input {
	display: flex;
	gap: 8px;
}

.field {
	position: relative;
	flex: 1;
}

.input {
	padding-right: 32px;
	font-variant-numeric: tabular-nums;
}

.unit {
	position: absolute;
	top: 50%;
	right: 10px;
	transform: translateY(-50%);
	font-size: 12px;
	color: var(--text-3);
	pointer-events: none;
}

.compact .field {
	flex: none;
	width: 110px;
}
</style>
