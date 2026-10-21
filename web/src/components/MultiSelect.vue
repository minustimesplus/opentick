<script setup lang="ts">
import {computed, ref} from 'vue'
import {Check, ChevronDown} from 'lucide-vue-next'
import Popover from './Popover.vue'
import {t} from '@/lib/i18n'

const props = defineProps<{options: {value: number, label: string}[], modelValue: number[]}>()
const emit = defineEmits<{'update:modelValue': [number[]]}>()

const el = ref<HTMLElement | null>(null)
const open = ref(false)

const summary = computed(() => {
	if (!props.modelValue.length) return t('all')
	return props.options.filter(o => props.modelValue.includes(o.value)).map(o => o.label).join(', ')
})

function toggle(v: number) {
	const set = new Set(props.modelValue)
	if (set.has(v)) set.delete(v)
	else set.add(v)
	emit('update:modelValue', props.options.map(o => o.value).filter(x => set.has(x)))
}
</script>

<template>
	<button ref="el" type="button" class="input select" @click="open = true">
		<span class="summary">{{ summary }}</span>
		<ChevronDown :size="15" class="chev" />
	</button>
	<Popover v-if="open" :anchor="el" @close="open = false">
		<div class="menu options" :style="{width: el ? `${el.offsetWidth}px` : undefined}">
			<button type="button" class="menu-item" @click="emit('update:modelValue', [])">
				<Check :size="15" :style="{visibility: modelValue.length ? 'hidden' : 'visible'}" /> {{ t('all') }}
			</button>
			<div class="menu-sep" />
			<button v-for="o in options" :key="o.value" type="button" class="menu-item" @click="toggle(o.value)">
				<Check :size="15" :style="{visibility: modelValue.includes(o.value) ? 'visible' : 'hidden'}" /> {{ o.label }}
			</button>
		</div>
	</Popover>
</template>

<style scoped>
.select {
	display: flex;
	align-items: center;
	gap: 6px;
	text-align: left;
}

.summary {
	flex: 1;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.chev {
	color: var(--text-3);
}

.options {
	max-height: 300px;
	overflow-y: auto;
}
</style>
