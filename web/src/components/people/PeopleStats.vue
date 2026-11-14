<script setup lang="ts">
import {computed, onMounted, ref, shallowRef, watch} from 'vue'
import {useRouter} from 'vue-router'
import {LocateFixed, X} from 'lucide-vue-next'
import ContactAvatar from './ContactAvatar.vue'
import StatsGlobe from './StatsGlobe.vue'
import Popover from '@/components/Popover.vue'
import type {ContactListItem, ContactSource} from '@/api/types'
import {usePeople} from '@/stores/people'
import {loadGeo, type Geo} from '@/lib/geo'
import {centroid, computeStats, type CompletenessField} from '@/lib/peopleStats'
import {contactSubtitle, nameColor} from '@/lib/people'
import {intlLocale, t, type MessageKey} from '@/lib/i18n'
import {prefs} from '@/lib/prefs'
import {clamp, startResize} from '@/lib/resize'
import {toast} from '@/lib/toast'

defineProps<{openId?: number}>()
const emit = defineEmits<{open: [id: number]}>()

const people = usePeople()
const router = useRouter()
const g = people.globe

const GLOBE_RANGE = [320, 900] as const

const geo = shallowRef<Geo | null>(null)
onMounted(async () => {
	try {
		geo.value = await loadGeo()
	} catch (e) {
		toast(String((e as Error).message))
	}
})

const stats = computed(() => geo.value && computeStats(people.contacts, people.groups, geo.value.resolve))
const byId = computed(() => new Map(people.contacts.map(c => [c.id, c])))

const globe = ref<InstanceType<typeof StatsGlobe> | null>(null)

function home(): [number, number, number] {
	const [lon, lat] = centroid(stats.value?.places ?? []) ?? [10, 30]
	return [lon, Math.max(-45, Math.min(45, lat * 0.65)), 1.15]
}

// Opens on the contacts' center of mass the first time, then wherever it was left.
watch(stats, s => {
	if (!s) return
	if (!g.placed && s.places.length) {
		[g.lon0, g.lat0, g.zoom] = home()
		g.placed = true
	}
	if (g.place && !s.places.some(p => p.label === g.place)) g.place = ''
}, {immediate: true})

function resetView() {
	g.place = ''
	globe.value?.flyTo(...home())
}

function fly(lon: number, lat: number, zoom: number, place = '') {
	g.place = place
	globe.value?.flyTo(lon, lat, zoom)
}

const selectedPlace = computed(() => stats.value?.places.find(p => p.label === g.place) ?? null)
const placeContacts = computed(() => (selectedPlace.value?.contactIds ?? [])
	.map(id => byId.value.get(id))
	.filter((c): c is ContactListItem => !!c))

const summary = computed(() => stats.value && t('statsWhereSummary', {
	mapped: stats.value.mapped,
	places: stats.value.places.length,
	countries: stats.value.countries.length,
}))

const unplacedCount = computed(() => stats.value?.unmapped.reduce((n, u) => n + u.count, 0) ?? 0)
const unplacedAnchor = ref<HTMLElement | null>(null)
const unplacedOpen = ref(false)

const tiles = computed(() => {
	const s = stats.value
	if (!s) return []
	const rows: [MessageKey, string | number][] = [
		['statsContacts', s.total],
		['statsCountries', s.countries.length],
		['statsPlaces', s.places.length],
		['statsCompanies', s.companiesTotal],
		['contactGroups', people.groups.length],
		['tags', people.tags.length],
		['contactNotes', s.notes],
		['statsMapped', s.total ? Math.floor(s.mapped * 100 / s.total) + '%' : '–'],
	]
	return rows
})

const topPlaces = computed(() => (stats.value?.places ?? [])
	.filter(p => p.precise)
	.slice(0, 8))

const COMPLETENESS: Record<CompletenessField, MessageKey> = {
	location: 'contactLocation',
	company: 'contactCompany',
	photo: 'statsPhoto',
	linkedin: 'contactLinkedin',
	experience: 'contactExperience',
	education: 'contactEducation',
	email: 'email',
	phone: 'contactPhone',
	birthday: 'contactBirthday',
}

const SOURCES: Record<ContactSource, MessageKey> = {
	manual: 'statsSourceManual',
	csv: 'contactSourceCsv',
	linkedin: 'contactSourceLinkedin',
	brightdata: 'contactSourceBrightdata',
}

const sourceLabel = (s: ContactSource) => SOURCES[s] ? t(SOURCES[s]) : s

const months = computed(() => {
	const list = stats.value?.months ?? []
	const max = Math.max(1, ...list.map(m => m.count))
	const fmt = new Intl.DateTimeFormat(intlLocale.value, {month: 'short', timeZone: 'UTC'})
	return list.map(m => ({
		...m,
		label: fmt.format(new Date(m.key + '-01T00:00:00Z')),
		height: m.count / max * 100,
		peak: m.count === max && m.count > 0,
	}))
})

const pct = (n: number, max: number) => Math.max(0, Math.min(100, n / Math.max(max, 1) * 100))
</script>

<template>
	<div v-if="!stats" class="empty-state" />
	<div v-else class="stats-root">
	<div class="people-stats" :style="{'--globe-w': clamp(prefs.peopleGlobeWidth, GLOBE_RANGE) + 'px'}">
		<section class="globe-pane">
			<header class="globe-head">
				<div class="grow">
					<h2>{{ t('statsWhere') }}</h2>
					<div class="muted small">
						{{ summary }}
					</div>
				</div>
				<button class="icon-btn" :title="t('statsResetView')" @click="resetView"><LocateFixed :size="18" /></button>
			</header>

			<StatsGlobe ref="globe" :places="stats.places" :dots="geo!.dots" :contacts="byId" />

			<div v-if="selectedPlace" class="card place-card">
				<div class="place-head">
					<span class="place-label">{{ selectedPlace.label }}</span>
					<span class="muted">{{ selectedPlace.contactIds.length }}</span>
					<span class="grow" />
					<button class="icon-btn small" :title="t('close')" @click="g.place = ''"><X :size="16" /></button>
				</div>
				<div class="place-list">
					<button
						v-for="c in placeContacts"
						:key="c.id"
						class="place-row"
						:class="{active: c.id === openId}"
						@click="emit('open', c.id)"
					>
						<ContactAvatar :contact="c" :size="26" />
						<span class="place-name">{{ c.name }}</span>
						<span class="muted place-sub">{{ contactSubtitle(c) }}</span>
					</button>
				</div>
			</div>
			<div v-else class="globe-foot muted small">
				<span>{{ t('statsGlobeHint') }}</span>
				<button v-if="unplacedCount" ref="unplacedAnchor" class="unplaced" @click="unplacedOpen = true">
					· {{ t('statsNotPlaced', {n: unplacedCount}) }}
				</button>
			</div>
		</section>

		<div
			class="resizer globe-resizer"
			@pointerdown="startResize($event, 'peopleGlobeWidth', GLOBE_RANGE, 1)"
		/>

		<div class="numbers">
			<div class="tiles">
				<div v-for="[label, value] in tiles" :key="label" class="card tile">
					<div class="tile-label">{{ t(label) }}</div>
					<div class="tile-value">{{ value }}</div>
				</div>
			</div>

			<div class="cards">
				<section class="card">
					<h3>{{ t('statsTopCountries') }}</h3>
					<button
						v-for="c in stats.countries.slice(0, 8)"
						:key="c.name"
						class="bar-row clickable"
						@click="fly(c.lon, c.lat, 2.2)"
					>
						<span class="bar-text"><span class="bar-label">{{ c.name }}</span><span class="muted">{{ c.count }}</span></span>
						<span class="track"><span class="fill" :style="{width: pct(c.count, stats.countries[0].count) + '%'}" /></span>
					</button>
					<div v-if="!stats.countries.length" class="muted small">{{ t('statsNoLocations') }}</div>
				</section>

				<section class="card">
					<h3>{{ t('statsTopPlaces') }}</h3>
					<button
						v-for="p in topPlaces"
						:key="p.label"
						class="bar-row clickable"
						:class="{active: g.place === p.label}"
						@click="fly(p.lon, p.lat, 4, p.label)"
					>
						<span class="bar-text">
							<span class="bar-label">{{ p.label }}</span><span class="muted">{{ p.contactIds.length }}</span>
						</span>
						<span class="track">
							<span class="fill" :style="{width: pct(p.contactIds.length, topPlaces[0].contactIds.length) + '%'}" />
						</span>
					</button>
					<div v-if="!topPlaces.length" class="muted small">{{ t('statsNoLocations') }}</div>
				</section>

				<section class="card">
					<h3>{{ t('statsPerMonth') }}</h3>
					<div class="months">
						<div v-for="m in months" :key="m.key" class="month" :title="`${m.label}: ${m.count}`">
							<div class="month-bar-wrap">
								<div v-if="m.count" class="month-bar" :style="{height: `max(3px, ${m.height}%)`}">
									<span class="month-count" :class="{'hover-only': !m.peak}">{{ m.count }}</span>
								</div>
							</div>
							<div class="month-label">{{ m.label }}</div>
						</div>
					</div>
				</section>

				<section class="card">
					<h3>{{ t('statsCompleteness') }}</h3>
					<div v-for="c in stats.completeness" :key="c.field" class="bar-row">
						<span class="bar-text">
							<span class="bar-label">{{ t(COMPLETENESS[c.field]) }}</span>
							<span class="muted">{{ stats.total ? Math.floor(c.count * 100 / stats.total) : 0 }}%</span>
						</span>
						<span class="track"><span class="fill" :style="{width: pct(c.count, stats.total) + '%'}" /></span>
					</div>
				</section>

				<section class="card">
					<h3>{{ t('statsTopCompanies') }}</h3>
					<div v-for="c in stats.companies" :key="c.name" class="bar-row">
						<span class="bar-text"><span class="bar-label">{{ c.name }}</span><span class="muted">{{ c.count }}</span></span>
						<span class="track"><span class="fill" :style="{width: pct(c.count, stats.companies[0].count) + '%'}" /></span>
					</div>
				</section>

				<section class="card">
					<h3>{{ t('statsTopSchools') }}</h3>
					<div v-for="s in stats.schools" :key="s.name" class="bar-row">
						<span class="bar-text"><span class="bar-label">{{ s.name }}</span><span class="muted">{{ s.count }}</span></span>
						<span class="track"><span class="fill" :style="{width: pct(s.count, stats.schools[0].count) + '%'}" /></span>
					</div>
					<div v-if="!stats.schools.length" class="muted small">{{ t('statsNoSchools') }}</div>
				</section>

				<section v-if="stats.groups.length" class="card">
					<h3>{{ t('contactGroups') }}</h3>
					<button
						v-for="gr in stats.groups"
						:key="gr.id"
						class="bar-row clickable"
						@click="router.push(`/people/g${gr.id}`)"
					>
						<span class="bar-text">
							<span class="bar-label">{{ gr.name }}</span><span class="muted">{{ gr.count }}</span>
						</span>
						<span class="track">
							<span
								class="fill"
								:style="{width: pct(gr.count, stats.groups[0].count) + '%', background: nameColor(gr.name, gr.hexColor)}"
							/>
						</span>
					</button>
				</section>

				<section class="card">
					<h3>{{ t('statsSources') }}</h3>
					<div v-for="s in stats.sources" :key="s.source" class="bar-row">
						<span class="bar-text">
							<span class="bar-label">{{ sourceLabel(s.source) }}</span><span class="muted">{{ s.count }}</span>
						</span>
						<span class="track"><span class="fill grey" :style="{width: pct(s.count, stats.sources[0].count) + '%'}" /></span>
					</div>
					<div v-if="stats.noLocation" class="muted small note">{{ t('statsNoLocationCount', {n: stats.noLocation}) }}</div>
				</section>
			</div>
		</div>

		<Popover v-if="unplacedOpen" :anchor="unplacedAnchor" @close="unplacedOpen = false">
			<div class="menu unplaced-menu">
				<div class="menu-label">{{ t('statsNotPlacedHint') }}</div>
				<div v-for="u in stats.unmapped" :key="u.name" class="unplaced-row">
					<span class="bar-label">{{ u.name }}</span>
					<span class="muted">{{ u.count }}</span>
				</div>
			</div>
		</Popover>
	</div>
	</div>
</template>

<style scoped>
.stats-root {
	display: flex;
	flex: 1;
	min-height: 0;
	container-type: inline-size;
}

.people-stats {
	display: flex;
	flex: 1;
	min-width: 0;
	min-height: 0;
}

.globe-pane {
	display: flex;
	flex-direction: column;
	width: var(--globe-w);
	flex-shrink: 0;
	min-height: 0;
	padding: 0 16px 16px 8px;
}

.globe-head {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-bottom: 4px;
}

.grow {
	flex: 1;
	min-width: 0;
}

h2 {
	margin: 0;
	font-size: 15px;
	font-weight: 600;
}

h3 {
	margin: 0 0 8px;
	font-size: 14px;
	font-weight: 600;
}

.small {
	font-size: 12px;
}

.globe-foot {
	display: flex;
	flex-wrap: wrap;
	gap: 4px;
	min-height: 36px;
	padding-top: 8px;
}

.unplaced {
	color: var(--prio-medium);
	font-size: 12px;
}

.unplaced:hover {
	text-decoration: underline;
}

.unplaced-menu {
	max-width: 320px;
	max-height: 360px;
	overflow: auto;
}

.unplaced-menu .menu-label {
	white-space: normal;
}

.unplaced-row {
	display: flex;
	justify-content: space-between;
	gap: 12px;
	padding: 4px 12px;
	font-size: 13px;
}

.card {
	min-width: 0;
	padding: 14px 16px;
	border: 1px solid var(--border);
	border-radius: 12px;
	background: var(--bg-panel);
}

.place-card {
	display: flex;
	flex-direction: column;
	max-height: 36%;
	min-height: 140px;
	margin-top: 8px;
	padding: 10px 8px 6px;
}

.place-head {
	display: flex;
	align-items: center;
	gap: 8px;
	padding: 0 6px 6px 8px;
}

.place-label {
	font-weight: 600;
}

.icon-btn.small {
	width: 26px;
	height: 26px;
}

.place-list {
	overflow: auto;
}

.place-row {
	display: flex;
	align-items: center;
	gap: 10px;
	width: 100%;
	height: 36px;
	padding: 0 8px;
	border-radius: 8px;
	text-align: left;
	font-size: 14px;
}

.place-row:hover,
.place-row.active {
	background: var(--bg-hover);
}

.place-name {
	flex-shrink: 0;
	max-width: 60%;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.place-sub {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 12px;
}

.numbers {
	flex: 1;
	min-width: 0;
	overflow: auto;
	padding: 0 8px 24px 16px;
}

.tiles {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
	gap: 10px;
	margin-bottom: 10px;
}

.tile {
	padding: 12px 14px;
}

.tile-label {
	font-size: 11px;
	letter-spacing: 0.04em;
	text-transform: uppercase;
	color: var(--text-3);
}

.tile-value {
	margin-top: 6px;
	font-size: 24px;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
}

.cards {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
	align-items: start;
	gap: 10px;
}

.bar-row {
	display: flex;
	flex-direction: column;
	gap: 5px;
	width: 100%;
	padding: 5px 6px;
	margin: 0 -6px;
	box-sizing: content-box;
	border-radius: 8px;
	text-align: left;
	font-size: 13px;
}

.bar-row.clickable {
	cursor: pointer;
}

.bar-row.clickable:hover,
.bar-row.active {
	background: var(--bg-hover);
}

.bar-text {
	display: flex;
	justify-content: space-between;
	gap: 10px;
}

.bar-label {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.track {
	display: block;
	height: 5px;
	border-radius: 3px;
	background: var(--bg-active);
	overflow: hidden;
}

.fill {
	display: block;
	height: 100%;
	min-width: 5px;
	border-radius: 3px;
	background: var(--accent);
}

.fill.grey {
	background: var(--text-3);
}

.note {
	margin-top: 6px;
}

.months {
	display: flex;
	gap: 4px;
	height: 140px;
	padding-top: 4px;
	border-bottom: 1px solid var(--border);
	margin-bottom: 18px;
}

.month {
	position: relative;
	display: flex;
	flex-direction: column;
	flex: 1;
	min-width: 0;
}

.month-bar-wrap {
	position: relative;
	flex: 1;
	margin-top: 16px;
}

.month-bar {
	position: absolute;
	bottom: 0;
	left: 50%;
	width: min(28px, 62%);
	transform: translateX(-50%);
	border-radius: 5px 5px 1px 1px;
	background: color-mix(in srgb, var(--accent) 75%, transparent);
}

.month:hover .month-bar {
	background: var(--accent);
}

.month-count {
	position: absolute;
	bottom: 100%;
	left: 50%;
	transform: translateX(-50%);
	padding-bottom: 2px;
	font-size: 11px;
	line-height: 14px;
}

.hover-only {
	visibility: hidden;
}

.month:hover .hover-only {
	visibility: visible;
}

.month-label {
	position: absolute;
	bottom: -17px;
	left: 0;
	right: 0;
	font-size: 10.5px;
	text-align: center;
	color: var(--text-3);
}

/* Too narrow for side by side: the globe sits above the numbers. */
@container (max-width: 720px) {
	.people-stats {
		flex-direction: column;
		overflow: auto;
	}

	.globe-pane {
		width: auto;
		height: min(70vh, 520px);
		flex-shrink: 0;
		padding: 0 8px 12px;
	}

	.globe-resizer {
		display: none;
	}

	.numbers {
		overflow: visible;
		padding: 0 8px 24px;
	}
}
</style>
