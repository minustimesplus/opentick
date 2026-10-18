<script setup lang="ts">
import {desktop, windowMaximized} from '@/lib/desktop'
import {t} from '@/lib/i18n'
</script>

<template>
	<!-- Flat glyphs on the frame color, like TickTick's own title bar. -->
	<div class="window-controls">
		<button class="wc-btn" :title="t('windowMinimize')" @click="desktop?.minimizeWindow?.()">
			<svg viewBox="0 0 10 10"><path d="M0 5.5h10" /></svg>
		</button>
		<button
			class="wc-btn"
			:title="windowMaximized ? t('windowRestore') : t('windowMaximize')"
			@click="desktop?.toggleMaximizeWindow?.()"
		>
			<svg v-if="windowMaximized" viewBox="0 0 10 10">
				<path d="M.5 2.5h7v7h-7z" />
				<path d="M2.5 2.5v-2h7v7h-2" />
			</svg>
			<svg v-else viewBox="0 0 10 10"><path d="M.5.5h9v9h-9z" /></svg>
		</button>
		<button class="wc-btn close" :title="t('windowClose')" @click="desktop?.closeWindow?.()">
			<svg viewBox="0 0 10 10"><path d="m.5.5 9 9m0-9-9 9" /></svg>
		</button>
	</div>
</template>

<style scoped>
.window-controls {
	position: fixed;
	top: 0;
	right: 0;
	z-index: 1000;
	display: flex;
	height: 24px;
	-webkit-app-region: no-drag;
}

.wc-btn {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 42px;
	height: 24px;
	color: var(--frame-glyph);
}

.wc-btn svg {
	width: 10px;
	height: 10px;
	fill: none;
	stroke: currentColor;
	stroke-width: 1;
}

.wc-btn:hover {
	background: var(--frame-hover);
	color: var(--rail-active);
}

.wc-btn.close:hover {
	background: #e81123;
	color: #fff;
}
</style>
