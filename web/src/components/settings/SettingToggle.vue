<script setup lang="ts">
const model = defineModel<boolean>({required: true})
</script>

<template>
	<button
		class="toggle"
		:class="{on: model}"
		role="switch"
		:aria-checked="model"
		@click="model = !model"
	>
		<span class="knob" />
	</button>
</template>

<style scoped>
.toggle {
	position: relative;
	width: 28px;
	height: 16px;
	border-radius: 8px;
	background: var(--border-strong);
	transition: background 0.15s;
	flex-shrink: 0;
}

:root[data-theme="dark"] .toggle:not(.on) {
	background: #4a4a4a;
}

.toggle.on {
	background: var(--accent);
}

.knob {
	position: absolute;
	top: 2px;
	left: 2px;
	width: 12px;
	height: 12px;
	border-radius: 50%;
	background: #fff;
	box-shadow: 0 1px 2px rgba(0, 0, 0, 0.25);
	transition: transform 0.15s;
}

.toggle.on .knob {
	transform: translateX(12px);
}
</style>
