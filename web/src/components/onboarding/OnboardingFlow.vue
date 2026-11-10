<script setup lang="ts">
// Setup wizard after signing up: bring tasks over from another app, pick the
// views and the look, start with a few lists. Every step comes preset, so
// clicking Continue through it (or "Skip setup") takes seconds; changes to
// views and look apply live behind it.
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import {useRouter} from 'vue-router'
import {ArrowLeft, Check, ExternalLink, FileSpreadsheet, LoaderCircle, Plus, Sparkles, Upload, X} from 'lucide-vue-next'
import FeaturePreview from '../FeaturePreview.vue'
import SettingToggle from '../settings/SettingToggle.vue'
import {useData} from '@/stores/data'
import {t, type MessageKey} from '@/lib/i18n'
import {prefs} from '@/lib/prefs'
import {OPTIONAL_FEATURES} from '@/lib/features'
import {colorSeries, colorSeriesLabels} from '@/lib/theme'
import {finishOnboarding} from '@/lib/onboarding'
import {NoTitleColumnError, startCsvImport, waitForImport} from '@/lib/backup'
import {toast} from '@/lib/toast'

const emit = defineEmits<{close: []}>()
const data = useData()
const router = useRouter()

type StepKey = 'welcome' | 'source' | 'views' | 'look' | 'lists' | 'done'
type Source = 'ticktick' | 'csv' | 'fresh'

// ---------- where tasks come from ----------

const source = ref<Source>('fresh')
const importState = ref<'idle' | 'running' | 'done' | 'failed'>('idle')
const importError = ref('')
const importName = ref('')
const dragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

const SOURCES: {key: Source, title: MessageKey, desc: MessageKey}[] = [
	{key: 'ticktick', title: 'onbSourceTickTick', desc: 'onbSourceTickTickDesc'},
	{key: 'csv', title: 'onbSourceCsv', desc: 'onbSourceCsvDesc'},
	{key: 'fresh', title: 'onbSourceFresh', desc: 'onbSourceFreshDesc'},
]

const TICKTICK_STEPS: MessageKey[] = ['onbTickTickStep1', 'onbTickTickStep2', 'onbTickTickStep3', 'onbTickTickStep4']

function onDrop(e: DragEvent) {
	dragging.value = false
	const file = e.dataTransfer?.files?.[0]
	if (file) runImport(file)
}

function onPicked() {
	const file = fileInput.value?.files?.[0]
	if (fileInput.value) fileInput.value.value = ''
	if (file) runImport(file)
}

/**
 * Starts the import and lets the wizard carry on: it runs on the server in
 * the background, and finishes with a sync and a toast even after the
 * wizard has closed.
 */
async function runImport(file: File) {
	if (importState.value === 'running') return
	importName.value = file.name
	importError.value = ''
	importState.value = 'running'
	try {
		const migrator = await startCsvImport(file)
		const status = await waitForImport(migrator)
		if (status?.error_kind) throw new Error(status.error_message || t('backupImportFailed'))
		if (!status) {
			toast(t('backupImportPending'))
			return
		}
		await data.loadAll()
		importState.value = 'done'
		toast(t('backupImportDone'))
	} catch (e) {
		importState.value = 'failed'
		importError.value = e instanceof NoTitleColumnError ? t('backupNoTitle') : (e as Error).message
	}
}

const importing = computed(() => importState.value === 'running' || importState.value === 'done')

// ---------- starter lists ----------

interface Starter {
	title: string
	emoji: string
	color: string
}

const STARTERS: {key: MessageKey, emoji: string, color: string}[] = [
	{key: 'onbListPersonal', emoji: '🏠', color: '4772fa'},
	{key: 'onbListWork', emoji: '💼', color: 'f5a623'},
	{key: 'onbListShopping', emoji: '🛒', color: '2bb673'},
	{key: 'onbListLearning', emoji: '📚', color: '9f92dd'},
	{key: 'onbListHealth', emoji: '💪', color: 'e5484d'},
	{key: 'onbListTravel', emoji: '✈️', color: '4fb3c2'},
]

const starters = ref<Starter[]>(STARTERS.map(s => ({title: t(s.key), emoji: s.emoji, color: s.color})))
const pickedLists = ref(new Set<string>([t('onbListPersonal'), t('onbListWork')]))
const customList = ref('')
const createdLists = ref(0)

function toggleList(title: string) {
	const next = new Set(pickedLists.value)
	if (next.has(title)) next.delete(title)
	else next.add(title)
	pickedLists.value = next
}

function addCustomList() {
	const title = customList.value.trim()
	customList.value = ''
	if (!title) return
	if (!starters.value.some(s => s.title.toLowerCase() === title.toLowerCase())) {
		starters.value = [...starters.value, {title, emoji: '📝', color: '8c8c8c'}]
	}
	pickedLists.value = new Set([...pickedLists.value, title])
}

async function createLists() {
	const existing = new Set(data.projects.map(p => p.title.toLowerCase()))
	for (const s of starters.value) {
		const title = `${s.emoji} ${s.title}`
		if (!pickedLists.value.has(s.title) || existing.has(title.toLowerCase()) || existing.has(s.title.toLowerCase())) continue
		if (await data.createProject({title, hex_color: s.color})) createdLists.value++
	}
}

// ---------- steps ----------

const steps = computed<StepKey[]>(() => {
	const out: StepKey[] = ['welcome', 'source', 'views', 'look']
	// An import brings its own lists.
	if (!importing.value) out.push('lists')
	out.push('done')
	return out
})

const step = ref<StepKey>('welcome')
const index = computed(() => steps.value.indexOf(step.value))
const progress = computed(() => index.value / (steps.value.length - 1))
// Slides forward or back depending on the direction of travel.
const direction = ref<'next' | 'back'>('next')
const busy = ref(false)

const enabledViews = computed(() => OPTIONAL_FEATURES.filter(f => prefs.features[f.key]).length)

async function next() {
	if (busy.value) return
	if (step.value === 'lists' && pickedLists.value.size) {
		busy.value = true
		try {
			await createLists()
		} finally {
			busy.value = false
		}
	}
	if (step.value === 'done') {
		finish()
		return
	}
	direction.value = 'next'
	step.value = steps.value[index.value + 1]
}

function back() {
	if (index.value <= 0) return
	direction.value = 'back'
	step.value = steps.value[index.value - 1]
}

function finish() {
	finishOnboarding()
	emit('close')
	router.push('/tasks/today')
}

const continueLabel = computed(() => {
	if (step.value === 'welcome') return t('onbStart')
	if (step.value === 'done') return t('onbOpenToday')
	if (step.value === 'lists' && pickedLists.value.size) return t('onbCreateLists', {n: pickedLists.value.size})
	return t('onbContinue')
})

function onKey(e: KeyboardEvent) {
	if (e.key !== 'Enter' || e.isComposing) return
	const target = e.target as HTMLElement
	// Enter in the custom list field adds the list; on a button it presses it.
	if (target.closest('input, textarea, button, [role="button"], a')) return
	e.preventDefault()
	next()
}

onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))

const firstName = computed(() => (data.user?.name || '').split(' ')[0])
</script>

<template>
	<Teleport to="body">
		<div class="onb-overlay">
			<div class="onb" role="dialog" aria-modal="true" :aria-label="t('onbAria')">
				<header class="onb-top">
					<div class="bar"><div class="fill" :style="{transform: `scaleX(${progress})`}" /></div>
					<button v-if="step !== 'done'" class="skip" type="button" @click="finish">{{ t('onbSkip') }}</button>
				</header>

				<div class="stage">
					<Transition :name="direction" mode="out-in">
						<!-- welcome -->
						<section v-if="step === 'welcome'" key="welcome" class="pane center">
							<svg class="logo" viewBox="0 0 32 32" width="72" height="72" aria-hidden="true">
								<rect width="32" height="32" rx="9" fill="#4772FA" />
								<path d="M9 16.5l4.5 4.5L23 11.5" fill="none" stroke="#fff" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
							</svg>
							<h1>{{ firstName ? t('onbWelcomeName', {name: firstName}) : t('onbWelcome') }}</h1>
							<p class="lead">{{ t('onbWelcomeSub') }}</p>
						</section>

						<!-- source -->
						<section v-else-if="step === 'source'" key="source" class="pane">
							<h2>{{ t('onbSourceTitle') }}</h2>
							<p class="lead">{{ t('onbSourceSub') }}</p>
							<div class="sources">
								<button
									v-for="(s, i) in SOURCES"
									:key="s.key"
									type="button"
									class="tile"
									:class="{on: source === s.key}"
									:style="{'--i': i}"
									:aria-pressed="source === s.key"
									@click="source = s.key"
								>
									<span class="tile-icon" :class="s.key">
										<svg v-if="s.key === 'ticktick'" viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2.2" /><path d="M8 12.3l2.8 2.8L16.5 9" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" /></svg>
										<FileSpreadsheet v-else-if="s.key === 'csv'" :size="20" />
										<Sparkles v-else :size="20" />
									</span>
									<strong>{{ t(s.title) }}</strong>
									<span>{{ t(s.desc) }}</span>
									<span v-if="source === s.key" class="tick"><Check :size="11" :stroke-width="3" /></span>
								</button>
							</div>

							<Transition name="expand">
								<div v-if="source !== 'fresh'" class="import">
									<ol v-if="source === 'ticktick'" class="howto">
										<li v-for="(k, i) in TICKTICK_STEPS" :key="k" :style="{'--i': i}">
											<span class="num">{{ i + 1 }}</span>
											<span>
												{{ t(k) }}
												<a v-if="i === 0" href="https://ticktick.com/webapp" target="_blank" rel="noopener">ticktick.com <ExternalLink :size="11" /></a>
											</span>
										</li>
									</ol>
									<p v-else class="hint">{{ t('onbCsvHint') }}</p>

									<div
										class="drop"
										:class="{dragging, [importState]: true}"
										role="button"
										tabindex="0"
										@click="importState !== 'running' && fileInput?.click()"
										@keydown.enter.prevent="importState !== 'running' && fileInput?.click()"
										@dragover.prevent="dragging = true"
										@dragleave="dragging = false"
										@drop.prevent="onDrop"
									>
										<template v-if="importState === 'idle'">
											<Upload :size="20" />
											<span>{{ t('onbDrop') }}</span>
										</template>
										<template v-else-if="importState === 'running'">
											<LoaderCircle :size="20" class="spin" />
											<span>{{ t('onbImporting', {file: importName}) }}</span>
										</template>
										<template v-else-if="importState === 'done'">
											<Check :size="20" />
											<span>{{ t('onbImported', {file: importName}) }}</span>
										</template>
										<template v-else>
											<X :size="20" />
											<span>{{ importError || t('backupImportFailed') }} · {{ t('onbTryAgain') }}</span>
										</template>
									</div>
									<input ref="fileInput" type="file" accept=".csv,text/csv" hidden @change="onPicked">
									<p class="fine">{{ t('onbImportLater') }}</p>
								</div>
							</Transition>
						</section>

						<!-- views -->
						<section v-else-if="step === 'views'" key="views" class="pane">
							<h2>{{ t('onbViewsTitle') }}</h2>
							<p class="lead">{{ t('onbViewsSub') }}</p>
							<div class="views">
								<div
									v-for="(f, i) in OPTIONAL_FEATURES"
									:key="f.key"
									class="view"
									:class="{off: !prefs.features[f.key]}"
									:style="{'--i': i}"
									role="switch"
									tabindex="0"
									:aria-checked="prefs.features[f.key]"
									@click="prefs.features[f.key] = !prefs.features[f.key]"
									@keydown.enter.prevent="prefs.features[f.key] = !prefs.features[f.key]"
									@keydown.space.prevent="prefs.features[f.key] = !prefs.features[f.key]"
								>
									<FeaturePreview :feature="f.key" :animated="prefs.features[f.key]" />
									<div class="view-foot">
										<span class="view-name">{{ t(f.title) }}</span>
										<span class="check" :class="{on: prefs.features[f.key]}"><Check :size="11" :stroke-width="3" /></span>
									</div>
									<span class="view-desc">{{ t(f.desc) }}</span>
								</div>
							</div>
						</section>

						<!-- look -->
						<section v-else-if="step === 'look'" key="look" class="pane">
							<h2>{{ t('onbLookTitle') }}</h2>
							<p class="lead">{{ t('onbLookSub') }}</p>
							<div class="look">
								<div class="mini" aria-hidden="true">
									<div class="mini-rail"><i /><i /><i /></div>
									<div class="mini-side">
										<b /><span /><span class="sel" /><span />
									</div>
									<div class="mini-main">
										<b />
										<div v-for="n in 4" :key="n" class="mini-row"><i /><span :style="{width: `${40 + n * 9}%`}" /></div>
									</div>
								</div>
								<div class="look-pick">
									<div class="swatches">
										<button
											v-for="(c, i) in colorSeries"
											:key="c.key"
											type="button"
											class="swatch"
											:class="{on: prefs.themeColor === c.key}"
											:style="{'--i': i, background: c.swatch}"
											:title="t(colorSeriesLabels[c.key])"
											:aria-label="t(colorSeriesLabels[c.key])"
											:aria-pressed="prefs.themeColor === c.key"
											@click="prefs.themeColor = c.key"
										>
											<Check v-if="prefs.themeColor === c.key" :size="13" :stroke-width="3" />
										</button>
									</div>
									<div class="row">
										<span>{{ t('followSystemDark') }}</span>
										<SettingToggle v-model="prefs.followSystemDark" />
									</div>
								</div>
							</div>
						</section>

						<!-- lists -->
						<section v-else-if="step === 'lists'" key="lists" class="pane">
							<h2>{{ t('onbListsTitle') }}</h2>
							<p class="lead">{{ t('onbListsSub') }}</p>
							<div class="chips">
								<button
									v-for="(s, i) in starters"
									:key="s.title"
									type="button"
									class="chip"
									:class="{on: pickedLists.has(s.title)}"
									:style="{'--i': i, '--c': `#${s.color}`}"
									:aria-pressed="pickedLists.has(s.title)"
									@click="toggleList(s.title)"
								>
									<span class="emoji">{{ s.emoji }}</span>
									{{ s.title }}
									<Check v-if="pickedLists.has(s.title)" :size="13" :stroke-width="3" />
								</button>
							</div>
							<form class="custom" @submit.prevent="addCustomList">
								<input v-model="customList" :placeholder="t('onbListCustom')" maxlength="60">
								<button type="submit" :disabled="!customList.trim()"><Plus :size="15" /></button>
							</form>
						</section>

						<!-- done -->
						<section v-else key="done" class="pane center">
							<div class="burst" aria-hidden="true">
								<i v-for="n in 14" :key="n" :style="{'--n': n}" />
								<span class="done-check"><Check :size="34" :stroke-width="3" /></span>
							</div>
							<h1>{{ t('onbDoneTitle') }}</h1>
							<ul class="summary">
								<li v-if="importState === 'running'"><LoaderCircle :size="15" class="spin" />{{ t('onbSummaryImporting') }}</li>
								<li v-else-if="importState === 'done'"><Check :size="15" />{{ t('onbSummaryImported') }}</li>
								<li><Check :size="15" />{{ enabledViews === 1 ? t('onbSummaryView') : t('onbSummaryViews', {n: enabledViews}) }}</li>
								<li v-if="createdLists"><Check :size="15" />{{ createdLists === 1 ? t('onbSummaryList') : t('onbSummaryLists', {n: createdLists}) }}</li>
							</ul>
							<p class="tip">{{ t('onbTip') }}</p>
						</section>
					</Transition>
				</div>

				<footer class="onb-foot">
					<span class="foot-start">
						<button v-if="index > 0 && step !== 'done'" class="back" type="button" @click="back">
							<ArrowLeft :size="15" /> {{ t('onbBack') }}
						</button>
					</span>
					<span class="dots" aria-hidden="true">
						<i v-for="(s, i) in steps" :key="s" :class="{on: i === index, past: i < index}" />
					</span>
					<span class="foot-end">
						<button class="go" type="button" :disabled="busy" @click="next">
							<LoaderCircle v-if="busy" :size="16" class="spin" />
							<template v-else>{{ continueLabel }}</template>
						</button>
					</span>
				</footer>
			</div>
		</div>
	</Teleport>
</template>

<style scoped>
.onb-overlay {
	position: fixed;
	inset: 0;
	z-index: 950;
	display: flex;
	align-items: center;
	justify-content: center;
	padding: 16px;
	background: rgba(0, 0, 0, 0.5);
	backdrop-filter: blur(6px);
	animation: fade 0.3s ease both;
}

.onb {
	display: flex;
	flex-direction: column;
	width: min(680px, 100%);
	height: min(640px, calc(100vh - 32px));
	border-radius: 16px;
	background: var(--bg-elev);
	box-shadow: var(--shadow);
	color: var(--text);
	overflow: hidden;
	transition: background-color 0.3s ease, color 0.3s ease;
	animation: pop 0.45s cubic-bezier(0.2, 0.9, 0.25, 1.15) both;
}

.onb-top {
	display: flex;
	align-items: center;
	gap: 16px;
	padding: 18px 22px 0;
}

.bar {
	flex: 1;
	height: 4px;
	border-radius: 2px;
	background: var(--bg-input);
	overflow: hidden;
}

.fill {
	height: 100%;
	background: var(--accent);
	transform-origin: left;
	transition: transform 0.5s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.skip {
	color: var(--text-3);
	font-size: 13px;
}

.skip:hover {
	color: var(--text);
}

.stage {
	position: relative;
	flex: 1;
	min-height: 0;
	overflow-x: hidden;
	overflow-y: auto;
}

.pane {
	display: flex;
	flex-direction: column;
	padding: 26px 36px 12px;
}

.pane.center {
	align-items: center;
	justify-content: center;
	min-height: 100%;
	text-align: center;
}

h1 {
	margin: 18px 0 8px;
	font-size: 28px;
	letter-spacing: -0.4px;
}

h2 {
	margin: 0;
	font-size: 22px;
	letter-spacing: -0.2px;
}

.lead {
	margin: 6px 0 20px;
	max-width: 480px;
	color: var(--text-2);
	font-size: 14px;
	line-height: 1.55;
}

/* ---------- welcome ---------- */

.logo {
	border-radius: 20px;
	box-shadow: 0 12px 32px rgba(71, 114, 250, 0.35);
	animation: pop 0.6s cubic-bezier(0.2, 0.9, 0.25, 1.3) 0.1s both;
}

.logo path {
	stroke-dasharray: 24;
	stroke-dashoffset: 24;
	animation: draw 0.5s ease 0.5s forwards;
}

/* ---------- source ---------- */

.sources {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: 10px;
}

.tile {
	position: relative;
	display: flex;
	flex-direction: column;
	align-items: flex-start;
	gap: 4px;
	padding: 14px;
	border: 1.5px solid var(--border-strong);
	border-radius: 12px;
	text-align: left;
	color: var(--text-3);
	font-size: 12px;
	line-height: 1.4;
	transition: border-color 0.15s, background-color 0.15s, transform 0.15s;
	animation: rise 0.4s cubic-bezier(0.2, 0.8, 0.2, 1) both;
	animation-delay: calc(var(--i) * 60ms);
}

.tile:hover {
	transform: translateY(-2px);
}

.tile.on {
	border-color: var(--accent);
	background: var(--accent-soft);
}

.tile strong {
	margin-top: 6px;
	color: var(--text);
	font-size: 14px;
}

.tile-icon {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 36px;
	height: 36px;
	border-radius: 10px;
	color: #fff;
	background: var(--accent);
}

.tile-icon.csv {
	background: #2bb673;
}

.tile-icon.fresh {
	background: #f5a623;
}

.tick {
	position: absolute;
	top: 10px;
	right: 10px;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 18px;
	height: 18px;
	border-radius: 50%;
	background: var(--accent);
	color: #fff;
	animation: pop 0.25s ease both;
}

.import {
	display: flex;
	flex-direction: column;
	gap: 12px;
	margin-top: 18px;
}

.howto {
	display: flex;
	flex-direction: column;
	gap: 8px;
	margin: 0;
	padding: 0;
	list-style: none;
	font-size: 13.5px;
}

.howto li {
	display: flex;
	align-items: center;
	gap: 10px;
	animation: rise 0.35s ease both;
	animation-delay: calc(var(--i) * 80ms + 80ms);
}

.howto a {
	display: inline-flex;
	align-items: center;
	gap: 3px;
	margin-left: 4px;
	color: var(--accent);
	text-decoration: none;
}

.num {
	display: flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	width: 22px;
	height: 22px;
	border-radius: 50%;
	background: var(--accent-soft);
	color: var(--accent);
	font-size: 12px;
	font-weight: 600;
}

.hint {
	margin: 0;
	color: var(--text-2);
	font-size: 13.5px;
	line-height: 1.5;
}

.drop {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 10px;
	min-height: 64px;
	padding: 12px 16px;
	border: 1.5px dashed var(--border-strong);
	border-radius: 12px;
	color: var(--text-2);
	font-size: 13.5px;
	text-align: center;
	cursor: pointer;
	transition: border-color 0.15s, background-color 0.15s, transform 0.15s;
}

.drop:hover,
.drop.dragging {
	border-color: var(--accent);
	background: var(--accent-soft);
	color: var(--accent);
}

.drop.dragging {
	transform: scale(1.02);
}

.drop.running {
	cursor: default;
	border-style: solid;
	color: var(--accent);
}

.drop.done {
	border-style: solid;
	border-color: #2bb673;
	background: color-mix(in srgb, #2bb673 12%, transparent);
	color: #2bb673;
}

.drop.failed {
	border-color: var(--danger);
	color: var(--danger);
}

.fine {
	margin: 0;
	color: var(--text-3);
	font-size: 12px;
}

/* ---------- views ---------- */

.views {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: 14px;
}

.view {
	display: flex;
	flex-direction: column;
	gap: 6px;
	padding: 8px;
	border: 1.5px solid var(--accent);
	border-radius: 12px;
	cursor: pointer;
	user-select: none;
	transition: border-color 0.2s, opacity 0.2s, transform 0.15s, filter 0.2s;
	animation: rise 0.4s cubic-bezier(0.2, 0.8, 0.2, 1) both;
	animation-delay: calc(var(--i) * 50ms);
}

.view:hover {
	transform: translateY(-2px);
}

.view.off {
	border-color: var(--border-strong);
	opacity: 0.6;
	filter: grayscale(0.8);
}

.view-foot {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 2px 2px 0;
}

.view-name {
	font-size: 13.5px;
	font-weight: 600;
}

.view-desc {
	padding: 0 2px;
	color: var(--text-3);
	font-size: 11.5px;
	line-height: 1.35;
}

.check {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 18px;
	height: 18px;
	border: 1.5px solid var(--border-strong);
	border-radius: 50%;
	color: transparent;
	transition: background-color 0.15s, border-color 0.15s, transform 0.2s;
}

.check.on {
	border-color: var(--accent);
	background: var(--accent);
	color: #fff;
	transform: scale(1.08);
}

/* ---------- look ---------- */

.look {
	display: grid;
	grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
	gap: 24px;
	align-items: start;
}

.mini {
	display: flex;
	height: 200px;
	padding: 8px 8px 8px 0;
	border-radius: 12px;
	background: var(--frame);
	box-shadow: 0 0 0 1px var(--border-strong);
	transition: background-color 0.35s;
	animation: rise 0.45s ease both;
}

.mini * {
	transition: background-color 0.35s, border-color 0.35s;
}

.mini-rail {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 10px;
	width: 30px;
	padding-top: 10px;
}

.mini-rail i {
	width: 12px;
	height: 12px;
	border-radius: 4px;
	background: var(--rail-icon);
	opacity: 0.6;
}

.mini-rail i:first-child {
	background: var(--accent);
	opacity: 1;
}

.mini-side {
	display: flex;
	flex-direction: column;
	gap: 8px;
	width: 34%;
	padding: 12px 10px;
	border-radius: 8px 0 0 8px;
	background: var(--bg-panel);
	border: 1px solid var(--border);
	border-right: 0;
}

.mini-side b,
.mini-main b {
	width: 50%;
	height: 7px;
	border-radius: 3px;
	background: var(--text-3);
}

.mini-side span {
	height: 14px;
	border-radius: 4px;
	background: var(--bg-hover);
}

.mini-side span.sel {
	background: var(--bg-active);
}

.mini-main {
	display: flex;
	flex: 1;
	flex-direction: column;
	gap: 12px;
	padding: 12px;
	border-radius: 0 8px 8px 0;
	background: var(--bg);
	border: 1px solid var(--border);
}

.mini-row {
	display: flex;
	align-items: center;
	gap: 8px;
}

.mini-row i {
	width: 10px;
	height: 10px;
	border: 1.5px solid var(--text-4);
	border-radius: 3px;
}

.mini-row span {
	height: 6px;
	border-radius: 3px;
	background: var(--text-4);
}

.swatches {
	display: grid;
	grid-template-columns: repeat(6, 1fr);
	gap: 10px;
}

.swatch {
	display: flex;
	align-items: center;
	justify-content: center;
	aspect-ratio: 1;
	border-radius: 50%;
	box-shadow: inset 0 0 0 1px rgba(128, 128, 128, 0.35);
	color: #fff;
	transition: transform 0.15s, box-shadow 0.15s;
	animation: pop 0.3s ease both;
	animation-delay: calc(var(--i) * 25ms);
}

.swatch:first-child {
	color: #4772fa;
}

.swatch:hover {
	transform: scale(1.1);
}

.swatch.on {
	box-shadow: 0 0 0 2px var(--bg-elev), 0 0 0 4px var(--accent);
}

.row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	margin-top: 18px;
	font-size: 13px;
	color: var(--text-2);
}

/* ---------- lists ---------- */

.chips {
	display: flex;
	flex-wrap: wrap;
	gap: 10px;
}

.chip {
	display: inline-flex;
	align-items: center;
	gap: 8px;
	height: 40px;
	padding: 0 16px 0 12px;
	border: 1.5px solid var(--border-strong);
	border-radius: 20px;
	color: var(--text);
	font-size: 14px;
	transition: border-color 0.15s, background-color 0.15s, transform 0.15s;
	animation: pop 0.3s ease both;
	animation-delay: calc(var(--i) * 40ms);
}

.chip:hover {
	transform: translateY(-1px);
}

.chip.on {
	border-color: var(--c);
	background: color-mix(in srgb, var(--c) 16%, transparent);
}

.chip svg {
	color: var(--c);
}

.emoji {
	font-size: 16px;
}

.custom {
	display: flex;
	gap: 8px;
	max-width: 320px;
	margin-top: 16px;
}

.custom input {
	flex: 1;
	height: 36px;
	padding: 0 12px;
	border: 1px solid var(--border-strong);
	border-radius: 8px;
	background: var(--bg-input);
	color: var(--text);
	font-size: 13.5px;
	outline: none;
}

.custom input:focus {
	border-color: var(--accent);
}

.custom button {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 36px;
	border-radius: 8px;
	background: var(--accent);
	color: #fff;
}

.custom button:disabled {
	opacity: 0.4;
}

/* ---------- done ---------- */

.burst {
	position: relative;
	width: 84px;
	height: 84px;
}

.done-check {
	position: absolute;
	inset: 0;
	display: flex;
	align-items: center;
	justify-content: center;
	border-radius: 50%;
	background: #2bb673;
	color: #fff;
	box-shadow: 0 10px 30px rgba(43, 182, 115, 0.4);
	animation: pop 0.5s cubic-bezier(0.2, 0.9, 0.25, 1.4) both;
}

.burst i {
	position: absolute;
	top: 50%;
	left: 50%;
	width: 8px;
	height: 8px;
	margin: -4px;
	border-radius: 2px;
	background: hsl(calc(var(--n) * 26deg) 85% 60%);
	animation: confetti 0.9s cubic-bezier(0.1, 0.7, 0.3, 1) 0.15s both;
	--a: calc(var(--n) * 25.7deg);
}

.summary {
	display: flex;
	flex-direction: column;
	gap: 8px;
	margin: 6px 0 0;
	padding: 0;
	list-style: none;
	color: var(--text-2);
	font-size: 14px;
}

.summary li {
	display: flex;
	align-items: center;
	gap: 8px;
}

.summary svg {
	color: #2bb673;
}

.tip {
	margin: 22px 0 0;
	padding: 8px 14px;
	border-radius: 8px;
	background: var(--bg-input);
	color: var(--text-3);
	font-size: 12.5px;
}

/* ---------- footer ---------- */

.onb-foot {
	display: grid;
	grid-template-columns: 1fr auto 1fr;
	align-items: center;
	gap: 12px;
	padding: 14px 22px 18px;
	border-top: 1px solid var(--border);
}

.back {
	display: inline-flex;
	align-items: center;
	gap: 5px;
	color: var(--text-2);
	font-size: 13.5px;
}

.back:hover {
	color: var(--text);
}

.dots {
	display: flex;
	justify-content: center;
	gap: 6px;
}

.dots i {
	width: 6px;
	height: 6px;
	border-radius: 3px;
	background: var(--border-strong);
	transition: width 0.3s, background-color 0.3s;
}

.dots i.past {
	background: color-mix(in srgb, var(--accent) 45%, transparent);
}

.dots i.on {
	width: 18px;
	background: var(--accent);
}

.foot-end {
	display: flex;
	justify-content: flex-end;
}

.go {
	display: flex;
	align-items: center;
	justify-content: center;
	min-width: 132px;
	height: 38px;
	padding: 0 18px;
	border-radius: 9px;
	background: var(--accent);
	color: #fff;
	font-size: 14px;
	font-weight: 500;
	transition: transform 0.1s, filter 0.15s;
}

.go:hover:not(:disabled) {
	filter: brightness(1.08);
}

.go:active {
	transform: scale(0.97);
}

.spin {
	animation: spin 0.9s linear infinite;
}

/* ---------- transitions ---------- */

.next-enter-active,
.next-leave-active,
.back-enter-active,
.back-leave-active {
	transition: opacity 0.22s ease, transform 0.28s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.next-enter-from,
.back-leave-to {
	opacity: 0;
	transform: translateX(36px);
}

.next-leave-to,
.back-enter-from {
	opacity: 0;
	transform: translateX(-36px);
}

.expand-enter-active,
.expand-leave-active {
	transition: opacity 0.25s ease, transform 0.25s ease;
}

.expand-enter-from,
.expand-leave-to {
	opacity: 0;
	transform: translateY(-8px);
}

@keyframes fade {
	from { opacity: 0; }
}

@keyframes pop {
	from {
		opacity: 0;
		transform: scale(0.9);
	}
}

@keyframes rise {
	from {
		opacity: 0;
		transform: translateY(10px);
	}
}

@keyframes draw {
	to { stroke-dashoffset: 0; }
}

@keyframes spin {
	to { transform: rotate(360deg); }
}

@keyframes confetti {
	from {
		opacity: 1;
		transform: rotate(var(--a)) translateY(0) scale(1);
	}
	to {
		opacity: 0;
		transform: rotate(var(--a)) translateY(-70px) scale(0.6);
	}
}

@media (max-width: 640px) {
	.onb {
		height: calc(100vh - 32px);
	}

	.pane {
		padding: 20px 20px 12px;
	}

	.sources,
	.views {
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}

	.look {
		grid-template-columns: 1fr;
	}

	.mini {
		height: 150px;
	}
}

@media (prefers-reduced-motion: reduce) {
	.onb *,
	.onb-overlay {
		animation-duration: 0.01ms !important;
		transition-duration: 0.01ms !important;
	}
}
</style>
