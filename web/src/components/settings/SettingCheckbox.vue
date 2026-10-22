<script setup lang="ts">
import {Check} from 'lucide-vue-next'

const model = defineModel<boolean>({required: true})
</script>

<template>
	<button class="checkbox" :class="{on: model}" role="checkbox" :aria-checked="model" @click="model = !model">
		<Check v-if="model" :size="10" :stroke-width="3.2" />
	</button>
</template>

<style scoped>
.checkbox {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 14px;
	height: 14px;
	flex-shrink: 0;
	border-radius: 3px;
	border: 1.5px solid var(--text-4);
	color: #fff;
}

.checkbox.on {
	border-color: var(--accent);
	background: var(--accent);
}
</style>
