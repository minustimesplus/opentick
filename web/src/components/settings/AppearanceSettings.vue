<script setup lang="ts">
import {ref} from 'vue'
import {Check, ImageOff, Plus, X} from 'lucide-vue-next'
import AppearanceDisplay from './AppearanceDisplay.vue'
import SettingSelect from './SettingSelect.vue'
import SettingToggle from './SettingToggle.vue'
import {prefs} from '@/lib/prefs'
import {t, type MessageKey} from '@/lib/i18n'
import {colorSeries, type ThemeColor} from '@/lib/theme'
import {background, removeBackground, uploadBackground} from '@/lib/background'
import {toast} from '@/lib/toast'

const labels: Record<ThemeColor, MessageKey> = {
	default: 'themeDefault',
	sky: 'themeSky',
	turquoise: 'themeTurquoise',
	teal: 'themeTeal',
	matcha: 'themeMatcha',
	sunshine: 'themeSunshine',
	peach: 'themePeach',
	lilac: 'themeLilac',
	ebony: 'themeEbony',
	navy: 'themeNavy',
	gray: 'themeGray',
	dark: 'themeDark',
}

const tab = ref<'theme' | 'display'>('theme')

// Not wired up yet: there is only the flat style.
const interfaceStyle = ref('flat')

const fileInput = ref<HTMLInputElement | null>(null)

async function onFile(e: Event) {
	const input = e.target as HTMLInputElement
	const file = input.files?.[0]
	input.value = ''
	if (!file) return
	try {
		await uploadBackground(file)
	} catch (err) {
		toast(String((err as Error).message))
	}
}

async function remove() {
	try {
		await removeBackground()
	} catch (err) {
		toast(String((err as Error).message))
	}
}
</script>

<template>
	<div class="tabs">
		<button :class="{active: tab === 'theme'}" @click="tab = 'theme'">{{ t('themeTab') }}</button>
		<button :class="{active: tab === 'display'}" @click="tab = 'display'">{{ t('displayTab') }}</button>
	</div>

	<AppearanceDisplay v-if="tab === 'display'" />
	<template v-else>
		<div class="card">
			<div class="row">
				<span>{{ t('interfaceStyle') }}</span>
				<SettingSelect v-model="interfaceStyle" :options="[{value: 'flat', label: t('interfaceFlat')}]" />
			</div>
			<div class="row">
				<span>{{ t('followSystemDark') }}</span>
				<SettingToggle v-model="prefs.followSystemDark" />
			</div>
		</div>

		<h3 class="group-title">{{ t('colorSeries') }}</h3>
		<div class="swatches">
			<button
				v-for="c in colorSeries"
				:key="c.key"
				class="swatch"
				:class="{active: prefs.themeColor === c.key}"
				:aria-pressed="prefs.themeColor === c.key"
				@click="prefs.themeColor = c.key"
			>
				<span class="chip" :class="c.key" :style="{background: c.swatch}">
					<span v-if="c.key === 'dark'" class="dots"><i /><i /><i /></span>
					<span v-if="prefs.themeColor === c.key" class="badge"><Check :size="10" :stroke-width="3" /></span>
				</span>
				<span class="name">{{ t(labels[c.key]) }}</span>
			</button>
		</div>

		<h3 class="group-title">{{ t('backgroundImage') }}</h3>
		<p class="hint">{{ background.uploadEnabled ? t('backgroundHint') : t('backgroundUnavailable') }}</p>
		<template v-if="background.uploadEnabled">
			<div class="images">
				<button class="image" :class="{active: !background.url}" :disabled="background.busy" @click="background.url && remove()">
					<span class="thumb none"><ImageOff :size="20" :stroke-width="1.6" /></span>
					<span class="name">{{ t('backgroundNone') }}</span>
				</button>
				<div v-if="background.url" class="image active">
					<span class="thumb" :style="{backgroundImage: `url(${background.url})`}">
						<span class="badge"><Check :size="10" :stroke-width="3" /></span>
						<button class="remove" :title="t('delete')" :aria-label="t('delete')" :disabled="background.busy" @click="remove">
							<X :size="12" :stroke-width="2.2" />
						</button>
					</span>
					<span class="name">{{ t('backgroundCustom') }}</span>
				</div>
				<button class="image" :disabled="background.busy" @click="fileInput?.click()">
					<span class="thumb upload">
						<span v-if="background.busy" class="spinner" />
						<Plus v-else :size="20" :stroke-width="1.6" />
					</span>
					<span class="name">{{ t('backgroundUpload') }}</span>
				</button>
				<input ref="fileInput" type="file" accept="image/*" hidden @change="onFile">
			</div>

			<div class="card">
				<div class="row">
					<span>{{ t('backgroundBlur') }}</span>
					<SettingToggle v-model="prefs.backgroundBlur" />
				</div>
			</div>
		</template>
	</template>
</template>

<style scoped>
.tabs {
	display: flex;
	width: fit-content;
	margin: -30px auto 16px;
	padding: 2px;
	border-radius: 8px;
	background: var(--settings-card);
}

.tabs button {
	height: 26px;
	padding: 0 16px;
	border-radius: 6px;
	font-size: 12px;
	font-weight: 500;
	color: var(--text-3);
}

.tabs button.active {
	background: var(--bg);
	color: var(--text);
	box-shadow: 0 1px 3px rgba(0, 0, 0, 0.12);
}

.card {
	margin-bottom: 14px;
	padding: 2px 15px;
	border-radius: 8px;
	background: var(--settings-card);
}

.row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 16px;
	min-height: 36px;
	font-size: 13px;
}

.group-title {
	margin: 22px 0 12px;
	font-size: 14px;
	font-weight: 600;
}

.hint {
	margin: -6px 0 12px;
	font-size: 12px;
	color: var(--text-3);
}

.swatches {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(54px, 1fr));
	gap: 14px 14px;
}

.swatch,
.image {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 7px;
	min-width: 0;
}

.chip {
	position: relative;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 100%;
	aspect-ratio: 1;
	border-radius: 8px;
	box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.08);
}

.chip.default {
	box-shadow: inset 0 0 0 1px var(--border-strong);
}

.swatch:hover .chip,
.image:hover:not(:disabled) .thumb {
	filter: brightness(1.05);
}

.dots {
	display: flex;
	gap: 4px;
}

.dots i {
	width: 6px;
	height: 6px;
	border-radius: 50%;
	background: #4772fa;
}

.badge {
	position: absolute;
	right: -5px;
	bottom: -5px;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 17px;
	height: 17px;
	border-radius: 50%;
	border: 2px solid var(--bg);
	background: var(--accent);
	color: #fff;
}

.name {
	max-width: 100%;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 12px;
	color: var(--text-3);
}

.active .name {
	color: var(--text);
}

.images {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
	gap: 14px;
	margin-bottom: 16px;
}

.thumb {
	position: relative;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 100%;
	aspect-ratio: 16 / 9;
	border-radius: 8px;
	background: var(--settings-card) center / cover;
	color: var(--text-3);
}

.thumb.upload {
	border: 1.5px dashed var(--border-strong);
	background: none;
}

.image.active .thumb.none {
	box-shadow: inset 0 0 0 1.5px var(--accent);
}

.image:disabled {
	cursor: default;
}

.remove {
	position: absolute;
	top: 5px;
	right: 5px;
	display: none;
	align-items: center;
	justify-content: center;
	width: 20px;
	height: 20px;
	border-radius: 50%;
	background: rgba(0, 0, 0, 0.55);
	color: #fff;
}

.image:hover .remove {
	display: flex;
}

.spinner {
	width: 18px;
	height: 18px;
	border: 2px solid var(--border-strong);
	border-top-color: var(--accent);
	border-radius: 50%;
	animation: spin 0.8s linear infinite;
}

@keyframes spin {
	to {
		transform: rotate(360deg);
	}
}
</style>
