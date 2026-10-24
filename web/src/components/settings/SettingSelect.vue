<script setup lang="ts" generic="T extends string | number | boolean">
import {computed, ref} from 'vue'
import {Check, ChevronsUpDown} from 'lucide-vue-next'
import Popover from '../Popover.vue'

const props = defineProps<{
	options: {value: T, label: string}[]
	disabled?: boolean
}>()

const model = defineModel<T>({required: true})

const btn = ref<HTMLElement | null>(null)
const open = ref(false)
const current = computed(() => props.options.find(o => o.value === model.value)?.label ?? '')

function pick(value: T) {
	model.value = value
	open.value = false
}
</script>

<template>
	<button ref="btn" class="value" :class="{open}" :disabled="disabled" @click="open = !open">
		<span>{{ current }}</span>
		<ChevronsUpDown :size="11" :stroke-width="2" />
	</button>
	<Popover v-if="open" :anchor="btn" placement="bottom-end" :offset="4" @close="open = false">
		<div class="menu">
			<button
				v-for="o in options"
				:key="String(o.value)"
				class="menu-item"
				:class="{active: o.value === model}"
				@click="pick(o.value)"
			>
				<span class="label">{{ o.label }}</span>
				<Check v-if="o.value === model" :size="15" class="check" />
			</button>
		</div>
	</Popover>
</template>

<style scoped>
.value {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	height: 26px;
	margin-right: -6px;
	padding: 0 6px;
	border-radius: 5px;
	color: var(--text-2);
	font-size: 13px;
}

.value:hover,
.value.open {
	color: var(--text);
}

.value svg {
	color: var(--text-3);
}

.label {
	flex: 1;
}

.check {
	color: var(--accent);
}

.menu-item.active {
	color: var(--accent);
	background: none;
}

.menu-item.active:hover {
	background: var(--bg-hover);
}
</style>
